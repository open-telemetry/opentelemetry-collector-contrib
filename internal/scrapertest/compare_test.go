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

func TestCompareMetrics(t *testing.T) {
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
			name: "resource-extra",
			withoutOptions: expectation{
				err:    errors.New("number of resources does not match expected: 1, actual: 2"),
				reason: "An extra resource should cause a failure.",
			},
		},
		{
			name: "resource-missing",
			withoutOptions: expectation{
				err:    errors.New("number of resources does not match expected: 2, actual: 1"),
				reason: "A missing resource should cause a failure.",
			},
		},
		{
			name: "resource-attributes-mismatch",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("missing expected resource with attributes: map[type:two]"),
					errors.New("extra resource with attributes: map[type:three]"),
				),
				reason: "A resource with a different set of attributes is a different resource.",
			},
		},
		{
			name: "resource-instrumentation-library-extra",
			withoutOptions: expectation{
				err:    errors.New("number of instrumentation libraries does not match expected: 1, actual: 2"),
				reason: "An extra instrumentation library should cause a failure.",
			},
		},
		{
			name: "resource-instrumentation-library-missing",
			withoutOptions: expectation{
				err:    errors.New("number of instrumentation libraries does not match expected: 2, actual: 1"),
				reason: "An missing instrumentation library should cause a failure.",
			},
		},
		{
			name: "resource-instrumentation-library-name-mismatch",
			withoutOptions: expectation{
				err:    errors.New("instrumentation library Name does not match expected: one, actual: two"),
				reason: "An instrumentation library with a different name is a different library.",
			},
		},
		{
			name: "resource-instrumentation-library-version-mismatch",
			withoutOptions: expectation{
				err:    errors.New("instrumentation library Version does not match expected: 1.0, actual: 2.0"),
				reason: "An instrumentation library with a different version is a different library.",
			},
		},
		{
			name: "metric-slice-extra",
			withoutOptions: expectation{
				err:    errors.New("number of metrics does not match expected: 1, actual: 2"),
				reason: "A metric slice with an extra metric should cause a failure.",
			},
		},
		{
			name: "metric-slice-missing",
			withoutOptions: expectation{
				err:    errors.New("number of metrics does not match expected: 1, actual: 0"),
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
			name: "data-point-slice-extra",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("datapoints for metric: `gauge.one`, do not match expected"),
					errors.New("number of datapoints does not match expected: 1, actual: 2"),
				),
				reason: "A data point slice with an extra data point should cause a failure.",
			},
		},
		{
			name: "data-point-slice-missing",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("datapoints for metric: `sum.one`, do not match expected"),
					errors.New("number of datapoints does not match expected: 2, actual: 1"),
				),
				reason: "A data point slice with a missing data point should cause a failure.",
			},
		},
		{
			name: "data-point-slice-dedup",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("datapoints for metric: `sum.one`, do not match expected"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.one:two]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.one:one]"),
				),
				reason: "Data point slice comparison must not match each data point more than once.",
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
				IgnoreMetricValues(),
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
				IgnoreMetricValues(),
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
		{
			name: "ignore-global-attribute-value",
			compareOptions: []CompareOption{
				IgnoreMetricAttributeValue("hostname"),
			},
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("datapoints for metric: `gauge.one`, do not match expected"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.two:value A hostname:unpredictable]"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.two:value B hostname:unpredictable]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.two:value A hostname:random]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.two:value B hostname:random]"),
				),
				reason: "An unpredictable attribute value will cause failures if not ignored.",
			},
			withOptions: expectation{
				err:    nil,
				reason: "The unpredictable attribute was ignored on all metrics that carried it.",
			},
		},
		{
			name: "ignore-one-attribute-value",
			compareOptions: []CompareOption{
				IgnoreMetricAttributeValue("hostname", "gauge.one"),
			},
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("datapoints for metric: `gauge.one`, do not match expected"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.two:value A hostname:unpredictable]"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.two:value B hostname:unpredictable]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.two:value A hostname:random]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.two:value B hostname:random]"),
				),
				reason: "An unpredictable attribute will cause failures if not ignored.",
			},
			withOptions: expectation{
				err: multierr.Combine(
					errors.New("datapoints for metric: `sum.one`, do not match expected"),
					errors.New("metric missing expected datapoint with attributes: map[hostname:also unpredictable]"),
					errors.New("metric has extra datapoint with attributes: map[hostname:also random]"),
				),
				reason: "Although the unpredictable attribute was ignored on one metric, it was not ignored on another.",
			},
		},
		{
			name: "ignore-one-resource-attribute",
			compareOptions: []CompareOption{
				IgnoreResourceAttributeValue("node_id"),
			},
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("missing expected resource with attributes: map[node_id:a-different-random-id]"),
					errors.New("extra resource with attributes: map[node_id:a-random-id]"),
				),
				reason: "An unpredictable resource attribute will cause failures if not ignored.",
			},
			withOptions: expectation{
				err:    nil,
				reason: "The unpredictable resource attribute was ignored on each resource that carried it.",
			},
		},
		{
			name: "ignore-one-resource-attribute-multiple-resources",
			compareOptions: []CompareOption{
				IgnoreResourceAttributeValue("node_id"),
			},
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("missing expected resource with attributes: map[node_id:BB902-expected]"),
					errors.New("missing expected resource with attributes: map[namespace:test node_id:BB902-expected]"),
					errors.New("missing expected resource with attributes: map[node_id:BB904-expected]"),
					errors.New("missing expected resource with attributes: map[namespace:test node_id:BB904-expected]"),
					errors.New("missing expected resource with attributes: map[node_id:BB903-expected]"),
					errors.New("missing expected resource with attributes: map[namespace:test node_id:BB903-expected]"),
					errors.New("extra resource with attributes: map[node_id:BB902-actual]"),
					errors.New("extra resource with attributes: map[namespace:test node_id:BB902-actual]"),
					errors.New("extra resource with attributes: map[node_id:BB904-actual]"),
					errors.New("extra resource with attributes: map[namespace:test node_id:BB904-actual]"),
					errors.New("extra resource with attributes: map[node_id:BB903-actual]"),
					errors.New("extra resource with attributes: map[namespace:test node_id:BB903-actual]"),
				),
				reason: "An unpredictable resource attribute will cause failures if not ignored.",
			},
			withOptions: expectation{
				err:    nil,
				reason: "The unpredictable resource attribute was ignored on each resource that carried it, but the predictable attributes were preserved.",
			},
		},
		{
			name: "ignore-each-attribute-value",
			compareOptions: []CompareOption{
				IgnoreMetricAttributeValue("hostname", "gauge.one", "sum.one"),
			},
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("datapoints for metric: `gauge.one`, do not match expected"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.two:value A hostname:unpredictable]"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.two:value B hostname:unpredictable]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.two:value A hostname:random]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.two:value B hostname:random]"),
				),
				reason: "An unpredictable attribute will cause failures if not ignored.",
			},
			withOptions: expectation{
				err:    nil,
				reason: "The unpredictable attribute was ignored on each metric that carried it.",
			},
		},
		{
			name: "ignore-attribute-set-collision",
			compareOptions: []CompareOption{
				IgnoreMetricAttributeValue("attribute.one"),
			},
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("datapoints for metric: `gauge.one`, do not match expected"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.one:one attribute.two:same]"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.one:two attribute.two:same]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.one:random.one attribute.two:same]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.one:random.two attribute.two:same]"),
				),
				reason: "An unpredictable attribute will cause failures if not ignored.",
			},
			withOptions: expectation{
				err:    nil,
				reason: "Ignoring the unpredictable attribute caused an attribute set collision, but comparison still works.",
			},
		},
		{
			name: "ignore-attribute-set-collision-order",
			compareOptions: []CompareOption{
				IgnoreMetricAttributeValue("attribute.one"),
			},
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("datapoints for metric: `gauge.one`, do not match expected"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.one:unpredictable.one attribute.two:same]"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.one:unpredictable.two attribute.two:same]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.one:random.two attribute.two:same]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.one:random.one attribute.two:same]"),
				),
				reason: "An unpredictable attribute will cause failures if not ignored.",
			},
			withOptions: expectation{
				err: nil,
				reason: "Ignoring the unpredictable attribute caused an attribute set collision where the data point values " +
					"where in different orders in expected vs actual, but comparison ignores order.",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			expected, err := golden.ReadMetrics(filepath.Join("testdata", tc.name, "expected.json"))
			require.NoError(t, err)

			actual, err := golden.ReadMetrics(filepath.Join("testdata", tc.name, "actual.json"))
			require.NoError(t, err)

			err = CompareMetrics(expected, actual)
			tc.withoutOptions.validate(t, err)

			if tc.compareOptions == nil {
				return
			}

			err = CompareMetrics(expected, actual, tc.compareOptions...)
			tc.withOptions.validate(t, err)
		})
	}
}
