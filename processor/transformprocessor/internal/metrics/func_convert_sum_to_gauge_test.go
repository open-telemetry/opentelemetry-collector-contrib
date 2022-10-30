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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoints"
)

func Test_convertSumToGauge(t *testing.T) {
	sumInput := pmetric.NewMetric()

	dp1 := sumInput.SetEmptySum().DataPoints().AppendEmpty()
	dp1.SetIntValue(10)

	dp2 := sumInput.Sum().DataPoints().AppendEmpty()
	dp2.SetDoubleValue(14.5)

	gaugeInput := pmetric.NewMetric()
	gaugeInput.SetEmptyGauge()

	histogramInput := pmetric.NewMetric()
	histogramInput.SetEmptyHistogram()

	expoHistogramInput := pmetric.NewMetric()
	expoHistogramInput.SetEmptyExponentialHistogram()

	summaryInput := pmetric.NewMetric()
	summaryInput.SetEmptySummary()

	tests := []struct {
		name  string
		input pmetric.Metric
		want  func(pmetric.Metric)
	}{
		{
			name:  "convert sum to gauge",
			input: sumInput,
			want: func(metric pmetric.Metric) {
				sumInput.CopyTo(metric)

				dps := sumInput.Sum().DataPoints()
				dps.CopyTo(metric.SetEmptyGauge().DataPoints())
			},
		},
		{
			name:  "noop for gauge",
			input: gaugeInput,
			want: func(metric pmetric.Metric) {
				gaugeInput.CopyTo(metric)
			},
		},
		{
			name:  "noop for histogram",
			input: histogramInput,
			want: func(metric pmetric.Metric) {
				histogramInput.CopyTo(metric)
			},
		},
		{
			name:  "noop for exponential histogram",
			input: expoHistogramInput,
			want: func(metric pmetric.Metric) {
				expoHistogramInput.CopyTo(metric)
			},
		},
		{
			name:  "noop for summary",
			input: summaryInput,
			want: func(metric pmetric.Metric) {
				summaryInput.CopyTo(metric)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := pmetric.NewMetric()
			tt.input.CopyTo(metric)

			ctx := ottldatapoints.NewTransformContext(pmetric.NewNumberDataPoint(), metric, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource())

			exprFunc, _ := convertSumToGauge()

			_, err := exprFunc(ctx)
			assert.Nil(t, err)

			expected := pmetric.NewMetric()
			tt.want(expected)

			assert.Equal(t, expected, metric)
		})
	}
}
