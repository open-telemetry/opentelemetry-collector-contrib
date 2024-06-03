package ottlfuncs

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"testing"
)

func TestScale(t *testing.T) {
	type args struct {
		value      ottl.Getter[any]
		multiplier float64
	}
	type testCase struct {
		name     string
		args     args
		wantFunc func() any
		wantErr  bool
	}
	tests := []testCase{
		{
			name: "scale float value",
			args: args{
				value: &ottl.StandardGetSetter[any]{
					Getter: func(ctx context.Context, tCtx any) (any, error) {
						return 1.05, nil
					},
				},
				multiplier: 10.0,
			},
			wantFunc: func() any {
				return 10.5
			},
			wantErr: false,
		},
		{
			name: "scale int value",
			args: args{
				value: &ottl.StandardGetSetter[any]{
					Getter: func(ctx context.Context, tCtx any) (any, error) {
						return int64(1), nil
					},
				},
				multiplier: 10.0,
			},
			wantFunc: func() any {
				return float64(10)
			},
			wantErr: false,
		},
		{
			name: "unsupported data type",
			args: args{
				value: &ottl.StandardGetSetter[any]{
					Getter: func(ctx context.Context, tCtx any) (any, error) {
						return "foo", nil
					},
				},
				multiplier: 10.0,
			},
			wantFunc: func() any {
				return nil
			},
			wantErr: true,
		},
		{
			name: "scale gauge float metric",
			args: args{
				value: &ottl.StandardGetSetter[any]{
					Getter: func(ctx context.Context, tCtx any) (any, error) {
						metric := pmetric.NewMetric()
						metric.SetName("test-metric")
						metric.SetEmptyGauge()
						metric.Gauge().DataPoints().AppendEmpty().SetDoubleValue(10.0)

						return metric.Gauge().DataPoints(), nil
					},
				},
				multiplier: 10.0,
			},
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
			args: args{
				value: &ottl.StandardGetSetter[any]{
					Getter: func(ctx context.Context, tCtx any) (any, error) {
						metric := pmetric.NewMetric()
						metric.SetName("test-metric")
						metric.SetEmptyGauge()
						metric.Gauge().DataPoints().AppendEmpty().SetIntValue(10)

						return metric.Gauge().DataPoints(), nil
					},
				},
				multiplier: 10.0,
			},
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
			args: args{
				value: &ottl.StandardGetSetter[any]{
					Getter: func(ctx context.Context, tCtx any) (any, error) {
						metric := pmetric.NewMetric()
						metric.SetName("test-metric")
						metric.SetEmptySum()
						metric.Sum().DataPoints().AppendEmpty().SetDoubleValue(10.0)

						return metric.Sum().DataPoints(), nil
					},
				},
				multiplier: 10.0,
			},
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
			args: args{
				value: &ottl.StandardGetSetter[any]{
					Getter: func(ctx context.Context, tCtx any) (any, error) {
						metric := getTestHistogramMetric(1, 4, 1, 3, []float64{1, 10}, []uint64{1, 2}, []float64{1.0}, 1, 1)
						return metric.Histogram().DataPoints(), nil

					},
				},
				multiplier: 10.0,
			},
			wantFunc: func() any {
				metric := getTestHistogramMetric(1, 40, 10, 30, []float64{10, 100}, []uint64{1, 2}, []float64{10.0}, 1, 1)
				return metric.Histogram().DataPoints()
			},
			wantErr: false,
		},
		{
			name: "scale SummaryDataPointValueAtQuantileSlice",
			args: args{
				value: &ottl.StandardGetSetter[any]{
					Getter: func(ctx context.Context, tCtx any) (any, error) {
						metric := pmetric.NewSummaryDataPointValueAtQuantileSlice()
						metric.AppendEmpty().SetValue(1.0)
						return metric, nil

					},
				},
				multiplier: 10.0,
			},
			wantFunc: func() any {
				metric := pmetric.NewSummaryDataPointValueAtQuantileSlice()
				metric.AppendEmpty().SetValue(10.0)
				return metric
			},
			wantErr: false,
		},
		{
			name: "scale ExemplarSlice",
			args: args{
				value: &ottl.StandardGetSetter[any]{
					Getter: func(ctx context.Context, tCtx any) (any, error) {
						metric := pmetric.NewExemplarSlice()
						metric.AppendEmpty().SetDoubleValue(1.0)
						return metric, nil
					},
				},
				multiplier: 10.0,
			},
			wantFunc: func() any {
				metric := pmetric.NewExemplarSlice()
				metric.AppendEmpty().SetDoubleValue(10.0)
				return metric
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressionFunc, _ := Scale(tt.args.value, tt.args.multiplier)
			got, err := expressionFunc(context.Background(), tt.args)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantFunc(), got)
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
