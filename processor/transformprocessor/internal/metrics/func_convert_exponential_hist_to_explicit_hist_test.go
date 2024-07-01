package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func Test_convert_exponential_hist_to_explicit_hist(t *testing.T) {
	exponentialHistInput := pmetric.NewMetric()
	exponentialHistInput.SetName("response_time")
	dp := exponentialHistInput.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
	exponentialHistInput.ExponentialHistogram().SetAggregationTemporality(1)
	dp.SetCount(2)
	dp.SetScale(7)
	dp.SetSum(361)
	dp.SetMax(195)
	dp.SetMin(166)

	ts := pcommon.NewTimestampFromTime(time.Now())
	dp.SetTimestamp(ts)

	// set attributes
	dp.Attributes().PutStr("metric_type", "timing")

	// set bucket counts
	dp.Positive().BucketCounts().Append(
		1,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		1)

	dp.Positive().SetOffset(944)
	nonExponentialHist := func() pmetric.Metric {
		m := pmetric.NewMetric()
		m.SetName("not-exponentialhist")
		m.SetEmptyGauge()
		return m
	}

	tests := []struct {
		name  string
		input pmetric.Metric
		arg   []float64 // ExplicitBounds
		want  func(pmetric.Metric)
	}{
		{
			name:  "convert exponential histogram to bucketed histogram",
			input: exponentialHistInput,
			arg:   []float64{160.0, 170.0, 180.0, 190.0, 200.0},
			want: func(metric pmetric.Metric) {

				metric.SetName("response_time")
				dp := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(1)
				dp.SetCount(2)
				dp.SetSum(361)
				dp.SetMax(195)
				dp.SetMin(166)
				dp.SetTimestamp(ts)

				// set attributes
				dp.Attributes().PutStr("metric_type", "timing")

				// set bucket counts
				dp.BucketCounts().Append(0, 1, 0, 0, 1, 0)

				// set explictbounds
				dp.ExplicitBounds().Append(160.0, 170.0, 180.0, 190.0, 200.0)

			},
		},
		{
			name:  "non-expontential histogram given",
			arg:   []float64{0},
			input: nonExponentialHist(),
			want: func(metric pmetric.Metric) {
				nonExponentialHist().CopyTo(metric)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := pmetric.NewMetric()
			tt.input.CopyTo(metric)

			ctx := ottlmetric.NewTransformContext(metric, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics())

			exprFunc, err := convertExponentialHistToExplicitHist(tt.arg)
			assert.NoError(t, err)
			_, err = exprFunc(nil, ctx)
			assert.NoError(t, err)

			expected := pmetric.NewMetric()
			tt.want(expected)

			assert.Equal(t, expected, metric)
		})
	}
}

func Test_convertExponentialHistToExplicitHist_validate(t *testing.T) {
	tests := []struct {
		name                    string
		sliceExplicitBoundsArgs []float64
	}{
		{
			name:                    "empty explicit bounds",
			sliceExplicitBoundsArgs: []float64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := convertExponentialHistToExplicitHist(tt.sliceExplicitBoundsArgs)
			assert.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), "explicit bounds must cannot be empty"))
		})
	}
}
