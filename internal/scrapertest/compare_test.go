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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"
)

// TestCompareMetrics tests the ability of comparing one metric slice to another
func TestCompareMetrics(t *testing.T) {
	tcs := []struct {
		name          string
		expected      pdata.MetricSlice
		actual        pdata.MetricSlice
		expectedError error
	}{
		{
			name:          "Equal MetricSlice",
			actual:        baseTestMetrics(),
			expected:      baseTestMetrics(),
			expectedError: nil,
		},
		{
			name: "Wrong DataType",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(0)
				m.SetDataType(pdata.MetricDataTypeSum)
				return metrics
			}(),
			expected:      baseTestMetrics(),
			expectedError: fmt.Errorf("metric datatype does not match expected: Gauge, actual: Sum"),
		},
		{
			name: "Wrong Name",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(0)
				m.SetName("wrong name")
				return metrics
			}(),
			expected: baseTestMetrics(),
			expectedError: multierr.Combine(
				errors.New("unexpected metric wrong name"),
				errors.New("missing expected metric test gauge multi"),
			),
		},
		{
			name: "Wrong Description",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(0)
				m.SetDescription("wrong description")
				return metrics
			}(),
			expected:      baseTestMetrics(),
			expectedError: fmt.Errorf("metric description does not match expected: multi gauge, actual: wrong description"),
		},
		{
			name: "Wrong Unit",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(0)
				m.SetUnit("Wrong Unit")
				return metrics
			}(),
			expected:      baseTestMetrics(),
			expectedError: fmt.Errorf("metric Unit does not match expected: 1, actual: Wrong Unit"),
		},
		{
			name: "Wrong doubleVal",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(0)
				dp := m.Gauge().DataPoints().At(0)
				dp.SetDoubleVal(123)
				return metrics
			}(),
			expected: baseTestMetrics(),
			expectedError: multierr.Combine(
				errors.New("datapoints for metric: `test gauge multi`, do not match expected"),
				errors.New("datapoint with label(s): map[testKey1:teststringvalue1 testKey2:testvalue1], does not match expected"),
				errors.New("metric datapoint DoubleVal doesn't match expected: 2.000000, actual: 123.000000"),
			),
		},
		{
			name: "Wrong datatype",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(0)
				dp := m.Gauge().DataPoints().At(0)
				dp.SetDoubleVal(0)
				dp.SetIntVal(2)
				return metrics
			}(),
			expected: baseTestMetrics(),
			expectedError: multierr.Combine(
				errors.New("datapoints for metric: `test gauge multi`, do not match expected"),
				errors.New("datapoint with label(s): map[testKey1:teststringvalue1 testKey2:testvalue1], does not match expected"),
				errors.New("metric datapoint types don't match: expected type: 2, actual type: 1"),
			),
		},
		{
			name: "Wrong attribute key",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(0)
				dp := m.Gauge().DataPoints().At(0)
				attributes := pdata.NewAttributeMap()
				attributes.Insert("wrong key", pdata.NewAttributeValueString("teststringvalue1"))
				dp.Attributes().Clear()
				attributes.CopyTo(dp.Attributes())
				return metrics
			}(),
			expected: baseTestMetrics(),
			expectedError: multierr.Combine(
				errors.New("datapoints for metric: `test gauge multi`, do not match expected"),
				errors.New("metric missing expected data point: Labels: map[testKey1:teststringvalue1 testKey2:testvalue1]"),
				errors.New("metric has extra data point: Labels: map[wrong key:teststringvalue1]"),
			),
		},
		{
			name: "Wrong attribute value",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(0)
				dp := m.Gauge().DataPoints().At(0)
				attributes := pdata.NewAttributeMap()
				attributes.Insert("testKey1", pdata.NewAttributeValueString("wrong value"))
				dp.Attributes().Clear()
				attributes.CopyTo(dp.Attributes())
				return metrics
			}(),
			expected: baseTestMetrics(),
			expectedError: multierr.Combine(
				errors.New("datapoints for metric: `test gauge multi`, do not match expected"),
				errors.New("metric missing expected data point: Labels: map[testKey1:teststringvalue1 testKey2:testvalue1]"),
				errors.New("metric has extra data point: Labels: map[testKey1:wrong value]"),
			),
		},
		{
			name: "Wrong aggregation value",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(2)
				m.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
				return metrics
			}(),
			expected: baseTestMetrics(),
			expectedError: multierr.Combine(
				errors.New("metric AggregationTemporality does not match expected: AGGREGATION_TEMPORALITY_DELTA, actual: AGGREGATION_TEMPORALITY_CUMULATIVE"),
			),
		},
		{
			name: "Wrong monotonic value",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(2)
				m.Sum().SetIsMonotonic(true)
				return metrics
			}(),
			expected: baseTestMetrics(),
			expectedError: multierr.Combine(
				errors.New("metric IsMonotonic does not match expected: false, actual: true"),
			),
		},
		{
			name: "Wrong timestamp value",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(3)
				m.Sum().DataPoints().At(0).SetTimestamp(pdata.NewTimestampFromTime(time.Date(1998, 06, 28, 1, 1, 1, 1, &time.Location{})))
				return metrics
			}(),
			expected: baseTestMetrics(),
			// Timestamps aren't checked so no error is expected.
		},
		{
			name: "Empty timestamp value",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(3)
				m.Sum().DataPoints().At(0).SetTimestamp(pdata.NewTimestampFromTime(time.Time{}))
				return metrics
			}(),
			expected: baseTestMetrics(),
			// Timestamps aren't checked so no error is expected.
		},
		{
			name: "Missing metric",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				third := metrics.At(2)
				metrics.RemoveIf(func(m pdata.Metric) bool {
					return m.Name() == third.Name()
				})
				return metrics
			}(),
			expected: baseTestMetrics(),
			expectedError: multierr.Combine(
				errors.New("metric slices not of same length"),
			),
		},
		{
			name: "Bonus metric",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.AppendEmpty()
				m.SetName("bonus")
				return metrics
			}(),
			expected: baseTestMetrics(),
			expectedError: multierr.Combine(
				errors.New("metric slices not of same length"),
			),
		},
		{
			name: "No attribute",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				attributes := pdata.NewAttributeMap()
				attributes.Insert("testKey2", pdata.NewAttributeValueString("teststringvalue2"))
				dp := metrics.At(3).Sum().DataPoints().At(0)
				attributes.CopyTo(dp.Attributes())
				return metrics
			}(),
			expected: baseTestMetrics(),
			expectedError: multierr.Combine(
				errors.New("datapoints for metric: `test cumulative sum single`, do not match expected"),
				errors.New("metric missing expected data point: Labels: map[]"),
				errors.New("metric has extra data point: Labels: map[testKey2:teststringvalue2]"),
			),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := CompareMetricSlices(tc.expected, tc.actual)
			if tc.expectedError != nil {
				require.Equal(t, tc.expectedError, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
