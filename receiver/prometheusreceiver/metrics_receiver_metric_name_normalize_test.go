// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var normalizeMetric = `# HELP http_connected connected clients
# TYPE http_connected counter
http_connected_total{method="post",port="6380"} 15
http_connected_total{method="get",port="6380"} 12
# HELP foo_gauge_total foo gauge with _total suffix
# TYPE foo_gauge_total gauge
foo_gauge_total{method="post",port="6380"} 7
foo_gauge_total{method="get",port="6380"} 13
# HELP http_connection_duration_seconds connection duration total
# TYPE http_connection_duration_seconds counter
# UNIT http_connection_duration_seconds seconds
http_connection_duration_seconds_total{method="post",port="6380"} 23
http_connection_duration_seconds_total{method="get",port="6380"} 41
# HELP foo_gauge_seconds foo gauge with unit suffix
# UNIT foo_gauge_seconds seconds
# TYPE foo_gauge_seconds gauge
foo_gauge_seconds{method="post",port="6380"} 732
foo_gauge_seconds{method="get",port="6380"} 5
# EOF
`

// TestMetricNormalize validates that type's and unit's suffixes are correctly trimmed.
func TestMetricNormalize(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: normalizeMetric, useOpenMetrics: true},
			},
			normalizedName: true,
			validateFunc:   verifyNormalizeMetric,
		},
	}

	testComponent(t, targets, func(c *Config) {
		c.TrimMetricSuffixes = true
	})
}

func verifyNormalizeMetric(t *testing.T, td *testData, resourceMetrics []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]

	// m1 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m1))

	wantAttributes := td.attributes

	metrics1 := m1.ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)
	e1 := []testExpectation{
		assertMetricPresent("http_connected",
			compareMetricType(pmetric.MetricTypeSum),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(15),
						compareAttributes(map[string]string{"method": "post", "port": "6380"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(12),
						compareAttributes(map[string]string{"method": "get", "port": "6380"}),
					},
				},
			}),
		assertMetricPresent("foo_gauge_total",
			compareMetricType(pmetric.MetricTypeGauge),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(7),
						compareAttributes(map[string]string{"method": "post", "port": "6380"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(13),
						compareAttributes(map[string]string{"method": "get", "port": "6380"}),
					},
				},
			}),
		assertMetricPresent("http_connection_duration",
			compareMetricType(pmetric.MetricTypeSum),
			compareMetricUnit("s"),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(23),
						compareAttributes(map[string]string{"method": "post", "port": "6380"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(41),
						compareAttributes(map[string]string{"method": "get", "port": "6380"}),
					},
				},
			}),
		assertMetricPresent("foo_gauge",
			compareMetricType(pmetric.MetricTypeGauge),
			compareMetricUnit("s"),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(732),
						compareAttributes(map[string]string{"method": "post", "port": "6380"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(5),
						compareAttributes(map[string]string{"method": "get", "port": "6380"}),
					},
				},
			}),
	}
	doCompareNormalized(t, "scrape-metricNormalize-1", wantAttributes, m1, e1, true)
}
