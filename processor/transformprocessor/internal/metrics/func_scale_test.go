// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

func TestScale(t *testing.T) {
	type testCase struct {
		name      string
		args      ScaleArguments
		valueFunc func() pmetric.Metric
		wantFunc  func() pmetric.Metric
		wantErr   bool
	}
	tests := []testCase{
		{
			name: "scale gauge float metric",
			valueFunc: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test-metric")
				metric.SetEmptyGauge()
				metric.Gauge().DataPoints().AppendEmpty().SetDoubleValue(10.0)

				return metric
			},
			args: ScaleArguments{
				Multiplier: 10.0,
				Unit: ottl.NewTestingOptional[ottl.StringGetter[ottlmetric.TransformContext]](ottl.StandardStringGetter[ottlmetric.TransformContext]{
					Getter: func(_ context.Context, _ ottlmetric.TransformContext) (any, error) {
						return "kWh", nil
					},
				}),
			},
			wantFunc: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test-metric")
				metric.SetEmptyGauge()
				metric.SetUnit("kWh")
				metric.Gauge().DataPoints().AppendEmpty().SetDoubleValue(100.0)

				return metric
			},
			wantErr: false,
		},
		{
			name: "scale gauge int metric",
			valueFunc: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test-metric")
				metric.SetEmptyGauge()
				metric.Gauge().DataPoints().AppendEmpty().SetIntValue(10)

				return metric
			},
			args: ScaleArguments{
				Multiplier: 10.0,
			},
			wantFunc: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test-metric")
				metric.SetEmptyGauge()
				metric.Gauge().DataPoints().AppendEmpty().SetIntValue(100.0)

				return metric
			},
			wantErr: false,
		},
		{
			name: "scale sum metric",
			valueFunc: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test-metric")
				metric.SetEmptySum()
				metric.Sum().DataPoints().AppendEmpty().SetDoubleValue(10.0)

				return metric
			},
			args: ScaleArguments{
				Multiplier: 10.0,
			},
			wantFunc: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test-metric")
				metric.SetEmptySum()
				metric.Sum().DataPoints().AppendEmpty().SetDoubleValue(100.0)

				return metric
			},
			wantErr: false,
		},
		{
			name: "scale histogram metric",
			valueFunc: func() pmetric.Metric {
				metric := getTestScalingHistogramMetric(1, 4, 1, 3, []float64{1, 10}, []uint64{1, 2}, []float64{1.0}, 1, 1)
				return metric
			},
			args: ScaleArguments{
				Multiplier: 10.0,
			},
			wantFunc: func() pmetric.Metric {
				metric := getTestScalingHistogramMetric(1, 40, 10, 30, []float64{10, 100}, []uint64{1, 2}, []float64{10.0}, 1, 1)
				return metric
			},
			wantErr: false,
		},
		{
			name: "scale summary metric",
			valueFunc: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				dp := metric.SetEmptySummary().DataPoints().AppendEmpty()
				dp.SetSum(10.0)
				qv := dp.QuantileValues().AppendEmpty()
				qv.SetValue(10.0)

				return metric
			},
			args: ScaleArguments{
				Multiplier: 10.0,
			},
			wantFunc: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				dp := metric.SetEmptySummary().DataPoints().AppendEmpty()
				dp.SetSum(100.0)
				qv := dp.QuantileValues().AppendEmpty()
				qv.SetValue(100.0)

				return metric
			},
			wantErr: false,
		},
		{
			name: "unsupported: exponential histogram metric",
			valueFunc: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetEmptyExponentialHistogram()
				return metric
			},
			args: ScaleArguments{
				Multiplier: 10.0,
			},
			wantFunc: func() pmetric.Metric {
				// value should not be modified
				metric := pmetric.NewMetric()
				metric.SetEmptyExponentialHistogram()
				return metric
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := ottlmetric.NewTransformContext(
				tt.valueFunc(),
				pmetric.NewMetricSlice(),
				pcommon.NewInstrumentationScope(),
				pcommon.NewResource(),
				pmetric.NewScopeMetrics(),
				pmetric.NewResourceMetrics(),
			)

			expressionFunc, _ := Scale(tt.args)
			_, err := expressionFunc(context.Background(), target)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantFunc(), target.GetMetric())
		})
	}
}

func getTestScalingHistogramMetric(count uint64, sum, minVal, maxVal float64, bounds []float64, bucketCounts []uint64, exemplars []float64, start, timestamp pcommon.Timestamp) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName("test-metric")
	metric.SetEmptyHistogram()
	histogramDatapoint := metric.Histogram().DataPoints().AppendEmpty()
	histogramDatapoint.SetCount(count)
	histogramDatapoint.SetSum(sum)
	histogramDatapoint.SetMin(minVal)
	histogramDatapoint.SetMax(maxVal)
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
