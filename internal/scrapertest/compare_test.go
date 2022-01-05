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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
)

// TestCompareMetrics tests the ability of comparing one metric slice to another
func TestCompareMetrics(t *testing.T) {
	tcs := []struct {
		name          string
		expectedError error
	}{
		{
			name: "equal",
		},
		{
			name:          "metric-slice-extra",
			expectedError: errors.New("metric slices not of same length"),
		},
		{
			name:          "metric-slice-missing",
			expectedError: errors.New("metric slices not of same length"),
		},
		{
			name:          "metric-type-expect-gauge",
			expectedError: errors.New("metric DataType does not match expected: Gauge, actual: Sum"),
		},
		{
			name:          "metric-type-expect-sum",
			expectedError: errors.New("metric DataType does not match expected: Sum, actual: Gauge"),
		},
		{
			name: "metric-name-mismatch",
			expectedError: multierr.Combine(
				errors.New("unexpected metric: wrong.name"),
				errors.New("missing expected metric: expected.name"),
			),
		},
		{
			name:          "metric-description-mismatch",
			expectedError: errors.New("metric Description does not match expected: Gauge One, actual: Gauge Two"),
		},
		{
			name:          "metric-unit-mismatch",
			expectedError: errors.New("metric Unit does not match expected: By, actual: 1"),
		},
		{
			name: "data-point-value-double-mismatch",
			expectedError: multierr.Combine(
				errors.New("datapoints for metric: `gauge.one`, do not match expected"),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint DoubleVal doesn't match expected: 123.456000, actual: 654.321000"),
			),
		},
		{
			name: "data-point-value-int-mismatch",
			expectedError: multierr.Combine(
				errors.New("datapoints for metric: `sum.one`, do not match expected"),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint IntVal doesn't match expected: 123, actual: 654"),
			),
		},
		{
			name: "data-point-attribute-extra",
			expectedError: multierr.Combine(
				errors.New("datapoints for metric: `gauge.one`, do not match expected"),
				errors.New("metric missing expected datapoint with attributes: map[attribute.one:one]"),
				errors.New("metric has extra datapoint with attributes: map[attribute.one:one attribute.two:two]"),
			),
		},
		{
			name: "data-point-attribute-missing",
			expectedError: multierr.Combine(
				errors.New("datapoints for metric: `sum.one`, do not match expected"),
				errors.New("metric missing expected datapoint with attributes: map[attribute.one:one attribute.two:two]"),
				errors.New("metric has extra datapoint with attributes: map[attribute.two:two]"),
			),
		},
		{
			name: "data-point-attribute-key",
			expectedError: multierr.Combine(
				errors.New("datapoints for metric: `sum.one`, do not match expected"),
				errors.New("metric missing expected datapoint with attributes: map[attribute.one:one]"),
				errors.New("metric has extra datapoint with attributes: map[attribute.two:one]"),
			),
		},
		{
			name: "data-point-attribute-value",
			expectedError: multierr.Combine(
				errors.New("datapoints for metric: `gauge.one`, do not match expected"),
				errors.New("metric missing expected datapoint with attributes: map[attribute.one:one]"),
				errors.New("metric has extra datapoint with attributes: map[attribute.one:two]"),
			),
		},
		{
			name:          "data-point-aggregation-expect-delta",
			expectedError: errors.New("metric AggregationTemporality does not match expected: AGGREGATION_TEMPORALITY_DELTA, actual: AGGREGATION_TEMPORALITY_CUMULATIVE"),
		},
		{
			name:          "data-point-aggregation-expect-cumulative",
			expectedError: errors.New("metric AggregationTemporality does not match expected: AGGREGATION_TEMPORALITY_CUMULATIVE, actual: AGGREGATION_TEMPORALITY_DELTA"),
		},
		{
			name:          "data-point-monotonic-expect-true",
			expectedError: errors.New("metric IsMonotonic does not match expected: true, actual: false"),
		},
		{
			name:          "data-point-monotonic-expect-false",
			expectedError: errors.New("metric IsMonotonic does not match expected: false, actual: true"),
		},
		{
			name: "data-point-value-expect-int",
			expectedError: multierr.Combine(
				errors.New("datapoints for metric: `gauge.one`, do not match expected"),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint types don't match: expected type: int, actual type: double"),
			),
		},
		{
			name: "data-point-value-expect-double",
			expectedError: multierr.Combine(
				errors.New("datapoints for metric: `gauge.one`, do not match expected"),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint types don't match: expected type: double, actual type: int"),
			),
		},
		{
			name: "ignore-timestamp",
			// Timestamps aren't checked so no error is expected.
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			expected, err := golden.ReadMetricSlice(filepath.Join("testdata", tc.name, "expected.json"))
			require.NoError(t, err)

			actual, err := golden.ReadMetricSlice(filepath.Join("testdata", tc.name, "actual.json"))
			require.NoError(t, err)

			err = CompareMetricSlices(expected, actual)
			if tc.expectedError != nil {
				require.Equal(t, tc.expectedError, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIgnoreValues(t *testing.T) {
	tcs := []struct {
		name          string
		expected      pdata.MetricSlice
		actual        pdata.MetricSlice
		unmaskedError string
	}{
		{
			name:          "data-point-value-double-mismatch",
			unmaskedError: `metric datapoint DoubleVal doesn't match expected`,
		},
		{
			name:          "data-point-value-int-mismatch",
			unmaskedError: `metric datapoint IntVal doesn't match expected`,
		},
	}

	for _, tc := range tcs {
		t.Run(fmt.Sprintf("ignore-%s", tc.name), func(t *testing.T) {
			expected, err := golden.ReadMetricSlice(filepath.Join("testdata", tc.name, "expected.json"))
			require.NoError(t, err)

			actual, err := golden.ReadMetricSlice(filepath.Join("testdata", tc.name, "actual.json"))
			require.NoError(t, err)

			err = CompareMetricSlices(expected, actual)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.unmaskedError)
			require.NoError(t, CompareMetricSlices(expected, actual, IgnoreValues()))
		})
	}
}
