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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
)

func TestCompareMetrics(t *testing.T) {
	tcs := []struct {
		name           string
		compareOptions []CompareMetricsOption
		withoutOptions error
		withOptions    error
	}{
		{
			name: "equal",
		},
		{
			name:           "resource-extra",
			withoutOptions: errors.New("number of resources does not match expected: 1, actual: 2"),
		},
		{
			name:           "resource-missing",
			withoutOptions: errors.New("number of resources does not match expected: 2, actual: 1"),
		},
		{
			name: "resource-attributes-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("missing expected resource with attributes: map[type:two]"),
				errors.New("extra resource with attributes: map[type:three]"),
			),
		},
		{
			name: "resource-instrumentation-library-extra",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New("number of ScopeMetrics does not match expected: 1, actual: 2"),
			),
		},
		{
			name: "resource-instrumentation-library-missing",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New("number of ScopeMetrics does not match expected: 2, actual: 1"),
			),
		},
		{
			name: "resource-instrumentation-library-name-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New("missing expected ScopeMetrics with scope name: one"),
				errors.New("extra ScopeMetrics with scope name: two"),
			),
		},
		{
			name: "resource-instrumentation-library-version-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "one" does not match expected`),
				errors.New("scope Version does not match expected: 1.0, actual: 2.0"),
			),
		},
		{
			name: "metric-slice-extra",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New("number of metrics does not match expected: 1, actual: 2"),
			),
		},
		{
			name: "metric-slice-missing",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New("number of metrics does not match expected: 1, actual: 0"),
			),
		},
		{
			name: "metric-type-expect-gauge",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric should.be.gauge does not match expected`),
				errors.New("metric DataType does not match expected: Gauge, actual: Sum"),
			),
		},
		{
			name: "metric-type-expect-sum",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric should.be.sum does not match expected`),
				errors.New("metric DataType does not match expected: Sum, actual: Gauge"),
			),
		},
		{
			name: "metric-name-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New("missing expected metric: expected.name"),
				errors.New("unexpected metric: wrong.name"),
			),
		},
		{
			name: "metric-description-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric gauge.one does not match expected`),
				errors.New("metric Description does not match expected: Gauge One, actual: Gauge Two"),
			),
		},
		{
			name: "metric-unit-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric gauge.one does not match expected`),
				errors.New("metric Unit does not match expected: By, actual: 1"),
			),
		},
		{
			name: "data-point-slice-extra",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric gauge.one does not match expected`),
				errors.New("number of datapoints does not match expected: 1, actual: 2"),
			),
		},
		{
			name: "data-point-slice-missing",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric sum.one does not match expected`),
				errors.New("number of datapoints does not match expected: 2, actual: 1"),
			),
		},
		{
			name: "data-point-slice-dedup",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric sum.one does not match expected`),
				errors.New("metric missing expected datapoint with attributes: map[attribute.one:two]"),
				errors.New("metric has extra datapoint with attributes: map[attribute.one:one]"),
			),
		},
		{
			name: "data-point-attribute-extra",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric gauge.one does not match expected`),
				errors.New("metric missing expected datapoint with attributes: map[attribute.one:one]"),
				errors.New("metric has extra datapoint with attributes: map[attribute.one:one attribute.two:two]"),
			),
		},
		{
			name: "data-point-attribute-missing",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric sum.one does not match expected`),
				errors.New("metric missing expected datapoint with attributes: map[attribute.one:one attribute.two:two]"),
				errors.New("metric has extra datapoint with attributes: map[attribute.two:two]"),
			),
		},
		{
			name: "data-point-attribute-key",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric sum.one does not match expected`),
				errors.New("metric missing expected datapoint with attributes: map[attribute.one:one]"),
				errors.New("metric has extra datapoint with attributes: map[attribute.two:one]"),
			),
		},
		{
			name: "data-point-attribute-value",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric gauge.one does not match expected`),
				errors.New("metric missing expected datapoint with attributes: map[attribute.one:one]"),
				errors.New("metric has extra datapoint with attributes: map[attribute.one:two]"),
			),
		},
		{
			name: "data-point-aggregation-expect-delta",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric delta.one does not match expected`),
				errors.New("metric AggregationTemporality does not match expected: Delta, actual: Cumulative"),
			),
		},
		{
			name: "data-point-aggregation-expect-cumulative",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric delta.one does not match expected`),
				errors.New("metric AggregationTemporality does not match expected: Cumulative, actual: Delta"),
			),
		},
		{
			name: "data-point-monotonic-expect-true",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric monotonic does not match expected`),
				errors.New("metric IsMonotonic does not match expected: true, actual: false"),
			),
		},
		{
			name: "data-point-monotonic-expect-false",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric nonmonotonic does not match expected`),
				errors.New("metric IsMonotonic does not match expected: false, actual: true"),
			),
		},
		{
			name: "data-point-value-double-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric gauge.one does not match expected`),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint DoubleVal doesn't match expected: 123.456000, actual: 654.321000"),
			),
		},
		{
			name: "data-point-value-int-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric sum.one does not match expected`),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint IntVal doesn't match expected: 123, actual: 654"),
			),
		},
		{
			name: "data-point-value-expect-int",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric gauge.one does not match expected`),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint types don't match: expected type: Int, actual type: Double"),
			),
		},
		{
			name: "data-point-value-expect-double",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric gauge.one does not match expected`),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint types don't match: expected type: Double, actual type: Int"),
			),
		},
		{
			name: "histogram-data-point-count-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric histogram.one does not match expected`),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint Count doesn't match expected: 123, actual: 654"),
			),
		},
		{
			name: "histogram-data-point-sum-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric histogram.one does not match expected`),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint Sum doesn't match expected: 123.456000, actual: 654.321000"),
			),
		},
		{
			name: "histogram-data-point-buckets-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric histogram.one does not match expected`),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint BucketCounts doesn't match expected: [1 2 3], actual: [3 2 1]"),
			),
		},
		{
			name: "exp-histogram-data-point-count-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric exponential_histogram.one does not match expected`),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint Count doesn't match expected: 123, actual: 654"),
			),
		},
		{
			name: "exp-histogram-data-point-sum-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric exponential_histogram.one does not match expected`),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint Sum doesn't match expected: 123.456000, actual: 654.321000"),
			),
		},
		{
			name: "exp-histogram-data-point-positive-buckets-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric exponential_histogram.one does not match expected`),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint Positive BucketCounts doesn't match expected: [1 2 3], "+
					"actual: [3 2 1]"),
			),
		},
		{
			name: "exp-histogram-data-point-negative-offset-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric exponential_histogram.one does not match expected`),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint Negative Offset doesn't match expected: 10, actual: 1"),
			),
		},
		{
			name: "summary-data-point-count-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric summary.one does not match expected`),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint Count doesn't match expected: 123, actual: 654"),
			),
		},
		{
			name: "summary-data-point-sum-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric summary.one does not match expected`),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint Sum doesn't match expected: 123.456000, actual: 654.321000"),
			),
		},
		{
			name: "summary-data-point-quantile-values-length-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric summary.one does not match expected`),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint QuantileValues length doesn't match expected: 3, actual: 2"),
			),
		},
		{
			name: "summary-data-point-quantile-values-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric summary.one does not match expected`),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint value at quantile 0.990000 doesn't match expected: 99.000000, "+
					"actual: 110.000000"),
			),
		},
		{
			name: "ignore-start-timestamp",
			compareOptions: []CompareMetricsOption{
				IgnoreStartTimestamp(),
			},
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New("metric gauge.one does not match expected"),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint StartTimestamp doesn't match expected: 0, actual: 11651379494838206464"),
			),
		},
		{
			name: "ignore-timestamp",
			compareOptions: []CompareMetricsOption{
				IgnoreTimestamp(),
			},
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New("metric gauge.one does not match expected"),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint Timestamp doesn't match expected: 0, actual: 11651379494838206464"),
			),
		},
		{
			name: "ignore-data-point-value-double-mismatch",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricValues(),
			},
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric gauge.one does not match expected`),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint DoubleVal doesn't match expected: 123.456000, actual: 654.321000"),
			),
		},
		{
			name: "ignore-data-point-value-int-mismatch",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricValues(),
			},
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric sum.one does not match expected`),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint IntVal doesn't match expected: 123, actual: 654"),
			),
		},
		{
			name: "ignore-subsequent-data-points-all",
			compareOptions: []CompareMetricsOption{
				IgnoreSubsequentDataPoints(),
			},
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric sum.one does not match expected`),
				errors.New("number of datapoints does not match expected: 1, actual: 2"),
			),
		},
		{
			name: "ignore-subsequent-data-points-one",
			compareOptions: []CompareMetricsOption{
				IgnoreSubsequentDataPoints("sum.one"),
			},
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric sum.one does not match expected`),
				errors.New("number of datapoints does not match expected: 1, actual: 2"),
			),
		},
		{
			name: "ignore-single-metric",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricValues("sum.two"),
			},
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric sum.two does not match expected`),
				errors.New("datapoint with attributes: map[], does not match expected"),
				errors.New("metric datapoint IntVal doesn't match expected: 123, actual: 654"),
			),
		},
		{
			name: "ignore-global-attribute-value",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricAttributeValue("hostname"),
			},
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric gauge.one does not match expected`),
				errors.New("metric missing expected datapoint with attributes: map[attribute.two:value A hostname:unpredictable]"),
				errors.New("metric missing expected datapoint with attributes: map[attribute.two:value B hostname:unpredictable]"),
				errors.New("metric has extra datapoint with attributes: map[attribute.two:value A hostname:random]"),
				errors.New("metric has extra datapoint with attributes: map[attribute.two:value B hostname:random]"),
			),
			withOptions: nil,
		},
		{
			name: "ignore-one-attribute-value",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricAttributeValue("hostname", "gauge.one"),
			},
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric gauge.one does not match expected`),
				errors.New("metric missing expected datapoint with attributes: map[attribute.two:value A hostname:unpredictable]"),
				errors.New("metric missing expected datapoint with attributes: map[attribute.two:value B hostname:unpredictable]"),
				errors.New("metric has extra datapoint with attributes: map[attribute.two:value A hostname:random]"),
				errors.New("metric has extra datapoint with attributes: map[attribute.two:value B hostname:random]"),
			),
			withOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric sum.one does not match expected`),
				errors.New("metric missing expected datapoint with attributes: map[hostname:also unpredictable]"),
				errors.New("metric has extra datapoint with attributes: map[hostname:also random]"),
			),
		},
		{
			name: "ignore-one-resource-attribute",
			compareOptions: []CompareMetricsOption{
				IgnoreResourceAttributeValue("node_id"),
			},
			withoutOptions: multierr.Combine(
				errors.New("missing expected resource with attributes: map[node_id:a-different-random-id]"),
				errors.New("extra resource with attributes: map[node_id:a-random-id]"),
			),
			withOptions: nil,
		},
		{
			name: "ignore-resource-order",
			compareOptions: []CompareMetricsOption{
				IgnoreResourceMetricsOrder(),
			},
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[node_id:BB903] expected at index 1, found at index 2"),
				errors.New("ResourceMetrics with attributes map[node_id:BB904] expected at index 2, found at index 1"),
			),
			withOptions: nil,
		},
		{
			name: "ignore-one-resource-attribute-multiple-resources",
			compareOptions: []CompareMetricsOption{
				IgnoreResourceAttributeValue("node_id"),
			},
			withoutOptions: multierr.Combine(
				errors.New("missing expected resource with attributes: map[namespace:BB902-test node_id:BB902-expected]"),
				errors.New("missing expected resource with attributes: map[namespace:BB904-test node_id:BB904-expected]"),
				errors.New("missing expected resource with attributes: map[namespace:BB903-test node_id:BB903-expected]"),
				errors.New("extra resource with attributes: map[namespace:BB902-test node_id:BB902-actual]"),
				errors.New("extra resource with attributes: map[namespace:BB904-test node_id:BB904-actual]"),
				errors.New("extra resource with attributes: map[namespace:BB903-test node_id:BB903-actual]"),
			),
			withOptions: nil,
		},
		{
			name: "ignore-metrics-order",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricsOrder(),
			},
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[namespace:test] does not match expected"),
				errors.New(`ScopeMetrics with scope name "otelcol/aerospikereceiver" does not match expected`),
				errors.New("metric aerospike.namespace.memory.free expected at index 0, found at index 2"),
				errors.New("metric aerospike.namespace.memory.usage expected at index 1, found at index 3"),
				errors.New("metric aerospike.namespace.disk.available expected at index 2, found at index 1"),
				errors.New("metric aerospike.namespace.scan.count expected at index 3, found at index 0"),
			),
			withOptions: nil,
		},
		{
			name: "ignore-data-points-order",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricDataPointsOrder(),
			},
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[namespace:test] does not match expected"),
				errors.New(`ScopeMetrics with scope name "otelcol/aerospikereceiver" does not match expected`),
				errors.New(`metric aerospike.namespace.scan.count does not match expected`),
				errors.New("datapoints are out of order, datapoint with attributes map[result:complete type:aggr] expected at index 1, found at index 2"),
				errors.New("datapoints are out of order, datapoint with attributes map[result:error type:aggr] expected at index 2, found at index 3"),
				errors.New("datapoints are out of order, datapoint with attributes map[result:abort type:basic] expected at index 3, found at index 1"),
			),
			withOptions: nil,
		},
		{
			name: "ignore-each-attribute-value",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricAttributeValue("hostname", "gauge.one", "sum.one"),
			},
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric gauge.one does not match expected`),
				errors.New("metric missing expected datapoint with attributes: map[attribute.two:value A hostname:unpredictable]"),
				errors.New("metric missing expected datapoint with attributes: map[attribute.two:value B hostname:unpredictable]"),
				errors.New("metric has extra datapoint with attributes: map[attribute.two:value A hostname:random]"),
				errors.New("metric has extra datapoint with attributes: map[attribute.two:value B hostname:random]"),
			),
			withOptions: nil,
		},
		{
			name: "ignore-attribute-set-collision",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricAttributeValue("attribute.one"),
			},
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric gauge.one does not match expected`),
				errors.New("metric missing expected datapoint with attributes: map[attribute.one:one attribute.two:same]"),
				errors.New("metric missing expected datapoint with attributes: map[attribute.one:two attribute.two:same]"),
				errors.New("metric has extra datapoint with attributes: map[attribute.one:random.one attribute.two:same]"),
				errors.New("metric has extra datapoint with attributes: map[attribute.one:random.two attribute.two:same]"),
			),
			withOptions: nil,
		},
		{
			name: "ignore-attribute-set-collision-order",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricAttributeValue("attribute.one"),
			},
			withoutOptions: multierr.Combine(
				errors.New("ResourceMetrics with attributes map[] does not match expected"),
				errors.New(`ScopeMetrics with scope name "" does not match expected`),
				errors.New(`metric gauge.one does not match expected`),
				errors.New("metric missing expected datapoint with attributes: map[attribute.one:unpredictable.one attribute.two:same]"),
				errors.New("metric missing expected datapoint with attributes: map[attribute.one:unpredictable.two attribute.two:same]"),
				errors.New("metric has extra datapoint with attributes: map[attribute.one:random.two attribute.two:same]"),
				errors.New("metric has extra datapoint with attributes: map[attribute.one:random.one attribute.two:same]"),
			),
			withOptions: nil,
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

			assert.Equal(t, tc.withoutOptions, CompareMetrics(expected, actual))

			if tc.compareOptions == nil {
				return
			}

			assert.Equal(t, tc.withOptions, CompareMetrics(expected, actual, tc.compareOptions...))
		})
	}
}
