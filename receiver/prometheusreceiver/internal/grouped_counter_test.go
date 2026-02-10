// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

// TestGroupedCounterMetrics tests that grouped counter metrics with multiple time series
// are not silently dropped, as reported in issue #34263
func TestGroupedCounterMetrics(t *testing.T) {
	// Create a metadata store that knows about the counter metric
	metadataStore := newFakeMetadataStore(map[string]scrape.MetricMetadata{
		"cnpg_pg_stat_database_blks_read": {
			MetricFamily: "cnpg_pg_stat_database_blks_read",
			Type:         model.MetricTypeCounter,
			Help:         "Number of disk blocks read in this database",
		},
		"cnpg_pg_stat_replication_flush_diff_bytes": {
			MetricFamily: "cnpg_pg_stat_replication_flush_diff_bytes",
			Type:         model.MetricTypeGauge,
			Help:         "Difference in bytes from the last write-ahead log location flushed to disk by this standby server",
		},
	})

	testGroupedCounterMetricsWithMetadata(t, metadataStore, "with metadata")
}

// TestGroupedCounterMetricsWithoutMetadata tests the case when metadata is not available
// This reproduces the actual bug from issue #34263
func TestGroupedCounterMetricsWithoutMetadata(t *testing.T) {
	// Create an EMPTY metadata store to simulate the Target Allocator scenario
	// where metadata may not be properly propagated
	metadataStore := newFakeMetadataStore(map[string]scrape.MetricMetadata{})

	testGroupedCounterMetricsWithMetadata(t, metadataStore, "without metadata")
}

func testGroupedCounterMetricsWithMetadata(t *testing.T, metadataStore *fakeMetadataStore, testName string) {
	// Add scrape target to context (using the same pattern as transaction_test.go)
	target := scrape.NewTarget(
		labels.FromMap(map[string]string{
			model.InstanceLabel: "localhost:8080",
		}),
		&config.ScrapeConfig{},
		map[model.LabelName]model.LabelValue{
			model.AddressLabel: "address:8080",
			model.SchemeLabel:  "http",
		},
		nil,
	)

	ctx := scrape.ContextWithMetricMetadataStore(
		scrape.ContextWithTarget(context.Background(), target),
		metadataStore)

	sink := new(consumertest.MetricsSink)

	tr := newTransaction(
		ctx,
		sink,
		labels.EmptyLabels(),
		receivertest.NewNopSettings(receivertest.NopType),
		nopObsRecv(t),
		false, // trimSuffixes
		true,  // useMetadata
	)

	// Simulate scraping grouped counter metrics (issue #34263)
	// These should NOT be dropped
	counterSamples := []struct {
		name   string
		labels labels.Labels
		value  float64
	}{
		{
			name:   "cnpg_pg_stat_database_blks_read",
			labels: labels.FromStrings(model.MetricNameLabel, "cnpg_pg_stat_database_blks_read", "datname", "", model.JobLabel, "test-job", model.InstanceLabel, "test-instance"),
			value:  122,
		},
		{
			name:   "cnpg_pg_stat_database_blks_read",
			labels: labels.FromStrings(model.MetricNameLabel, "cnpg_pg_stat_database_blks_read", "datname", "my-database", model.JobLabel, "test-job", model.InstanceLabel, "test-instance"),
			value:  426,
		},
		{
			name:   "cnpg_pg_stat_database_blks_read",
			labels: labels.FromStrings(model.MetricNameLabel, "cnpg_pg_stat_database_blks_read", "datname", "postgres", model.JobLabel, "test-job", model.InstanceLabel, "test-instance"),
			value:  359,
		},
	}

	// Simulate scraping grouped gauge metrics (these work fine according to the issue)
	gaugeSamples := []struct {
		name   string
		labels labels.Labels
		value  float64
	}{
		{
			name:   "cnpg_pg_stat_replication_flush_diff_bytes",
			labels: labels.FromStrings(model.MetricNameLabel, "cnpg_pg_stat_replication_flush_diff_bytes", "application_name", "my-cnpg-cluster-2", "client_addr", "10.244.0.99/32", model.JobLabel, "test-job", model.InstanceLabel, "test-instance"),
			value:  0,
		},
		{
			name:   "cnpg_pg_stat_replication_flush_diff_bytes",
			labels: labels.FromStrings(model.MetricNameLabel, "cnpg_pg_stat_replication_flush_diff_bytes", "application_name", "my-cnpg-cluster-3", "client_addr", "10.244.3.70/32", model.JobLabel, "test-job", model.InstanceLabel, "test-instance"),
			value:  0,
		},
	}

	// Append counter samples
	for _, sample := range counterSamples {
		_, err := tr.Append(0, sample.labels, 1000, sample.value)
		require.NoError(t, err, "Failed to append counter sample %s", sample.name)
	}

	// Append gauge samples
	for _, sample := range gaugeSamples {
		_, err := tr.Append(0, sample.labels, 1000, sample.value)
		require.NoError(t, err, "Failed to append gauge sample %s", sample.name)
	}

	// Commit the transaction
	require.NoError(t, tr.Commit())

	// Verify that metrics were created
	require.Equal(t, 1, len(sink.AllMetrics()), "Expected 1 metrics batch")
	md := sink.AllMetrics()[0]
	require.Equal(t, 1, md.ResourceMetrics().Len(), "Expected 1 resource")

	rm := md.ResourceMetrics().At(0)
	require.Equal(t, 1, rm.ScopeMetrics().Len(), "Expected 1 scope")

	sm := rm.ScopeMetrics().At(0)
	metrics := sm.Metrics()

	// Debug: print all metrics
	t.Logf("%s: Found %d metrics", testName, metrics.Len())
	for i := 0; i < metrics.Len(); i++ {
		m := metrics.At(i)
		t.Logf("%s: Metric %d: name=%s, type=%s", testName, i, m.Name(), m.Type())
	}

	// Find counter and gauge metrics
	var counterMetric, gaugeMetric pmetric.Metric
	for i := 0; i < metrics.Len(); i++ {
		m := metrics.At(i)
		if m.Name() == "cnpg_pg_stat_database_blks_read" {
			counterMetric = m
		}
		if m.Name() == "cnpg_pg_stat_replication_flush_diff_bytes" {
			gaugeMetric = m
		}
	}

	// Verify gauge metric (should work)
	require.NotEmpty(t, gaugeMetric.Name(), "Gauge metric should be present")
	require.Equal(t, pmetric.MetricTypeGauge, gaugeMetric.Type(), "Expected gauge type")
	require.Equal(t, 2, gaugeMetric.Gauge().DataPoints().Len(), "Expected 2 gauge data points")

	// Verify counter metric (this is where the bug happens in issue #34263)
	require.NotEmpty(t, counterMetric.Name(), "Counter metric should be present")

	// Debug: check what type it actually is
	t.Logf("%s: Counter metric type: %s", testName, counterMetric.Type())

	// With the heuristic fix, counters should now be correctly typed as Sum even without metadata
	require.Equal(t, pmetric.MetricTypeSum, counterMetric.Type(), "Expected sum type for counter")
	require.Equal(t, 3, counterMetric.Sum().DataPoints().Len(), "Expected 3 counter data points")

	// Verify that the counter is monotonic
	require.True(t, counterMetric.Sum().IsMonotonic(), "Counter should be monotonic")
}
