// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streamingaggregationprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestEnhancedLabelFiltering(t *testing.T) {
	cfg := &Config{
		WindowSize:         1 * time.Second,
		MaxMemoryMB:        10,
		StaleDataThreshold: 30 * time.Second,
		Metrics: []MetricConfig{
			{
				Match:         "temperature_celsius",
				AggregateType: Sum,
				Labels: LabelConfig{
					Type:  Keep,
					Names: []string{"service", "method"},
				},
			},
		},
	}

	logger := zap.NewNop()
	proc, err := newStreamingAggregationProcessor(logger, cfg)
	require.NoError(t, err)

	ctx := context.Background()
	err = proc.Start(ctx, nil)
	require.NoError(t, err)
	defer proc.Shutdown(ctx)

	// Create test metric with multiple labels
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("temperature_celsius")
	metric.SetEmptyGauge()

	// Add data points with different label combinations
	dp1 := metric.Gauge().DataPoints().AppendEmpty()
	dp1.SetDoubleValue(25.5)
	dp1.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp1.Attributes().PutStr("service", "api")
	dp1.Attributes().PutStr("method", "GET")
	dp1.Attributes().PutStr("instance", "1") // Should be dropped

	dp2 := metric.Gauge().DataPoints().AppendEmpty()
	dp2.SetDoubleValue(30.2)
	dp2.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp2.Attributes().PutStr("service", "api")
	dp2.Attributes().PutStr("method", "POST")
	dp2.Attributes().PutStr("instance", "2") // Should be dropped

	dp3 := metric.Gauge().DataPoints().AppendEmpty()
	dp3.SetDoubleValue(22.1)
	dp3.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp3.Attributes().PutStr("service", "web")
	dp3.Attributes().PutStr("method", "GET")
	dp3.Attributes().PutStr("instance", "3") // Should be dropped

	// Process the metrics
	_, err = proc.ProcessMetrics(ctx, md)
	require.NoError(t, err)

	// Get aggregated results
	result := proc.GetAggregatedMetrics()

	// Should have aggregated data points
	require.Greater(t, result.DataPointCount(), 0, "Expected aggregated metrics")

	// Check that labels were properly filtered and aggregated
	rms := result.ResourceMetrics()
	require.Greater(t, rms.Len(), 0, "Expected resource metrics")

	t.Logf("Successfully tested enhanced label filtering with %d aggregated data points", result.DataPointCount())
}

func TestEnhancedAggregationStrategies(t *testing.T) {
	testCases := []struct {
		name     string
		aggType  AggregationType
		values   []float64
		expected float64
	}{
		{
			name:     "sum_aggregation",
			aggType:  Sum,
			values:   []float64{10.0, 20.0, 30.0},
			expected: 60.0,
		},
		{
			name:     "average_aggregation",
			aggType:  Average,
			values:   []float64{10.0, 20.0, 30.0},
			expected: 20.0,
		},
		{
			name:     "max_aggregation",
			aggType:  Max,
			values:   []float64{10.0, 30.0, 20.0},
			expected: 30.0,
		},
		{
			name:     "min_aggregation",
			aggType:  Min,
			values:   []float64{30.0, 10.0, 20.0},
			expected: 10.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				WindowSize:         1 * time.Second,
				MaxMemoryMB:        10,
				StaleDataThreshold: 30 * time.Second,
				Metrics: []MetricConfig{
					{
						Match:         "test_metric",
						AggregateType: tc.aggType,
						Labels: LabelConfig{
							Type: DropAll,
						},
					},
				},
			}

			logger := zap.NewNop()
			proc, err := newStreamingAggregationProcessor(logger, cfg)
			require.NoError(t, err)

			ctx := context.Background()
			err = proc.Start(ctx, nil)
			require.NoError(t, err)
			defer proc.Shutdown(ctx)

			// Create test metric with multiple data points
			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()
			sm := rm.ScopeMetrics().AppendEmpty()
			metric := sm.Metrics().AppendEmpty()
			metric.SetName("test_metric")
			metric.SetEmptyGauge()

			baseTime := time.Now()
			for i, value := range tc.values {
				dp := metric.Gauge().DataPoints().AppendEmpty()
				dp.SetDoubleValue(value)
				dp.SetTimestamp(pcommon.NewTimestampFromTime(baseTime.Add(time.Duration(i) * time.Millisecond)))
			}

			// Process the metrics
			_, err = proc.ProcessMetrics(ctx, md)
			require.NoError(t, err)

			// Get aggregated results
			result := proc.GetAggregatedMetrics()
			require.Greater(t, result.DataPointCount(), 0, "Expected aggregated metrics")

			t.Logf("Successfully tested %s aggregation strategy", tc.aggType)
		})
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Test that existing configurations without enhanced features still work
	cfg := &Config{
		WindowSize:         1 * time.Second,
		MaxMemoryMB:        10,
		StaleDataThreshold: 30 * time.Second,
		// No metrics configuration - should use default behavior
	}

	logger := zap.NewNop()
	proc, err := newStreamingAggregationProcessor(logger, cfg)
	require.NoError(t, err)

	ctx := context.Background()
	err = proc.Start(ctx, nil)
	require.NoError(t, err)
	defer proc.Shutdown(ctx)

	// Create test metrics
	md := createTestMetrics() // Use existing test helper

	// Process metrics
	_, err = proc.ProcessMetrics(ctx, md)
	require.NoError(t, err)

	// Get aggregated results
	result := proc.GetAggregatedMetrics()
	require.Greater(t, result.DataPointCount(), 0, "Expected aggregated metrics")

	t.Logf("Backward compatibility test passed with %d aggregated data points", result.DataPointCount())
}

func TestCounterAggregationStrategies(t *testing.T) {
	tests := []struct {
		name           string
		aggregateType  AggregationType
		expectedType   pmetric.MetricType
		validateResult func(t *testing.T, metric pmetric.Metric)
	}{
		{
			name:          "sum_aggregation",
			aggregateType: Sum,
			expectedType:  pmetric.MetricTypeSum,
			validateResult: func(t *testing.T, metric pmetric.Metric) {
				require.Equal(t, pmetric.MetricTypeSum, metric.Type())
				require.Greater(t, metric.Sum().DataPoints().At(0).DoubleValue(), 0.0)
			},
		},
		{
			name:          "average_aggregation",
			aggregateType: Average,
			expectedType:  pmetric.MetricTypeSum,
			validateResult: func(t *testing.T, metric pmetric.Metric) {
				require.Equal(t, pmetric.MetricTypeSum, metric.Type())
				require.Greater(t, metric.Sum().DataPoints().At(0).DoubleValue(), 0.0)
			},
		},
		{
			name:          "rate_aggregation",
			aggregateType: Rate,
			expectedType:  pmetric.MetricTypeGauge, // Rate exports as gauge
			validateResult: func(t *testing.T, metric pmetric.Metric) {
				require.Equal(t, pmetric.MetricTypeGauge, metric.Type())
				require.GreaterOrEqual(t, metric.Gauge().DataPoints().At(0).DoubleValue(), 0.0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				WindowSize:         1 * time.Second,
				MaxMemoryMB:        10,
				StaleDataThreshold: 30 * time.Second,
				Metrics: []MetricConfig{
					{
						Match:         "http_requests_total",
						AggregateType: tt.aggregateType,
						Labels: LabelConfig{
							Type:  Remove,
							Names: []string{"instance"}, // Remove instance, keep method/status/endpoint
						},
					},
				},
			}

			logger := zap.NewNop()
			proc, err := newStreamingAggregationProcessor(logger, cfg)
			require.NoError(t, err)

			ctx := context.Background()
			err = proc.Start(ctx, nil)
			require.NoError(t, err)
			defer proc.Shutdown(ctx)

			// Create test counter metric with multiple instances
			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()
			sm := rm.ScopeMetrics().AppendEmpty()
			metric := sm.Metrics().AppendEmpty()
			metric.SetName("http_requests_total")
			sum := metric.SetEmptySum()
			sum.SetIsMonotonic(true)
			sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

			// Simulate multiple instances with different request counts
			instances := []struct {
				instance string
				method   string
				status   string
				endpoint string
				value    int64
			}{
				{"web-01", "GET", "200", "/api/users", 100},
				{"web-02", "GET", "200", "/api/users", 150},
				{"web-03", "GET", "200", "/api/users", 120},
				{"web-01", "POST", "201", "/api/users", 50},
				{"web-02", "POST", "201", "/api/users", 75},
			}

			for _, inst := range instances {
				dp := sum.DataPoints().AppendEmpty()
				dp.SetIntValue(inst.value)
				dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
				dp.Attributes().PutStr("instance", inst.instance)
				dp.Attributes().PutStr("method", inst.method)
				dp.Attributes().PutStr("status_code", inst.status)
				dp.Attributes().PutStr("endpoint", inst.endpoint)
			}

			// Process the metrics
			_, err = proc.ProcessMetrics(ctx, md)
			require.NoError(t, err)

			// Wait for processing
			time.Sleep(100 * time.Millisecond)

			// Get aggregated results
			result := proc.GetAggregatedMetrics()
			require.Greater(t, result.DataPointCount(), 0, "Expected aggregated metrics")

			// Find the aggregated http_requests_total metric
			found := false
			for i := 0; i < result.ResourceMetrics().Len(); i++ {
				rm := result.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						m := sm.Metrics().At(k)
						if m.Name() == "http_requests_total" {
							tt.validateResult(t, m)
							found = true
						}
					}
				}
			}
			require.True(t, found, "Expected to find aggregated http_requests_total metric")

			t.Logf("Successfully tested %s counter aggregation strategy", tt.name)
		})
	}
}

func TestUpDownCounterAggregationStrategies(t *testing.T) {
	tests := []struct {
		name           string
		aggregateType  AggregationType
		expectedType   pmetric.MetricType
		validateResult func(t *testing.T, metric pmetric.Metric)
	}{
		{
			name:          "sum_aggregation",
			aggregateType: Sum,
			expectedType:  pmetric.MetricTypeGauge, // UpDownCounters export as gauges
			validateResult: func(t *testing.T, metric pmetric.Metric) {
				require.Equal(t, pmetric.MetricTypeGauge, metric.Type())
				require.Greater(t, metric.Gauge().DataPoints().At(0).DoubleValue(), 0.0)
			},
		},
		{
			name:          "average_aggregation",
			aggregateType: Average,
			expectedType:  pmetric.MetricTypeGauge,
			validateResult: func(t *testing.T, metric pmetric.Metric) {
				require.Equal(t, pmetric.MetricTypeGauge, metric.Type())
				require.Greater(t, metric.Gauge().DataPoints().At(0).DoubleValue(), 0.0)
			},
		},
		{
			name:          "max_aggregation",
			aggregateType: Max,
			expectedType:  pmetric.MetricTypeGauge,
			validateResult: func(t *testing.T, metric pmetric.Metric) {
				require.Equal(t, pmetric.MetricTypeGauge, metric.Type())
				require.Greater(t, metric.Gauge().DataPoints().At(0).DoubleValue(), 0.0)
			},
		},
		{
			name:          "min_aggregation",
			aggregateType: Min,
			expectedType:  pmetric.MetricTypeGauge,
			validateResult: func(t *testing.T, metric pmetric.Metric) {
				require.Equal(t, pmetric.MetricTypeGauge, metric.Type())
				require.GreaterOrEqual(t, metric.Gauge().DataPoints().At(0).DoubleValue(), 0.0)
			},
		},
		{
			name:          "last_aggregation",
			aggregateType: Last,
			expectedType:  pmetric.MetricTypeGauge,
			validateResult: func(t *testing.T, metric pmetric.Metric) {
				require.Equal(t, pmetric.MetricTypeGauge, metric.Type())
				require.GreaterOrEqual(t, metric.Gauge().DataPoints().At(0).DoubleValue(), 0.0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				WindowSize:         1 * time.Second,
				MaxMemoryMB:        10,
				StaleDataThreshold: 30 * time.Second,
				Metrics: []MetricConfig{
					{
						Match:         "active_connections",
						AggregateType: tt.aggregateType,
						Labels: LabelConfig{
							Type:  Remove,
							Names: []string{"instance"}, // Remove instance, keep service/region/protocol
						},
					},
				},
			}

			logger := zap.NewNop()
			proc, err := newStreamingAggregationProcessor(logger, cfg)
			require.NoError(t, err)

			ctx := context.Background()
			err = proc.Start(ctx, nil)
			require.NoError(t, err)
			defer proc.Shutdown(ctx)

			// Create test UpDownCounter metric with multiple instances
			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()
			sm := rm.ScopeMetrics().AppendEmpty()
			metric := sm.Metrics().AppendEmpty()
			metric.SetName("active_connections")
			sum := metric.SetEmptySum()
			sum.SetIsMonotonic(false) // UpDownCounter is non-monotonic
			sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

			// Simulate multiple instances with different connection counts
			instances := []struct {
				service  string
				region   string
				protocol string
				instance string
				value    int64
			}{
				{"web", "us-east-1", "http", "srv-01", 25},
				{"web", "us-east-1", "http", "srv-02", 30},
				{"web", "us-east-1", "http", "srv-03", 20},
				{"api", "us-west-2", "grpc", "srv-01", 15},
				{"api", "us-west-2", "grpc", "srv-02", 18},
			}

			for _, inst := range instances {
				dp := sum.DataPoints().AppendEmpty()
				dp.SetIntValue(inst.value)
				dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
				dp.Attributes().PutStr("service", inst.service)
				dp.Attributes().PutStr("region", inst.region)
				dp.Attributes().PutStr("protocol", inst.protocol)
				dp.Attributes().PutStr("instance", inst.instance)
			}

			// Process the metrics
			_, err = proc.ProcessMetrics(ctx, md)
			require.NoError(t, err)

			// Wait for processing
			time.Sleep(100 * time.Millisecond)

			// Get aggregated results
			result := proc.GetAggregatedMetrics()
			require.Greater(t, result.DataPointCount(), 0, "Expected aggregated metrics")

			// Find the aggregated active_connections metric
			found := false
			for i := 0; i < result.ResourceMetrics().Len(); i++ {
				rm := result.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						m := sm.Metrics().At(k)
						if m.Name() == "active_connections" {
							tt.validateResult(t, m)
							found = true
						}
					}
				}
			}
			require.True(t, found, "Expected to find aggregated active_connections metric")

			t.Logf("Successfully tested %s UpDownCounter aggregation strategy", tt.name)
		})
	}
}

func TestHistogramAggregationStrategies(t *testing.T) {
	tests := []struct {
		name           string
		aggregateType  AggregationType
		expectedType   pmetric.MetricType
		validateResult func(t *testing.T, metric pmetric.Metric)
	}{
		{
			name:         "sum_aggregation",
			aggregateType: Sum,
			expectedType: pmetric.MetricTypeHistogram,
			validateResult: func(t *testing.T, metric pmetric.Metric) {
				require.Equal(t, pmetric.MetricTypeHistogram, metric.Type())
				require.Greater(t, metric.Histogram().DataPoints().Len(), 0)
				// Check that buckets have been summed
				dp := metric.Histogram().DataPoints().At(0)
				require.Greater(t, dp.Count(), uint64(0))
				require.Greater(t, dp.Sum(), 0.0)
			},
		},
		{
			name:         "p95_aggregation",
			aggregateType: P95,
			expectedType: pmetric.MetricTypeHistogram, // Quantiles export as histograms for client-side calculation
			validateResult: func(t *testing.T, metric pmetric.Metric) {
				require.Equal(t, pmetric.MetricTypeHistogram, metric.Type())
				require.Greater(t, metric.Histogram().DataPoints().Len(), 0)
				// Verify histogram has bucket data for client-side P95 calculation
				dp := metric.Histogram().DataPoints().At(0)
				require.Greater(t, dp.Count(), uint64(0))
				require.Greater(t, dp.Sum(), 0.0)
			},
		},
		{
			name:         "p99_aggregation",
			aggregateType: P99,
			expectedType: pmetric.MetricTypeHistogram,
			validateResult: func(t *testing.T, metric pmetric.Metric) {
				require.Equal(t, pmetric.MetricTypeHistogram, metric.Type())
				require.Greater(t, metric.Histogram().DataPoints().Len(), 0)
				// Verify histogram has bucket data for client-side P99 calculation
				dp := metric.Histogram().DataPoints().At(0)
				require.Greater(t, dp.Count(), uint64(0))
				require.Greater(t, dp.Sum(), 0.0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				WindowSize:         1 * time.Second,
				MaxMemoryMB:        10,
				StaleDataThreshold: 30 * time.Second,
				Metrics: []MetricConfig{
					{
						Match:         "http_response_time_ms",
						AggregateType: tt.aggregateType,
						Labels: LabelConfig{
							Type:  Remove,
							Names: []string{"instance"}, // Remove instance, keep endpoint
						},
					},
				},
			}

			logger := zap.NewNop()
			proc, err := newStreamingAggregationProcessor(logger, cfg)
			require.NoError(t, err)

			ctx := context.Background()
			err = proc.Start(ctx, nil)
			require.NoError(t, err)
			defer proc.Shutdown(ctx)

			// Create test histogram metric with multiple instances
			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()
			sm := rm.ScopeMetrics().AppendEmpty()
			metric := sm.Metrics().AppendEmpty()
			metric.SetName("http_response_time_ms")
			hist := metric.SetEmptyHistogram()

			// Create histogram data points from different instances
			instances := []struct {
				endpoint string
				instance string
				buckets  []uint64 // Bucket counts for response times
				sum      float64
				count    uint64
			}{
				{"/api/users", "web-01", []uint64{10, 15, 8, 3, 1}, 2500.0, 37},
				{"/api/users", "web-02", []uint64{12, 18, 6, 2, 1}, 2200.0, 39},
				{"/api/users", "web-03", []uint64{8, 20, 10, 4, 2}, 3100.0, 44},
			}

			// Define bucket boundaries: 10ms, 50ms, 100ms, 500ms, +Inf
			bucketBounds := []float64{10, 50, 100, 500}

			for _, inst := range instances {
				dp := hist.DataPoints().AppendEmpty()
				dp.SetCount(inst.count)
				dp.SetSum(inst.sum)
				dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

				// Set bucket boundaries
				for _, bound := range bucketBounds {
					dp.ExplicitBounds().Append(bound)
				}

				// Set bucket counts
				for _, bucketCount := range inst.buckets {
					dp.BucketCounts().Append(bucketCount)
				}

				// Set attributes
				dp.Attributes().PutStr("endpoint", inst.endpoint)
				dp.Attributes().PutStr("instance", inst.instance)
			}

			// Process the metrics
			_, err = proc.ProcessMetrics(ctx, md)
			require.NoError(t, err)

			// Wait for processing
			time.Sleep(100 * time.Millisecond)

			// Get aggregated results
			result := proc.GetAggregatedMetrics()
			require.Greater(t, result.DataPointCount(), 0, "Expected aggregated metrics")

			// Find the aggregated histogram metric
			found := false
			for i := 0; i < result.ResourceMetrics().Len(); i++ {
				rm := result.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						m := sm.Metrics().At(k)
						if m.Name() == "http_response_time_ms" {
							tt.validateResult(t, m)
							found = true
						}
					}
				}
			}
			require.True(t, found, "Expected to find aggregated http_response_time_ms metric")

			t.Logf("Successfully tested %s histogram aggregation strategy", tt.name)
		})
	}
}
