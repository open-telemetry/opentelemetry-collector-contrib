// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/common/promslog"
	promConfig "github.com/prometheus/prometheus/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
)

var tsZero = pcommon.Timestamp(0)

// Test data and validation functions for all four core metrics for Prometheus Receiver.
// Make sure every page has a gauge, we are relying on it to figure out the start time if needed

// target1 has one gauge, two counts of a same family, one histogram and one summary. We are expecting the both
// successful scrapes will produce all metrics using the first scrape's timestamp as start time.
var target1Page1 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 19

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 100
http_requests_total{method="post",code="400"} 5

# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.05"} 1000
http_request_duration_seconds_bucket{le="0.5"} 1500
http_request_duration_seconds_bucket{le="1"} 2000
http_request_duration_seconds_bucket{le="+Inf"} 2500
http_request_duration_seconds_sum 5000
http_request_duration_seconds_count 2500

# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{quantile="0.01"} 1
rpc_duration_seconds{quantile="0.9"} 5
rpc_duration_seconds{quantile="0.99"} 8
rpc_duration_seconds_sum 5000
rpc_duration_seconds_count 1000
`

var target1Page2 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 18

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 199
http_requests_total{method="post",code="400"} 12

# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.05"} 1100
http_request_duration_seconds_bucket{le="0.5"} 1600
http_request_duration_seconds_bucket{le="1"} 2100
http_request_duration_seconds_bucket{le="+Inf"} 2600
http_request_duration_seconds_sum 5050
http_request_duration_seconds_count 2600

# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{quantile="0.01"} 1
rpc_duration_seconds{quantile="0.9"} 6
rpc_duration_seconds{quantile="0.99"} 8
rpc_duration_seconds_sum 5002
rpc_duration_seconds_count 1001
`

// target1Page3 has lower values than previous scrape.
// So, even after seeing a failed scrape, start_timestamp should be reset for target1Page3
var target1Page3 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 16

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 99
http_requests_total{method="post",code="400"} 3

# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.05"} 900
http_request_duration_seconds_bucket{le="0.5"} 1400
http_request_duration_seconds_bucket{le="1"} 1900
http_request_duration_seconds_bucket{le="+Inf"} 2400
http_request_duration_seconds_sum 4900
http_request_duration_seconds_count 2400

# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{quantile="0.01"} 1
rpc_duration_seconds{quantile="0.9"} 4
rpc_duration_seconds{quantile="0.99"} 6
rpc_duration_seconds_sum 4900
rpc_duration_seconds_count 900
`

func verifyTarget1(t *testing.T, td *testData, resourceMetrics []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]

	// m1 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m1))

	wantAttributes := td.attributes

	metrics1 := m1.ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)
	e1 := []metricExpectation{
		{
			"go_threads",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(19),
					},
				},
			},
			nil,
		},
		{
			"http_requests_total",
			pmetric.MetricTypeSum,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(tsZero),
						compareTimestamp(ts1),
						compareDoubleValue(100),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(tsZero),
						compareTimestamp(ts1),
						compareDoubleValue(5),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			},
			nil,
		},
		{
			"http_request_duration_seconds",
			pmetric.MetricTypeHistogram,
			"",
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts1),
						compareHistogram(2500, 5000, []float64{0.05, 0.5, 1}, []uint64{1000, 500, 500, 500}),
					},
				},
			},
			nil,
		},
		{
			"rpc_duration_seconds",
			pmetric.MetricTypeSummary,
			"",
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts1),
						compareSummary(1000, 5000, [][]float64{{0.01, 1}, {0.9, 5}, {0.99, 8}}),
					},
				},
			},
			nil,
		},
	}
	doCompare(t, "scrape1", wantAttributes, m1, e1)

	m2 := resourceMetrics[1]
	// m2 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m2))

	metricsScrape2 := m2.ScopeMetrics().At(0).Metrics()
	ts2 := getTS(metricsScrape2)
	e2 := []metricExpectation{
		{
			"go_threads",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts2),
						compareDoubleValue(18),
					},
				},
			},
			nil,
		},
		{
			"http_requests_total",
			pmetric.MetricTypeSum,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(tsZero),
						compareTimestamp(ts2),
						compareDoubleValue(199),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(tsZero),
						compareTimestamp(ts2),
						compareDoubleValue(12),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			},
			nil,
		},
		{
			"http_request_duration_seconds",
			pmetric.MetricTypeHistogram,
			"",
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						// TODO: Prometheus Receiver Issue- start_timestamp are incorrect for Summary and Histogram metrics after a failed scrape (issue not yet posted on collector-contrib repo)
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts2),
						compareHistogram(2600, 5050, []float64{0.05, 0.5, 1}, []uint64{1100, 500, 500, 500}),
					},
				},
			},
			nil,
		},
		{
			"rpc_duration_seconds",
			pmetric.MetricTypeSummary,
			"",
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						// TODO: Prometheus Receiver Issue- start_timestamp are incorrect for Summary and Histogram metrics after a failed scrape (issue not yet posted on collector-contrib repo)
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts2),
						compareSummary(1001, 5002, [][]float64{{0.01, 1}, {0.9, 6}, {0.99, 8}}),
					},
				},
			},
			nil,
		},
	}
	doCompare(t, "scrape2", wantAttributes, m2, e2)

	m3 := resourceMetrics[2]
	// m3 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m3))
	metricsScrape3 := m3.ScopeMetrics().At(0).Metrics()
	ts3 := getTS(metricsScrape3)
	e3 := []metricExpectation{
		{
			"go_threads",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts3),
						compareDoubleValue(16),
					},
				},
			},
			nil,
		},
		{
			"http_requests_total",
			pmetric.MetricTypeSum,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						// TODO: #6360 Prometheus Receiver Issue- start_timestamp should reset if the prior scrape had higher value
						compareStartTimestamp(tsZero),
						compareTimestamp(ts3),
						compareDoubleValue(99),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						// TODO: #6360 Prometheus Receiver Issue- start_timestamp should reset if the prior scrape had higher value
						compareStartTimestamp(tsZero),
						compareTimestamp(ts3),
						compareDoubleValue(3),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			},
			nil,
		},
		{
			"http_request_duration_seconds",
			pmetric.MetricTypeHistogram,
			"",
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						// TODO: #6360 Prometheus Receiver Issue- start_timestamp should reset if the prior scrape had higher value
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts3),
						compareHistogram(2400, 4900, []float64{0.05, 0.5, 1}, []uint64{900, 500, 500, 500}),
					},
				},
			},
			nil,
		},
		{
			"rpc_duration_seconds",
			pmetric.MetricTypeSummary,
			"",
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						// TODO: #6360 Prometheus Receiver Issue- start_timestamp should reset if the prior scrape had higher value
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts3),
						compareSummary(900, 4900, [][]float64{{0.01, 1}, {0.9, 4}, {0.99, 6}}),
					},
				},
			},
			nil,
		},
	}
	doCompare(t, "scrape3", wantAttributes, m3, e3)
}

// target2 is going to have 5 pages, and there's a newly added item on the 2nd page.
// with the 4th page, we are simulating a reset (values smaller than previous), start times should be from
// this run for the 4th and 5th scrapes.
var target2Page1 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 18

# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{method="post",code="200",le="1"} 8
http_request_duration_seconds_bucket{method="post",code="200",le="+Inf"} 10
http_request_duration_seconds_sum{method="post",code="200"} 7
http_request_duration_seconds_count{method="post",code="200"} 10
http_request_duration_seconds_bucket{method="post",code="400",le="1"} 30
http_request_duration_seconds_bucket{method="post",code="400",le="+Inf"} 50
http_request_duration_seconds_sum{method="post",code="400"} 25
http_request_duration_seconds_count{method="post",code="400"} 50

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 10
http_requests_total{method="post",code="400"} 50

# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{code="0" quantile="0.5"} 47
rpc_duration_seconds_sum{code="0"} 100
rpc_duration_seconds_count{code="0"} 50
rpc_duration_seconds{code="5" quantile="0.5"} 35
rpc_duration_seconds_sum{code="5"} 180
rpc_duration_seconds_count{code="5"} 400
`

var target2Page2 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 16

# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{method="post",code="200",le="1"} 40
http_request_duration_seconds_bucket{method="post",code="200",le="+Inf"} 50
http_request_duration_seconds_sum{method="post",code="200"} 43
http_request_duration_seconds_count{method="post",code="200"} 50
http_request_duration_seconds_bucket{method="post",code="300",le="1"} 3
http_request_duration_seconds_bucket{method="post",code="300",le="+Inf"} 3
http_request_duration_seconds_sum{method="post",code="300"} 2
http_request_duration_seconds_count{method="post",code="300"} 3
http_request_duration_seconds_bucket{method="post",code="400",le="1"} 35
http_request_duration_seconds_bucket{method="post",code="400",le="+Inf"} 60
http_request_duration_seconds_sum{method="post",code="400"} 30
http_request_duration_seconds_count{method="post",code="400"} 60

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 50
http_requests_total{method="post",code="300"} 3
http_requests_total{method="post",code="400"} 60

# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{code="0" quantile="0.5"} 57
rpc_duration_seconds_sum{code="0"} 110
rpc_duration_seconds_count{code="0"} 60
rpc_duration_seconds{code="3" quantile="0.5"} 42
rpc_duration_seconds_sum{code="3"} 50
rpc_duration_seconds_count{code="3"} 30
rpc_duration_seconds{code="5" quantile="0.5"} 45
rpc_duration_seconds_sum{code="5"} 190
rpc_duration_seconds_count{code="5"} 410
`

var target2Page3 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 16

# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{method="post",code="200",le="1"} 40
http_request_duration_seconds_bucket{method="post",code="200",le="+Inf"} 50
http_request_duration_seconds_sum{method="post",code="200"} 43
http_request_duration_seconds_count{method="post",code="200"} 50
http_request_duration_seconds_bucket{method="post",code="300",le="1"} 3
http_request_duration_seconds_bucket{method="post",code="300",le="+Inf"} 5
http_request_duration_seconds_sum{method="post",code="300"} 7
http_request_duration_seconds_count{method="post",code="300"} 5
http_request_duration_seconds_bucket{method="post",code="400",le="1"} 35
http_request_duration_seconds_bucket{method="post",code="400",le="+Inf"} 60
http_request_duration_seconds_sum{method="post",code="400"} 30
http_request_duration_seconds_count{method="post",code="400"} 60

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 50
http_requests_total{method="post",code="300"} 5
http_requests_total{method="post",code="400"} 60

# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{code="0" quantile="0.5"} 67
rpc_duration_seconds_sum{code="0"} 120
rpc_duration_seconds_count{code="0"} 70
rpc_duration_seconds{code="3" quantile="0.5"} 52
rpc_duration_seconds_sum{code="3"} 60
rpc_duration_seconds_count{code="3"} 40
rpc_duration_seconds{code="5" quantile="0.5"} 55
rpc_duration_seconds_sum{code="5"} 200
rpc_duration_seconds_count{code="5"} 420
`

var target2Page4 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 16

# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{method="post",code="200",le="1"} 40
http_request_duration_seconds_bucket{method="post",code="200",le="+Inf"} 49
http_request_duration_seconds_sum{method="post",code="200"} 42
http_request_duration_seconds_count{method="post",code="200"} 49
http_request_duration_seconds_bucket{method="post",code="300",le="1"} 2
http_request_duration_seconds_bucket{method="post",code="300",le="+Inf"} 3
http_request_duration_seconds_sum{method="post",code="300"} 4
http_request_duration_seconds_count{method="post",code="300"} 3
http_request_duration_seconds_bucket{method="post",code="400",le="1"} 34
http_request_duration_seconds_bucket{method="post",code="400",le="+Inf"} 59
http_request_duration_seconds_sum{method="post",code="400"} 29
http_request_duration_seconds_count{method="post",code="400"} 59

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 49
http_requests_total{method="post",code="300"} 3
http_requests_total{method="post",code="400"} 59

# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{code="0" quantile="0.5"} 66
rpc_duration_seconds_sum{code="0"} 119
rpc_duration_seconds_count{code="0"} 69
rpc_duration_seconds{code="3" quantile="0.5"} 51
rpc_duration_seconds_sum{code="3"} 59
rpc_duration_seconds_count{code="3"} 39
rpc_duration_seconds{code="5" quantile="0.5"} 54
rpc_duration_seconds_sum{code="5"} 199
rpc_duration_seconds_count{code="5"} 419
`

var target2Page5 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 16

# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{method="post",code="200",le="1"} 41
http_request_duration_seconds_bucket{method="post",code="200",le="+Inf"} 50
http_request_duration_seconds_sum{method="post",code="200"} 43
http_request_duration_seconds_count{method="post",code="200"} 50
http_request_duration_seconds_bucket{method="post",code="300",le="1"} 4
http_request_duration_seconds_bucket{method="post",code="300",le="+Inf"} 5
http_request_duration_seconds_sum{method="post",code="300"} 4
http_request_duration_seconds_count{method="post",code="300"} 5
http_request_duration_seconds_bucket{method="post",code="400",le="1"} 34
http_request_duration_seconds_bucket{method="post",code="400",le="+Inf"} 59
http_request_duration_seconds_sum{method="post",code="400"} 29
http_request_duration_seconds_count{method="post",code="400"} 59

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 50
http_requests_total{method="post",code="300"} 5
http_requests_total{method="post",code="400"} 59

# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{code="0" quantile="0.5"} 76
rpc_duration_seconds_sum{code="0"} 129
rpc_duration_seconds_count{code="0"} 79
rpc_duration_seconds{code="3" quantile="0.5"} 61
rpc_duration_seconds_sum{code="3"} 69
rpc_duration_seconds_count{code="3"} 49
rpc_duration_seconds{code="5" quantile="0.5"} 64
rpc_duration_seconds_sum{code="5"} 209
rpc_duration_seconds_count{code="5"} 429
`

func verifyTarget2(t *testing.T, td *testData, resourceMetrics []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]
	// m1 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m1))

	wantAttributes := td.attributes

	metrics1 := m1.ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)
	e1 := []metricExpectation{
		{
			"go_threads",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(18),
					},
				},
			},
			nil,
		},
		{
			"http_request_duration_seconds",
			pmetric.MetricTypeHistogram,
			"",
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts1),
						compareHistogramAttributes(map[string]string{"method": "post", "code": "200"}),
						compareHistogram(10, 7, []float64{1}, []uint64{8, 2}),
					},
				},
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts1),
						compareHistogramAttributes(map[string]string{"method": "post", "code": "400"}),
						compareHistogram(50, 25, []float64{1}, []uint64{30, 20}),
					},
				},
			},
			nil,
		},
		{
			"http_requests_total",
			pmetric.MetricTypeSum,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(tsZero),
						compareTimestamp(ts1),
						compareDoubleValue(10),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(tsZero),
						compareTimestamp(ts1),
						compareDoubleValue(50),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			},
			nil,
		},
		{
			"rpc_duration_seconds",
			pmetric.MetricTypeSummary,
			"",
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts1),
						compareSummaryAttributes(map[string]string{"code": "0"}),
						compareSummary(50, 100, [][]float64{{0.5, 47}}),
					},
				},
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts1),
						compareSummaryAttributes(map[string]string{"code": "5"}),
						compareSummary(400, 180, [][]float64{{0.5, 35}}),
					},
				},
			},
			nil,
		},
	}
	doCompare(t, "scrape1", wantAttributes, m1, e1)

	m2 := resourceMetrics[1]
	// m2 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m2))

	metricsScrape2 := m2.ScopeMetrics().At(0).Metrics()
	ts2 := getTS(metricsScrape2)
	e2 := []metricExpectation{
		{
			"go_threads",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts2),
						compareDoubleValue(16),
					},
				},
			},
			nil,
		},
		{
			"http_request_duration_seconds",
			pmetric.MetricTypeHistogram,
			"",
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts2),
						compareHistogramAttributes(map[string]string{"method": "post", "code": "200"}),
						compareHistogram(50, 43, []float64{1}, []uint64{40, 10}),
					},
				},
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts2),
						compareHistogramAttributes(map[string]string{"method": "post", "code": "300"}),
						compareHistogram(3, 2, []float64{1}, []uint64{3, 0}),
					},
				},
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts2),
						compareHistogramAttributes(map[string]string{"method": "post", "code": "400"}),
						compareHistogram(60, 30, []float64{1}, []uint64{35, 25}),
					},
				},
			},
			nil,
		},
		{
			"http_requests_total",
			pmetric.MetricTypeSum,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(tsZero),
						compareTimestamp(ts2),
						compareDoubleValue(50),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(tsZero),
						compareTimestamp(ts2),
						compareDoubleValue(3),
						compareAttributes(map[string]string{"method": "post", "code": "300"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(tsZero),
						compareTimestamp(ts2),
						compareDoubleValue(60),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			},
			nil,
		},
		{
			"rpc_duration_seconds",
			pmetric.MetricTypeSummary,
			"",
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts2),
						compareSummaryAttributes(map[string]string{"code": "0"}),
						compareSummary(60, 110, [][]float64{{0.5, 57}}),
					},
				},
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts2),
						compareSummaryAttributes(map[string]string{"code": "3"}),
						compareSummary(30, 50, [][]float64{{0.5, 42}}),
					},
				},
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts2),
						compareSummaryAttributes(map[string]string{"code": "5"}),
						compareSummary(410, 190, [][]float64{{0.5, 45}}),
					},
				},
			},
			nil,
		},
	}
	doCompare(t, "scrape2", wantAttributes, m2, e2)

	m3 := resourceMetrics[2]
	// m3 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m3))

	metricsScrape3 := m3.ScopeMetrics().At(0).Metrics()
	ts3 := getTS(metricsScrape3)
	e3 := []metricExpectation{
		{
			"go_threads",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts3),
						compareDoubleValue(16),
					},
				},
			},
			nil,
		},
		{
			"http_request_duration_seconds",
			pmetric.MetricTypeHistogram,
			"",
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts3),
						compareHistogramAttributes(map[string]string{"method": "post", "code": "200"}),
						compareHistogram(50, 43, []float64{1}, []uint64{40, 10}),
					},
				},
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts3),
						compareHistogramAttributes(map[string]string{"method": "post", "code": "300"}),
						compareHistogram(5, 7, []float64{1}, []uint64{3, 2}),
					},
				},
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts3),
						compareHistogramAttributes(map[string]string{"method": "post", "code": "400"}),
						compareHistogram(60, 30, []float64{1}, []uint64{35, 25}),
					},
				},
			},
			nil,
		},
		{
			"http_requests_total",
			pmetric.MetricTypeSum,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(tsZero),
						compareTimestamp(ts3),
						compareDoubleValue(50),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(tsZero),
						compareTimestamp(ts3),
						compareDoubleValue(5),
						compareAttributes(map[string]string{"method": "post", "code": "300"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(tsZero),
						compareTimestamp(ts3),
						compareDoubleValue(60),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			},
			nil,
		},
		{
			"rpc_duration_seconds",
			pmetric.MetricTypeSummary,
			"",
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts3),
						compareSummaryAttributes(map[string]string{"code": "0"}),
						compareSummary(70, 120, [][]float64{{0.5, 67}}),
					},
				},
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts3),
						compareSummaryAttributes(map[string]string{"code": "3"}),
						compareSummary(40, 60, [][]float64{{0.5, 52}}),
					},
				},
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts3),
						compareSummaryAttributes(map[string]string{"code": "5"}),
						compareSummary(420, 200, [][]float64{{0.5, 55}}),
					},
				},
			},
			nil,
		},
	}
	doCompare(t, "scrape3", wantAttributes, m3, e3)

	m4 := resourceMetrics[3]
	// m4 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m4))

	metricsScrape4 := m4.ScopeMetrics().At(0).Metrics()
	ts4 := getTS(metricsScrape4)
	e4 := []metricExpectation{
		{
			"go_threads",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts4),
						compareDoubleValue(16),
					},
				},
			},
			nil,
		},
		{
			"http_request_duration_seconds",
			pmetric.MetricTypeHistogram,
			"",
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts4),
						compareHistogramAttributes(map[string]string{"method": "post", "code": "200"}),
						compareHistogram(49, 42, []float64{1}, []uint64{40, 9}),
					},
				},
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts4),
						compareHistogramAttributes(map[string]string{"method": "post", "code": "300"}),
						compareHistogram(3, 4, []float64{1}, []uint64{2, 1}),
					},
				},
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts4),
						compareHistogramAttributes(map[string]string{"method": "post", "code": "400"}),
						compareHistogram(59, 29, []float64{1}, []uint64{34, 25}),
					},
				},
			},
			nil,
		},
		{
			"http_requests_total",
			pmetric.MetricTypeSum,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(tsZero),
						compareTimestamp(ts4),
						compareDoubleValue(49),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(tsZero),
						compareTimestamp(ts4),
						compareDoubleValue(3),
						compareAttributes(map[string]string{"method": "post", "code": "300"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(tsZero),
						compareTimestamp(ts4),
						compareDoubleValue(59),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			},
			nil,
		},
		{
			"rpc_duration_seconds",
			pmetric.MetricTypeSummary,
			"",
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts4),
						compareSummaryAttributes(map[string]string{"code": "0"}),
						compareSummary(69, 119, [][]float64{{0.5, 66}}),
					},
				},
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts4),
						compareSummaryAttributes(map[string]string{"code": "3"}),
						compareSummary(39, 59, [][]float64{{0.5, 51}}),
					},
				},
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts4),
						compareSummaryAttributes(map[string]string{"code": "5"}),
						compareSummary(419, 199, [][]float64{{0.5, 54}}),
					},
				},
			},
			nil,
		},
	}
	doCompare(t, "scrape4", wantAttributes, m4, e4)

	m5 := resourceMetrics[4]
	// m5 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m5))

	metricsScrape5 := m5.ScopeMetrics().At(0).Metrics()
	ts5 := getTS(metricsScrape5)
	e5 := []metricExpectation{
		{
			"go_threads",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts5),
						compareDoubleValue(16),
					},
				},
			},
			nil,
		},
		{
			"http_request_duration_seconds",
			pmetric.MetricTypeHistogram,
			"",
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts5),
						compareHistogramAttributes(map[string]string{"method": "post", "code": "200"}),
						compareHistogram(50, 43, []float64{1}, []uint64{41, 9}),
					},
				},
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts5),
						compareHistogramAttributes(map[string]string{"method": "post", "code": "300"}),
						compareHistogram(5, 4, []float64{1}, []uint64{4, 1}),
					},
				},
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts5),
						compareHistogramAttributes(map[string]string{"method": "post", "code": "400"}),
						compareHistogram(59, 29, []float64{1}, []uint64{34, 25}),
					},
				},
			},
			nil,
		},
		{
			"http_requests_total",
			pmetric.MetricTypeSum,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(tsZero),
						compareTimestamp(ts5),
						compareDoubleValue(50),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(tsZero),
						compareTimestamp(ts5),
						compareDoubleValue(5),
						compareAttributes(map[string]string{"method": "post", "code": "300"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(tsZero),
						compareTimestamp(ts5),
						compareDoubleValue(59),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			},
			nil,
		},
		{
			"rpc_duration_seconds",
			pmetric.MetricTypeSummary,
			"",
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts5),
						compareSummaryAttributes(map[string]string{"code": "0"}),
						compareSummary(79, 129, [][]float64{{0.5, 76}}),
					},
				},
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts5),
						compareSummaryAttributes(map[string]string{"code": "3"}),
						compareSummary(49, 69, [][]float64{{0.5, 61}}),
					},
				},
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts5),
						compareSummaryAttributes(map[string]string{"code": "5"}),
						compareSummary(429, 209, [][]float64{{0.5, 64}}),
					},
				},
			},
			nil,
		},
	}
	doCompare(t, "scrape5", wantAttributes, m5, e5)
}

// target3 for complicated data types, including summaries and histograms. one of the summary and histogram have only
// sum/count, for the summary it's valid, however the histogram one is not, but it shall not cause the scrape to fail
var target3Page1 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 18

# A histogram, which has a pretty complex representation in the text format:
# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.2"} 10000
http_request_duration_seconds_bucket{le="0.5"} 11000
http_request_duration_seconds_bucket{le="1"} 12001
http_request_duration_seconds_bucket{le="+Inf"} 13003
http_request_duration_seconds_sum 50000
http_request_duration_seconds_count 13003

# A corrupted histogram with only sum and count
# HELP corrupted_hist A corrupted_hist.
# TYPE corrupted_hist histogram
corrupted_hist_sum 100
corrupted_hist_count 10

# Finally a summary, which has a complex representation, too:
# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{foo="bar" quantile="0.01"} 31
rpc_duration_seconds{foo="bar" quantile="0.05"} 35
rpc_duration_seconds{foo="bar" quantile="0.5"} 47
rpc_duration_seconds{foo="bar" quantile="0.9"} 70
rpc_duration_seconds{foo="bar" quantile="0.99"} 76
rpc_duration_seconds_sum{foo="bar"} 8000
rpc_duration_seconds_count{foo="bar"} 900
rpc_duration_seconds_sum{foo="no_quantile"} 100
rpc_duration_seconds_count{foo="no_quantile"} 50
`

var target3Page2 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 16

# A histogram, which has a pretty complex representation in the text format:
# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.2"} 11000
http_request_duration_seconds_bucket{le="0.5"} 12000
http_request_duration_seconds_bucket{le="1"} 13001
http_request_duration_seconds_bucket{le="+Inf"} 14003
http_request_duration_seconds_sum 50100
http_request_duration_seconds_count 14003

# A corrupted histogram with only sum and count	
# HELP corrupted_hist A corrupted_hist.
# TYPE corrupted_hist histogram
corrupted_hist_sum 101
corrupted_hist_count 15

# Finally a summary, which has a complex representation, too:
# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{foo="bar" quantile="0.01"} 32
rpc_duration_seconds{foo="bar" quantile="0.05"} 35
rpc_duration_seconds{foo="bar" quantile="0.5"} 47
rpc_duration_seconds{foo="bar" quantile="0.9"} 70
rpc_duration_seconds{foo="bar" quantile="0.99"} 77
rpc_duration_seconds_sum{foo="bar"} 8100
rpc_duration_seconds_count{foo="bar"} 950
rpc_duration_seconds_sum{foo="no_quantile"} 101
rpc_duration_seconds_count{foo="no_quantile"} 55
`

var target4Page1 = `
# A simple counter
# TYPE foo counter
foo 0
# Another counter with the same name but also _total suffix
# TYPE foo_total counter
foo_total 1
`

func verifyTarget3(t *testing.T, td *testData, resourceMetrics []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]
	// m1 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m1))

	wantAttributes := td.attributes

	metrics1 := m1.ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)
	e1 := []metricExpectation{
		{
			"go_threads",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(18),
					},
				},
			},
			nil,
		},
		{
			"http_request_duration_seconds",
			pmetric.MetricTypeHistogram,
			"",
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts1),
						compareHistogram(13003, 50000, []float64{0.2, 0.5, 1}, []uint64{10000, 1000, 1001, 1002}),
					},
				},
			},
			nil,
		},
		{
			"corrupted_hist",
			pmetric.MetricTypeHistogram,
			"",
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts1),
						compareHistogram(10, 100, nil, []uint64{10}),
					},
				},
			},
			nil,
		},
		{
			"rpc_duration_seconds",
			pmetric.MetricTypeSummary,
			"",
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts1),
						compareSummaryAttributes(map[string]string{"foo": "bar"}),
						compareSummary(900, 8000, [][]float64{{0.01, 31}, {0.05, 35}, {0.5, 47}, {0.9, 70}, {0.99, 76}}),
					},
				},
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts1),
						compareSummaryAttributes(map[string]string{"foo": "no_quantile"}),
						compareSummary(50, 100, [][]float64{}),
					},
				},
			},
			nil,
		},
	}
	doCompare(t, "scrape1", wantAttributes, m1, e1)

	m2 := resourceMetrics[1]
	// m2 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m2))

	metricsScrape2 := m2.ScopeMetrics().At(0).Metrics()
	ts2 := getTS(metricsScrape2)
	e2 := []metricExpectation{
		{
			"go_threads",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts2),
						compareDoubleValue(16),
					},
				},
			},
			nil,
		},
		{
			"http_request_duration_seconds",
			pmetric.MetricTypeHistogram,
			"",
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts2),
						compareHistogram(14003, 50100, []float64{0.2, 0.5, 1}, []uint64{11000, 1000, 1001, 1002}),
					},
				},
			},
			nil,
		},
		{
			"corrupted_hist",
			pmetric.MetricTypeHistogram,
			"",
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(tsZero),
						compareHistogramTimestamp(ts2),
						compareHistogram(15, 101, nil, []uint64{15}),
					},
				},
			},
			nil,
		},
		{
			"rpc_duration_seconds",
			pmetric.MetricTypeSummary,
			"",
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts2),
						compareSummaryAttributes(map[string]string{"foo": "bar"}),
						compareSummary(950, 8100, [][]float64{{0.01, 32}, {0.05, 35}, {0.5, 47}, {0.9, 70}, {0.99, 77}}),
					},
				},
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(tsZero),
						compareSummaryTimestamp(ts2),
						compareSummaryAttributes(map[string]string{"foo": "no_quantile"}),
						compareSummary(55, 101, [][]float64{}),
					},
				},
			},
			nil,
		},
	}
	doCompare(t, "scrape2", wantAttributes, m2, e2)
}

func verifyTarget4(t *testing.T, td *testData, resourceMetrics []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]

	// m1 has 2 metrics + 5 internal scraper metrics
	assert.Equal(t, 7, metricsCount(m1))

	wantAttributes := td.attributes

	metrics1 := m1.ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)
	e1 := []metricExpectation{
		{
			"foo",
			pmetric.MetricTypeSum,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(0),
					},
				},
			},
			compareMetricIsMonotonic(true),
		},
		{
			"foo_total",
			pmetric.MetricTypeSum,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(1.0),
					},
				},
			},
			compareMetricIsMonotonic(true),
		},
	}
	doCompare(t, "scrape-infostatesetmetrics-1", wantAttributes, m1, e1)
}

// TestCoreMetricsEndToEnd end to end test executor
func TestCoreMetricsEndToEnd(t *testing.T) {
	// 1. setup input data
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: target1Page1},
				{code: 500, data: ""},
				{code: 200, data: target1Page2},
				{code: 500, data: ""},
				{code: 200, data: target1Page3},
			},
			validateFunc: verifyTarget1,
		},
		{
			name: "target2",
			pages: []mockPrometheusResponse{
				{code: 200, data: target2Page1},
				{code: 200, data: target2Page2},
				{code: 500, data: ""},
				{code: 200, data: target2Page3},
				{code: 200, data: target2Page4},
				{code: 500, data: ""},
				{code: 200, data: target2Page5},
			},
			validateFunc: verifyTarget2,
		},
		{
			name: "target3",
			pages: []mockPrometheusResponse{
				{code: 200, data: target3Page1},
				{code: 200, data: target3Page2},
			},
			validateFunc: verifyTarget3,
		},
		{
			name: "target4",
			pages: []mockPrometheusResponse{
				{code: 200, data: target4Page1, useOpenMetrics: false},
			},
			validateFunc:    verifyTarget4,
			validateScrapes: true,
		},
	}
	testComponent(t, targets, nil)
}

// metric type is defined as 'untyped' in the first metric
// and, type hint is missing in the 2nd metric
var untypedMetrics = `
# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total untyped
http_requests_total{method="post",code="200"} 100
http_requests_total{method="post",code="400"} 5

# HELP redis_connected_clients Redis connected clients
redis_connected_clients{name="rough-snowflake-web",port="6380"} 10.0
redis_connected_clients{name="rough-snowflake-web",port="6381"} 12.0
`

// TestUntypedMetrics validates the pass through of untyped metrics
// through metric receiver and the conversion of untyped to gauge double
func TestUntypedMetrics(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: untypedMetrics},
			},
			validateFunc: verifyUntypedMetrics,
		},
	}

	testComponent(t, targets, nil)
}

func verifyUntypedMetrics(t *testing.T, td *testData, resourceMetrics []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]

	// m1 has 2 metrics + 5 internal scraper metrics
	assert.Equal(t, 7, metricsCount(m1))

	wantAttributes := td.attributes

	metrics1 := m1.ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)
	e1 := []metricExpectation{
		{
			"http_requests_total",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(100),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(5),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			},
			nil,
		},
		{
			"redis_connected_clients",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(10),
						compareAttributes(map[string]string{"name": "rough-snowflake-web", "port": "6380"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(12),
						compareAttributes(map[string]string{"name": "rough-snowflake-web", "port": "6381"}),
					},
				},
			},
			nil,
		},
	}
	doCompare(t, "scrape-untypedMetric-1", wantAttributes, m1, e1)
}

func TestGCInterval(t *testing.T) {
	for _, tc := range []struct {
		desc  string
		input *PromConfig
		want  time.Duration
	}{
		{
			desc:  "default",
			input: &PromConfig{},
			want:  defaultGCInterval,
		},
		{
			desc: "global override",
			input: &PromConfig{
				GlobalConfig: promConfig.GlobalConfig{
					ScrapeInterval: model.Duration(10 * time.Minute),
				},
			},
			want: 11 * time.Minute,
		},
		{
			desc: "scrape config override",
			input: &PromConfig{
				ScrapeConfigs: []*promConfig.ScrapeConfig{
					{
						ScrapeInterval: model.Duration(10 * time.Minute),
					},
				},
			},
			want: 11 * time.Minute,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			got := gcInterval(tc.input)
			if got != tc.want {
				t.Errorf("gcInterval(%+v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestUserAgent(t *testing.T) {
	uaCh := make(chan string, 1)
	svr := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case uaCh <- r.UserAgent():
		default:
		}
	}))
	defer svr.Close()

	cfg, err := promConfig.Load(fmt.Sprintf(`
scrape_configs:
- job_name: foo
  scrape_interval: 100ms
  static_configs:
    - targets:
      - %s
        `, strings.TrimPrefix(svr.URL, "http://")), promslog.NewNopLogger())
	require.NoError(t, err)
	set := receivertest.NewNopSettings(metadata.Type)
	receiver, err := newPrometheusReceiver(set, &Config{
		PrometheusConfig: (*PromConfig)(cfg),
	}, new(consumertest.MetricsSink))
	require.NoError(t, err)

	ctx := t.Context()

	require.NoError(t, receiver.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, receiver.Shutdown(ctx))
	})

	gotUA := <-uaCh

	require.Contains(t, gotUA, set.BuildInfo.Command)
	require.Contains(t, gotUA, set.BuildInfo.Version)
}
