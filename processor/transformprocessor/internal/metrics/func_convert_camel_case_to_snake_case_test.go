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

func getTestCamelCaseMetric(name string) pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetName(name)
	return metricInput
}

func Test_convertCamelCaseToSnakeCase(t *testing.T) {

	input := pmetric.NewMetric()
	pmetric.NewMetric().SetName("metricTest")
	input.SetEmptyGauge()

	tests := []struct {
		name  string
		input pmetric.Metric
		want  func(pmetric.Metric)
	}{
		{
			name:  "two word convert",
			input: getTestCamelCaseMetric("metricTest"),
			want: func(metric pmetric.Metric) {
				metric.SetName("metric_test")
			},
		},
		{
			name:  "noop already snake case",
			input: getTestCamelCaseMetric("metric_test"),
			want: func(metric pmetric.Metric) {
				metric.SetName("metric_test")
			},
		},
		{
			name:  "multiple uppercase",
			input: getTestCamelCaseMetric("CPUUtilizationMetric"),
			want: func(metric pmetric.Metric) {
				metric.SetName("cpu_utilization_metric")
			},
		},
		{
			name:  "hyphen replacement",
			input: getTestCamelCaseMetric("metric-test"),
			want: func(metric pmetric.Metric) {
				metric.SetName("metric_test")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := pmetric.NewMetric()
			tt.input.CopyTo(metric)

			ctx := ottldatapoints.NewTransformContext(pmetric.NewGauge(), metric, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource())

			exprFunc, _ := convertCamelCaseToSnakeCase()

			_, err := exprFunc(ctx)
			assert.Nil(t, err)

			expected := pmetric.NewMetric()
			tt.want(expected)

			assert.Equal(t, expected, metric)
		})
	}
}
