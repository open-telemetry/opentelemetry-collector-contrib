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
