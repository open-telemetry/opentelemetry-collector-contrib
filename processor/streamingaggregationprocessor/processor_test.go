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

// mockConsumer is a simple mock consumer for testing
type mockConsumer struct {
	metrics []pmetric.Metrics
}

func (m *mockConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	m.metrics = append(m.metrics, md)
	return nil
}

func (m *mockConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func TestProcessorCreation(t *testing.T) {
	// Create a test configuration
	cfg := &Config{
		WindowSize:         30 * time.Second,
		MaxMemoryMB:        100,
		StaleDataThreshold: 5 * time.Minute,
	}

	// Create processor
	logger := zap.NewNop()
	proc, err := newStreamingAggregationProcessor(logger, cfg)
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	// Verify processor was created with correct settings
	if proc == nil {
		t.Fatal("Processor is nil")
	}

	// In double-buffer architecture, we have exactly 2 windows
	if proc.windowA == nil || proc.windowB == nil {
		t.Error("Both windowA and windowB should be initialized")
	}

	if proc.windowSize != 30*time.Second {
		t.Errorf("Expected window size of 30s, got %v", proc.windowSize)
	}
}

func TestProcessorStartStop(t *testing.T) {
	cfg := &Config{
		WindowSize:         1 * time.Second,
		MaxMemoryMB:        10,
		StaleDataThreshold: 30 * time.Second,
	}

	logger := zap.NewNop()
	proc, err := newStreamingAggregationProcessor(logger, cfg)
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	// Start the processor
	ctx := context.Background()
	err = proc.Start(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to start processor: %v", err)
	}

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown the processor
	err = proc.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Failed to shutdown processor: %v", err)
	}
}

func TestProcessMetrics(t *testing.T) {
	cfg := &Config{
		WindowSize:         1 * time.Second,
		MaxMemoryMB:        10,
		StaleDataThreshold: 30 * time.Second,
	}

	logger := zap.NewNop()
	proc, err := newStreamingAggregationProcessor(logger, cfg)
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	// Start the processor
	ctx := context.Background()
	err = proc.Start(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to start processor: %v", err)
	}
	defer proc.Shutdown(ctx)

	// Create test metrics
	md := createTestMetrics()

	// Process metrics multiple times to aggregate
	for i := 0; i < 3; i++ {
		_, err = proc.ProcessMetrics(ctx, md)
		if err != nil {
			t.Fatalf("Failed to process metrics: %v", err)
		}
	}

	// Get aggregated metrics directly
	result := proc.GetAggregatedMetrics()

	// Check that metrics were aggregated
	if result.DataPointCount() == 0 {
		t.Error("Expected aggregated metrics, but got none")
	} else {
		t.Logf("Successfully aggregated %d data points", result.DataPointCount())
		
		// Verify we have the expected metrics
		expectedMetrics := map[string]bool{
			"test.gauge": false,
			"test.sum":   false,
		}
		
		rms := result.ResourceMetrics()
		for i := 0; i < rms.Len(); i++ {
			sms := rms.At(i).ScopeMetrics()
			for j := 0; j < sms.Len(); j++ {
				metrics := sms.At(j).Metrics()
				for k := 0; k < metrics.Len(); k++ {
					metric := metrics.At(k)
					if _, ok := expectedMetrics[metric.Name()]; ok {
						expectedMetrics[metric.Name()] = true
						t.Logf("Found aggregated metric: %s", metric.Name())
					}
				}
			}
		}
		
		for name, found := range expectedMetrics {
			if !found {
				t.Errorf("Expected metric %s was not found in aggregated results", name)
			}
		}
	}
}

func TestProcessorWithDifferentMetricTypes(t *testing.T) {
	tests := []struct {
		name       string
		createFunc func() pmetric.Metrics
	}{
		{
			name:       "gauge",
			createFunc: createGaugeMetric,
		},
		{
			name:       "sum",
			createFunc: createSumMetric,
		},
		{
			name:       "histogram",
			createFunc: createHistogramMetric,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				WindowSize:         1 * time.Second,
				MaxMemoryMB:        10,
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

			// Process the specific metric type
			md := tt.createFunc()
			_, err = proc.ProcessMetrics(ctx, md)
			if err != nil {
				t.Fatalf("Failed to process %s metrics: %v", tt.name, err)
			}

			// Get aggregated results directly
			result := proc.GetAggregatedMetrics()

			// Check that metrics were aggregated
			if result.DataPointCount() == 0 {
				t.Errorf("%s metrics: expected aggregated data but got none", tt.name)
			} else {
				t.Logf("%s metrics: successfully aggregated %d data points", tt.name, result.DataPointCount())
				
				// Verify the metric type is correct
				rms := result.ResourceMetrics()
				if rms.Len() > 0 {
					sms := rms.At(0).ScopeMetrics()
					if sms.Len() > 0 {
						metrics := sms.At(0).Metrics()
						if metrics.Len() > 0 {
							metric := metrics.At(0)
							t.Logf("  Metric name: %s, type: %v", metric.Name(), metric.Type())
						}
					}
				}
			}
		})
	}
}

func TestWindowCreation(t *testing.T) {
	cfg := &Config{
		WindowSize:         30 * time.Second,
		MaxMemoryMB:        50,
		StaleDataThreshold: 5 * time.Minute,
	}

	logger := zap.NewNop()
	proc, err := newStreamingAggregationProcessor(logger, cfg)
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	// Verify double-buffer windows are initialized
	if proc.windowA == nil || proc.windowB == nil {
		t.Error("Both windowA and windowB should be initialized")
	}
	if proc.activeWindow == nil {
		t.Error("Active window should be set")
	}

	// Verify window has proper duration
	if proc.activeWindow != nil {
		duration := proc.activeWindow.end.Sub(proc.activeWindow.start)
		if duration != 30*time.Second {
			t.Errorf("Active window duration is %v, expected 30s", duration)
		}
	}
}

// Helper functions to create test metrics

func createTestMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	
	// Add a gauge metric
	gauge := sm.Metrics().AppendEmpty()
	gauge.SetName("test.gauge")
	g := gauge.SetEmptyGauge()
	dp := g.DataPoints().AppendEmpty()
	dp.SetDoubleValue(42.0)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	
	// Add a sum metric
	sum := sm.Metrics().AppendEmpty()
	sum.SetName("test.sum")
	s := sum.SetEmptySum()
	s.SetIsMonotonic(true)
	s.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	sdp := s.DataPoints().AppendEmpty()
	sdp.SetDoubleValue(100.0)
	sdp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	
	return md
}

func createGaugeMetric() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	
	gauge := sm.Metrics().AppendEmpty()
	gauge.SetName("test.gauge")
	g := gauge.SetEmptyGauge()
	
	// Add multiple data points
	for i := 0; i < 5; i++ {
		dp := g.DataPoints().AppendEmpty()
		dp.SetDoubleValue(float64(i * 10))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.Attributes().PutStr("index", string(rune('0'+i)))
	}
	
	return md
}

func createSumMetric() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	
	sum := sm.Metrics().AppendEmpty()
	sum.SetName("test.counter")
	s := sum.SetEmptySum()
	s.SetIsMonotonic(true)
	s.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	
	// Add multiple data points
	for i := 0; i < 5; i++ {
		dp := s.DataPoints().AppendEmpty()
		dp.SetDoubleValue(float64(i * 100))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.Attributes().PutStr("index", string(rune('0'+i)))
	}
	
	return md
}

func createHistogramMetric() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	
	hist := sm.Metrics().AppendEmpty()
	hist.SetName("test.histogram")
	h := hist.SetEmptyHistogram()
	h.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	
	dp := h.DataPoints().AppendEmpty()
	dp.SetCount(100)
	dp.SetSum(5000)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	
	// Set bucket bounds and counts
	dp.ExplicitBounds().FromRaw([]float64{10, 20, 50, 100, 200})
	dp.BucketCounts().FromRaw([]uint64{10, 20, 30, 25, 10, 5})
	
	return md
}

// TestHistogramLabelDropping verifies that histograms with different labels
// are aggregated into a single series (label dropping behavior)
func TestHistogramLabelDropping(t *testing.T) {
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

	// Create metrics with multiple histogram series having different labels
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	// Add resource attributes that should be dropped
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	rm.Resource().Attributes().PutStr("host", "test-host")
	
	sm := rm.ScopeMetrics().AppendEmpty()
	
	hist := sm.Metrics().AppendEmpty()
	hist.SetName("http.request.duration")
	h := hist.SetEmptyHistogram()
	h.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	
	// Create 5 data points with different endpoint labels
	endpoints := []string{"/api/v1/users", "/api/v1/products", "/api/v1/orders", "/api/v1/payments", "/api/v1/shipping"}
	expectedTotalCount := uint64(0)
	expectedTotalSum := float64(0)
	
	for i, endpoint := range endpoints {
		dp := h.DataPoints().AppendEmpty()
		count := uint64((i + 1) * 100)
		sum := float64((i + 1) * 5000)
		
		dp.SetCount(count)
		dp.SetSum(sum)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		
		// Add labels that should be dropped during aggregation
		dp.Attributes().PutStr("endpoint", endpoint)
		dp.Attributes().PutStr("method", "GET")
		dp.Attributes().PutStr("status", "200")
		
		// Set bucket bounds and counts
		dp.ExplicitBounds().FromRaw([]float64{10, 20, 50, 100, 200})
		dp.BucketCounts().FromRaw([]uint64{10, 20, 30, 25, 10, 5})
		
		expectedTotalCount += count
		expectedTotalSum += sum
	}
	
	// Process the metrics
	_, err = proc.ProcessMetrics(ctx, md)
	if err != nil {
		t.Fatalf("Failed to process metrics: %v", err)
	}
	
	// Get aggregated results
	result := proc.GetAggregatedMetrics()
	
	// Verify we have exactly one aggregated histogram
	if result.DataPointCount() != 1 {
		t.Errorf("Expected 1 aggregated data point, got %d", result.DataPointCount())
	}
	
	// Verify the aggregated histogram has the correct values
	rms := result.ResourceMetrics()
	if rms.Len() != 1 {
		t.Fatalf("Expected 1 resource metric, got %d", rms.Len())
	}
	
	sms := rms.At(0).ScopeMetrics()
	if sms.Len() != 1 {
		t.Fatalf("Expected 1 scope metric, got %d", sms.Len())
	}
	
	metrics := sms.At(0).Metrics()
	if metrics.Len() != 1 {
		t.Fatalf("Expected 1 metric, got %d", metrics.Len())
	}
	
	metric := metrics.At(0)
	if metric.Name() != "http.request.duration" {
		t.Errorf("Expected metric name 'http.request.duration', got '%s'", metric.Name())
	}
	
	if metric.Type() != pmetric.MetricTypeHistogram {
		t.Errorf("Expected histogram metric type, got %v", metric.Type())
	}
	
	// Check the aggregated histogram data point
	histData := metric.Histogram()
	if histData.DataPoints().Len() != 1 {
		t.Fatalf("Expected 1 histogram data point, got %d", histData.DataPoints().Len())
	}
	
	dp := histData.DataPoints().At(0)
	
	// Verify count is the sum of all input counts
	if dp.Count() != expectedTotalCount {
		t.Errorf("Expected aggregated count %d, got %d", expectedTotalCount, dp.Count())
	}
	
	// Verify sum is the sum of all input sums
	if dp.Sum() != expectedTotalSum {
		t.Errorf("Expected aggregated sum %f, got %f", expectedTotalSum, dp.Sum())
	}
	
	// Verify no labels are present (all dropped)
	if dp.Attributes().Len() != 0 {
		t.Errorf("Expected no attributes (labels dropped), got %d attributes", dp.Attributes().Len())
		dp.Attributes().Range(func(k string, v pcommon.Value) bool {
			t.Logf("  Unexpected attribute: %s=%s", k, v.AsString())
			return true
		})
	}
	
	t.Logf("Successfully aggregated %d histogram series into 1 series", len(endpoints))
	t.Logf("  Total count: %d", dp.Count())
	t.Logf("  Total sum: %f", dp.Sum())
	t.Logf("  Labels dropped: ✓")
}

// TestSumLabelDropping verifies that sum/counter metrics continue to work correctly
// with label dropping (they already had this behavior)
func TestSumLabelDropping(t *testing.T) {
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

	// Create sum metrics with multiple series having different labels
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	
	sum := sm.Metrics().AppendEmpty()
	sum.SetName("request.count")
	s := sum.SetEmptySum()
	s.SetIsMonotonic(true)
	s.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	
	// Create multiple data points with different labels
	expectedTotal := float64(0)
	for i := 0; i < 5; i++ {
		dp := s.DataPoints().AppendEmpty()
		value := float64((i + 1) * 100)
		dp.SetDoubleValue(value)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		
		// Add labels that should be dropped
		dp.Attributes().PutStr("endpoint", "/api/endpoint"+string(rune('0'+i)))
		dp.Attributes().PutStr("status", "200")
		
		expectedTotal += value
	}
	
	// Process the metrics
	_, err = proc.ProcessMetrics(ctx, md)
	if err != nil {
		t.Fatalf("Failed to process metrics: %v", err)
	}
	
	// Get aggregated results
	result := proc.GetAggregatedMetrics()
	
	// Verify we have exactly one aggregated sum
	if result.DataPointCount() != 1 {
		t.Errorf("Expected 1 aggregated data point, got %d", result.DataPointCount())
	}
	
	// Check the aggregated sum
	rms := result.ResourceMetrics()
	sms := rms.At(0).ScopeMetrics()
	metrics := sms.At(0).Metrics()
	metric := metrics.At(0)
	
	if metric.Type() != pmetric.MetricTypeSum {
		t.Errorf("Expected sum metric type, got %v", metric.Type())
	}
	
	sumData := metric.Sum()
	dp := sumData.DataPoints().At(0)
	
	// Verify the sum is correct
	if dp.DoubleValue() != expectedTotal {
		t.Errorf("Expected aggregated sum %f, got %f", expectedTotal, dp.DoubleValue())
	}
	
	// Verify no labels are present
	if dp.Attributes().Len() != 0 {
		t.Errorf("Expected no attributes (labels dropped), got %d attributes", dp.Attributes().Len())
	}
	
	t.Logf("Sum/Counter metrics continue to work correctly with label dropping")
	t.Logf("  Aggregated value: %f", dp.DoubleValue())
	t.Logf("  Labels dropped: ✓")
}
