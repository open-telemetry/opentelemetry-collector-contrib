// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streamingaggregationprocessor

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// TestHistogramWindowRotation tests that histogram cumulative state is preserved across window rotations
func TestHistogramWindowRotation(t *testing.T) {
	cfg := &Config{
		WindowSize:         500 * time.Millisecond, // Short window for testing
		MaxMemoryMB:        10,
		StaleDataThreshold: 30 * time.Second,
	}

	logger := zap.NewNop()
	proc, err := newStreamingAggregationProcessor(logger, cfg)
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	// Set up a mock consumer to capture exported metrics
	testConsumer := &histogramTestConsumer{}
	proc.nextConsumer = testConsumer

	ctx := context.Background()
	err = proc.Start(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to start processor: %v", err)
	}
	defer proc.Shutdown(ctx)

	// Track cumulative values we send
	var sentCumulativeCount uint64 = 0
	var sentCumulativeSum float64 = 0

	// Process histograms across multiple windows
	for window := 0; window < 4; window++ {
		t.Logf("=== Window %d ===", window)
		
		// Send cumulative histogram data
		// Each window, we add 100 more to the cumulative count
		sentCumulativeCount += 100
		sentCumulativeSum += 5000
		
		md := createCumulativeHistogram("test.histogram", sentCumulativeCount, sentCumulativeSum)
		_, err = proc.ProcessMetrics(ctx, md)
		if err != nil {
			t.Fatalf("Failed to process histogram in window %d: %v", window, err)
		}
		
		t.Logf("Sent cumulative histogram: count=%d, sum=%f", sentCumulativeCount, sentCumulativeSum)
		
		// Wait for window to rotate
		time.Sleep(600 * time.Millisecond)
	}
	
	// Wait for final exports
	time.Sleep(1 * time.Second)
	
	// Verify exported metrics
	if len(testConsumer.metrics) == 0 {
		t.Fatal("No metrics were exported")
	}
	
	// Check the exported metrics
	var lastExportedCount uint64 = 0
	var lastExportedSum float64 = 0
	
	for i, metrics := range testConsumer.metrics {
		t.Logf("Export batch %d:", i)
		rms := metrics.ResourceMetrics()
		for j := 0; j < rms.Len(); j++ {
			sms := rms.At(j).ScopeMetrics()
			for k := 0; k < sms.Len(); k++ {
				ms := sms.At(k).Metrics()
				for l := 0; l < ms.Len(); l++ {
					metric := ms.At(l)
					if metric.Name() == "test.histogram" && metric.Type() == pmetric.MetricTypeHistogram {
						hist := metric.Histogram()
						if hist.DataPoints().Len() > 0 {
							dp := hist.DataPoints().At(0)
							exportedCount := dp.Count()
							exportedSum := dp.Sum()
							
							t.Logf("  Exported histogram: count=%d, sum=%f", exportedCount, exportedSum)
							
							// The exported count should be monotonically increasing (cumulative)
							if exportedCount < lastExportedCount {
								t.Errorf("Exported count decreased: %d -> %d", lastExportedCount, exportedCount)
							}
							if exportedSum < lastExportedSum {
								t.Errorf("Exported sum decreased: %f -> %f", lastExportedSum, exportedSum)
							}
							
							lastExportedCount = exportedCount
							lastExportedSum = exportedSum
						}
					}
				}
			}
		}
	}
	
	// The final exported values should match what we sent
	// (or be close, depending on timing of the last export)
	t.Logf("Final exported: count=%d, sum=%f", lastExportedCount, lastExportedSum)
	t.Logf("Total sent: count=%d, sum=%f", sentCumulativeCount, sentCumulativeSum)
	
	// Allow for one window's worth of data to not be exported yet
	if lastExportedCount < sentCumulativeCount-100 {
		t.Errorf("Final exported count %d is too far behind sent count %d", 
			lastExportedCount, sentCumulativeCount)
	}
}

// TestHistogramStatePreservationAcrossWindows verifies that histogram state is preserved correctly
func TestHistogramStatePreservationAcrossWindows(t *testing.T) {
	// Create a window and aggregator
	window := NewWindow(time.Now(), time.Now().Add(1*time.Second))
	agg := window.GetOrCreateAggregator("test.histogram|", pmetric.MetricTypeHistogram)
	
	// Send first cumulative histogram
	dp1 := pmetric.NewHistogramDataPoint()
	dp1.SetCount(100)
	dp1.SetSum(5000)
	dp1.ExplicitBounds().FromRaw([]float64{10, 50, 100})
	dp1.BucketCounts().FromRaw([]uint64{20, 40, 30, 10})
	
	agg.MergeHistogramWithTemporality(dp1, pmetric.AggregationTemporalityCumulative)
	
	// Verify initial state
	if agg.histogramTotalCount != 100 {
		t.Errorf("Initial: expected total count 100, got %d", agg.histogramTotalCount)
	}
	if agg.histogramCount != 100 {
		t.Errorf("Initial: expected window count 100, got %d", agg.histogramCount)
	}
	
	// Simulate window rotation
	agg.ResetForNewWindow()
	
	// After reset, window-specific values should be 0, but totals preserved
	if agg.histogramCount != 0 {
		t.Errorf("After reset: window count should be 0, got %d", agg.histogramCount)
	}
	if agg.histogramSum != 0 {
		t.Errorf("After reset: window sum should be 0, got %f", agg.histogramSum)
	}
	
	// Total values should be preserved
	if agg.histogramTotalCount != 100 {
		t.Errorf("After reset: total count should still be 100, got %d", agg.histogramTotalCount)
	}
	if agg.histogramTotalSum != 5000 {
		t.Errorf("After reset: total sum should still be 5000, got %f", agg.histogramTotalSum)
	}
	
	// Verify cumulative state is preserved (per-source tracking now)
	// The new implementation tracks state per source, so we need to check differently
	if len(agg.lastHistogramValues) == 0 {
		t.Error("After reset: lastHistogramValues should have entries")
	}
	
	// Send second cumulative histogram (simulating next window)
	dp2 := pmetric.NewHistogramDataPoint()
	dp2.SetCount(150)  // 50 new
	dp2.SetSum(7500)   // 2500 new
	dp2.ExplicitBounds().FromRaw([]float64{10, 50, 100})
	dp2.BucketCounts().FromRaw([]uint64{30, 60, 45, 15})
	
	agg.MergeHistogramWithTemporality(dp2, pmetric.AggregationTemporalityCumulative)
	
	// Verify delta computation worked correctly
	if agg.histogramCount != 50 {
		t.Errorf("Second window: expected window count 50, got %d", agg.histogramCount)
	}
	if agg.histogramSum != 2500 {
		t.Errorf("Second window: expected window sum 2500, got %f", agg.histogramSum)
	}
	
	// Total should accumulate the delta
	if agg.histogramTotalCount != 150 {
		t.Errorf("Second window: expected total count 150, got %d", agg.histogramTotalCount)
	}
	if agg.histogramTotalSum != 7500 {
		t.Errorf("Second window: expected total sum 7500, got %f", agg.histogramTotalSum)
	}
	
	// Export and verify the output uses total values
	metric := pmetric.NewMetric()
	metric.SetName("test.histogram")
	metric.SetEmptyHistogram()
	
	agg.ExportTo(metric, window.start, window.end, map[string]string{}, false)
	
	hist := metric.Histogram()
	if hist.DataPoints().Len() == 0 {
		t.Fatal("No data points exported")
	}
	
	dp := hist.DataPoints().At(0)
	if dp.Count() != 150 {
		t.Errorf("Exported count should be total (150), got %d", dp.Count())
	}
	if dp.Sum() != 7500 {
		t.Errorf("Exported sum should be total (7500), got %f", dp.Sum())
	}
}

// TestMultipleHistogramSeriesWindowRotation tests multiple histogram series across rotations
func TestMultipleHistogramSeriesWindowRotation(t *testing.T) {
	cfg := &Config{
		WindowSize:         500 * time.Millisecond,
		MaxMemoryMB:        10,
		StaleDataThreshold: 30 * time.Second,
	}

	logger := zap.NewNop()
	proc, err := newStreamingAggregationProcessor(logger, cfg)
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	testConsumer := &histogramTestConsumer{}
	proc.nextConsumer = testConsumer

	ctx := context.Background()
	err = proc.Start(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to start processor: %v", err)
	}
	defer proc.Shutdown(ctx)

	// Send multiple histogram series
	series := []string{"histogram.a", "histogram.b", "histogram.c"}
	
	for window := 0; window < 3; window++ {
		for i, name := range series {
			count := uint64((window+1) * 100 * (i+1))
			sum := float64((window+1) * 5000 * (i+1))
			
			md := createCumulativeHistogram(name, count, sum)
			_, err = proc.ProcessMetrics(ctx, md)
			if err != nil {
				t.Fatalf("Failed to process histogram %s: %v", name, err)
			}
		}
		
		// Wait for window rotation
		time.Sleep(600 * time.Millisecond)
	}
	
	// Wait for exports
	time.Sleep(1 * time.Second)
	
	// Verify we got exports
	if len(testConsumer.metrics) == 0 {
		t.Fatal("No metrics were exported")
	}
	
	// Track what series we've seen
	seenSeries := make(map[string]bool)
	
	for _, metrics := range testConsumer.metrics {
		rms := metrics.ResourceMetrics()
		for i := 0; i < rms.Len(); i++ {
			sms := rms.At(i).ScopeMetrics()
			for j := 0; j < sms.Len(); j++ {
				ms := sms.At(j).Metrics()
				for k := 0; k < ms.Len(); k++ {
					metric := ms.At(k)
					seenSeries[metric.Name()] = true
				}
			}
		}
	}
	
	// Verify all series were exported
	for _, name := range series {
		if !seenSeries[name] {
			t.Errorf("Series %s was not exported", name)
		}
	}
	
	t.Logf("Successfully exported %d different histogram series", len(seenSeries))
}

// Helper function to create cumulative histogram
func createCumulativeHistogram(name string, count uint64, sum float64) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	
	metric := sm.Metrics().AppendEmpty()
	metric.SetName(name)
	h := metric.SetEmptyHistogram()
	h.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	
	dp := h.DataPoints().AppendEmpty()
	dp.SetCount(count)
	dp.SetSum(sum)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	
	// Set some bucket bounds and counts
	dp.ExplicitBounds().FromRaw([]float64{10, 50, 100, 500})
	
	// Distribute count across buckets
	bucketSize := count / 5
	remainder := count % 5
	dp.BucketCounts().FromRaw([]uint64{
		bucketSize,
		bucketSize,
		bucketSize,
		bucketSize,
		bucketSize + remainder, // Put remainder in last bucket
	})
	
	return md
}

// histogramTestConsumer captures exported metrics for testing
type histogramTestConsumer struct {
	metrics []pmetric.Metrics
}

func (m *histogramTestConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	// Make a copy to avoid data races
	copy := pmetric.NewMetrics()
	md.CopyTo(copy)
	m.metrics = append(m.metrics, copy)
	return nil
}

func (m *histogramTestConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}
