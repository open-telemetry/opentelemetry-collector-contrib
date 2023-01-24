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

package pmetrictest

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/internal"
)

func TestCompareMetrics(t *testing.T) {
	tcs := []struct {
		name           string
		compareOptions []CompareMetricsOption
		withoutOptions internal.Expectation
		withOptions    internal.Expectation
	}{
		{
			name: "equal",
		},
		{
			name: "resource-extra",
			withoutOptions: internal.Expectation{
				Err:    errors.New("number of resources does not match expected: 1, actual: 2"),
				Reason: "An extra resource should cause a failure.",
			},
		},
		{
			name: "resource-missing",
			withoutOptions: internal.Expectation{
				Err:    errors.New("number of resources does not match expected: 2, actual: 1"),
				Reason: "A missing resource should cause a failure.",
			},
		},
		{
			name: "resource-attributes-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("missing expected resource with attributes: map[type:two]"),
					errors.New("extra resource with attributes: map[type:three]"),
				),
				Reason: "A resource with a different set of attributes is a different resource.",
			},
		},
		{
			name: "resource-instrumentation-library-extra",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New("number of ScopeMetrics does not match expected: 1, actual: 2"),
				),
				Reason: "An extra scope should cause a failure.",
			},
		},
		{
			name: "resource-instrumentation-library-missing",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New("number of ScopeMetrics does not match expected: 2, actual: 1"),
				),
				Reason: "An missing scope should cause a failure.",
			},
		},
		{
			name: "resource-instrumentation-library-name-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New("missing expected ScopeMetrics with scope name: one"),
					errors.New("extra ScopeMetrics with scope name: two"),
				),
				Reason: "An scope with a different name is a different library.",
			},
		},
		{
			name: "resource-instrumentation-library-version-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "one" does not match expected`),
					errors.New("scope Version does not match expected: 1.0, actual: 2.0"),
				),
				Reason: "An scope with a different version is a different library.",
			},
		},
		{
			name: "metric-slice-extra",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New("number of metrics does not match expected: 1, actual: 2"),
				),
				Reason: "A metric slice with an extra metric should cause a failure.",
			},
		},
		{
			name: "metric-slice-missing",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New("number of metrics does not match expected: 1, actual: 0"),
				),
				Reason: "A metric slice with a missing metric should cause a failure.",
			},
		},
		{
			name: "metric-type-expect-gauge",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric should.be.gauge does not match expected`),
					errors.New("metric DataType does not match expected: Gauge, actual: Sum"),
				),
				Reason: "A metric with the wrong instrument type should cause a failure.",
			},
		},
		{
			name: "metric-type-expect-sum",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric should.be.sum does not match expected`),
					errors.New("metric DataType does not match expected: Sum, actual: Gauge"),
				),
				Reason: "A metric with the wrong instrument type should cause a failure.",
			},
		},
		{
			name: "metric-name-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New("missing expected metric: expected.name"),
					errors.New("unexpected metric: wrong.name"),
				),
				Reason: "A metric with a different name is a different (extra) metric. The expected metric is missing.",
			},
		},
		{
			name: "metric-description-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric gauge.one does not match expected`),
					errors.New("metric Description does not match expected: Gauge One, actual: Gauge Two"),
				),
				Reason: "A metric with the wrong description should cause a failure.",
			},
		},
		{
			name: "metric-unit-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric gauge.one does not match expected`),
					errors.New("metric Unit does not match expected: By, actual: 1"),
				),
				Reason: "A metric with the wrong unit should cause a failure.",
			},
		},
		{
			name: "data-point-slice-extra",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric gauge.one does not match expected`),
					errors.New("number of datapoints does not match expected: 1, actual: 2"),
				),
				Reason: "A data point slice with an extra data point should cause a failure.",
			},
		},
		{
			name: "data-point-slice-missing",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric sum.one does not match expected`),
					errors.New("number of datapoints does not match expected: 2, actual: 1"),
				),
				Reason: "A data point slice with a missing data point should cause a failure.",
			},
		},
		{
			name: "data-point-slice-dedup",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric sum.one does not match expected`),
					errors.New("metric missing expected datapoint with attributes: map[attribute.one:two]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.one:one]"),
				),
				Reason: "Data point slice comparison must not match each data point more than once.",
			},
		},
		{
			name: "data-point-attribute-extra",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric gauge.one does not match expected`),
					errors.New("metric missing expected datapoint with attributes: map[attribute.one:one]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.one:one attribute.two:two]"),
				),
				Reason: "A data point with an extra attribute is a different (extra) data point. The expected data point is missing.",
			},
		},
		{
			name: "data-point-attribute-missing",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric sum.one does not match expected`),
					errors.New("metric missing expected datapoint with attributes: map[attribute.one:one attribute.two:two]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.two:two]"),
				),
				Reason: "A data point with a missing attribute is a different (extra) data point. The expected data point is missing.",
			},
		},
		{
			name: "data-point-attribute-key",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric sum.one does not match expected`),
					errors.New("metric missing expected datapoint with attributes: map[attribute.one:one]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.two:one]"),
				),
				Reason: "A data point with the wrong attribute key is a different (extra) data point. The expected data point is missing.",
			},
		},
		{
			name: "data-point-attribute-value",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric gauge.one does not match expected`),
					errors.New("metric missing expected datapoint with attributes: map[attribute.one:one]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.one:two]"),
				),
				Reason: "A data point with the wrong attribute value is a different (extra) data point. The expected data point is missing.",
			},
		},
		{
			name: "data-point-aggregation-expect-delta",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric delta.one does not match expected`),
					errors.New("metric AggregationTemporality does not match expected: Delta, actual: Cumulative"),
				),
				Reason: "A data point with the wrong aggregation temporality should cause a failure.",
			},
		},
		{
			name: "data-point-aggregation-expect-cumulative",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric delta.one does not match expected`),
					errors.New("metric AggregationTemporality does not match expected: Cumulative, actual: Delta"),
				),
				Reason: "A data point with the wrong aggregation temporality should cause a failure.",
			},
		},
		{
			name: "data-point-monotonic-expect-true",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric monotonic does not match expected`),
					errors.New("metric IsMonotonic does not match expected: true, actual: false"),
				),
				Reason: "A data point with the wrong monoticity should cause a failure.",
			},
		},
		{
			name: "data-point-monotonic-expect-false",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric nonmonotonic does not match expected`),
					errors.New("metric IsMonotonic does not match expected: false, actual: true"),
				),
				Reason: "A data point with the wrong monoticity should cause a failure.",
			},
		},
		{
			name: "data-point-value-double-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric gauge.one does not match expected`),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint DoubleVal doesn't match expected: 123.456000, actual: 654.321000"),
				),
				Reason: "A data point with the wrong value should cause a failure.",
			},
		},
		{
			name: "data-point-value-int-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric sum.one does not match expected`),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint IntVal doesn't match expected: 123, actual: 654"),
				),
				Reason: "A data point with the wrong value should cause a failure.",
			},
		},
		{
			name: "data-point-value-expect-int",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric gauge.one does not match expected`),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint types don't match: expected type: Int, actual type: Double"),
				),
				Reason: "A data point with the wrong type of value should cause a failure.",
			},
		},
		{
			name: "data-point-value-expect-double",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric gauge.one does not match expected`),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint types don't match: expected type: Double, actual type: Int"),
				),
				Reason: "A data point with the wrong type of value should cause a failure.",
			},
		},
		{
			name: "histogram-data-point-count-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric histogram.one does not match expected`),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint Count doesn't match expected: 123, actual: 654"),
				),
				Reason: "A data point with the wrong bucket count should cause a failure.",
			},
		},
		{
			name: "histogram-data-point-sum-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric histogram.one does not match expected`),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint Sum doesn't match expected: 123.456000, actual: 654.321000"),
				),
			},
		},
		{
			name: "histogram-data-point-buckets-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric histogram.one does not match expected`),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint BucketCounts doesn't match expected: [1 2 3], actual: [3 2 1]"),
				),
			},
		},
		{
			name: "exp-histogram-data-point-count-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric exponential_histogram.one does not match expected`),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint Count doesn't match expected: 123, actual: 654"),
				),
				Reason: "A data point with the wrong bucket count should cause a failure.",
			},
		},
		{
			name: "exp-histogram-data-point-sum-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric exponential_histogram.one does not match expected`),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint Sum doesn't match expected: 123.456000, actual: 654.321000"),
				),
			},
		},
		{
			name: "exp-histogram-data-point-positive-buckets-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric exponential_histogram.one does not match expected`),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint Positive BucketCounts doesn't match expected: [1 2 3], "+
						"actual: [3 2 1]"),
				),
			},
		},
		{
			name: "exp-histogram-data-point-negative-offset-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric exponential_histogram.one does not match expected`),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint Negative Offset doesn't match expected: 10, actual: 1"),
				),
			},
		},
		{
			name: "summary-data-point-count-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric summary.one does not match expected`),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint Count doesn't match expected: 123, actual: 654"),
				),
				Reason: "A data point with the wrong bucket count should cause a failure.",
			},
		},
		{
			name: "summary-data-point-sum-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric summary.one does not match expected`),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint Sum doesn't match expected: 123.456000, actual: 654.321000"),
				),
			},
		},
		{
			name: "summary-data-point-quantile-values-length-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric summary.one does not match expected`),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint QuantileValues length doesn't match expected: 3, actual: 2"),
				),
			},
		},
		{
			name: "summary-data-point-quantile-values-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric summary.one does not match expected`),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint value at quantile 0.990000 doesn't match expected: 99.000000, "+
						"actual: 110.000000"),
				),
			},
		},
		{
			name: "ignore-timestamp",
			withoutOptions: internal.Expectation{
				Err:    nil,
				Reason: "Timestamps are always ignored, so no error is expected.",
			},
		},
		{
			name: "ignore-data-point-value-double-mismatch",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricValues(),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric gauge.one does not match expected`),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint DoubleVal doesn't match expected: 123.456000, actual: 654.321000"),
				),
				Reason: "An unpredictable data point value will cause failures if not ignored.",
			},
		},
		{
			name: "ignore-data-point-value-int-mismatch",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricValues(),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric sum.one does not match expected`),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint IntVal doesn't match expected: 123, actual: 654"),
				),
				Reason: "An unpredictable data point value will cause failures if not ignored.",
			},
		},
		{
			name: "ignore-subsequent-data-points-all",
			compareOptions: []CompareMetricsOption{
				IgnoreSubsequentDataPoints(),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric sum.one does not match expected`),
					errors.New("number of datapoints does not match expected: 1, actual: 2"),
				),
				Reason: "An unpredictable data point value will cause failures if not ignored.",
			},
		},
		{
			name: "ignore-subsequent-data-points-one",
			compareOptions: []CompareMetricsOption{
				IgnoreSubsequentDataPoints("sum.one"),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric sum.one does not match expected`),
					errors.New("number of datapoints does not match expected: 1, actual: 2"),
				),
				Reason: "An unpredictable data point value will cause failures if not ignored.",
			},
		},
		{
			name: "ignore-single-metric",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricValues("sum.two"),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric sum.two does not match expected`),
					errors.New("datapoint with attributes: map[], does not match expected"),
					errors.New("metric datapoint IntVal doesn't match expected: 123, actual: 654"),
				),
				Reason: "An unpredictable data point value will cause failures if not ignored.",
			},
		},
		{
			name: "ignore-global-attribute-value",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricAttributeValue("hostname"),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric gauge.one does not match expected`),
					errors.New("metric missing expected datapoint with attributes: map[attribute.two:value A hostname:unpredictable]"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.two:value B hostname:unpredictable]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.two:value A hostname:random]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.two:value B hostname:random]"),
				),
				Reason: "An unpredictable attribute value will cause failures if not ignored.",
			},
			withOptions: internal.Expectation{
				Err:    nil,
				Reason: "The unpredictable attribute was ignored on all metrics that carried it.",
			},
		},
		{
			name: "ignore-one-attribute-value",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricAttributeValue("hostname", "gauge.one"),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric gauge.one does not match expected`),
					errors.New("metric missing expected datapoint with attributes: map[attribute.two:value A hostname:unpredictable]"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.two:value B hostname:unpredictable]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.two:value A hostname:random]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.two:value B hostname:random]"),
				),
				Reason: "An unpredictable attribute will cause failures if not ignored.",
			},
			withOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric sum.one does not match expected`),
					errors.New("metric missing expected datapoint with attributes: map[hostname:also unpredictable]"),
					errors.New("metric has extra datapoint with attributes: map[hostname:also random]"),
				),
				Reason: "Although the unpredictable attribute was ignored on one metric, it was not ignored on another.",
			},
		},
		{
			name: "ignore-one-resource-attribute",
			compareOptions: []CompareMetricsOption{
				IgnoreResourceAttributeValue("node_id"),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("missing expected resource with attributes: map[node_id:a-different-random-id]"),
					errors.New("extra resource with attributes: map[node_id:a-random-id]"),
				),
				Reason: "An unpredictable resource attribute will cause failures if not ignored.",
			},
			withOptions: internal.Expectation{
				Err:    nil,
				Reason: "The unpredictable resource attribute was ignored on each resource that carried it.",
			},
		},
		{
			name: "ignore-resource-order",
			compareOptions: []CompareMetricsOption{
				IgnoreResourceMetricsOrder(),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[node_id:BB903] expected at index 1, found at index 2"),
					errors.New("ResourceMetrics with attributes map[node_id:BB904] expected at index 2, found at index 1"),
				),
				Reason: "Resource order mismatch will cause failures if not ignored.",
			},
			withOptions: internal.Expectation{
				Err:    nil,
				Reason: "Ignored resource order mismatch should not cause a failure.",
			},
		},
		{
			name: "ignore-one-resource-attribute-multiple-resources",
			compareOptions: []CompareMetricsOption{
				IgnoreResourceAttributeValue("node_id"),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("missing expected resource with attributes: map[namespace:BB902-test node_id:BB902-expected]"),
					errors.New("missing expected resource with attributes: map[namespace:BB904-test node_id:BB904-expected]"),
					errors.New("missing expected resource with attributes: map[namespace:BB903-test node_id:BB903-expected]"),
					errors.New("extra resource with attributes: map[namespace:BB902-test node_id:BB902-actual]"),
					errors.New("extra resource with attributes: map[namespace:BB904-test node_id:BB904-actual]"),
					errors.New("extra resource with attributes: map[namespace:BB903-test node_id:BB903-actual]"),
				),
				Reason: "An unpredictable resource attribute will cause failures if not ignored.",
			},
			withOptions: internal.Expectation{
				Err:    nil,
				Reason: "The unpredictable resource attribute was ignored on each resource that carried it, but the predictable attributes were preserved.",
			},
		},
		{
			name: "ignore-metrics-order",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricsOrder(),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[namespace:test] does not match expected"),
					errors.New(`ScopeMetrics with scope name "otelcol/aerospikereceiver" does not match expected`),
					errors.New("metric aerospike.namespace.memory.free expected at index 0, found at index 2"),
					errors.New("metric aerospike.namespace.memory.usage expected at index 1, found at index 3"),
					errors.New("metric aerospike.namespace.disk.available expected at index 2, found at index 1"),
					errors.New("metric aerospike.namespace.scan.count expected at index 3, found at index 0"),
				),
				Reason: "metrics with different order should cause a failure.",
			},
			withOptions: internal.Expectation{
				Err:    nil,
				Reason: "metrics with different order should not cause a failure if IgnoreMetricsOrder is applied.",
			},
		},
		{
			name: "ignore-data-points-order",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricDataPointsOrder(),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[namespace:test] does not match expected"),
					errors.New(`ScopeMetrics with scope name "otelcol/aerospikereceiver" does not match expected`),
					errors.New(`metric aerospike.namespace.scan.count does not match expected`),
					errors.New("datapoints are out of order, datapoint with attributes map[result:complete type:aggr] expected at index 1, found at index 2"),
					errors.New("datapoints are out of order, datapoint with attributes map[result:error type:aggr] expected at index 2, found at index 3"),
					errors.New("datapoints are out of order, datapoint with attributes map[result:abort type:basic] expected at index 3, found at index 1"),
				),
				Reason: "datapoints with different order should cause a failure.",
			},
			withOptions: internal.Expectation{
				Err:    nil,
				Reason: "datapoints with different order should not cause a failure if IgnoreMetricsOrder is applied.",
			},
		},
		{
			name: "ignore-each-attribute-value",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricAttributeValue("hostname", "gauge.one", "sum.one"),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric gauge.one does not match expected`),
					errors.New("metric missing expected datapoint with attributes: map[attribute.two:value A hostname:unpredictable]"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.two:value B hostname:unpredictable]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.two:value A hostname:random]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.two:value B hostname:random]"),
				),
				Reason: "An unpredictable attribute will cause failures if not ignored.",
			},
			withOptions: internal.Expectation{
				Err:    nil,
				Reason: "The unpredictable attribute was ignored on each metric that carried it.",
			},
		},
		{
			name: "ignore-attribute-set-collision",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricAttributeValue("attribute.one"),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric gauge.one does not match expected`),
					errors.New("metric missing expected datapoint with attributes: map[attribute.one:one attribute.two:same]"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.one:two attribute.two:same]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.one:random.one attribute.two:same]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.one:random.two attribute.two:same]"),
				),
				Reason: "An unpredictable attribute will cause failures if not ignored.",
			},
			withOptions: internal.Expectation{
				Err:    nil,
				Reason: "Ignoring the unpredictable attribute caused an attribute set collision, but comparison still works.",
			},
		},
		{
			name: "ignore-attribute-set-collision-order",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricAttributeValue("attribute.one"),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceMetrics with attributes map[] does not match expected"),
					errors.New(`ScopeMetrics with scope name "" does not match expected`),
					errors.New(`metric gauge.one does not match expected`),
					errors.New("metric missing expected datapoint with attributes: map[attribute.one:unpredictable.one attribute.two:same]"),
					errors.New("metric missing expected datapoint with attributes: map[attribute.one:unpredictable.two attribute.two:same]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.one:random.two attribute.two:same]"),
					errors.New("metric has extra datapoint with attributes: map[attribute.one:random.one attribute.two:same]"),
				),
				Reason: "An unpredictable attribute will cause failures if not ignored.",
			},
			withOptions: internal.Expectation{
				Err: nil,
				Reason: "Ignoring the unpredictable attribute caused an attribute set collision where the data point values " +
					"where in different orders in expected vs actual, but comparison ignores order.",
			},
		},
		{
			name: "exemplar",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join("testdata", tc.name)

			expected, err := golden.ReadMetrics(filepath.Join(dir, "expected.json"))
			require.NoError(t, err)

			actual, err := golden.ReadMetrics(filepath.Join(dir, "actual.json"))
			require.NoError(t, err)

			err = CompareMetrics(expected, actual)
			tc.withoutOptions.Validate(t, err)

			if tc.compareOptions == nil {
				return
			}

			err = CompareMetrics(expected, actual, tc.compareOptions...)
			tc.withOptions.Validate(t, err)
		})
	}
}
