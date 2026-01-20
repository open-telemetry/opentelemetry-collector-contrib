// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

type histogramConfig struct {
	name           string
	count          uint64
	min            ottl.Optional[float64]
	max            ottl.Optional[float64]
	bucketCounts   []uint64
	explicitBounds []float64
	startTime      ottl.Optional[uint64]
	timestamp      ottl.Optional[uint64]
}

func getConfiguredHistogramMetric(config histogramConfig) pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyHistogram()

	if config.name != "" {
		metricInput.SetName(config.name)
	} else {
		metricInput.SetName("histogram_metric")
	}

	input := metricInput.Histogram().DataPoints().AppendEmpty()
	input.SetCount(config.count)

	if !config.min.IsEmpty() {
		input.SetMin(config.min.Get())
	}

	if !config.max.IsEmpty() {
		input.SetMax(config.max.Get())
	}

	if len(config.bucketCounts) > 0 {
		input.BucketCounts().FromRaw(config.bucketCounts)
	}

	if len(config.explicitBounds) > 0 {
		input.ExplicitBounds().FromRaw(config.explicitBounds)
	}

	if !config.startTime.IsEmpty() {
		input.SetStartTimestamp(pcommon.Timestamp(config.startTime.Get()))
	}

	if !config.timestamp.IsEmpty() {
		input.SetTimestamp(pcommon.Timestamp(config.timestamp.Get()))
	}

	attrs := getTestAttributes()
	attrs.CopyTo(input.Attributes())

	return metricInput
}

func Test_createExtractPercentileMetricFunction(t *testing.T) {
	tests := []struct {
		name string
		args any
		want string
	}{
		{
			name: "wrong args type",
			args: &struct{}{},
			want: "extractPercentileMetricFactory args must be of type *extractPercentileMetricArguments",
		},
		{
			name: "negative percentile",
			args: &extractPercentileMetricArguments{
				Percentile: -1.0,
			},
			want: "percentile must be between 0 and 100, got -1.000000",
		},
		{
			name: "percentile above 100",
			args: &extractPercentileMetricArguments{
				Percentile: 101.0,
			},
			want: "percentile must be between 0 and 100, got 101.000000",
		},
		{
			name: "valid percentile",
			args: &extractPercentileMetricArguments{
				Percentile: 95.0,
			},
		},
		{
			name: "valid percentile with suffix",
			args: &extractPercentileMetricArguments{
				Percentile: 90,
				Suffix:     ottl.NewTestingOptional("_custom"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := createExtractPercentileMetricFunction(ottl.FunctionContext{}, tt.args)
			if tt.want == "" {
				require.NoError(t, err)
				require.NotNil(t, exprFunc)
				return
			}
			assert.EqualError(t, err, tt.want)
			require.Nil(t, exprFunc)
		})
	}
}

func Test_extractPercentileMetric_Histogram(t *testing.T) {
	tests := []struct {
		name            string
		histogramConfig histogramConfig
		percentile      float64
		suffix          ottl.Optional[string]
		wantSuffix      string
		wantValue       float64
	}{
		{
			name: "p50 in first bucket",
			histogramConfig: histogramConfig{
				name:           "response_time",
				count:          100,
				bucketCounts:   []uint64{60, 40}, // p50 falls in first bucket [0, 1.0]
				explicitBounds: []float64{1.0},
			},
			percentile: 50.0,
			wantSuffix: "_p50",
			wantValue:  0.8333333333333334,
		},
		{
			name: "p99 in last bucket",
			histogramConfig: histogramConfig{
				name:           "latency",
				count:          115,
				bucketCounts:   []uint64{50, 35, 50}, // p99 falls in last bucket (5.0, +Inf)]
				explicitBounds: []float64{1.0, 5.0},
			},
			percentile: 99.0,
			wantSuffix: "_p99",
			wantValue:  5.0, // max is not set, so use lower bound of last bucket
		},
		{
			name: "p95 in third bucket",
			histogramConfig: histogramConfig{
				name:           "latency",
				count:          115,
				bucketCounts:   []uint64{50, 35, 50, 15}, // p95 falls in third bucket (3.0, 5.0]
				explicitBounds: []float64{1.0, 3.0, 5.0},
			},
			percentile: 95.0,
			wantSuffix: "_p95",
			wantValue:  4.0,
		},
		{
			name: "p99.5 with custom suffix w/o max value set",
			histogramConfig: histogramConfig{
				name:           "request_duration",
				count:          1000,
				bucketCounts:   []uint64{100, 400, 400, 95, 5},
				explicitBounds: []float64{0.1, 0.5, 1.0, 5.0},
			},
			percentile: 99.5,
			suffix:     ottl.NewTestingOptional("_percentile_995"),
			wantSuffix: "_percentile_995",
			wantValue:  5.0, // lower bound of last bucket
		},
		{
			name: "p99 in last bucket with max",
			histogramConfig: histogramConfig{
				name:           "response_size",
				count:          100,
				bucketCounts:   []uint64{25, 50, 25},
				explicitBounds: []float64{100.0, 500.0},
				max:            ottl.NewTestingOptional(750.0),
			},
			percentile: 99.0,
			wantSuffix: "_p99",
			wantValue:  740.0, // interpolated value with max set [500, 750]
		},
		{
			name: "single bucket histogram p75",
			histogramConfig: histogramConfig{
				name:           "simple",
				count:          40,
				bucketCounts:   []uint64{40},
				explicitBounds: []float64{100.0},
			},
			percentile: 75.0,
			wantSuffix: "_p75",
			wantValue:  75.0,
		},
		{
			name: "many buckets p90",
			histogramConfig: histogramConfig{
				name:           "detailed",
				count:          1000,
				bucketCounts:   []uint64{50, 100, 200, 300, 200, 100, 50},
				explicitBounds: []float64{10, 25, 50, 100, 250, 500},
			},
			percentile: 90.0,
			wantSuffix: "_p90",
			wantValue:  375.0,
		},
		{
			name: "p50 with timestamps",
			histogramConfig: histogramConfig{
				name:           "timed_metric",
				count:          100,
				bucketCounts:   []uint64{40, 60},
				explicitBounds: []float64{10.0},
				startTime:      ottl.NewTestingOptional[uint64](1000000000),
				timestamp:      ottl.NewTestingOptional[uint64](2000000000),
			},
			percentile: 50.0,
			wantSuffix: "_p50",
			wantValue:  10.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := getConfiguredHistogramMetric(tt.histogramConfig)

			exprFunc, err := extractPercentileMetric(tt.percentile, tt.suffix)
			require.NoError(t, err)

			scopeMetrics := pmetric.NewScopeMetrics()
			metric.CopyTo(scopeMetrics.Metrics().AppendEmpty())

			tCtx := ottlmetric.NewTransformContextPtr(pmetric.NewResourceMetrics(), scopeMetrics, metric)

			_, err = exprFunc(context.Background(), tCtx)
			require.NoError(t, err)

			validatePercentileMetric(t, tCtx, metric, tt.wantSuffix, tt.wantValue)
		})
	}
}

type exponentialHistogramConfig struct {
	name                 string
	scale                int32
	count                uint64
	zeroCount            uint64
	zeroThreshold        float64
	positiveBucketCounts []uint64
	negativeBucketCounts []uint64
	startTime            ottl.Optional[uint64]
	timestamp            ottl.Optional[uint64]
}

func getConfiguredExponentialHistogramMetric(config exponentialHistogramConfig) pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyExponentialHistogram()

	if config.name != "" {
		metricInput.SetName(config.name)
	} else {
		metricInput.SetName("exponential_histogram_metric")
	}

	input := metricInput.ExponentialHistogram().DataPoints().AppendEmpty()
	input.SetScale(config.scale)
	input.SetCount(config.count)

	input.SetZeroCount(config.zeroCount)
	if config.zeroThreshold > 0 {
		input.SetZeroThreshold(config.zeroThreshold)
	}

	if len(config.positiveBucketCounts) > 0 {
		input.Positive().BucketCounts().FromRaw(config.positiveBucketCounts)
	}

	if len(config.negativeBucketCounts) > 0 {
		input.Negative().BucketCounts().FromRaw(config.negativeBucketCounts)
	}

	if !config.startTime.IsEmpty() {
		input.SetStartTimestamp(pcommon.Timestamp(config.startTime.Get()))
	}

	if !config.timestamp.IsEmpty() {
		input.SetTimestamp(pcommon.Timestamp(config.timestamp.Get()))
	}

	attrs := getTestAttributes()
	attrs.CopyTo(input.Attributes())

	return metricInput
}

func Test_extractPercentileMetric_ExponentialHistogram(t *testing.T) {
	tests := []struct {
		name                       string
		exponentialHistogramConfig exponentialHistogramConfig
		percentile                 float64
		suffix                     ottl.Optional[string]
		wantSuffix                 string
		wantValue                  float64
	}{
		{
			name: "p50 with scale 0",
			exponentialHistogramConfig: exponentialHistogramConfig{
				name:                 "latency_exp",
				scale:                0,
				count:                100,
				positiveBucketCounts: []uint64{30, 40, 30},
			},
			percentile: 50.0,
			suffix:     ottl.NewTestingOptional("_median"),
			wantSuffix: "_median",
			wantValue:  2.82842712474619, // scale 0, bucket 1: [2, 4]
		},
		{
			name: "p95 with scale 2",
			exponentialHistogramConfig: exponentialHistogramConfig{
				name:                 "response_time",
				scale:                2,
				count:                210,
				positiveBucketCounts: []uint64{20, 30, 40, 50, 30, 35, 5},
			},
			percentile: 95.0,
			wantSuffix: "_p95",
			wantValue:  2.7592682406908096, // scale 2, bucket 5: [2.378, 2.828]
		},
		{
			name: "p99 with scale -1",
			exponentialHistogramConfig: exponentialHistogramConfig{
				name:                 "coarse_metric",
				scale:                -1,
				count:                500,
				positiveBucketCounts: []uint64{100, 200, 150, 40, 10},
			},
			percentile: 99.0,
			wantSuffix: "_p99",
			wantValue:  512.0, // scale -1, bucket 4: [256, 1024]
		},
		{
			name: "p75 with zero count",
			exponentialHistogramConfig: exponentialHistogramConfig{
				name:                 "with_zeros",
				scale:                1,
				count:                120,
				zeroCount:            20,
				positiveBucketCounts: []uint64{40, 60},
			},
			percentile: 75.0,
			wantSuffix: "_p75",
			wantValue:  1.6817928305074292, // scale 1, bucket 1: [1.0, 2.0]
		},
		{
			name: "p50 in negative buckets",
			exponentialHistogramConfig: exponentialHistogramConfig{
				name:                 "negative_heavy",
				scale:                2,
				count:                95,
				negativeBucketCounts: []uint64{20, 30, 20},
				zeroCount:            10,
				positiveBucketCounts: []uint64{15},
			},
			percentile: 50.0,
			wantSuffix: "_p50",
			wantValue:  -1.2030250360821166, // scale 2, bucket -1: [-1.1892, -1.4142]
		},
		{
			name: "p25 in negative buckets",
			exponentialHistogramConfig: exponentialHistogramConfig{
				name:                 "mostly_negative",
				scale:                0,
				count:                100,
				negativeBucketCounts: []uint64{15, 35, 20, 10},
				zeroCount:            5,
				positiveBucketCounts: []uint64{15},
			},
			percentile: 25.0,
			wantSuffix: "_p25",
			wantValue:  -3.2813414240305523, // scale 0, bucket 1: [-2.0, -4.0]
		},
		{
			name: "p99 with timestamps",
			exponentialHistogramConfig: exponentialHistogramConfig{
				name:                 "timed_exponential",
				scale:                1,
				count:                200,
				positiveBucketCounts: []uint64{100, 80, 20},
				startTime:            ottl.NewTestingOptional[uint64](1500000000),
				timestamp:            ottl.NewTestingOptional[uint64](2500000000),
			},
			percentile: 99.0,
			wantSuffix: "_p99",
			wantValue:  2.7320805135087904, // scale 1, bucket 2: [2.0, 2.828]
		},
		{
			name: "p50 falls in zero bucket with both negative and positive",
			exponentialHistogramConfig: exponentialHistogramConfig{
				name:                 "zero_heavy",
				scale:                0,
				count:                100,
				negativeBucketCounts: []uint64{10},
				zeroCount:            80,
				zeroThreshold:        1.0,
				positiveBucketCounts: []uint64{10},
			},
			percentile: 50.0,
			wantSuffix: "_p50",
			wantValue:  0, // percentile falls in zero bucket, interpolates around zero
		},
		{
			name: "p60 in zero bucket with only positive buckets",
			exponentialHistogramConfig: exponentialHistogramConfig{
				name:                 "zero_with_positive_only",
				scale:                0,
				count:                100,
				zeroCount:            80,
				zeroThreshold:        1.0,
				positiveBucketCounts: []uint64{20},
			},
			percentile: 60.0,
			wantSuffix: "_p60",
			wantValue:  0.75, // positive zero bucket [0, 1]
		},
		{
			name: "p30 in zero bucket with only negative buckets",
			exponentialHistogramConfig: exponentialHistogramConfig{
				name:                 "zero_with_negative_only",
				scale:                0,
				count:                100,
				negativeBucketCounts: []uint64{20},
				zeroCount:            80,
				zeroThreshold:        1.0,
			},
			percentile: 30.0,
			wantSuffix: "_p30",
			wantValue:  -0.875, // negativezero bucket [-1, 0]
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := getConfiguredExponentialHistogramMetric(tt.exponentialHistogramConfig)

			exprFunc, err := extractPercentileMetric(tt.percentile, tt.suffix)
			require.NoError(t, err)

			scopeMetrics := pmetric.NewScopeMetrics()
			metric.CopyTo(scopeMetrics.Metrics().AppendEmpty())

			tCtx := ottlmetric.NewTransformContextPtr(pmetric.NewResourceMetrics(), scopeMetrics, metric)

			_, err = exprFunc(context.Background(), tCtx)
			require.NoError(t, err)

			validatePercentileMetric(t, tCtx, metric, tt.wantSuffix, tt.wantValue)
		})
	}
}

func validatePercentileMetric(t *testing.T, tCtx *ottlmetric.TransformContext, originalMetric pmetric.Metric, expectedSuffix string, expectedValue float64) {
	metrics := tCtx.GetMetrics()
	require.Equal(t, 2, metrics.Len(), "should have original + percentile metric")

	originalName := originalMetric.Name()
	assert.Equal(t, originalName, metrics.At(0).Name(), "original metric should be unchanged")

	percentileMetric := metrics.At(1)
	assert.Equal(t, originalName+expectedSuffix, percentileMetric.Name(), "percentile metric name should have suffix")
	assert.Equal(t, pmetric.MetricTypeGauge, percentileMetric.Type(), "percentile metric should be gauge type")
	require.Equal(t, 1, percentileMetric.Gauge().DataPoints().Len(), "gauge should have exactly one data point")

	gaugeDataPoint := percentileMetric.Gauge().DataPoints().At(0)
	assert.Equal(t, expectedValue, gaugeDataPoint.DoubleValue(), "percentile value should match expected")

	var originalDataPoint dataPoint[any]
	switch originalMetric.Type() {
	case pmetric.MetricTypeHistogram:
		originalDataPoint = originalMetric.Histogram().DataPoints().At(0)
	case pmetric.MetricTypeExponentialHistogram:
		originalDataPoint = originalMetric.ExponentialHistogram().DataPoints().At(0)
	}

	assert.Equal(t, originalDataPoint.Attributes().Len(), gaugeDataPoint.Attributes().Len(), "attributes should be copied")
	assert.Equal(t, originalDataPoint.Timestamp(), gaugeDataPoint.Timestamp(), "timestamp should be copied")
}
