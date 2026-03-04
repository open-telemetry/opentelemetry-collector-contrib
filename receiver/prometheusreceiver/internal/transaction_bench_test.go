// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"fmt"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/tsdb/tsdbutil"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
)

const (
	numSeries = 10000
)

var (
	benchTarget = scrape.NewTarget(
		labels.FromMap(map[string]string{
			model.InstanceLabel: "localhost:8080",
			model.JobLabel:      "benchmark",
		}),
		&config.ScrapeConfig{},
		map[model.LabelName]model.LabelValue{
			model.AddressLabel: "localhost:8080",
			model.SchemeLabel:  "http",
		},
		nil,
	)

	benchCtx = scrape.ContextWithTarget(context.Background(), benchTarget)
)

// BenchmarkAppend benchmarks the Append method of the transaction.
// It tests the performance of appending classic metric types (counters, gauges, summaries, histograms).
func BenchmarkAppend(b *testing.B) {
	benchmarkAppend(b)
}

func benchmarkAppend(b *testing.B) {
	labelSets := generateLabelSets(numSeries, 50)
	timestamp := int64(1234567890)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		tx := newBenchmarkTransaction(b)
		b.StartTimer()

		for j, ls := range labelSets {
			value := float64(j)
			_, err := tx.Append(0, ls, timestamp, value)
			assert.NoError(b, err)
		}
	}
}

// BenchmarkAppendHistogram benchmarks the AppendHistogram method of the transaction.
// It tests the performance of appending native histogram metrics.
func BenchmarkAppendHistogram(b *testing.B) {
	benchmarkAppendHistogram(b)
}

func benchmarkAppendHistogram(b *testing.B) {
	labelSets := generateLabelSets(numSeries, 50)
	histograms := generateNativeHistograms(numSeries)
	timestamp := int64(1234567890)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		tx := newBenchmarkTransaction(b)
		tx.enableNativeHistograms = true
		b.StartTimer()

		for j := range labelSets {
			_, err := tx.AppendHistogram(0, labelSets[j], timestamp, histograms[j], nil)
			assert.NoError(b, err)
		}
	}
}

// BenchmarkCommit benchmarks the Commit method which converts accumulated metrics to pmetrics format
// and delivers them to the consumer. This is separate from Append/AppendHistogram to measure the
// conversion and delivery overhead independently.
// Note: The presence of target_info and otel_scope_info metrics affects the performance of the Commit method,
// so they are benchmarked in sub-benchmarks.
func BenchmarkCommit(b *testing.B) {
	b.Run("ClassicMetrics", func(b *testing.B) {
		b.Run("Baseline", func(b *testing.B) {
			benchmarkCommit(b, false, false, false)
		})

		b.Run("WithTargetInfo", func(b *testing.B) {
			benchmarkCommit(b, false, true, false)
		})

		b.Run("WithScopeInfo", func(b *testing.B) {
			benchmarkCommit(b, false, false, true)
		})
	})

	b.Run("NativeHistogram", func(b *testing.B) {
		b.Run("Baseline", func(b *testing.B) {
			benchmarkCommit(b, true, false, false)
		})

		b.Run("WithTargetInfo", func(b *testing.B) {
			benchmarkCommit(b, true, true, false)
		})

		b.Run("WithScopeInfo", func(b *testing.B) {
			benchmarkCommit(b, true, false, true)
		})
	})
}

func benchmarkCommit(b *testing.B, useNativeHistograms, withTargetInfo, withScopeInfo bool) {
	labelSets := generateLabelSets(numSeries, 50)
	var histograms []*histogram.Histogram
	if useNativeHistograms {
		histograms = generateNativeHistograms(numSeries)
	}
	timestamp := int64(1234567890)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Setup: Create transaction and append all data (not timed)
		b.StopTimer()
		tx := newBenchmarkTransaction(b)
		if useNativeHistograms {
			tx.enableNativeHistograms = true
		}

		if withTargetInfo {
			targetInfoLabels := createTargetInfoLabels()
			_, _ = tx.Append(0, targetInfoLabels, timestamp, 1)
		}

		if withScopeInfo {
			scopeInfoLabels := createScopeInfoLabels()
			_, _ = tx.Append(0, scopeInfoLabels, timestamp, 1)
		}

		if useNativeHistograms {
			for j := range labelSets {
				_, _ = tx.AppendHistogram(0, labelSets[j], timestamp, histograms[j], nil)
			}
		} else {
			for j, ls := range labelSets {
				_, _ = tx.Append(0, ls, timestamp, float64(j))
			}
		}
		b.StartTimer()

		// Benchmark: Only measure Commit
		err := tx.Commit()
		assert.NoError(b, err)
	}
}

// newBenchmarkTransaction creates a new transaction configured for benchmarking.
// It uses a no-op consumer and minimal configuration to isolate transaction performance.
func newBenchmarkTransaction(b *testing.B) *transaction {
	b.Helper()

	sink := new(consumertest.MetricsSink)
	settings := receivertest.NewNopSettings(metadata.Type)
	adjuster := &noOpAdjuster{}
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             component.MustNewID("prometheus"),
		Transport:              "http",
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		b.Fatalf("Failed to create ObsReport: %v", err)
	}

	tx := newTransaction(
		benchCtx,
		adjuster,
		sink,
		labels.EmptyLabels(), // no external labels
		settings,
		obsrecv,
		false, // trimSuffixes
		false, // enableNativeHistograms (not needed for Append benchmark)
		false, // useMetadata
	)

	// Set a mock MetricMetadataStore to avoid nil pointer issues
	tx.mc = &mockMetadataStore{}

	return tx
}

// generateLabelSets creates label sets for benchmarking with the specified cardinality.
func generateLabelSets(seriesCount, cardinality int) []labels.Labels {
	result := make([]labels.Labels, seriesCount)

	for i := 0; i < seriesCount; i++ {
		lbls := labels.NewBuilder(labels.EmptyLabels())
		lbls.Set(model.MetricNameLabel, fmt.Sprintf("metric_%d", i))

		for j := 0; j < cardinality; j++ {
			lbls.Set(fmt.Sprintf("label_%d", j), fmt.Sprintf("value_%d_%d", i, j))
		}

		result[i] = lbls.Labels()
	}

	return result
}

// generateNativeHistograms creates native histogram instances for benchmarking.
// Uses Prometheus's test histogram generator for realistic histogram structures.
func generateNativeHistograms(count int) []*histogram.Histogram {
	result := make([]*histogram.Histogram, count)

	for i := 0; i < count; i++ {
		// Use tsdbutil.GenerateTestHistogram to create realistic native histograms
		// The parameter controls the histogram ID, which varies the bucket counts slightly
		result[i] = tsdbutil.GenerateTestHistogram(int64(i))
	}

	return result
}

// createTargetInfoLabels creates labels for a target_info metric.
// The target_info metric is used to add resource attributes to metrics.
func createTargetInfoLabels() labels.Labels {
	return labels.FromMap(map[string]string{
		model.MetricNameLabel: prometheus.TargetInfoMetricName,
		model.JobLabel:        "benchmark",
		model.InstanceLabel:   "localhost:8080",
		"environment":         "test",
		"region":              "us-west-2",
		"cluster":             "benchmark-cluster",
	})
}

// createScopeInfoLabels creates labels for an otel_scope_info metric.
// The otel_scope_info metric is used to add scope-level attributes.
func createScopeInfoLabels() labels.Labels {
	return labels.FromMap(map[string]string{
		model.MetricNameLabel:           prometheus.ScopeInfoMetricName,
		model.JobLabel:                  "benchmark",
		model.InstanceLabel:             "localhost:8080",
		prometheus.ScopeNameLabelKey:    "benchmark.scope",
		prometheus.ScopeVersionLabelKey: "1.0.0",
		"scope_attribute":               "test_value",
	})
}

// noOpAdjuster is a MetricsAdjuster that doesn't modify metrics.
// This isolates the transaction performance from adjustment overhead.
type noOpAdjuster struct{}

func (*noOpAdjuster) AdjustMetrics(_ pmetric.Metrics) error {
	return nil
}

// mockMetadataStore is a minimal implementation of scrape.MetricMetadataStore for testing
type mockMetadataStore struct{}

func (*mockMetadataStore) ListMetadata() []scrape.MetricMetadata {
	return nil
}

func (*mockMetadataStore) GetMetadata(_ string) (scrape.MetricMetadata, bool) {
	return scrape.MetricMetadata{}, false
}

func (*mockMetadataStore) SizeMetadata() int {
	return 0
}

func (*mockMetadataStore) LengthMetadata() int {
	return 0
}
