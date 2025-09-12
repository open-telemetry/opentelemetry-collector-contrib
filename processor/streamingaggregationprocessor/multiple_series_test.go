// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streamingaggregationprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// TestMultipleHistogramSeriesAggregation tests that multiple histogram series with different labels
// are correctly summed together when labels are dropped for streaming aggregation
func TestMultipleHistogramSeriesAggregation(t *testing.T) {
	logger := zap.NewNop()
	config := &Config{
		WindowSize:         10 * time.Second,
		StaleDataThreshold: 30 * time.Second,
	}
	
	processor, err := newStreamingAggregationProcessor(logger, config)
	require.NoError(t, err)
	
	ctx := context.Background()
	err = processor.Start(ctx, nil)
	require.NoError(t, err)
	defer processor.Shutdown(ctx)
	
	// Create metrics with 5 different histogram series (different endpoints)
	// Each series has a count of ~2300
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	
	// Create histogram metric
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("http_response_time_ms")
	hist := metric.SetEmptyHistogram()
	hist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	
	// Define the endpoints and their expected counts
	endpoints := []string{
		"/api/orders",
		"/api/products", 
		"/api/users",
		"/health",
		"/metrics",
	}
	
	expectedCountPerEndpoint := uint64(2300)
	expectedTotalCount := expectedCountPerEndpoint * uint64(len(endpoints))
	
	// Add data points for each endpoint
	for _, endpoint := range endpoints {
		dp := hist.DataPoints().AppendEmpty()
		dp.SetCount(expectedCountPerEndpoint)
		dp.SetSum(float64(expectedCountPerEndpoint) * 100) // Average of 100ms
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-1 * time.Second)))
		
		// Set the endpoint label (this will be dropped during aggregation)
		dp.Attributes().PutStr("endpoint", endpoint)
		
		// Set bucket bounds and counts
		dp.ExplicitBounds().FromRaw([]float64{10, 50, 100, 250, 500, 1000})
		dp.BucketCounts().FromRaw([]uint64{100, 500, 800, 600, 200, 100, 0})
	}
	
	// Process the metrics
	_, err = processor.ProcessMetrics(ctx, md)
	require.NoError(t, err)
	
	// Get aggregated metrics
	aggregated := processor.GetAggregatedMetrics()
	
	// Verify the aggregation
	require.Equal(t, 1, aggregated.ResourceMetrics().Len())
	require.Equal(t, 1, aggregated.ResourceMetrics().At(0).ScopeMetrics().Len())
	require.Equal(t, 1, aggregated.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
	
	aggregatedMetric := aggregated.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, "http_response_time_ms", aggregatedMetric.Name())
	assert.Equal(t, pmetric.MetricTypeHistogram, aggregatedMetric.Type())
	
	aggregatedHist := aggregatedMetric.Histogram()
	assert.Equal(t, 1, aggregatedHist.DataPoints().Len())
	
	dp := aggregatedHist.DataPoints().At(0)
	
	// The key assertion: count should be the sum of all series
	assert.Equal(t, expectedTotalCount, dp.Count(), 
		"Expected aggregated count to be %d (sum of all series), but got %d", 
		expectedTotalCount, dp.Count())
	
	// Verify sum is also aggregated correctly
	expectedTotalSum := float64(expectedTotalCount) * 100
	assert.Equal(t, expectedTotalSum, dp.Sum(),
		"Expected aggregated sum to be %f, but got %f",
		expectedTotalSum, dp.Sum())
	
	// Verify no labels remain (all dropped for streaming aggregation)
	assert.Equal(t, 0, dp.Attributes().Len(), "Expected all labels to be dropped")
	
	// Verify bucket counts are summed correctly
	assert.Equal(t, 6, dp.ExplicitBounds().Len())
	assert.Equal(t, 7, dp.BucketCounts().Len()) // 6 bounds + 1 infinity bucket
	
	// Each endpoint contributed these bucket counts: [100, 500, 800, 600, 200, 100, 0]
	// So total should be 5x those values
	expectedBucketCounts := []uint64{500, 2500, 4000, 3000, 1000, 500, 0}
	for i := 0; i < dp.BucketCounts().Len(); i++ {
		assert.Equal(t, expectedBucketCounts[i], dp.BucketCounts().At(i),
			"Bucket %d count mismatch", i)
	}
}

// TestMultipleHistogramSeriesCumulative tests aggregation with cumulative temporality
func TestMultipleHistogramSeriesCumulative(t *testing.T) {
	logger := zap.NewNop()
	config := &Config{
		WindowSize:         10 * time.Second,
		StaleDataThreshold: 30 * time.Second,
	}
	
	processor, err := newStreamingAggregationProcessor(logger, config)
	require.NoError(t, err)
	
	ctx := context.Background()
	err = processor.Start(ctx, nil)
	require.NoError(t, err)
	defer processor.Shutdown(ctx)
	
	// First batch: Send initial cumulative values
	md1 := pmetric.NewMetrics()
	rm1 := md1.ResourceMetrics().AppendEmpty()
	sm1 := rm1.ScopeMetrics().AppendEmpty()
	
	metric1 := sm1.Metrics().AppendEmpty()
	metric1.SetName("http_response_time_ms")
	hist1 := metric1.SetEmptyHistogram()
	hist1.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	
	// Initial cumulative values for 3 endpoints
	endpoints := []string{"/api/orders", "/api/products", "/api/users"}
	initialCount := uint64(1000)
	
	for _, endpoint := range endpoints {
		dp := hist1.DataPoints().AppendEmpty()
		dp.SetCount(initialCount)
		dp.SetSum(float64(initialCount) * 50)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.Attributes().PutStr("endpoint", endpoint)
		dp.ExplicitBounds().FromRaw([]float64{10, 50, 100})
		dp.BucketCounts().FromRaw([]uint64{200, 500, 300, 0})
	}
	
	_, err = processor.ProcessMetrics(ctx, md1)
	require.NoError(t, err)
	
	// Second batch: Send updated cumulative values
	md2 := pmetric.NewMetrics()
	rm2 := md2.ResourceMetrics().AppendEmpty()
	sm2 := rm2.ScopeMetrics().AppendEmpty()
	
	metric2 := sm2.Metrics().AppendEmpty()
	metric2.SetName("http_response_time_ms")
	hist2 := metric2.SetEmptyHistogram()
	hist2.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	
	// Updated cumulative values (added 500 more to each)
	updatedCount := uint64(1500)
	
	for _, endpoint := range endpoints {
		dp := hist2.DataPoints().AppendEmpty()
		dp.SetCount(updatedCount)
		dp.SetSum(float64(updatedCount) * 50)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.Attributes().PutStr("endpoint", endpoint)
		dp.ExplicitBounds().FromRaw([]float64{10, 50, 100})
		dp.BucketCounts().FromRaw([]uint64{300, 750, 450, 0})
	}
	
	_, err = processor.ProcessMetrics(ctx, md2)
	require.NoError(t, err)
	
	// Get aggregated metrics
	aggregated := processor.GetAggregatedMetrics()
	
	// Verify aggregation
	require.Equal(t, 1, aggregated.ResourceMetrics().Len())
	aggregatedMetric := aggregated.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	dp := aggregatedMetric.Histogram().DataPoints().At(0)
	
	// The aggregated count should be the sum of all deltas
	// Initial: 3 * 1000 = 3000
	// Delta: 3 * 500 = 1500
	// Total: 4500
	assert.Equal(t, uint64(4500), dp.Count(),
		"Expected aggregated count to be 4500, but got %d", dp.Count())
}

// TestConcurrentHistogramAggregation tests that concurrent updates to the same histogram work correctly
func TestConcurrentHistogramAggregation(t *testing.T) {
	logger := zap.NewNop()
	config := &Config{
		WindowSize:         10 * time.Second,
		StaleDataThreshold: 30 * time.Second,
	}
	
	processor, err := newStreamingAggregationProcessor(logger, config)
	require.NoError(t, err)
	
	ctx := context.Background()
	err = processor.Start(ctx, nil)
	require.NoError(t, err)
	defer processor.Shutdown(ctx)
	
	// Send multiple batches concurrently
	numBatches := 10
	countPerBatch := uint64(100)
	
	errChan := make(chan error, numBatches)
	
	for i := 0; i < numBatches; i++ {
		go func(batchNum int) {
			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()
			sm := rm.ScopeMetrics().AppendEmpty()
			
			metric := sm.Metrics().AppendEmpty()
			metric.SetName("concurrent_histogram")
			hist := metric.SetEmptyHistogram()
			hist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			
			dp := hist.DataPoints().AppendEmpty()
			dp.SetCount(countPerBatch)
			dp.SetSum(float64(countPerBatch) * 10)
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			dp.Attributes().PutStr("batch", string(rune(batchNum)))
			
			_, err := processor.ProcessMetrics(ctx, md)
			errChan <- err
		}(i)
	}
	
	// Wait for all batches to complete
	for i := 0; i < numBatches; i++ {
		err := <-errChan
		require.NoError(t, err)
	}
	
	// Small delay to ensure processing completes
	time.Sleep(100 * time.Millisecond)
	
	// Get aggregated metrics
	aggregated := processor.GetAggregatedMetrics()
	
	// Verify aggregation
	require.Equal(t, 1, aggregated.ResourceMetrics().Len())
	metrics := aggregated.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	
	// Find the concurrent_histogram metric
	var found bool
	for i := 0; i < metrics.Len(); i++ {
		if metrics.At(i).Name() == "concurrent_histogram" {
			found = true
			dp := metrics.At(i).Histogram().DataPoints().At(0)
			
			expectedTotalCount := countPerBatch * uint64(numBatches)
			assert.Equal(t, expectedTotalCount, dp.Count(),
				"Expected total count %d, got %d", expectedTotalCount, dp.Count())
			
			expectedTotalSum := float64(expectedTotalCount) * 10
			assert.Equal(t, expectedTotalSum, dp.Sum(),
				"Expected total sum %f, got %f", expectedTotalSum, dp.Sum())
			break
		}
	}
	
	assert.True(t, found, "concurrent_histogram metric not found")
}
