// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/tsdbutil"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
	mdata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
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
	impl := benchmarkImpl(b)
	labelSets := generateLabelSets(numSeries, 50)
	timestamp := int64(1234567890)
	opts := storage.AppendV2Options{
		Metadata: metadata.Metadata{
			Type: model.MetricTypeGauge,
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		var txV1 *transaction
		var txV2 *transactionV2
		if impl == "v2" {
			txV2 = newBenchmarkTransactionV2(b)
		} else {
			txV1 = newBenchmarkTransaction(b)
		}
		b.StartTimer()

		for j, ls := range labelSets {
			value := float64(j)
			var err error
			if impl == "v2" {
				_, err = txV2.Append(0, ls, 0, timestamp, value, nil, nil, opts)
			} else {
				_, err = txV1.Append(0, ls, timestamp, value)
			}
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
	impl := benchmarkImpl(b)
	labelSets := generateLabelSets(numSeries, 50)
	histograms := generateNativeHistograms(numSeries)
	timestamp := int64(1234567890)
	opts := storage.AppendV2Options{
		Metadata: metadata.Metadata{
			Type: model.MetricTypeHistogram,
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		var txV1 *transaction
		var txV2 *transactionV2
		if impl == "v2" {
			txV2 = newBenchmarkTransactionV2(b)
		} else {
			txV1 = newBenchmarkTransaction(b)
		}
		b.StartTimer()

		for j := range labelSets {
			var err error
			if impl == "v2" {
				_, err = txV2.Append(0, labelSets[j], 0, timestamp, 0, histograms[j], nil, opts)
			} else {
				_, err = txV1.AppendHistogram(0, labelSets[j], timestamp, histograms[j], nil)
			}
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
	impl := benchmarkImpl(b)
	b.Run("ClassicMetrics", func(b *testing.B) {
		b.Run("Baseline", func(b *testing.B) {
			benchmarkCommit(b, impl, false, false, false)
		})

		b.Run("WithTargetInfo", func(b *testing.B) {
			benchmarkCommit(b, impl, false, true, false)
		})

		b.Run("WithScopeInfo", func(b *testing.B) {
			benchmarkCommit(b, impl, false, false, true)
		})
	})

	b.Run("NativeHistogram", func(b *testing.B) {
		b.Run("Baseline", func(b *testing.B) {
			benchmarkCommit(b, impl, true, false, false)
		})

		b.Run("WithTargetInfo", func(b *testing.B) {
			benchmarkCommit(b, impl, true, true, false)
		})

		b.Run("WithScopeInfo", func(b *testing.B) {
			benchmarkCommit(b, impl, true, false, true)
		})
	})
}

// BenchmarkAppendCommit benchmarks append(s) plus commit together.
// This measures end-to-end transaction cost for a single scrape.
func BenchmarkAppendCommit(b *testing.B) {
	impl := benchmarkImpl(b)
	b.Run("ClassicMetrics", func(b *testing.B) {
		b.Run("Baseline", func(b *testing.B) {
			benchmarkAppendCommit(b, impl, false, false, false)
		})
		b.Run("WithTargetInfo", func(b *testing.B) {
			benchmarkAppendCommit(b, impl, false, true, false)
		})
		b.Run("WithScopeInfo", func(b *testing.B) {
			benchmarkAppendCommit(b, impl, false, false, true)
		})
	})
	b.Run("NativeHistogram", func(b *testing.B) {
		b.Run("Baseline", func(b *testing.B) {
			benchmarkAppendCommit(b, impl, true, false, false)
		})
		b.Run("WithTargetInfo", func(b *testing.B) {
			benchmarkAppendCommit(b, impl, true, true, false)
		})
		b.Run("WithScopeInfo", func(b *testing.B) {
			benchmarkAppendCommit(b, impl, true, false, true)
		})
	})
}

func benchmarkImpl(b *testing.B) string {
	b.Helper()
	impl := strings.ToLower(strings.TrimSpace(os.Getenv("APPENDER_IMPL")))
	if impl == "" {
		return "v1"
	}
	if impl != "v1" && impl != "v2" {
		b.Fatalf("invalid APPENDER_IMPL %q (expected v1 or v2)", impl)
	}
	return impl
}
func benchmarkCommit(b *testing.B, impl string, useNativeHistograms, withTargetInfo, withScopeInfo bool) {
	labelSets := generateLabelSets(numSeries, 50)
	var histograms []*histogram.Histogram
	if useNativeHistograms {
		histograms = generateNativeHistograms(numSeries)
	}
	timestamp := int64(1234567890)
	metricType := model.MetricTypeGauge
	if useNativeHistograms {
		metricType = model.MetricTypeHistogram
	}
	opts := storage.AppendV2Options{
		Metadata: metadata.Metadata{
			Type: metricType,
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Setup is intentionally included in timing to avoid excessive wall-clock time
		// when Commit is very fast (particularly in V2). This keeps b.N reasonable.
		var txV1 *transaction
		var txV2 *transactionV2
		if impl == "v2" {
			txV2 = newBenchmarkTransactionV2(b)
		} else {
			txV1 = newBenchmarkTransaction(b)
		}

		if withTargetInfo {
			targetInfoLabels := createTargetInfoLabels()
			if impl == "v2" {
				_, _ = txV2.Append(0, targetInfoLabels, 0, timestamp, 1, nil, nil, opts)
			} else {
				_, _ = txV1.Append(0, targetInfoLabels, timestamp, 1)
			}
		}

		if withScopeInfo {
			scopeInfoLabels := createScopeInfoLabels()
			if impl == "v2" {
				_, _ = txV2.Append(0, scopeInfoLabels, 0, timestamp, 1, nil, nil, opts)
			} else {
				_, _ = txV1.Append(0, scopeInfoLabels, timestamp, 1)
			}
		}

		if useNativeHistograms {
			for j := range labelSets {
				if impl == "v2" {
					_, _ = txV2.Append(0, labelSets[j], 0, timestamp, 0, histograms[j], nil, opts)
				} else {
					_, _ = txV1.AppendHistogram(0, labelSets[j], timestamp, histograms[j], nil)
				}
			}
		} else {
			for j, ls := range labelSets {
				if impl == "v2" {
					_, _ = txV2.Append(0, ls, 0, timestamp, float64(j), nil, nil, opts)
				} else {
					_, _ = txV1.Append(0, ls, timestamp, float64(j))
				}
			}
		}
		// Benchmark: Commit plus setup for each iteration
		var err error
		if impl == "v2" {
			err = txV2.Commit()
		} else {
			err = txV1.Commit()
		}
		assert.NoError(b, err)
	}
}

func benchmarkAppendCommit(b *testing.B, impl string, useNativeHistograms, withTargetInfo, withScopeInfo bool) {
	labelSets := generateLabelSets(numSeries, 50)
	var histograms []*histogram.Histogram
	if useNativeHistograms {
		histograms = generateNativeHistograms(numSeries)
	}
	timestamp := int64(1234567890)
	metricType := model.MetricTypeGauge
	if useNativeHistograms {
		metricType = model.MetricTypeHistogram
	}
	opts := storage.AppendV2Options{
		Metadata: metadata.Metadata{
			Type: metricType,
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var txV1 *transaction
		var txV2 *transactionV2
		if impl == "v2" {
			txV2 = newBenchmarkTransactionV2(b)
		} else {
			txV1 = newBenchmarkTransaction(b)
		}

		if withTargetInfo {
			targetInfoLabels := createTargetInfoLabels()
			if impl == "v2" {
				_, _ = txV2.Append(0, targetInfoLabels, 0, timestamp, 1, nil, nil, opts)
			} else {
				_, _ = txV1.Append(0, targetInfoLabels, timestamp, 1)
			}
		}
		if withScopeInfo {
			scopeInfoLabels := createScopeInfoLabels()
			if impl == "v2" {
				_, _ = txV2.Append(0, scopeInfoLabels, 0, timestamp, 1, nil, nil, opts)
			} else {
				_, _ = txV1.Append(0, scopeInfoLabels, timestamp, 1)
			}
		}

		if useNativeHistograms {
			for j := range labelSets {
				if impl == "v2" {
					_, _ = txV2.Append(0, labelSets[j], 0, timestamp, 0, histograms[j], nil, opts)
				} else {
					_, _ = txV1.AppendHistogram(0, labelSets[j], timestamp, histograms[j], nil)
				}
			}
		} else {
			for j, ls := range labelSets {
				if impl == "v2" {
					_, _ = txV2.Append(0, ls, 0, timestamp, float64(j), nil, nil, opts)
				} else {
					_, _ = txV1.Append(0, ls, timestamp, float64(j))
				}
			}
		}

		var err error
		if impl == "v2" {
			err = txV2.Commit()
		} else {
			err = txV1.Commit()
		}
		assert.NoError(b, err)
	}
}

// newBenchmarkTransaction creates a new transaction configured for benchmarking.
// It uses a no-op consumer and minimal configuration to isolate transaction performance.
func newBenchmarkTransaction(b *testing.B) *transaction {
	b.Helper()

	sink := new(consumertest.MetricsSink)
	settings := receivertest.NewNopSettings(mdata.Type)
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
		sink,
		labels.EmptyLabels(), // no external labels
		settings,
		obsrecv,
		false, // trimSuffixes
		false, // useMetadata
	)

	// Set a mock MetricMetadataStore to avoid nil pointer issues
	tx.mc = &mockMetadataStore{}

	return tx
}

func newBenchmarkTransactionV2(b *testing.B) *transactionV2 {
	b.Helper()

	sink := new(consumertest.MetricsSink)
	settings := receivertest.NewNopSettings(mdata.Type)
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             component.MustNewID("prometheus"),
		Transport:              "http",
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		b.Fatalf("Failed to create ObsReport: %v", err)
	}

	return newTransactionV2(
		benchCtx,
		sink,
		labels.EmptyLabels(), // no external labels
		settings,
		obsrecv,
		false, // trimSuffixes
	)
}

// generateLabelSets creates label sets for benchmarking with the specified cardinality.
func generateLabelSets(seriesCount, cardinality int) []labels.Labels {
	result := make([]labels.Labels, seriesCount)

	for i := range seriesCount {
		lbls := labels.NewBuilder(labels.EmptyLabels())
		lbls.Set(model.MetricNameLabel, fmt.Sprintf("metric_%d", i))

		for j := range cardinality {
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

	for i := range count {
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
