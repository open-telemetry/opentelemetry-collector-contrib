// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// BenchmarkTimeseriesSignature benchmarks the timeseriesSignature function which is
// the primary source of memory allocations in the exporter based on pprof analysis.
// This function is called for every single data point on every scrape.
func BenchmarkTimeseriesSignature(b *testing.B) {
	metric := createTestGaugeMetric("http_requests_total")
	scopeName := "otelcol/prometheusreceiver"
	scopeVersion := "0.130.0"
	scopeSchemaURL := "https://opentelemetry.io/schemas/1.9.0"

	scopeAttributes := pcommon.NewMap()
	scopeAttributes.PutStr("service.name", "prometheus-receiver")
	scopeAttributes.PutStr("service.version", "1.0.0")

	attributes := pcommon.NewMap()
	attributes.PutStr("method", "GET")
	attributes.PutStr("status", "200")
	attributes.PutStr("endpoint", "/api/v1/metrics")
	attributes.PutStr("host", "localhost:8080")

	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("service.instance.id", "instance-12345")
	resourceAttrs.PutStr("job", "prometheus-scraper")
	resourceAttrs.PutStr("instance", "localhost:8080")

	b.Run("SingleCall", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_ = timeseriesSignature(scopeName, scopeVersion, scopeSchemaURL, scopeAttributes, metric, attributes, resourceAttrs)
		}
	})

	b.Run("HighCardinality", func(b *testing.B) {
		// Simulate high cardinality labels (50 labels per metric)
		highCardAttrs := pcommon.NewMap()
		for i := range 50 {
			highCardAttrs.PutStr(fmt.Sprintf("label_%d", i), fmt.Sprintf("value_%d", i))
		}

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_ = timeseriesSignature(scopeName, scopeVersion, scopeSchemaURL, scopeAttributes, metric, highCardAttrs, resourceAttrs)
		}
	})

	b.Run("ManyMetrics", func(b *testing.B) {
		numMetrics := 11697
		metrics := make([]pmetric.Metric, numMetrics)
		attrSets := make([]pcommon.Map, numMetrics)

		for i := range numMetrics {
			metrics[i] = createTestGaugeMetric(fmt.Sprintf("metric_%d", i))
			attrSets[i] = pcommon.NewMap()
			attrSets[i].PutStr("series_id", fmt.Sprintf("series_%d", i))
			attrSets[i].PutStr("method", "GET")
			attrSets[i].PutStr("status", "200")
		}

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			for j := range numMetrics {
				_ = timeseriesSignature(scopeName, scopeVersion, scopeSchemaURL, scopeAttributes, metrics[j], attrSets[j], resourceAttrs)
			}
		}
	})
}

// BenchmarkAccumulatorConcurrent simulates the high-concurrency behavior on 128-core systems.
// This reproduces the memory leak scenario where multiple goroutines are calling Accumulate simultaneously.
func BenchmarkAccumulatorConcurrent(b *testing.B) {
	// Test with different concurrency levels
	concurrencyLevels := []int{1, 8, 16, 32, 64, 128}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency-%d", concurrency), func(b *testing.B) {
			// Match GOMAXPROCS to the concurrency level
			oldMaxProcs := runtime.GOMAXPROCS(concurrency)
			defer runtime.GOMAXPROCS(oldMaxProcs)

			accumulator := newAccumulator(zap.NewNop(), 5*time.Minute)

			// Create test metrics (11,697 metrics as in issue #36574)
			resourceMetrics := createTestResourceMetrics(11697, 5)

			b.ReportAllocs()
			b.ResetTimer()

			// Run concurrent accumulations
			var wg sync.WaitGroup
			iterations := max(b.N/concurrency, 1)

			for range concurrency {
				wg.Go(func() {
					for range iterations {
						accumulator.Accumulate(resourceMetrics)
					}
				})
			}

			wg.Wait()
		})
	}
}

// BenchmarkAccumulateScrape simulates a complete scrape cycle as it would happen in production.
// This includes creating the ResourceMetrics, accumulating, and collecting.
func BenchmarkAccumulateScrape(b *testing.B) {
	accumulator := newAccumulator(zap.NewNop(), 5*time.Minute)

	b.Run("SmallScrape", func(b *testing.B) {
		// Small scrape: 100 metrics
		resourceMetrics := createTestResourceMetrics(100, 5)

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			accumulator.Accumulate(resourceMetrics)
		}
	})

	b.Run("MediumScrape", func(b *testing.B) {
		// Medium scrape: 1000 metrics
		resourceMetrics := createTestResourceMetrics(1000, 5)

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			accumulator.Accumulate(resourceMetrics)
		}
	})

	b.Run("LargeScrape", func(b *testing.B) {
		// Large scrape: 11,697 metrics (from issue #36574)
		resourceMetrics := createTestResourceMetrics(11697, 5)

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			accumulator.Accumulate(resourceMetrics)
		}
	})
}

// BenchmarkAccumulateAndCollect benchmarks the full cycle of accumulating and collecting metrics.
// This measures the memory retention behavior.
func BenchmarkAccumulateAndCollect(b *testing.B) {
	accumulator := newAccumulator(zap.NewNop(), 5*time.Minute)
	resourceMetrics := createTestResourceMetrics(11697, 5)

	b.ReportAllocs()

	for b.Loop() {
		accumulator.Accumulate(resourceMetrics)

		// Collect after every accumulate (simulates Prometheus scrape)
		_, _, _, _, _, _ = accumulator.Collect()
	}
}

// BenchmarkMemoryGrowth tests memory growth over repeated scrapes.
// This is a stress test to reproduce the 5MB-per-scrape leak.
func BenchmarkMemoryGrowth(b *testing.B) {
	accumulator := newAccumulator(zap.NewNop(), 5*time.Minute)
	resourceMetrics := createTestResourceMetrics(11697, 7)

	// Force GC before starting
	runtime.GC()

	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	b.ReportAllocs()
	b.ResetTimer()

	// Simulate 100 scrape cycles
	for i := 0; i < b.N; i++ {
		for range 100 {
			accumulator.Accumulate(resourceMetrics)
			_, _, _, _, _, _ = accumulator.Collect()
		}

		// Force GC after each batch
		runtime.GC()
	}

	b.StopTimer()
	runtime.ReadMemStats(&m2)

	allocPerOp := float64(m2.TotalAlloc-m1.TotalAlloc) / float64(b.N*100)
	b.ReportMetric(allocPerOp, "alloc-bytes/scrape")
}

// Helper functions
func createTestGaugeMetric(name string) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDescription(fmt.Sprintf("Test metric: %s", name))
	metric.SetUnit("1")

	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(42.0)

	return metric
}

func createTestResourceMetrics(numMetrics, numLabelsPerMetric int) pmetric.ResourceMetrics {
	rm := pmetric.NewResourceMetrics()

	// Set resource attributes
	resourceAttrs := rm.Resource().Attributes()
	resourceAttrs.PutStr("service.name", "test-service")
	resourceAttrs.PutStr("service.instance.id", "instance-123")
	resourceAttrs.PutStr("job", "test-job")
	resourceAttrs.PutStr("instance", "localhost:8080")

	// Create scope metrics
	scopeMetrics := rm.ScopeMetrics().AppendEmpty()
	scopeMetrics.Scope().SetName("otelcol/prometheusreceiver")
	scopeMetrics.Scope().SetVersion("0.130.0")

	// Add metrics
	for i := range numMetrics {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName(fmt.Sprintf("metric_%d", i))
		metric.SetDescription(fmt.Sprintf("Test metric %d", i))
		metric.SetUnit("1")

		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetDoubleValue(float64(i))

		// Add labels
		attrs := dp.Attributes()
		for j := range numLabelsPerMetric {
			attrs.PutStr(fmt.Sprintf("label_%d", j), fmt.Sprintf("value_%d_%d", i, j))
		}
	}

	return rm
}
