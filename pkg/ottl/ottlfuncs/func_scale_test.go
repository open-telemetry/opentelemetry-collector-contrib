// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestScale(t *testing.T) {
	type testCase struct {
		name       string
		multiplier float64
		valueFunc  func() any
		wantFunc   func() any
		wantErr    bool
	}
	tests := []testCase{
		{
			name: "unsupported data type",
			valueFunc: func() any {
				return "foo"
			},
			multiplier: 10.0,
			wantFunc: func() any {
				// value should not be modified
				return "foo"
			},
			wantErr: true,
		},
		{
			name: "scale gauge float metric",
			valueFunc: func() any {
				metric := pmetric.NewMetric()
				metric.SetName("test-metric")
				metric.SetEmptyGauge()
				metric.Gauge().DataPoints().AppendEmpty().SetDoubleValue(10.0)

				return metric.Gauge().DataPoints()
			},
			multiplier: 10.0,
			wantFunc: func() any {
				metric := pmetric.NewMetric()
				metric.SetName("test-metric")
				metric.SetEmptyGauge()
				metric.Gauge().DataPoints().AppendEmpty().SetDoubleValue(100.0)

				return metric.Gauge().DataPoints()
			},
			wantErr: false,
		},
		{
			name: "scale gauge int metric",
			valueFunc: func() any {
				metric := pmetric.NewMetric()
				metric.SetName("test-metric")
				metric.SetEmptyGauge()
				metric.Gauge().DataPoints().AppendEmpty().SetIntValue(10)

				return metric.Gauge().DataPoints()
			},
			multiplier: 10.0,
			wantFunc: func() any {
				metric := pmetric.NewMetric()
				metric.SetName("test-metric")
				metric.SetEmptyGauge()
				metric.Gauge().DataPoints().AppendEmpty().SetIntValue(100.0)

				return metric.Gauge().DataPoints()
			},
			wantErr: false,
		},
		{
			name: "scale sum metric",
			valueFunc: func() any {
				metric := pmetric.NewMetric()
				metric.SetName("test-metric")
				metric.SetEmptySum()
				metric.Sum().DataPoints().AppendEmpty().SetDoubleValue(10.0)

				return metric.Sum().DataPoints()
			},
			multiplier: 10.0,
			wantFunc: func() any {
				metric := pmetric.NewMetric()
				metric.SetName("test-metric")
				metric.SetEmptySum()
				metric.Sum().DataPoints().AppendEmpty().SetDoubleValue(100.0)

				return metric.Sum().DataPoints()
			},
			wantErr: false,
		},
		{
			name: "scale histogram metric",
			valueFunc: func() any {
				metric := getTestHistogramMetric(1, 4, 1, 3, []float64{1, 10}, []uint64{1, 2}, []float64{1.0}, 1, 1)
				return metric.Histogram().DataPoints()
			},
			multiplier: 10.0,
			wantFunc: func() any {
				metric := getTestHistogramMetric(1, 40, 10, 30, []float64{10, 100}, []uint64{1, 2}, []float64{10.0}, 1, 1)
				return metric.Histogram().DataPoints()
			},
			wantErr: false,
		},
		{
			name: "scale SummaryDataPointValueAtQuantileSlice",
			valueFunc: func() any {
				metric := pmetric.NewSummaryDataPointValueAtQuantileSlice()
				metric.AppendEmpty().SetValue(1.0)
				return metric
			},
			multiplier: 10.0,
			wantFunc: func() any {
				metric := pmetric.NewSummaryDataPointValueAtQuantileSlice()
				metric.AppendEmpty().SetValue(10.0)
				return metric
			},
			wantErr: false,
		},
		{
			name: "scale ExemplarSlice",
			valueFunc: func() any {
				metric := pmetric.NewExemplarSlice()
				metric.AppendEmpty().SetDoubleValue(1.0)
				return metric
			},
			multiplier: 10.0,
			wantFunc: func() any {
				metric := pmetric.NewExemplarSlice()
				metric.AppendEmpty().SetDoubleValue(10.0)
				return metric
			},
			wantErr: false,
		},
		{
			name: "unsupported: exponential histogram metric",
			valueFunc: func() any {
				return pmetric.NewExponentialHistogramDataPointSlice()
			},
			multiplier: 10.0,
			wantFunc: func() any {
				// value should not be modified
				return pmetric.NewExponentialHistogramDataPointSlice()
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			value := tt.valueFunc()
			target := &ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return value, nil
				},
				Setter: func(_ context.Context, _ any, val any) error {
					value = val
					return nil
				},
			}
			expressionFunc, _ := Scale[any](target, tt.multiplier)
			_, err := expressionFunc(context.Background(), target)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.EqualValues(t, tt.wantFunc(), value)
		})
	}
}

func getTestHistogramMetric(count uint64, sum, min, max float64, bounds []float64, bucketCounts []uint64, exemplars []float64, start, timestamp pcommon.Timestamp) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName("test-metric")
	metric.SetEmptyHistogram()
	histogramDatapoint := metric.Histogram().DataPoints().AppendEmpty()
	histogramDatapoint.SetCount(count)
	histogramDatapoint.SetSum(sum)
	histogramDatapoint.SetMin(min)
	histogramDatapoint.SetMax(max)
	histogramDatapoint.ExplicitBounds().FromRaw(bounds)
	histogramDatapoint.BucketCounts().FromRaw(bucketCounts)
	for i := 0; i < len(exemplars); i++ {
		exemplar := histogramDatapoint.Exemplars().AppendEmpty()
		exemplar.SetTimestamp(1)
		exemplar.SetDoubleValue(exemplars[i])
	}
	histogramDatapoint.SetStartTimestamp(start)
	histogramDatapoint.SetTimestamp(timestamp)
	return metric
}
