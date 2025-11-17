// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestAddTargetInfo(t *testing.T) {
	tr := newTransaction(scrapeCtx, &nopAdjuster{}, consumertest.NewNop(), labels.EmptyLabels(), receivertest.NewNopSettings(receivertest.NopType), nopObsRecv(t), false, false, true)

	// Initialize transaction with a metric
	ls := labels.FromStrings(
		model.JobLabel, "job",
		model.InstanceLabel, "instance",
		model.MetricNameLabel, "test_metric",
	)
	_, err := tr.Append(0, ls, ts, 1.0)
	require.NoError(t, err)

	// Now add target_info labels
	targetInfoLabels := labels.FromStrings(
		model.JobLabel, "job",
		model.InstanceLabel, "instance",
		model.MetricNameLabel, "target_info",
		"service_name", "my-service",
		"service_version", "1.0.0",
		"env", "prod",
	)

	rKey := resourceKey{job: "job", instance: "instance"}
	tr.AddTargetInfo(rKey, targetInfoLabels)

	// Verify that the resource attributes were added
	require.NoError(t, tr.Commit())
	md, err := tr.getMetrics()
	require.NoError(t, err)
	require.Equal(t, 1, md.ResourceMetrics().Len())

	attrs := md.ResourceMetrics().At(0).Resource().Attributes()
	val, ok := attrs.Get("service_name")
	require.True(t, ok)
	require.Equal(t, "my-service", val.AsString())

	val, ok = attrs.Get("service_version")
	require.True(t, ok)
	require.Equal(t, "1.0.0", val.AsString())

	val, ok = attrs.Get("env")
	require.True(t, ok)
	require.Equal(t, "prod", val.AsString())

	// Verify that job, instance, and __name__ were not added
	_, ok = attrs.Get(model.JobLabel)
	require.False(t, ok)
	_, ok = attrs.Get(model.InstanceLabel)
	require.False(t, ok)
	_, ok = attrs.Get(model.MetricNameLabel)
	require.False(t, ok)
}

func TestAddScopeInfo(t *testing.T) {
	tr := newTransaction(scrapeCtx, &nopAdjuster{}, consumertest.NewNop(), labels.EmptyLabels(), receivertest.NewNopSettings(receivertest.NopType), nopObsRecv(t), false, false, true)

	// Initialize transaction with a metric
	ls := labels.FromStrings(
		model.JobLabel, "job",
		model.InstanceLabel, "instance",
		model.MetricNameLabel, "test_metric",
		"otel_scope_name", "my.scope",
		"otel_scope_version", "2.0.0",
	)
	_, err := tr.Append(0, ls, ts, 1.0)
	require.NoError(t, err)

	// Now add otel_scope_info labels
	scopeInfoLabels := labels.FromStrings(
		model.JobLabel, "job",
		model.InstanceLabel, "instance",
		model.MetricNameLabel, "otel_scope_info",
		"otel_scope_name", "my.scope",
		"otel_scope_version", "2.0.0",
		"scope_attr1", "value1",
		"scope_attr2", "value2",
	)

	rKey := resourceKey{job: "job", instance: "instance"}
	tr.addScopeInfo(rKey, scopeInfoLabels)

	// Verify that the scope attributes were added
	require.NoError(t, tr.Commit())
	md, err := tr.getMetrics()
	require.NoError(t, err)
	require.Equal(t, 1, md.ResourceMetrics().Len())
	require.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().Len())

	scope := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope()
	require.Equal(t, "my.scope", scope.Name())
	require.Equal(t, "2.0.0", scope.Version())

	attrs := scope.Attributes()
	val, ok := attrs.Get("scope_attr1")
	require.True(t, ok)
	require.Equal(t, "value1", val.AsString())

	val, ok = attrs.Get("scope_attr2")
	require.True(t, ok)
	require.Equal(t, "value2", val.AsString())

	// Verify that standard labels were not added
	_, ok = attrs.Get(model.JobLabel)
	require.False(t, ok)
	_, ok = attrs.Get(model.InstanceLabel)
	require.False(t, ok)
	_, ok = attrs.Get(model.MetricNameLabel)
	require.False(t, ok)
	_, ok = attrs.Get("otel_scope_name")
	require.False(t, ok)
	_, ok = attrs.Get("otel_scope_version")
	require.False(t, ok)
}

func TestDetectAndStoreNativeHistogramStaleness(t *testing.T) {
	// Create a metadata store with a native histogram
	md := map[string]scrape.MetricMetadata{
		"native_hist": {
			MetricFamily: "native_hist",
			Type:         model.MetricTypeHistogram,
			Help:         "A native histogram",
			Unit:         "",
		},
	}

	scrapeCtxWithMeta := scrape.ContextWithMetricMetadataStore(
		scrape.ContextWithTarget(t.Context(), target),
		testMetadataStore(md))

	tr := newTransaction(scrapeCtxWithMeta, &nopAdjuster{}, consumertest.NewNop(), labels.EmptyLabels(), receivertest.NewNopSettings(receivertest.NopType), nopObsRecv(t), false, true, true)

	// Initialize transaction
	initLabels := labels.FromStrings(
		model.JobLabel, "job",
		model.InstanceLabel, "instance",
		model.MetricNameLabel, "init_metric",
	)
	_, err := tr.Append(0, initLabels, ts, 1.0)
	require.NoError(t, err)

	// Add a native histogram staleness marker
	nativeHistLabels := labels.FromStrings(
		model.JobLabel, "job",
		model.InstanceLabel, "instance",
		model.MetricNameLabel, "native_hist",
		"le", "0.5",
	)

	rKey := resourceKey{job: "job", instance: "instance"}
	result := tr.detectAndStoreNativeHistogramStaleness(ts, &rKey, emptyScopeID, "native_hist", nativeHistLabels)

	// Should return true as it detected and stored the staleness marker
	require.True(t, result)
}

func TestDetectAndStoreNativeHistogramStaleness_NotNativeHistogram(t *testing.T) {
	// Test cases where it should return false
	tests := []struct {
		name           string
		metadata       map[string]scrape.MetricMetadata
		metricName     string
		expectedResult bool
	}{
		{
			name:           "no metadata",
			metadata:       map[string]scrape.MetricMetadata{},
			metricName:     "unknown_metric",
			expectedResult: false,
		},
		{
			name: "not a histogram type",
			metadata: map[string]scrape.MetricMetadata{
				"counter_metric": {
					MetricFamily: "counter_metric",
					Type:         model.MetricTypeCounter,
				},
			},
			metricName:     "counter_metric",
			expectedResult: false,
		},
		{
			name: "histogram with suffix (classic histogram)",
			metadata: map[string]scrape.MetricMetadata{
				"hist_bucket": {
					MetricFamily: "hist",
					Type:         model.MetricTypeHistogram,
				},
			},
			metricName:     "hist_bucket",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scrapeCtxWithMeta := scrape.ContextWithMetricMetadataStore(
				scrape.ContextWithTarget(t.Context(), target),
				testMetadataStore(tt.metadata))

			tr := newTransaction(scrapeCtxWithMeta, &nopAdjuster{}, consumertest.NewNop(), labels.EmptyLabels(), receivertest.NewNopSettings(receivertest.NopType), nopObsRecv(t), false, true, true)

			// Initialize transaction
			initLabels := labels.FromStrings(
				model.JobLabel, "job",
				model.InstanceLabel, "instance",
				model.MetricNameLabel, "init_metric",
			)
			_, err := tr.Append(0, initLabels, ts, 1.0)
			require.NoError(t, err)

			testLabels := labels.FromStrings(
				model.JobLabel, "job",
				model.InstanceLabel, "instance",
				model.MetricNameLabel, tt.metricName,
			)

			rKey := resourceKey{job: "job", instance: "instance"}
			result := tr.detectAndStoreNativeHistogramStaleness(ts, &rKey, emptyScopeID, tt.metricName, testLabels)

			require.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestSetOptions(t *testing.T) {
	tr := newTransaction(scrapeCtx, &nopAdjuster{}, consumertest.NewNop(), labels.EmptyLabels(), receivertest.NewNopSettings(receivertest.NopType), nopObsRecv(t), false, false, true)

	// SetOptions is currently a no-op, just verify it doesn't panic
	tr.SetOptions(nil)
}

func TestAppendWithExternalLabels(t *testing.T) {
	externalLabels := labels.FromStrings(
		"cluster", "prod-cluster",
		"region", "us-west",
	)

	tr := newTransaction(scrapeCtx, &nopAdjuster{}, consumertest.NewNop(), externalLabels, receivertest.NewNopSettings(receivertest.NopType), nopObsRecv(t), false, false, true)

	ls := labels.FromStrings(
		model.JobLabel, "job",
		model.InstanceLabel, "instance",
		model.MetricNameLabel, "counter_test", // Use a known counter from testMetadata
		"app", "myapp",
	)

	_, err := tr.Append(0, ls, ts, 42.0)
	require.NoError(t, err)

	require.NoError(t, tr.Commit())
	md, err := tr.getMetrics()
	require.NoError(t, err)

	// Verify external labels were added
	require.Equal(t, 1, md.ResourceMetrics().Len())
	require.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().Len())
	require.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())

	metric := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	var dp pmetric.NumberDataPoint

	// The metric type depends on metadata; counter_test should be a Sum
	switch metric.Type() {
	case pmetric.MetricTypeSum:
		require.Equal(t, 1, metric.Sum().DataPoints().Len())
		dp = metric.Sum().DataPoints().At(0)
	case pmetric.MetricTypeGauge:
		require.Equal(t, 1, metric.Gauge().DataPoints().Len())
		dp = metric.Gauge().DataPoints().At(0)
	default:
		t.Fatalf("unexpected metric type: %v", metric.Type())
	}

	attrs := dp.Attributes()

	// Check that external labels are present
	val, ok := attrs.Get("cluster")
	require.True(t, ok)
	require.Equal(t, "prod-cluster", val.AsString())

	val, ok = attrs.Get("region")
	require.True(t, ok)
	require.Equal(t, "us-west", val.AsString())

	// Original label should still be there
	val, ok = attrs.Get("app")
	require.True(t, ok)
	require.Equal(t, "myapp", val.AsString())
}
