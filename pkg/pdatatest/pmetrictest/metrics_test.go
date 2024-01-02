// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetrictest

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
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
			withoutOptions: errors.New("number of resources doesn't match expected: 1, actual: 2"),
		},
		{
			name:           "resource-missing",
			withoutOptions: errors.New("number of resources doesn't match expected: 2, actual: 1"),
		},
		{
			name: "resource-attributes-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("missing expected resource: map[type:two]"),
				errors.New("unexpected resource: map[type:three]"),
			),
		},
		{
			name:           "scope-extra",
			withoutOptions: errors.New(`resource "map[]": number of scopes doesn't match expected: 1, actual: 2`),
		},
		{
			name:           "scope-missing",
			withoutOptions: errors.New(`resource "map[]": number of scopes doesn't match expected: 2, actual: 1`),
		},
		{
			name: "scope-name-mismatch",
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[]": missing expected scope: one`),
				errors.New(`resource "map[]": unexpected scope: two`),
			),
		},
		{
			name:           "scope-version-mismatch",
			withoutOptions: errors.New(`resource "map[]": scope "one": version doesn't match expected: 1.0, actual: 2.0`),
		},
		{
			name:           "metric-slice-extra",
			withoutOptions: errors.New(`resource "map[]": scope "": number of metrics doesn't match expected: 1, actual: 2`),
		},
		{
			name:           "metric-slice-missing",
			withoutOptions: errors.New(`resource "map[]": scope "": number of metrics doesn't match expected: 1, actual: 0`),
		},
		{
			name:           "metric-type-expect-gauge",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "should.be.gauge": type doesn't match expected: Gauge, actual: Sum`),
		},
		{
			name:           "metric-type-expect-sum",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "should.be.sum": type doesn't match expected: Sum, actual: Gauge`),
		},
		{
			name: "metric-name-mismatch",
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[]": scope "": missing expected metric: expected.name`),
				errors.New(`resource "map[]": scope "": unexpected metric: wrong.name`),
			),
		},
		{
			name:           "metric-description-mismatch",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "gauge.one": description doesn't match expected: Gauge One, actual: Gauge Two`),
		},
		{
			name:           "metric-unit-mismatch",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "gauge.one": unit doesn't match expected: By, actual: 1`),
		},
		{
			name:           "data-point-slice-extra",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "gauge.one": number of datapoints doesn't match expected: 1, actual: 2`),
		},
		{
			name:           "data-point-slice-missing",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "sum.one": number of datapoints doesn't match expected: 2, actual: 1`),
		},
		{
			name: "data-point-slice-dedup",
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[]": scope "": metric "sum.one": missing expected datapoint: map[attribute.one:two]`),
				errors.New(`resource "map[]": scope "": metric "sum.one": unexpected datapoint: map[attribute.one:one]`),
			),
		},
		{
			name: "data-point-attribute-extra",
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[]": scope "": metric "gauge.one": missing expected datapoint: map[attribute.one:one]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": unexpected datapoint: map[attribute.one:one attribute.two:two]`),
			),
		},
		{
			name: "data-point-attribute-missing",
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[]": scope "": metric "sum.one": missing expected datapoint: map[attribute.one:one attribute.two:two]`),
				errors.New(`resource "map[]": scope "": metric "sum.one": unexpected datapoint: map[attribute.two:two]`),
			),
		},
		{
			name: "data-point-attribute-key",
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[]": scope "": metric "sum.one": missing expected datapoint: map[attribute.one:one]`),
				errors.New(`resource "map[]": scope "": metric "sum.one": unexpected datapoint: map[attribute.two:one]`),
			),
		},
		{
			name: "data-point-attribute-value",
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[]": scope "": metric "gauge.one": missing expected datapoint: map[attribute.one:one]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": unexpected datapoint: map[attribute.one:two]`),
			),
		},
		{
			name:           "data-point-aggregation-expect-delta",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "delta.one": aggregation temporality doesn't match expected: Delta, actual: Cumulative`),
		},
		{
			name:           "data-point-aggregation-expect-cumulative",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "delta.one": aggregation temporality doesn't match expected: Cumulative, actual: Delta`),
		},
		{
			name:           "data-point-monotonic-expect-true",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "monotonic": is monotonic doesn't match expected: true, actual: false`),
		},
		{
			name:           "data-point-monotonic-expect-false",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "nonmonotonic": is monotonic doesn't match expected: false, actual: true`),
		},
		{
			name:           "data-point-value-double-mismatch",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "gauge.one": datapoint "map[]": double value doesn't match expected: 123.456000, actual: 654.321000`),
		},
		{
			name:           "data-point-value-int-mismatch",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "sum.one": datapoint "map[]": int value doesn't match expected: 123, actual: 654`),
		},
		{
			name:           "data-point-value-expect-int",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "gauge.one": datapoint "map[]": value type doesn't match expected: Int, actual: Double`),
		},
		{
			name:           "data-point-value-expect-double",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "gauge.one": datapoint "map[]": value type doesn't match expected: Double, actual: Int`),
		},
		{
			name:           "data-point-flags-mismatch",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "sum.one": datapoint "map[]": flags don't match expected: 1, actual: 0`),
		},
		{
			name:           "histogram-data-point-count-mismatch",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "histogram.one": datapoint "map[]": count doesn't match expected: 123, actual: 654`),
		},
		{
			name:           "histogram-data-point-sum-mismatch",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "histogram.one": datapoint "map[]": sum doesn't match expected: 123.456000, actual: 654.321000`),
		},
		{
			name:           "histogram-data-point-buckets-mismatch",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "histogram.one": datapoint "map[]": bucket counts don't match expected: [1 2 3], actual: [3 2 1]`),
		},
		{
			name:           "exp-histogram-data-point-count-mismatch",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "exponential_histogram.one": datapoint "map[]": count doesn't match expected: 123, actual: 654`),
		},
		{
			name:           "exp-histogram-data-point-sum-mismatch",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "exponential_histogram.one": datapoint "map[]": sum doesn't match expected: 123.456000, actual: 654.321000`),
		},
		{
			name:           "exp-histogram-data-point-positive-buckets-mismatch",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "exponential_histogram.one": datapoint "map[]": positive bucket counts don't match expected: [1 2 3], actual: [3 2 1]`),
		},
		{
			name:           "exp-histogram-data-point-negative-offset-mismatch",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "exponential_histogram.one": datapoint "map[]": negative offset doesn't match expected: 10, actual: 1`),
		},
		{
			name:           "summary-data-point-count-mismatch",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "summary.one": datapoint "map[]": count doesn't match expected: 123, actual: 654`),
		},
		{
			name:           "summary-data-point-sum-mismatch",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "summary.one": datapoint "map[]": sum doesn't match expected: 123.456000, actual: 654.321000`),
		},
		{
			name:           "summary-data-point-quantile-values-length-mismatch",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "summary.one": datapoint "map[]": quantile values length doesn't match expected: 3, actual: 2`),
		},
		{
			name:           "summary-data-point-quantile-values-mismatch",
			withoutOptions: errors.New(`resource "map[]": scope "": metric "summary.one": datapoint "map[]": value at quantile 0.990000 doesn't match expected: 99.000000, actual: 110.000000`),
		},
		{
			name: "ignore-start-timestamp",
			compareOptions: []CompareMetricsOption{
				IgnoreStartTimestamp(),
			},
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[]": scope "": metric "gauge.one": datapoint "map[]": start timestamp doesn't match expected: 0, actual: 11651379494838206464`),
				errors.New(`resource "map[]": scope "": metric "sum.one": datapoint "map[]": start timestamp doesn't match expected: 0, actual: 11651379494838206464`),
			),
		},
		{
			name: "ignore-timestamp",
			compareOptions: []CompareMetricsOption{
				IgnoreTimestamp(),
			},
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[]": scope "": metric "gauge.one": datapoint "map[]": timestamp doesn't match expected: 0, actual: 11651379494838206464`),
				errors.New(`resource "map[]": scope "": metric "sum.one": datapoint "map[]": timestamp doesn't match expected: 0, actual: 11651379494838206464`),
			),
		},
		{
			name: "ignore-data-point-value-double-mismatch",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricValues(),
			},
			withoutOptions: errors.New(`resource "map[]": scope "": metric "gauge.one": datapoint "map[]": double value doesn't match expected: 123.456000, actual: 654.321000`),
		},
		{
			name: "ignore-data-point-value-int-mismatch",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricValues(),
			},
			withoutOptions: errors.New(`resource "map[]": scope "": metric "sum.one": datapoint "map[]": int value doesn't match expected: 123, actual: 654`),
		},
		{
			name: "ignore-subsequent-data-points-all",
			compareOptions: []CompareMetricsOption{
				IgnoreSubsequentDataPoints(),
			},
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[]": scope "": metric "sum.one": number of datapoints doesn't match expected: 1, actual: 2`),
				errors.New(`resource "map[]": scope "": metric "sum.two": number of datapoints doesn't match expected: 1, actual: 2`),
			),
		},
		{
			name: "ignore-subsequent-data-points-one",
			compareOptions: []CompareMetricsOption{
				IgnoreSubsequentDataPoints("sum.one"),
			},
			withoutOptions: errors.New(`resource "map[]": scope "": metric "sum.one": number of datapoints doesn't match expected: 1, actual: 2`),
		},
		{
			name: "ignore-single-metric",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricValues("sum.two"),
			},
			withoutOptions: errors.New(`resource "map[]": scope "": metric "sum.two": datapoint "map[]": int value doesn't match expected: 123, actual: 654`),
		},
		{
			name: "ignore-global-attribute-value",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricAttributeValue("hostname"),
			},
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[]": scope "": metric "gauge.one": missing expected datapoint: map[attribute.two:value A hostname:unpredictable]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": missing expected datapoint: map[attribute.two:value B hostname:unpredictable]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": unexpected datapoint: map[attribute.two:value A hostname:random]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": unexpected datapoint: map[attribute.two:value B hostname:random]`),
				errors.New(`resource "map[]": scope "": metric "sum.one": missing expected datapoint: map[hostname:unpredictable]`),
				errors.New(`resource "map[]": scope "": metric "sum.one": unexpected datapoint: map[hostname:randomvalue]`),
			),
			withOptions: nil,
		},
		{
			name: "ignore-one-attribute-value",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricAttributeValue("hostname", "gauge.one"),
			},
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[]": scope "": metric "gauge.one": missing expected datapoint: map[attribute.two:value A hostname:unpredictable]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": missing expected datapoint: map[attribute.two:value B hostname:unpredictable]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": unexpected datapoint: map[attribute.two:value A hostname:random]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": unexpected datapoint: map[attribute.two:value B hostname:random]`),
				errors.New(`resource "map[]": scope "": metric "sum.one": missing expected datapoint: map[hostname:also unpredictable]`),
				errors.New(`resource "map[]": scope "": metric "sum.one": unexpected datapoint: map[hostname:also random]`),
			),
			withOptions: multierr.Combine(
				errors.New(`resource "map[]": scope "": metric "sum.one": missing expected datapoint: map[hostname:also unpredictable]`),
				errors.New(`resource "map[]": scope "": metric "sum.one": unexpected datapoint: map[hostname:also random]`),
			),
		},
		{
			name: "match-one-attribute-value",
			compareOptions: []CompareMetricsOption{
				MatchMetricAttributeValue("hostname", "also", "gauge.one", "sum.one"),
			},
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[]": scope "": metric "gauge.one": missing expected datapoint: map[attribute.two:value A hostname:unpredictable]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": missing expected datapoint: map[attribute.two:value B hostname:unpredictable]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": unexpected datapoint: map[attribute.two:value A hostname:random]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": unexpected datapoint: map[attribute.two:value B hostname:random]`),
				errors.New(`resource "map[]": scope "": metric "sum.one": missing expected datapoint: map[hostname:also unpredictable]`),
				errors.New(`resource "map[]": scope "": metric "sum.one": unexpected datapoint: map[hostname:also random]`),
			),
			withOptions: multierr.Combine(
				errors.New(`resource "map[]": scope "": metric "gauge.one": missing expected datapoint: map[attribute.two:value A hostname:unpredictable]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": missing expected datapoint: map[attribute.two:value B hostname:unpredictable]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": unexpected datapoint: map[attribute.two:value A hostname:random]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": unexpected datapoint: map[attribute.two:value B hostname:random]`),
			),
		},
		{
			name: "ignore-one-resource-attribute",
			compareOptions: []CompareMetricsOption{
				IgnoreResourceAttributeValue("node_id"),
			},
			withoutOptions: multierr.Combine(
				errors.New("missing expected resource: map[node_id:a-different-random-id]"),
				errors.New("unexpected resource: map[node_id:a-random-id]"),
			),
			withOptions: nil,
		},
		{
			name: "match-one-resource-attribute",
			compareOptions: []CompareMetricsOption{
				MatchResourceAttributeValue("node_id", "random"),
			},
			withoutOptions: multierr.Combine(
				errors.New("missing expected resource: map[node_id:a-different-random-id]"),
				errors.New("unexpected resource: map[node_id:a-random-id]"),
			),
			withOptions: nil,
		},
		{
			name: "ignore-resource-order",
			compareOptions: []CompareMetricsOption{
				IgnoreResourceMetricsOrder(),
			},
			withoutOptions: multierr.Combine(
				errors.New(`resources are out of order: resource "map[node_id:BB903]" expected at index 1, found at index 2`),
				errors.New(`resources are out of order: resource "map[node_id:BB904]" expected at index 2, found at index 1`),
			),
			withOptions: nil,
		},
		{
			name: "ignore-one-resource-attribute-multiple-resources",
			compareOptions: []CompareMetricsOption{
				IgnoreResourceAttributeValue("node_id"),
			},
			withoutOptions: multierr.Combine(
				errors.New("missing expected resource: map[namespace:BB902-test node_id:BB902-expected]"),
				errors.New("missing expected resource: map[namespace:BB904-test node_id:BB904-expected]"),
				errors.New("missing expected resource: map[namespace:BB903-test node_id:BB903-expected]"),
				errors.New("unexpected resource: map[namespace:BB902-test node_id:BB902-actual]"),
				errors.New("unexpected resource: map[namespace:BB904-test node_id:BB904-actual]"),
				errors.New("unexpected resource: map[namespace:BB903-test node_id:BB903-actual]"),
			),
			withOptions: nil,
		},
		{
			name: "ignore-metrics-order",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricsOrder(),
			},
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[namespace:test]": scope "otelcol/aerospikereceiver": metrics are out of order: metric "aerospike.namespace.memory.free" expected at index 0, found at index 2`),
				errors.New(`resource "map[namespace:test]": scope "otelcol/aerospikereceiver": metrics are out of order: metric "aerospike.namespace.memory.usage" expected at index 1, found at index 3`),
				errors.New(`resource "map[namespace:test]": scope "otelcol/aerospikereceiver": metrics are out of order: metric "aerospike.namespace.disk.available" expected at index 2, found at index 1`),
				errors.New(`resource "map[namespace:test]": scope "otelcol/aerospikereceiver": metrics are out of order: metric "aerospike.namespace.scan.count" expected at index 3, found at index 0`),
			),
			withOptions: nil,
		},
		{
			name: "ignore-data-points-order",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricDataPointsOrder(),
			},
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[namespace:test]": scope "otelcol/aerospikereceiver": metric "aerospike.namespace.scan.count": datapoints are out of order: datapoint "map[result:complete type:aggr]" expected at index 1, found at index 2`),
				errors.New(`resource "map[namespace:test]": scope "otelcol/aerospikereceiver": metric "aerospike.namespace.scan.count": datapoints are out of order: datapoint "map[result:error type:aggr]" expected at index 2, found at index 3`),
				errors.New(`resource "map[namespace:test]": scope "otelcol/aerospikereceiver": metric "aerospike.namespace.scan.count": datapoints are out of order: datapoint "map[result:abort type:basic]" expected at index 3, found at index 1`),
			),
			withOptions: nil,
		},
		{
			name: "ignore-each-attribute-value",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricAttributeValue("hostname", "gauge.one", "sum.one"),
			},
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[]": scope "": metric "gauge.one": missing expected datapoint: map[attribute.two:value A hostname:unpredictable]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": missing expected datapoint: map[attribute.two:value B hostname:unpredictable]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": unexpected datapoint: map[attribute.two:value A hostname:random]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": unexpected datapoint: map[attribute.two:value B hostname:random]`),
				errors.New(`resource "map[]": scope "": metric "sum.one": missing expected datapoint: map[hostname:unpredictable]`),
				errors.New(`resource "map[]": scope "": metric "sum.one": unexpected datapoint: map[hostname:randomvalue]`),
			),
			withOptions: nil,
		},
		{
			name: "ignore-attribute-set-collision",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricAttributeValue("attribute.one"),
			},
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[]": scope "": metric "gauge.one": missing expected datapoint: map[attribute.one:one attribute.two:same]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": missing expected datapoint: map[attribute.one:two attribute.two:same]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": unexpected datapoint: map[attribute.one:random.one attribute.two:same]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": unexpected datapoint: map[attribute.one:random.two attribute.two:same]`),
			),
			withOptions: nil,
		},
		{
			name: "ignore-attribute-set-collision-order",
			compareOptions: []CompareMetricsOption{
				IgnoreMetricAttributeValue("attribute.one"),
			},
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[]": scope "": metric "gauge.one": missing expected datapoint: map[attribute.one:unpredictable.one attribute.two:same]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": missing expected datapoint: map[attribute.one:unpredictable.two attribute.two:same]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": unexpected datapoint: map[attribute.one:random.two attribute.two:same]`),
				errors.New(`resource "map[]": scope "": metric "gauge.one": unexpected datapoint: map[attribute.one:random.one attribute.two:same]`),
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

			expected, err := golden.ReadMetrics(filepath.Join(dir, "expected.yaml"))
			require.NoError(t, err)

			actual, err := golden.ReadMetrics(filepath.Join(dir, "actual.yaml"))
			require.NoError(t, err)

			err = CompareMetrics(expected, actual)
			if tc.withoutOptions == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, tc.withoutOptions, err.Error())
			}

			if tc.compareOptions == nil {
				return
			}

			err = CompareMetrics(expected, actual, tc.compareOptions...)
			if tc.withOptions == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.withOptions.Error())
			}
		})
	}
}
