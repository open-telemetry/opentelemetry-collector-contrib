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
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"

	"go.opentelemetry.io/collector/model/pdata"
)

func baseTestMetrics() pdata.MetricSlice {
	slice := pdata.NewMetricSlice()
	metric := slice.AppendEmpty()
	metric.SetDataType(pdata.MetricDataTypeGauge)
	metric.SetDescription("test description")
	metric.SetName("test name")
	metric.SetUnit("test unit")
	dps := metric.Gauge().DataPoints()
	dp := dps.AppendEmpty()
	dp.SetDoubleVal(1)
	dp.SetTimestamp(pdata.NewTimestampFromTime(time.Time{}))
	attributes := pdata.NewAttributeMap()
	attributes.Insert("testKey1", pdata.NewAttributeValueString("teststringvalue1"))
	attributes.CopyTo(dp.Attributes())
	return slice
}

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
				errors.New("missing expected metric test name"),
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
			expectedError: fmt.Errorf("metric description does not match expected: test description, actual: wrong description"),
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
			expectedError: fmt.Errorf("metric Unit does not match expected: test unit, actual: Wrong Unit"),
		},
		{
			name: "Wrong doubleVal",
			actual: func() pdata.MetricSlice {
				metrics := baseTestMetrics()
				m := metrics.At(0)
				dp := m.Gauge().DataPoints().At(0)
				dp.SetDoubleVal(2)
				return metrics
			}(),
			expected: baseTestMetrics(),
			expectedError: multierr.Combine(
				errors.New("datapoints for metric: `test name`, do not match expected"),
				errors.New("datapoint with label(s): map[testKey1:teststringvalue1], does not match expected"),
				errors.New("metric datapoint DoubleVal doesn't match expected: 1.000000, actual: 2.000000"),
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
				errors.New("datapoints for metric: `test name`, do not match expected"),
				errors.New("datapoint with label(s): map[testKey1:teststringvalue1], does not match expected"),
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
				errors.New("datapoints for metric: `test name`, do not match expected"),
				errors.New("metric missing expected data point: Labels: map[testKey1:teststringvalue1]"),
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
				errors.New("datapoints for metric: `test name`, do not match expected"),
				errors.New("metric missing expected data point: Labels: map[testKey1:teststringvalue1]"),
				errors.New("metric has extra data point: Labels: map[testKey1:wrong value]"),
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

func TestRoundTrip(t *testing.T) {
	metricslice := baseTestMetrics()
	expectedMetrics := pdata.NewMetrics()
	metricslice.CopyTo(expectedMetrics.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics())

	tempDir := filepath.Join(t.TempDir(), "metrics.json")
	err := WriteExpected(tempDir, expectedMetrics)
	require.NoError(t, err)

	actualMetrics, err := ReadExpected(tempDir)
	require.NoError(t, err)
	require.Equal(t, expectedMetrics, actualMetrics)
}

func TestMetricsToFile(t *testing.T) {
	metricslice := baseTestMetrics()
	metrics := pdata.NewMetrics()
	metricslice.CopyTo(metrics.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics())

	tempDir := filepath.Join(t.TempDir(), "metrics.json")
	WriteExpected(tempDir, metrics)

	actualBytes, err := ioutil.ReadFile(tempDir)
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "roundtrip", "expected.json")
	expectedBytes, err := ioutil.ReadFile(expectedFile)
	require.NoError(t, err)

	require.Equal(t, expectedBytes, actualBytes)
}

func TestFileToMetrics(t *testing.T) {
	metricslice := baseTestMetrics()
	expectedMetrics := pdata.NewMetrics()
	metricslice.CopyTo(expectedMetrics.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics())

	expectedFile := filepath.Join("testdata", "roundtrip", "expected.json")
	actualMetrics, err := ReadExpected(expectedFile)
	require.NoError(t, err)
	require.Equal(t, expectedMetrics, actualMetrics)
}
