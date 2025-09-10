// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streamingaggregationprocessor

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// TestHistogramCumulativeTemporalityDeltaComputation verifies that cumulative histograms
// are correctly converted to deltas before aggregation
func TestHistogramCumulativeTemporalityDeltaComputation(t *testing.T) {
	// Create an aggregator for histogram
	agg := NewAggregator(pmetric.MetricTypeHistogram)
	
	// Simulate cumulative histogram data points over time
	// These represent the same histogram sending cumulative values
	
	// Time 1: Initial cumulative values
	dp1 := pmetric.NewHistogramDataPoint()
	dp1.SetCount(100)
	dp1.SetSum(5000)
	dp1.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp1.ExplicitBounds().FromRaw([]float64{10, 20, 50, 100})
	dp1.BucketCounts().FromRaw([]uint64{10, 20, 30, 25, 15}) // 15 is infinity bucket
	
	// Process first data point
	agg.MergeHistogramWithTemporality(dp1, pmetric.AggregationTemporalityCumulative)
	
	// Verify initial values
	if agg.histogramTotalCount != 100 {
		t.Errorf("Expected initial total count 100, got %d", agg.histogramTotalCount)
	}
	if agg.histogramTotalSum != 5000 {
		t.Errorf("Expected initial total sum 5000, got %f", agg.histogramTotalSum)
	}
	
	// Time 2: Updated cumulative values (80 new requests)
	dp2 := pmetric.NewHistogramDataPoint()
	dp2.SetCount(180) // 100 + 80 new
	dp2.SetSum(9000)  // 5000 + 4000 new
	dp2.SetTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(30 * time.Second)))
	dp2.ExplicitBounds().FromRaw([]float64{10, 20, 50, 100})
	dp2.BucketCounts().FromRaw([]uint64{15, 30, 50, 40, 25}) // Cumulative bucket counts
	
	// Process second data point
	agg.MergeHistogramWithTemporality(dp2, pmetric.AggregationTemporalityCumulative)
	
	// Verify delta computation: should have added only the delta (80 requests, 4000 sum)
	expectedTotalCount := uint64(180) // 100 + 80
	expectedTotalSum := float64(9000) // 5000 + 4000
	
	if agg.histogramTotalCount != expectedTotalCount {
		t.Errorf("Expected total count %d after delta computation, got %d", 
			expectedTotalCount, agg.histogramTotalCount)
	}
	if agg.histogramTotalSum != expectedTotalSum {
		t.Errorf("Expected total sum %f after delta computation, got %f", 
			expectedTotalSum, agg.histogramTotalSum)
	}
	
	// Verify bucket deltas
	expectedBucket10 := uint64(15)  // 15 - 10 = 5 new
	expectedBucket20 := uint64(30)  // 30 - 20 = 10 new
	expectedBucket50 := uint64(50)  // 50 - 30 = 20 new
	expectedBucket100 := uint64(40) // 40 - 25 = 15 new
	expectedBucketInf := uint64(25) // 25 - 15 = 10 new
	
	if agg.histogramTotalBuckets[10] != expectedBucket10 {
		t.Errorf("Expected bucket[10]=%d, got %d", expectedBucket10, agg.histogramTotalBuckets[10])
	}
	if agg.histogramTotalBuckets[20] != expectedBucket20 {
		t.Errorf("Expected bucket[20]=%d, got %d", expectedBucket20, agg.histogramTotalBuckets[20])
	}
	if agg.histogramTotalBuckets[50] != expectedBucket50 {
		t.Errorf("Expected bucket[50]=%d, got %d", expectedBucket50, agg.histogramTotalBuckets[50])
	}
	if agg.histogramTotalBuckets[100] != expectedBucket100 {
		t.Errorf("Expected bucket[100]=%d, got %d", expectedBucket100, agg.histogramTotalBuckets[100])
	}
	if agg.histogramTotalBuckets[1e308] != expectedBucketInf {
		t.Errorf("Expected bucket[inf]=%d, got %d", expectedBucketInf, agg.histogramTotalBuckets[1e308])
	}
	
	// Time 3: Test counter reset handling
	dp3 := pmetric.NewHistogramDataPoint()
	dp3.SetCount(50)  // Reset to lower value
	dp3.SetSum(2500)  // Reset to lower value
	dp3.SetTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(60 * time.Second)))
	dp3.ExplicitBounds().FromRaw([]float64{10, 20, 50, 100})
	dp3.BucketCounts().FromRaw([]uint64{5, 10, 15, 10, 10})
	
	// Process third data point (counter reset)
	agg.MergeHistogramWithTemporality(dp3, pmetric.AggregationTemporalityCumulative)
	
	// After reset, should add the new values as-is
	expectedTotalCountAfterReset := uint64(230) // 180 + 50
	expectedTotalSumAfterReset := float64(11500) // 9000 + 2500
	
	if agg.histogramTotalCount != expectedTotalCountAfterReset {
		t.Errorf("Expected total count %d after reset, got %d", 
			expectedTotalCountAfterReset, agg.histogramTotalCount)
	}
	if agg.histogramTotalSum != expectedTotalSumAfterReset {
		t.Errorf("Expected total sum %f after reset, got %f", 
			expectedTotalSumAfterReset, agg.histogramTotalSum)
	}
}

// TestHistogramDeltaTemporality verifies that delta histograms are aggregated correctly
func TestHistogramDeltaTemporality(t *testing.T) {
	agg := NewAggregator(pmetric.MetricTypeHistogram)
	
	// Delta histogram data points - each represents new data
	dp1 := pmetric.NewHistogramDataPoint()
	dp1.SetCount(50)
	dp1.SetSum(2500)
	dp1.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp1.ExplicitBounds().FromRaw([]float64{10, 20, 50})
	dp1.BucketCounts().FromRaw([]uint64{10, 15, 20, 5})
	
	dp2 := pmetric.NewHistogramDataPoint()
	dp2.SetCount(30)
	dp2.SetSum(1500)
	dp2.SetTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(30 * time.Second)))
	dp2.ExplicitBounds().FromRaw([]float64{10, 20, 50})
	dp2.BucketCounts().FromRaw([]uint64{5, 10, 10, 5})
	
	// Process delta data points
	agg.MergeHistogramWithTemporality(dp1, pmetric.AggregationTemporalityDelta)
	agg.MergeHistogramWithTemporality(dp2, pmetric.AggregationTemporalityDelta)
	
	// For delta, values should be summed directly
	expectedCount := uint64(80)  // 50 + 30
	expectedSum := float64(4000) // 2500 + 1500
	
	if agg.histogramTotalCount != expectedCount {
		t.Errorf("Expected total count %d for delta temporality, got %d", 
			expectedCount, agg.histogramTotalCount)
	}
	if agg.histogramTotalSum != expectedSum {
		t.Errorf("Expected total sum %f for delta temporality, got %f", 
			expectedSum, agg.histogramTotalSum)
	}
	
	// Verify bucket sums
	if agg.histogramTotalBuckets[10] != 15 { // 10 + 5
		t.Errorf("Expected bucket[10]=15, got %d", agg.histogramTotalBuckets[10])
	}
	if agg.histogramTotalBuckets[20] != 25 { // 15 + 10
		t.Errorf("Expected bucket[20]=25, got %d", agg.histogramTotalBuckets[20])
	}
	if agg.histogramTotalBuckets[50] != 30 { // 20 + 10
		t.Errorf("Expected bucket[50]=30, got %d", agg.histogramTotalBuckets[50])
	}
	if agg.histogramTotalBuckets[1e308] != 10 { // 5 + 5 (infinity bucket)
		t.Errorf("Expected bucket[inf]=10, got %d", agg.histogramTotalBuckets[1e308])
	}
}

// TestHistogramMixedSources simulates the streaming aggregation scenario
// where multiple sources send cumulative histograms that get aggregated
func TestHistogramMixedSources(t *testing.T) {
	// This simulates the processor dropping labels and aggregating
	// multiple histogram series into one
	agg := NewAggregator(pmetric.MetricTypeHistogram)
	
	// Source A: First cumulative value
	dpA1 := pmetric.NewHistogramDataPoint()
	dpA1.SetCount(100)
	dpA1.SetSum(5000)
	dpA1.ExplicitBounds().FromRaw([]float64{10, 50, 100})
	dpA1.BucketCounts().FromRaw([]uint64{20, 40, 30, 10})
	
	// Source B: First cumulative value  
	dpB1 := pmetric.NewHistogramDataPoint()
	dpB1.SetCount(150)
	dpB1.SetSum(7500)
	dpB1.ExplicitBounds().FromRaw([]float64{10, 50, 100})
	dpB1.BucketCounts().FromRaw([]uint64{30, 60, 45, 15})
	
	// Process first values from both sources
	agg.MergeHistogramWithTemporality(dpA1, pmetric.AggregationTemporalityCumulative)
	
	// After first source A
	if agg.histogramTotalCount != 100 {
		t.Errorf("After source A: expected count 100, got %d", agg.histogramTotalCount)
	}
	
	// Note: In real streaming aggregation, source B would be a different aggregator
	// since they have different labels. But after label dropping, they merge.
	// For this test, we're simulating the merged result.
	
	// Create a second aggregator for source B to simulate separate tracking
	aggB := NewAggregator(pmetric.MetricTypeHistogram)
	aggB.MergeHistogramWithTemporality(dpB1, pmetric.AggregationTemporalityCumulative)
	
	// Source A: Second cumulative value (20 new requests)
	dpA2 := pmetric.NewHistogramDataPoint()
	dpA2.SetCount(120)  // 100 + 20
	dpA2.SetSum(6000)   // 5000 + 1000
	dpA2.ExplicitBounds().FromRaw([]float64{10, 50, 100})
	dpA2.BucketCounts().FromRaw([]uint64{24, 48, 36, 12})
	
	// Source B: Second cumulative value (30 new requests)
	dpB2 := pmetric.NewHistogramDataPoint()
	dpB2.SetCount(180)  // 150 + 30
	dpB2.SetSum(9000)   // 7500 + 1500
	dpB2.ExplicitBounds().FromRaw([]float64{10, 50, 100})
	dpB2.BucketCounts().FromRaw([]uint64{36, 72, 54, 18})
	
	// Process second values
	agg.MergeHistogramWithTemporality(dpA2, pmetric.AggregationTemporalityCumulative)
	aggB.MergeHistogramWithTemporality(dpB2, pmetric.AggregationTemporalityCumulative)
	
	// Verify each aggregator computed deltas correctly
	// Source A: 100 initial + 20 new = 120 total
	if agg.histogramTotalCount != 120 {
		t.Errorf("Source A: expected total count 120, got %d", agg.histogramTotalCount)
	}
	if agg.histogramTotalSum != 6000 {
		t.Errorf("Source A: expected total sum 6000, got %f", agg.histogramTotalSum)
	}
	
	// Source B: 150 initial + 30 new = 180 total
	if aggB.histogramTotalCount != 180 {
		t.Errorf("Source B: expected total count 180, got %d", aggB.histogramTotalCount)
	}
	if aggB.histogramTotalSum != 9000 {
		t.Errorf("Source B: expected total sum 9000, got %f", aggB.histogramTotalSum)
	}
	
	// In the real processor, these would be combined after label dropping
	// Total across both sources: 120 + 180 = 300
	combinedCount := agg.histogramTotalCount + aggB.histogramTotalCount
	combinedSum := agg.histogramTotalSum + aggB.histogramTotalSum
	
	if combinedCount != 300 {
		t.Errorf("Combined: expected total count 300, got %d", combinedCount)
	}
	if combinedSum != 15000 {
		t.Errorf("Combined: expected total sum 15000, got %f", combinedSum)
	}
}
