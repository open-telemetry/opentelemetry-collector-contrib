// Copyright  The OpenTelemetry Authors
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

package scrapertest

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestIgnoreValues(t *testing.T) {
	tcs := []struct {
		name          string
		expected      pdata.MetricSlice
		actual        pdata.MetricSlice
		unmaskedError string
	}{
		{
			name: "Ignore mismatched doubleVal",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(0)
				dp := m.Gauge().DataPoints().At(0)
				dp.SetDoubleVal(123)
				return metrics
			}(),
			expected:      baseTestMetrics(),
			unmaskedError: `metric datapoint DoubleVal doesn't match expected`,
		},
		{
			name: "Ignore mismatched intVal",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(2)
				dp := m.Sum().DataPoints().At(0)
				dp.SetIntVal(123)
				return metrics
			}(),
			expected:      baseTestMetrics(),
			unmaskedError: `metric datapoint IntVal doesn't match expected`,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := CompareMetricSlices(tc.expected, tc.actual)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.unmaskedError)
			require.NoError(t, CompareMetricSlices(tc.expected, tc.actual, IgnoreValues()))
		})
	}
}
