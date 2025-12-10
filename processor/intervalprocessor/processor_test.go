// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package intervalprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor"

import (
	"context"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor/internal/metadata"
)

func TestAggregation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                 string
		passThrough          bool
		aggregateToHistogram []AggregateToHistogram
	}{
		{name: "basic_aggregation"},
		{name: "histograms_are_aggregated"},
		{name: "exp_histograms_are_aggregated"},
		{name: "gauges_are_aggregated"},
		{name: "summaries_are_aggregated"},
		{name: "all_delta_metrics_are_passed_through"},  // Deltas are passed through even when aggregation is enabled
		{name: "non_monotonic_sums_are_passed_through"}, // Non-monotonic sums are passed through even when aggregation is enabled
		{name: "gauges_are_passed_through", passThrough: true},
		{name: "summaries_are_passed_through", passThrough: true},
		{
			name: "gauge_to_histogram",
			aggregateToHistogram: []AggregateToHistogram{
				{
					MetricName: "response.latency",
					Buckets:    []float64{10, 25, 50, 100},
				},
			},
		},
		{
			name: "counter_to_histogram",
			aggregateToHistogram: []AggregateToHistogram{
				{
					MetricName: "request.size",
					Buckets:    []float64{100, 500, 1000, 5000},
				},
			},
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var config *Config
	for _, tc := range testCases {
		config = &Config{
			Interval:             time.Second,
			PassThrough:          PassThrough{Gauge: tc.passThrough, Summary: tc.passThrough},
			AggregateToHistogram: tc.aggregateToHistogram,
		}

		t.Run(tc.name, func(t *testing.T) {
			// next stores the results of the filter metric processor
			next := &consumertest.MetricsSink{}

			factory := NewFactory()
			mgp, err := factory.CreateMetrics(
				t.Context(),
				processortest.NewNopSettings(metadata.Type),
				config,
				next,
			)
			require.NoError(t, err)

			dir := filepath.Join("testdata", tc.name)

			md, err := golden.ReadMetrics(filepath.Join(dir, "input.yaml"))
			require.NoError(t, err)

			// Test that ConsumeMetrics works
			err = mgp.ConsumeMetrics(ctx, md)
			require.NoError(t, err)

			require.IsType(t, &intervalProcessor{}, mgp)
			processor := mgp.(*intervalProcessor)

			// Pretend we hit the interval timer and call export
			processor.exportMetrics()

			// All the lookup tables should now be empty
			require.Empty(t, processor.rmLookup)
			require.Empty(t, processor.smLookup)
			require.Empty(t, processor.mLookup)
			require.Empty(t, processor.numberLookup)
			require.Empty(t, processor.histogramLookup)
			require.Empty(t, processor.expHistogramLookup)
			require.Empty(t, processor.summaryLookup)
			require.Empty(t, processor.histogramAggregationLookup)

			// Exporting again should return nothing
			processor.exportMetrics()

			// Next should have gotten three data sets:
			// 1. Anything left over from ConsumeMetrics()
			// 2. Anything exported from exportMetrics()
			// 3. An empty entry for the second call to exportMetrics()
			allMetrics := next.AllMetrics()
			require.Len(t, allMetrics, 3)

			nextData := allMetrics[0]
			exportData := allMetrics[1]
			secondExportData := allMetrics[2]

			expectedNextData, err := golden.ReadMetrics(filepath.Join(dir, "next.yaml"))
			require.NoError(t, err)
			require.NoError(t, pmetrictest.CompareMetrics(expectedNextData, nextData))

			expectedExportData, err := golden.ReadMetrics(filepath.Join(dir, "output.yaml"))
			require.NoError(t, err)
			require.NoError(t, pmetrictest.CompareMetrics(expectedExportData, exportData))

			require.NoError(t, pmetrictest.CompareMetrics(pmetric.NewMetrics(), secondExportData), "the second export data should be empty")
		})
	}
}

func TestExponentialHistogramAggregation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	config := &Config{
		Interval:    time.Second,
		PassThrough: PassThrough{Gauge: false, Summary: false},
		AggregateToHistogram: []AggregateToHistogram{
			{
				MetricName:    "response.latency",
				OutputName:    "response.latency.distribution",
				HistogramType: HistogramTypeExponential,
				MaxSize:       160,
			},
		},
	}

	next := &consumertest.MetricsSink{}

	factory := NewFactory()
	mgp, err := factory.CreateMetrics(
		t.Context(),
		processortest.NewNopSettings(metadata.Type),
		config,
		next,
	)
	require.NoError(t, err)

	// Create input metrics with gauge values
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.SetSchemaUrl("https://test-res-schema.com/schema")
	rm.Resource().Attributes().PutStr("service.name", "test-service")

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("https://test-scope-schema.com/schema")
	sm.Scope().SetName("MyTestInstrument")
	sm.Scope().SetVersion("1.2.3")

	m := sm.Metrics().AppendEmpty()
	m.SetName("response.latency")
	m.SetUnit("ms")
	m.SetDescription("Response latency in milliseconds")

	gauge := m.SetEmptyGauge()

	// Input values: 5.5, 15.2, 25.8, 150.0, 8.3
	inputValues := []float64{5.5, 15.2, 25.8, 150.0, 8.3}
	expectedSum := 0.0
	expectedMin := inputValues[0]
	expectedMax := inputValues[0]

	for i, v := range inputValues {
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.Timestamp(10 + i*10))
		dp.SetDoubleValue(v)
		dp.Attributes().PutStr("endpoint", "/api/v1")

		expectedSum += v
		if v < expectedMin {
			expectedMin = v
		}
		if v > expectedMax {
			expectedMax = v
		}
	}

	// Test that ConsumeMetrics works
	err = mgp.ConsumeMetrics(ctx, md)
	require.NoError(t, err)

	require.IsType(t, &intervalProcessor{}, mgp)
	processor := mgp.(*intervalProcessor)

	// Pretend we hit the interval timer and call export
	processor.exportMetrics()

	// All the lookup tables should now be empty
	require.Empty(t, processor.histogramAggregationLookup)

	// Get the exported metrics
	allMetrics := next.AllMetrics()
	require.Len(t, allMetrics, 2)

	// First is passthrough (empty since gauge was captured)
	// Second is exported data
	exportData := allMetrics[1]

	// Validate the exponential histogram
	require.Equal(t, 1, exportData.ResourceMetrics().Len())
	exportRM := exportData.ResourceMetrics().At(0)
	require.Equal(t, 1, exportRM.ScopeMetrics().Len())
	exportSM := exportRM.ScopeMetrics().At(0)
	require.Equal(t, 1, exportSM.Metrics().Len())

	exportMetric := exportSM.Metrics().At(0)
	assert.Equal(t, "response.latency.distribution", exportMetric.Name())
	assert.Equal(t, "ms", exportMetric.Unit())
	assert.Equal(t, "Response latency in milliseconds", exportMetric.Description())
	assert.Equal(t, pmetric.MetricTypeExponentialHistogram, exportMetric.Type())

	expHist := exportMetric.ExponentialHistogram()
	assert.Equal(t, pmetric.AggregationTemporalityDelta, expHist.AggregationTemporality())
	require.Equal(t, 1, expHist.DataPoints().Len())

	dp := expHist.DataPoints().At(0)
	assert.Equal(t, uint64(len(inputValues)), dp.Count())
	assert.InDelta(t, expectedSum, dp.Sum(), 0.001)
	assert.InDelta(t, expectedMin, dp.Min(), 0.001)
	assert.InDelta(t, expectedMax, dp.Max(), 0.001)

	// Verify attributes are preserved
	endpointVal, ok := dp.Attributes().Get("endpoint")
	assert.True(t, ok)
	assert.Equal(t, "/api/v1", endpointVal.Str())

	// Verify bucket structure is valid (positive buckets since all values are positive)
	assert.Positive(t, dp.Positive().BucketCounts().Len())
	// Scale should be set
	assert.NotEqual(t, int32(0), dp.Scale())
}

func TestExponentialHistogramWithNegativeValues(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	config := &Config{
		Interval:    time.Second,
		PassThrough: PassThrough{Gauge: false, Summary: false},
		AggregateToHistogram: []AggregateToHistogram{
			{
				MetricName:    "temperature",
				HistogramType: HistogramTypeExponential,
			},
		},
	}

	next := &consumertest.MetricsSink{}

	factory := NewFactory()
	mgp, err := factory.CreateMetrics(
		t.Context(),
		processortest.NewNopSettings(metadata.Type),
		config,
		next,
	)
	require.NoError(t, err)

	// Create input metrics with gauge values including negative values
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("temperature")
	m.SetUnit("celsius")

	gauge := m.SetEmptyGauge()

	// Input values include negative numbers
	inputValues := []float64{-10.5, -5.2, 0, 15.8, 25.0}
	expectedSum := 0.0
	expectedMin := math.MaxFloat64
	expectedMax := -math.MaxFloat64

	for i, v := range inputValues {
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.Timestamp(10 + i*10))
		dp.SetDoubleValue(v)
		dp.Attributes().PutStr("location", "office")

		expectedSum += v
		if v < expectedMin {
			expectedMin = v
		}
		if v > expectedMax {
			expectedMax = v
		}
	}

	err = mgp.ConsumeMetrics(ctx, md)
	require.NoError(t, err)

	processor := mgp.(*intervalProcessor)
	processor.exportMetrics()

	allMetrics := next.AllMetrics()
	require.Len(t, allMetrics, 2)

	exportData := allMetrics[1]
	exportMetric := exportData.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)

	assert.Equal(t, "temperature_histogram", exportMetric.Name())
	assert.Equal(t, pmetric.MetricTypeExponentialHistogram, exportMetric.Type())

	expHist := exportMetric.ExponentialHistogram()
	dp := expHist.DataPoints().At(0)

	assert.Equal(t, uint64(len(inputValues)), dp.Count())
	assert.InDelta(t, expectedSum, dp.Sum(), 0.001)
	assert.InDelta(t, expectedMin, dp.Min(), 0.001)
	assert.InDelta(t, expectedMax, dp.Max(), 0.001)

	// Should have both positive and negative buckets
	assert.Positive(t, dp.Positive().BucketCounts().Len(), "should have positive buckets")
	assert.Positive(t, dp.Negative().BucketCounts().Len(), "should have negative buckets")
	// Zero count should be 1 (for the 0 value)
	assert.Equal(t, uint64(1), dp.ZeroCount())
}
