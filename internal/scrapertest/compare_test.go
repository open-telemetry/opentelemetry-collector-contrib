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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
)

type expectation struct {
	err    error
	reason string
}

func (e expectation) validate(t *testing.T, err error) {
	if e.err == nil {
		require.NoError(t, err, e.reason)
		return
	}
	require.Equal(t, e.err, err, e.reason)
}

func TestCompareMetricSlices(t *testing.T) {
	tcs := []struct {
		name           string
		compareOptions []CompareOption // when no options are used
		withoutOptions expectation
		withOptions    expectation
	}{
		{
			name: "equal",
		},
		{
			name: "metric-slice-extra",
			withoutOptions: expectation{
				err:    errors.New("metric slices not of same length"),
				reason: "A metric slice with an extra metric should cause a failure.",
			},
		},
		{
			name: "metric-slice-missing",
			withoutOptions: expectation{
				err:    errors.New("metric slices not of same length"),
				reason: "A metric slice with a missing metric should cause a failure.",
			},
		},
		{
			name: "metric-type-expect-gauge",
			withoutOptions: expectation{
				err:    errors.New("metric DataType does not match expected: Gauge, actual: Sum"),
				reason: "A metric with the wrong instrument type should cause a failure.",
			},
		},
		{
			name: "metric-type-expect-sum",
			withoutOptions: expectation{
				err:    errors.New("metric DataType does not match expected: Sum, actual: Gauge"),
				reason: "A metric with the wrong instrument type should cause a failure.",
			},
		},
		{
			name: "metric-name-mismatch",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("unexpected metric: wrong.name"),
					errors.New("missing expected metric: expected.name"),
				),
				reason: "A metric with a different name is a different (extra) metric. The expected metric is missing.",
			},
		},
		{
			name: "metric-description-mismatch",
			withoutOptions: expectation{
				err:    errors.New("metric Description does not match expected: Gauge One, actual: Gauge Two"),
				reason: "A metric with the wrong description should cause a failure.",
			},
		},
		{
			name: "metric-unit-mismatch",
			withoutOptions: expectation{
				err:    errors.New("metric Unit does not match expected: By, actual: 1"),
				reason: "A metric with the wrong unit should cause a failure.",
			},
		},
		{
			name: "data-point-attribute-extra",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("datapoints for metric: `gauge.one`, do not match expected"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.one:one]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.one:one attribute.two:two]"),
				),
				reason: "A data point with an extra attribute is a different (extra) data point. The expected data point is missing.",
			},
		},
		{
			name: "data-point-attribute-missing",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("datapoints for metric: `sum.one`, do not match expected"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.one:one attribute.two:two]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.two:two]"),
				),
				reason: "A data point with a missing attribute is a different (extra) data point. The expected data point is missing.",
			},
		},
		{
			name: "data-point-attribute-key",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("datapoints for metric: `sum.one`, do not match expected"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.one:one]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.two:one]"),
				),
				reason: "A data point with the wrong attribute key is a different (extra) data point. The expected data point is missing.",
			},
		},
		{
			name: "data-point-attribute-value",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("datapoints for metric: `gauge.one`, do not match expected"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.one:one]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.one:two]"),
				),
				reason: "A data point with the wrong attribute value is a different (extra) data point. The expected data point is missing.",
			},
		},
		{
			name: "data-point-aggregation-expect-delta",
			withoutOptions: expectation{
				err:    errors.New("metric AggregationTemporality does not match expected: AGGREGATION_TEMPORALITY_DELTA, actual: AGGREGATION_TEMPORALITY_CUMULATIVE"),
				reason: "A data point with the wrong aggregation temporality should cause a failure.",
			},
		},
		{
			name: "data-point-aggregation-expect-cumulative",
			withoutOptions: expectation{
				err:    errors.New("metric AggregationTemporality does not match expected: AGGREGATION_TEMPORALITY_CUMULATIVE, actual: AGGREGATION_TEMPORALITY_DELTA"),
				reason: "A data point with the wrong aggregation temporality should cause a failure.",
			},
		},
		{
			name: "data-point-monotonic-expect-true",
			withoutOptions: expectation{
				err:    errors.New("metric IsMonotonic does not match expected: true, actual: false"),
				reason: "A data point with the wrong monoticity should cause a failure.",
			},
		},
		{
			name: "data-point-monotonic-expect-false",
			withoutOptions: expectation{
				err:    errors.New("metric IsMonotonic does not match expected: false, actual: true"),
				reason: "A data point with the wrong monoticity should cause a failure.",
			},
		},
		{
			name: "data-point-value-double-mismatch",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("datapoints for metric: `gauge.one`, do not match expected"),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint DoubleVal doesn't match expected: 123.456000, actual: 654.321000"),
				),
				reason: "A data point with the wrong value should cause a failure.",
			},
		},
		{
			name: "data-point-value-int-mismatch",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("datapoints for metric: `sum.one`, do not match expected"),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint IntVal doesn't match expected: 123, actual: 654"),
				),
				reason: "A data point with the wrong value should cause a failure.",
			},
		},
		{
			name: "data-point-value-expect-int",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("datapoints for metric: `gauge.one`, do not match expected"),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint types don't match: expected type: int, actual type: double"),
				),
				reason: "A data point with the wrong type of value should cause a failure.",
			},
		},
		{
			name: "data-point-value-expect-double",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("datapoints for metric: `gauge.one`, do not match expected"),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint types don't match: expected type: double, actual type: int"),
				),
				reason: "A data point with the wrong type of value should cause a failure.",
			},
		},
		{
			name: "ignore-timestamp",
			withoutOptions: expectation{
				err:    nil,
				reason: "Timestamps are always ignored, so no error is expected.",
			},
		},
		{
			name: "ignore-data-point-value-double-mismatch",
			compareOptions: []CompareOption{
				IgnoreValues(),
			},
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("datapoints for metric: `gauge.one`, do not match expected"),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint DoubleVal doesn't match expected: 123.456000, actual: 654.321000"),
				),
				reason: "An unpredictable data point value will cause failures if not ignored.",
			},
		},
		{
			name: "ignore-data-point-value-int-mismatch",
			compareOptions: []CompareOption{
				IgnoreValues(),
			},
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("datapoints for metric: `sum.one`, do not match expected"),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint IntVal doesn't match expected: 123, actual: 654"),
				),
				reason: "An unpredictable data point value will cause failures if not ignored.",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			expected, err := golden.ReadMetricSlice(filepath.Join("testdata", tc.name, "expected.json"))
			require.NoError(t, err)

			actual, err := golden.ReadMetricSlice(filepath.Join("testdata", tc.name, "actual.json"))
			require.NoError(t, err)

			err = CompareMetricSlices(expected, actual)
			tc.withoutOptions.validate(t, err)

			if tc.compareOptions == nil {
				return
			}

			err = CompareMetricSlices(expected, actual, tc.compareOptions...)
			tc.withOptions.validate(t, err)
		})
	}
}
