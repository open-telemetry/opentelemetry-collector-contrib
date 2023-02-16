// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
)

func truncate(f float64) float64 {
	bf := big.NewFloat(0).SetPrec(1000).SetFloat64(f)
	bu := big.NewFloat(0).SetPrec(1000).SetFloat64(0.001)

	bf.Quo(bf, bu)

	// Truncate:
	i := big.NewInt(0)
	bf.Int(i)
	bf.SetInt(i)

	f, _ = bf.Mul(bf, bu).Float64()
	return f
}

// this fixes the float values for clean comparison
func truncateHistogramValues(histogramMetric pmetric.Metric) {
	histogramObj := histogramMetric.Histogram()
	histogram := histogramObj.DataPoints().At(0)
	histogram.SetMax(truncate(histogram.Max()))
	histogram.SetMin(truncate(histogram.Min()))
	histogram.SetSum(truncate(histogram.Sum()))
	for i := 0; i < histogram.ExplicitBounds().Len(); i++ {
		histogram.ExplicitBounds().SetAt(i, truncate(histogram.ExplicitBounds().At(i)))
	}
}
func Test_convertGaugeToHistogram(t *testing.T) {

	floatGaugeInput := pmetric.NewMetric()
	floatGaugeInput.SetEmptyGauge()
	floatDataPoints := []float64{9.532, 4.3248, 2.329, 7.24, 5.548, 1.762, 9.570}
	for _, dp := range floatDataPoints {
		floatGaugeInput.Gauge().DataPoints().AppendEmpty().SetDoubleValue(dp)
	}

	integerGaugeInput := pmetric.NewMetric()
	integerGaugeInput.SetEmptyGauge()
	integerDataPoints := []int64{3, 4, 1, 7, 6, 8, 9}
	for _, dp := range integerDataPoints {
		integerGaugeInput.Gauge().DataPoints().AppendEmpty().SetIntValue(dp)
	}

	singleGaugeInput := pmetric.NewMetric()
	singleGaugeInput.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(5.2)

	tests := []struct {
		name          string
		stringAggTemp string
		bucketCount   int64
		lowerBound    float64
		upperBound    float64
		input         pmetric.Metric
		want          func(pmetric.Metric)
	}{
		{
			name:          "convert gauge with float datapoints to histogram, no bounds",
			stringAggTemp: "delta",
			bucketCount:   4,
			lowerBound:    0.0,
			upperBound:    0.0,
			input:         floatGaugeInput,
			want: func(metric pmetric.Metric) {
				histogram := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				histogram.SetMin(1.762)
				histogram.SetMax(9.570)
				histogram.SetSum(40.3058)
				histogram.SetCount(7)
				/*
					scale: 1.952
						2		2		1		2
					|--------|-------|-------|------|
					1.762    3.714   5.666   7.618  9.570
				*/
				histogram.ExplicitBounds().FromRaw([]float64{3.714, 5.666, 7.618})
				histogram.BucketCounts().FromRaw([]uint64{2, 2, 1, 2})
			},
		},
		{
			name:          "convert gauge with float datapoints to histogram, with bounds",
			stringAggTemp: "delta",
			bucketCount:   4,
			lowerBound:    0.0,
			upperBound:    10.0,
			input:         floatGaugeInput,
			want: func(metric pmetric.Metric) {
				histogram := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				histogram.SetMin(1.762)
				histogram.SetMax(9.570)
				histogram.SetSum(40.3058)
				histogram.SetCount(7)
				/*
					scale: 2.5
						2		1		2		2
					|--------|-------|-------|------|
					0        2.5     5       7.5    10
				*/
				histogram.ExplicitBounds().FromRaw([]float64{2.5, 5, 7.5})
				histogram.BucketCounts().FromRaw([]uint64{2, 1, 2, 2})
			},
		},
		{
			name:          "convert gauge with int datapoints to histogram, no bounds",
			stringAggTemp: "delta",
			bucketCount:   4,
			lowerBound:    0.0,
			upperBound:    0.0,
			input:         integerGaugeInput,
			want: func(metric pmetric.Metric) {
				histogram := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				histogram.SetMin(1)
				histogram.SetMax(9)
				histogram.SetSum(38)
				histogram.SetCount(7)
				/*
					scale: 1.952
						2		1		2		2
					|--------|-------|-------|------|
					1    	 3   	 5   	 7  	9
				*/
				histogram.ExplicitBounds().FromRaw([]float64{3, 5, 7})
				histogram.BucketCounts().FromRaw([]uint64{2, 1, 2, 2})
			},
		},
		{
			name:          "convert gauge with int datapoints to histogram, with bounds",
			stringAggTemp: "delta",
			bucketCount:   4,
			lowerBound:    0.0,
			upperBound:    10.0,
			input:         integerGaugeInput,
			want: func(metric pmetric.Metric) {
				histogram := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				histogram.SetMin(1)
				histogram.SetMax(9)
				histogram.SetSum(38)
				histogram.SetCount(7)
				/*
					scale: 2.5
						1		2		2		2
					|--------|-------|-------|------|
					0    	 2.5   	 5   	 7.5  	10
				*/
				histogram.ExplicitBounds().FromRaw([]float64{2.5, 5, 7.5})
				histogram.BucketCounts().FromRaw([]uint64{1, 2, 2, 2})
			},
		},
		{
			name:          "convert gauge with single datapoint to histogram, no bounds",
			stringAggTemp: "delta",
			bucketCount:   4,
			lowerBound:    0.0,
			upperBound:    0.0,
			input:         singleGaugeInput,
			want: func(metric pmetric.Metric) {
				histogram := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				histogram.SetMin(5.2)
				histogram.SetMax(5.2)
				histogram.SetSum(5.2)
				histogram.SetCount(1)
				/*
					scale: 1.952
						1		2		1		3
					|--------|-------|-------|------|
					1    	 3   	 5   	 7  	9
				*/
			},
		},
		{
			name:          "convert gauge with single datapoint to histogram, with bounds",
			stringAggTemp: "delta",
			bucketCount:   4,
			lowerBound:    0.0,
			upperBound:    10.0,
			input:         singleGaugeInput,
			want: func(metric pmetric.Metric) {
				histogram := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				histogram.SetMin(5.2)
				histogram.SetMax(5.2)
				histogram.SetSum(5.2)
				histogram.SetCount(1)
				/*
					scale: 1.952
						1		2		1		3
					|--------|-------|-------|------|
					1    	 3   	 5   	 7  	9
				*/
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := pmetric.NewMetric()
			tt.input.CopyTo(metric)

			ctx := ottldatapoint.NewTransformContext(pmetric.NewNumberDataPoint(), metric, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource())

			exprFunc, _ := convertGaugeToHistogram(tt.stringAggTemp, tt.bucketCount, tt.lowerBound, tt.upperBound)

			_, err := exprFunc(nil, ctx)
			assert.Nil(t, err)

			expected := pmetric.NewMetric()
			tt.want(expected)

			truncateHistogramValues(metric)
			truncateHistogramValues(expected)

			assert.Equal(t, expected, metric)
		})
	}
}

func Test_convertGaugeToHistogram_noop(t *testing.T) {
	sumInput := pmetric.NewMetric()
	sumInput.SetEmptySum()

	histogramInput := pmetric.NewMetric()
	histogramInput.SetEmptyHistogram()

	expoHistogramInput := pmetric.NewMetric()
	expoHistogramInput.SetEmptyHistogram()

	summaryInput := pmetric.NewMetric()
	summaryInput.SetEmptySummary()

	emptyGaugeInput := pmetric.NewMetric()
	emptyGaugeInput.SetEmptyGauge()

	tests := []struct {
		name          string
		stringAggTemp string
		bucketCount   int64
		lowerBound    float64
		upperBound    float64
		input         pmetric.Metric
		want          func(pmetric.Metric)
	}{
		{
			name:          "noop for sum",
			stringAggTemp: "delta",
			bucketCount:   5,
			lowerBound:    0.0,
			upperBound:    0.0,
			input:         sumInput,
			want: func(metric pmetric.Metric) {
				sumInput.CopyTo(metric)
			},
		},
		{
			name:          "noop for histogram",
			stringAggTemp: "delta",
			bucketCount:   5,
			lowerBound:    0.0,
			upperBound:    0.0,
			input:         histogramInput,
			want: func(metric pmetric.Metric) {
				histogramInput.CopyTo(metric)
			},
		},
		{
			name:          "noop for exponential histogram",
			stringAggTemp: "delta",
			bucketCount:   5,
			lowerBound:    0.0,
			upperBound:    0.0,
			input:         expoHistogramInput,
			want: func(metric pmetric.Metric) {
				expoHistogramInput.CopyTo(metric)
			},
		},
		{
			name:          "noop for summary",
			stringAggTemp: "delta",
			bucketCount:   5,
			lowerBound:    0.0,
			upperBound:    0.0,
			input:         summaryInput,
			want: func(metric pmetric.Metric) {
				summaryInput.CopyTo(metric)
			},
		},
		{
			name:          "convert gauge with no datapoints to histogram",
			stringAggTemp: "delta",
			bucketCount:   4,
			lowerBound:    0.0,
			upperBound:    0.0,
			input:         emptyGaugeInput,
			want: func(metric pmetric.Metric) {
				metric.SetEmptyHistogram()
				metric.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				/*
					scale: 1.952
						1		2		1		3
					|--------|-------|-------|------|
					1    	 3   	 5   	 7  	9
				*/
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := pmetric.NewMetric()
			tt.input.CopyTo(metric)

			ctx := ottldatapoint.NewTransformContext(pmetric.NewNumberDataPoint(), metric, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource())

			exprFunc, _ := convertGaugeToHistogram(tt.stringAggTemp, tt.bucketCount, tt.lowerBound, tt.upperBound)

			_, err := exprFunc(nil, ctx)
			assert.Nil(t, err)

			expected := pmetric.NewMetric()
			tt.want(expected)
			assert.Equal(t, expected, metric)
		})
	}
}

func Test_convertGaugeToHistogram_validation(t *testing.T) {
	tests := []struct {
		name          string
		stringAggTemp string
		bucketCount   int64
		lowerBound    float64
		upperBound    float64
		wantErrString string
	}{
		{
			name:          "invalid aggregation temporality",
			stringAggTemp: "not a real aggregation temporality",
			bucketCount:   2,
			lowerBound:    0.0,
			upperBound:    0.0,
			wantErrString: "unknown aggregation temporality: not a real aggregation temporality",
		},
		{
			name:          "negative bucket value",
			stringAggTemp: "delta",
			bucketCount:   -5,
			lowerBound:    0.0,
			upperBound:    0.0,
			wantErrString: "Bucket value must be greater than 0. Current value: -5",
		},
		{
			name:          "zero bucket value",
			stringAggTemp: "delta",
			bucketCount:   0,
			lowerBound:    0.0,
			upperBound:    0.0,
			wantErrString: "Bucket value must be greater than 0. Current value: 0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := convertGaugeToHistogram(tt.stringAggTemp, tt.bucketCount, tt.lowerBound, tt.upperBound)
			assert.Error(t, err, tt.wantErrString)
		})
	}
}
