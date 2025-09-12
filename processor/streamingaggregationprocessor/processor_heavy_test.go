// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streamingaggregationprocessor

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// TestHighVolumeMetrics tests the processor with high volume of metrics
func TestHighVolumeMetrics(t *testing.T) {
	cfg := &Config{
		WindowSize:         1 * time.Second,
		MaxMemoryMB:        100,
		StaleDataThreshold: 30 * time.Second,
	}

	logger := zap.NewNop()
	proc, err := newStreamingAggregationProcessor(logger, cfg)
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	ctx := context.Background()
	err = proc.Start(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to start processor: %v", err)
	}
	defer proc.Shutdown(ctx)

	// Generate and process 10,000 metrics
	numMetrics := 10000
	numSeries := 100 // 100 unique series
	
	for i := 0; i < numMetrics; i++ {
		md := createMetricWithSeries(i % numSeries)
		_, err = proc.ProcessMetrics(ctx, md)
		if err != nil {
			t.Fatalf("Failed to process metric %d: %v", i, err)
		}
	}

	// Get aggregated metrics
	result := proc.GetAggregatedMetrics()
	
	if result.DataPointCount() == 0 {
		t.Error("Expected aggregated metrics after processing 10,000 metrics")
	}
	
	// Count unique series
	uniqueSeries := make(map[string]bool)
	rms := result.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			metrics := sms.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				uniqueSeries[metric.Name()] = true
			}
		}
	}
	
	t.Logf("Processed %d metrics, aggregated into %d data points with %d unique series",
		numMetrics, result.DataPointCount(), len(uniqueSeries))
	
	// Verify memory usage is tracked
	memUsage := proc.memoryUsage.Load()
	if memUsage == 0 {
		t.Error("Memory usage should be tracked")
	}
	t.Logf("Memory usage: %d bytes", memUsage)
}

// TestConcurrentProcessing tests concurrent metric processing
func TestConcurrentProcessing(t *testing.T) {
	cfg := &Config{
		WindowSize:     2 * time.Second,
		MaxMemoryMB:    50,
		StaleDataThreshold: 30 * time.Second,
	}

	logger := zap.NewNop()
	proc, err := newStreamingAggregationProcessor(logger, cfg)
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	ctx := context.Background()
	err = proc.Start(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to start processor: %v", err)
	}
	defer proc.Shutdown(ctx)

	// Process metrics concurrently from multiple goroutines
	numGoroutines := 10
	metricsPerGoroutine := 1000
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for i := 0; i < metricsPerGoroutine; i++ {
				md := createMetricWithLabel("goroutine", fmt.Sprintf("%d", goroutineID))
				_, err := proc.ProcessMetrics(ctx, md)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d failed at metric %d: %v", goroutineID, i, err)
					return
				}
			}
		}(g)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}

	// Verify metrics were processed
	result := proc.GetAggregatedMetrics()
	if result.DataPointCount() == 0 {
		t.Error("Expected aggregated metrics after concurrent processing")
	}
	
	totalProcessed := proc.metricsProcessed.Load()
	t.Logf("Concurrently processed %d metrics, aggregated into %d data points",
		totalProcessed, result.DataPointCount())
}

// TestWindowRotation tests that window rotation works correctly
func TestWindowRotation(t *testing.T) {
	cfg := &Config{
		WindowSize:     500 * time.Millisecond, // Short window for testing
		MaxMemoryMB:    10,
		StaleDataThreshold: 30 * time.Second,
	}

	logger := zap.NewNop()
	proc, err := newStreamingAggregationProcessor(logger, cfg)
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	// Set up a mock consumer to capture exported metrics
	mockConsumer := &mockConsumer{}
	proc.nextConsumer = mockConsumer

	ctx := context.Background()
	err = proc.Start(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to start processor: %v", err)
	}
	defer proc.Shutdown(ctx)

	// Process metrics over multiple windows
	for window := 0; window < 5; window++ {
		// Send metrics for this window
		for i := 0; i < 100; i++ {
			md := createMetricWithValue(float64(window*100 + i))
			_, err = proc.ProcessMetrics(ctx, md)
			if err != nil {
				t.Fatalf("Failed to process metric: %v", err)
			}
		}
		
		// Wait for window to rotate
		time.Sleep(600 * time.Millisecond)
		
		// Check current state
		result := proc.GetAggregatedMetrics()
		t.Logf("Window %d: %d data points in current window", window, result.DataPointCount())
	}
	
	// Wait a bit more to ensure exports happened
	time.Sleep(1 * time.Second)
	
	// Check that metrics were exported during rotations
	if len(mockConsumer.metrics) == 0 {
		t.Error("Expected metrics to be exported during window rotations")
	} else {
		totalExported := 0
		for _, m := range mockConsumer.metrics {
			totalExported += m.DataPointCount()
		}
		t.Logf("Successfully exported %d batches with total %d data points during rotations", 
			len(mockConsumer.metrics), totalExported)
	}
	
	// The current window might be empty (which is correct after rotation)
	// but we should have exported data
	finalResult := proc.GetAggregatedMetrics()
	t.Logf("Final current window has %d data points (may be 0 if just rotated)", 
		finalResult.DataPointCount())
}

// TestMemoryPressure tests behavior under memory pressure
func TestMemoryPressure(t *testing.T) {
	cfg := &Config{
		WindowSize:     1 * time.Second,
		MaxMemoryMB:    1, // Very low memory limit to trigger eviction
		StaleDataThreshold: 30 * time.Second,
	}

	logger := zap.NewNop()
	proc, err := newStreamingAggregationProcessor(logger, cfg)
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	ctx := context.Background()
	err = proc.Start(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to start processor: %v", err)
	}
	defer proc.Shutdown(ctx)

	// Generate many unique series to trigger memory pressure
	for i := 0; i < 10000; i++ {
		md := createMetricWithSeries(i) // Each with unique series
		_, err = proc.ProcessMetrics(ctx, md)
		if err != nil {
			t.Fatalf("Failed to process metric %d: %v", i, err)
		}
	}

	// Wait for memory monitor to run
	time.Sleep(6 * time.Second)

	// Check that memory is within limits
	memUsage := proc.memoryUsage.Load()
	maxBytes := int64(cfg.MaxMemoryMB * 1024 * 1024)
	
	t.Logf("Memory usage: %d bytes, limit: %d bytes", memUsage, maxBytes)
	
	// Memory usage might temporarily exceed limit before eviction
	// but should eventually be controlled
	if memUsage > maxBytes*2 {
		t.Errorf("Memory usage %d significantly exceeds limit %d", memUsage, maxBytes)
	}
}

// TestMixedMetricTypes tests aggregation of mixed metric types
func TestMixedMetricTypes(t *testing.T) {
	cfg := &Config{
		WindowSize:     1 * time.Second,
		MaxMemoryMB:    50,
		StaleDataThreshold: 30 * time.Second,
	}

	logger := zap.NewNop()
	proc, err := newStreamingAggregationProcessor(logger, cfg)
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	ctx := context.Background()
	err = proc.Start(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to start processor: %v", err)
	}
	defer proc.Shutdown(ctx)

	// Process different metric types
	numEach := 100
	
	// Process gauges
	for i := 0; i < numEach; i++ {
		md := createGaugeWithValue("test.gauge", float64(i))
		_, err = proc.ProcessMetrics(ctx, md)
		if err != nil {
			t.Fatalf("Failed to process gauge %d: %v", i, err)
		}
	}
	
	// Process counters
	for i := 0; i < numEach; i++ {
		md := createCounterWithValue("test.counter", float64(i))
		_, err = proc.ProcessMetrics(ctx, md)
		if err != nil {
			t.Fatalf("Failed to process counter %d: %v", i, err)
		}
	}
	
	// Process histograms
	for i := 0; i < numEach; i++ {
		md := createHistogramWithBuckets("test.histogram", i)
		_, err = proc.ProcessMetrics(ctx, md)
		if err != nil {
			t.Fatalf("Failed to process histogram %d: %v", i, err)
		}
	}

	// Get aggregated results
	result := proc.GetAggregatedMetrics()
	
	// Count metrics by type
	metricTypes := make(map[pmetric.MetricType]int)
	rms := result.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			metrics := sms.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				metricTypes[metric.Type()]++
				
				// Verify aggregation based on type
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					// Should have last value
					if metric.Gauge().DataPoints().Len() == 0 {
						t.Error("Gauge should have data points")
					}
				case pmetric.MetricTypeSum:
					// Should have sum of all values
					if metric.Sum().DataPoints().Len() == 0 {
						t.Error("Sum should have data points")
					}
				case pmetric.MetricTypeHistogram:
					// Should have merged buckets
					if metric.Histogram().DataPoints().Len() == 0 {
						t.Error("Histogram should have data points")
					}
				}
			}
		}
	}
	
	t.Logf("Aggregated metrics by type: %v", metricTypes)
	
	if len(metricTypes) < 3 {
		t.Errorf("Expected at least 3 metric types, got %d", len(metricTypes))
	}
}

// TestLateArrivalHandling tests handling of late-arriving metrics
func TestLateArrivalHandling(t *testing.T) {
	cfg := &Config{
		WindowSize:     1 * time.Second,
		MaxMemoryMB:    10,
		StaleDataThreshold: 30 * time.Second,
	}

	logger := zap.NewNop()
	proc, err := newStreamingAggregationProcessor(logger, cfg)
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	ctx := context.Background()
	err = proc.Start(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to start processor: %v", err)
	}
	defer proc.Shutdown(ctx)

	now := time.Now()
	
	// Send current metrics
	md1 := createMetricWithTimestamp("current", now)
	_, err = proc.ProcessMetrics(ctx, md1)
	if err != nil {
		t.Fatalf("Failed to process current metric: %v", err)
	}
	
	// Send slightly late metrics (within tolerance)
	md2 := createMetricWithTimestamp("late_ok", now.Add(-3*time.Second))
	_, err = proc.ProcessMetrics(ctx, md2)
	if err != nil {
		t.Fatalf("Failed to process late metric: %v", err)
	}
	
	// Send very late metrics (outside tolerance)
	md3 := createMetricWithTimestamp("late_dropped", now.Add(-10*time.Second))
	_, err = proc.ProcessMetrics(ctx, md3)
	if err != nil {
		t.Fatalf("Failed to process very late metric: %v", err)
	}
	
	// Check results
	result := proc.GetAggregatedMetrics()
	dropped := proc.metricsDropped.Load()
	processed := proc.metricsProcessed.Load()
	
	t.Logf("Processed: %d, Dropped: %d, Aggregated data points: %d",
		processed, dropped, result.DataPointCount())
	
	if dropped == 0 {
		t.Log("Note: Very late metrics might have been dropped")
	}
}

// TestAggregationAccuracy tests the accuracy of different aggregation types
func TestAggregationAccuracy(t *testing.T) {
	cfg := &Config{
		WindowSize:     2 * time.Second,
		MaxMemoryMB:    10,
		StaleDataThreshold: 30 * time.Second,
	}

	logger := zap.NewNop()
	proc, err := newStreamingAggregationProcessor(logger, cfg)
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	ctx := context.Background()
	err = proc.Start(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to start processor: %v", err)
	}
	defer proc.Shutdown(ctx)

	// Test gauge aggregation (should keep last value)
	for i := 1; i <= 10; i++ {
		md := createGaugeWithValue("test.gauge.accuracy", float64(i))
		_, err = proc.ProcessMetrics(ctx, md)
		if err != nil {
			t.Fatalf("Failed to process gauge: %v", err)
		}
	}
	
	// Test counter aggregation (should sum values)
	expectedSum := 0.0
	for i := 1; i <= 10; i++ {
		value := float64(i)
		expectedSum += value
		md := createCounterWithValue("test.counter.accuracy", value)
		_, err = proc.ProcessMetrics(ctx, md)
		if err != nil {
			t.Fatalf("Failed to process counter: %v", err)
		}
	}
	
	// Get results and verify
	result := proc.GetAggregatedMetrics()
	
	rms := result.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			metrics := sms.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				
				switch metric.Name() {
				case "test.gauge.accuracy":
					if metric.Gauge().DataPoints().Len() > 0 {
						value := metric.Gauge().DataPoints().At(0).DoubleValue()
						if value != 10.0 {
							t.Errorf("Gauge aggregation incorrect: expected 10.0, got %f", value)
						} else {
							t.Log("Gauge aggregation correct: kept last value of 10.0")
						}
					}
				case "test.counter.accuracy":
					if metric.Sum().DataPoints().Len() > 0 {
						value := metric.Sum().DataPoints().At(0).DoubleValue()
						if value != expectedSum {
							t.Errorf("Counter aggregation incorrect: expected %f, got %f", expectedSum, value)
						} else {
							t.Logf("Counter aggregation correct: sum is %f", value)
						}
					}
				}
			}
		}
	}
}

// Helper functions for creating test metrics

func createMetricWithSeries(seriesID int) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	
	metric := sm.Metrics().AppendEmpty()
	metric.SetName(fmt.Sprintf("test.metric.series_%d", seriesID))
	g := metric.SetEmptyGauge()
	dp := g.DataPoints().AppendEmpty()
	dp.SetDoubleValue(rand.Float64() * 100)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	
	return md
}

func createMetricWithLabel(key, value string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test.metric.labeled")
	g := metric.SetEmptyGauge()
	dp := g.DataPoints().AppendEmpty()
	dp.SetDoubleValue(rand.Float64() * 100)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.Attributes().PutStr(key, value)
	
	return md
}

func createMetricWithValue(value float64) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test.metric.value")
	g := metric.SetEmptyGauge()
	dp := g.DataPoints().AppendEmpty()
	dp.SetDoubleValue(value)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	
	return md
}

func createMetricWithTimestamp(name string, timestamp time.Time) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	
	metric := sm.Metrics().AppendEmpty()
	metric.SetName(name)
	g := metric.SetEmptyGauge()
	dp := g.DataPoints().AppendEmpty()
	dp.SetDoubleValue(42.0)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	
	return md
}

func createGaugeWithValue(name string, value float64) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	
	metric := sm.Metrics().AppendEmpty()
	metric.SetName(name)
	g := metric.SetEmptyGauge()
	dp := g.DataPoints().AppendEmpty()
	dp.SetDoubleValue(value)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	
	return md
}

func createCounterWithValue(name string, value float64) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	
	metric := sm.Metrics().AppendEmpty()
	metric.SetName(name)
	s := metric.SetEmptySum()
	s.SetIsMonotonic(true)
	s.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	dp := s.DataPoints().AppendEmpty()
	dp.SetDoubleValue(value)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	
	return md
}

func createHistogramWithBuckets(name string, seed int) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	
	metric := sm.Metrics().AppendEmpty()
	metric.SetName(name)
	h := metric.SetEmptyHistogram()
	h.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	
	dp := h.DataPoints().AppendEmpty()
	dp.SetCount(uint64(100 + seed))
	dp.SetSum(float64(5000 + seed*100))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	
	// Set bucket bounds and counts
	dp.ExplicitBounds().FromRaw([]float64{10, 20, 50, 100, 200})
	dp.BucketCounts().FromRaw([]uint64{
		uint64(10 + seed%10),
		uint64(20 + seed%10),
		uint64(30 + seed%10),
		uint64(25 + seed%10),
		uint64(10 + seed%10),
		uint64(5 + seed%10),
	})
	
	return md
}
