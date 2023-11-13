// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"fmt"
	"sync"
	"testing"
	"time"

	promcfg "github.com/prometheus/prometheus/config"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var (
	timeNow = time.Now()
	ts1     = timeNow.Add(10 * time.Second).UnixMilli()
	ts2     = timeNow.Add(20 * time.Second).UnixMilli()
	ts3     = timeNow.Add(30 * time.Second).UnixMilli()
	ts4     = timeNow.Add(40 * time.Second).UnixMilli()
	ts5     = timeNow.Add(50 * time.Second).UnixMilli()
	ts6     = timeNow.Add(60 * time.Second).UnixMilli()
	ts7     = timeNow.Add(70 * time.Second).UnixMilli()
	ts8     = timeNow.Add(80 * time.Second).UnixMilli()
	ts9     = timeNow.Add(90 * time.Second).UnixMilli()
	ts10    = timeNow.Add(100 * time.Second).UnixMilli()
	ts11    = timeNow.Add(110 * time.Second).UnixMilli()
	ts12    = timeNow.Add(120 * time.Second).UnixMilli()
	ts13    = timeNow.Add(130 * time.Second).UnixMilli()
	ts14    = timeNow.Add(140 * time.Second).UnixMilli()
	ts15    = timeNow.Add(150 * time.Second).UnixMilli()
)

var onlyOnce sync.Once

// honorTimestampsPage2 has lower values for metrics than honorTimestampsPage1.
// So, the start_timestamp should get reset.
var honorTimestampsPage1 = `
# HELP go_threads Number of OS threads created
# TYPE go_thread gauge
go_threads 19 %v

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 100 %v
http_requests_total{method="post",code="400"} 5 %v

# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.05"} 1000 %v
http_request_duration_seconds_bucket{le="0.5"} 1500 %v
http_request_duration_seconds_bucket{le="1"} 2000 %v
http_request_duration_seconds_bucket{le="+Inf"} 2500 %v
http_request_duration_seconds_sum 5000 %v
http_request_duration_seconds_count 2500 %v

# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{quantile="0.01"} 1 %v
rpc_duration_seconds{quantile="0.9"} 5 %v
rpc_duration_seconds{quantile="0.99"} 8 %v
rpc_duration_seconds_sum 5000 %v
rpc_duration_seconds_count 1000 %v
`

var honorTimestampsPage2 = `
# HELP go_threads Number of OS threads created
# TYPE go_thread gauge
go_threads 18 %v

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 99 %v
http_requests_total{method="post",code="400"} 3 %v

# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.05"} 900 %v
http_request_duration_seconds_bucket{le="0.5"} 1400 %v
http_request_duration_seconds_bucket{le="1"} 1900 %v
http_request_duration_seconds_bucket{le="+Inf"} 2400 %v
http_request_duration_seconds_sum 4950 %v
http_request_duration_seconds_count 2400 %v

# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{quantile="0.01"} 1 %v
rpc_duration_seconds{quantile="0.9"} 6 %v
rpc_duration_seconds{quantile="0.99"} 8 %v
rpc_duration_seconds_sum 4980 %v
rpc_duration_seconds_count 900 %v
`

// honorTimestampsPage3 has higher value than previous scrape.
// So, start_timestamp should not be reset for honorTimestampsPage3
var honorTimestampsPage3 = `
# HELP go_threads Number of OS threads created
# TYPE go_thread gauge
go_threads 19 %v

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 100 %v
http_requests_total{method="post",code="400"} 5 %v

# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.05"} 1000 %v
http_request_duration_seconds_bucket{le="0.5"} 1500 %v
http_request_duration_seconds_bucket{le="1"} 2000 %v
http_request_duration_seconds_bucket{le="+Inf"} 2500 %v
http_request_duration_seconds_sum 5000 %v
http_request_duration_seconds_count 2500 %v

# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{quantile="0.01"} 1 %v
rpc_duration_seconds{quantile="0.9"} 5 %v
rpc_duration_seconds{quantile="0.99"} 8 %v
rpc_duration_seconds_sum 5000 %v
rpc_duration_seconds_count 1000 %v
`

// TestHonorTimeStampsWithTrue validates honor_timestamp configuration
// where all metricFamilies (and each datapoint) in the testdata has explicit timestamps.
// TestHonorTimeStampsWithTrue does not check for scenario where timestamp is not provided
// to all metric families in a scrape- as those situations are rare though valid.

// TestHonorTimeStampsWithTrue has testdata such that
// For valid data- Start_timestamps should not be ahead of point_timestamps.
// TestHonorTimeStampsWithTrue validates:
// - For initial scrape, start_timestamp is same as point timestamp,
// - Start_timestamp should be the explicit timestamp of first-time a particular metric is seen
// - Start_timestamp should get reset if current scrape has lower value than previous scrape

func TestHonorTimeStampsWithTrue(t *testing.T) {
	setMetricsTimestamp()
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: honorTimestampsPage1},
				{code: 200, data: honorTimestampsPage2},
				{code: 200, data: honorTimestampsPage3},
			},
			validateFunc: verifyHonorTimeStampsTrue,
		},
	}

	testComponent(t, targets, nil)
}

// TestHonorTimeStampsWithFalse validates that with honor_timestamp config set to false,
// valid testdata provided with explicit timestamps does not get honored.
func TestHonorTimeStampsWithFalse(t *testing.T) {
	setMetricsTimestamp()
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: honorTimestampsPage1},
				{code: 200, data: honorTimestampsPage2},
			},
			validateFunc: verifyHonorTimeStampsFalse,
		},
	}

	testComponent(t, targets, nil, func(cfg *promcfg.Config) {
		for _, scrapeConfig := range cfg.ScrapeConfigs {
			scrapeConfig.HonorTimestamps = false
		}
	})
}

func setMetricsTimestamp() {
	onlyOnce.Do(func() {
		honorTimestampsPage1 = fmt.Sprintf(honorTimestampsPage1,
			ts1,      // timestamp for gauge
			ts2, ts3, // timestamp for counter
			ts4, ts4, ts4, ts4, ts4, ts4, // timestamp for histogram
			ts5, ts5, ts5, ts5, ts5, // timestamp for summary
		)
		honorTimestampsPage2 = fmt.Sprintf(honorTimestampsPage2,
			ts6,      // timestamp for gauge
			ts7, ts8, // timestamp for counter
			ts9, ts9, ts9, ts9, ts9, ts9, // timestamp for histogram
			ts10, ts10, ts10, ts10, ts10, // timestamp for summary
		)
		honorTimestampsPage3 = fmt.Sprintf(honorTimestampsPage3,
			ts11,       // timestamp for gauge
			ts12, ts13, // timestamp for counter
			ts14, ts14, ts14, ts14, ts14, ts14, // timestamp for histogram
			ts15, ts15, ts15, ts15, ts15, // timestamp for summary
		)
	})
}

func verifyHonorTimeStampsTrue(t *testing.T, td *testData, resourceMetrics []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]
	// m1 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m1))

	wantAttributes := td.attributes
	e1 := []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pmetric.MetricTypeGauge),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts1))),
						compareDoubleValue(19),
					},
				},
			}),
		assertMetricPresent("http_requests_total",
			compareMetricType(pmetric.MetricTypeSum),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts2))),
						compareTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts2))),
						compareDoubleValue(100),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts3))),
						compareTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts3))),
						compareDoubleValue(5),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			}),
		assertMetricPresent("http_request_duration_seconds",
			compareMetricType(pmetric.MetricTypeHistogram),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts4))),
						compareHistogramTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts4))),
						compareHistogram(2500, 5000, []float64{0.05, 0.5, 1}, []uint64{1000, 500, 500, 500}),
					},
				},
			}),
		assertMetricPresent("rpc_duration_seconds",
			compareMetricType(pmetric.MetricTypeSummary),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts5))),
						compareSummaryTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts5))),
						compareSummary(1000, 5000, [][]float64{{0.01, 1}, {0.9, 5}, {0.99, 8}}),
					},
				},
			}),
	}
	doCompare(t, "scrape-honorTimestamp-1", wantAttributes, m1, e1)

	m2 := resourceMetrics[1]
	// m1 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m2))

	e2 := []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pmetric.MetricTypeGauge),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts6))),
						compareDoubleValue(18),
					},
				},
			}),
		assertMetricPresent("http_requests_total",
			compareMetricType(pmetric.MetricTypeSum),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts7))),
						compareTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts7))),
						compareDoubleValue(99),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts8))),
						compareTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts8))),
						compareDoubleValue(3),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			}),
		assertMetricPresent("http_request_duration_seconds",
			compareMetricType(pmetric.MetricTypeHistogram),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts9))),
						compareHistogramTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts9))),
						compareHistogram(2400, 4950, []float64{0.05, 0.5, 1}, []uint64{900, 500, 500, 500}),
					},
				},
			}),
		assertMetricPresent("rpc_duration_seconds",
			compareMetricType(pmetric.MetricTypeSummary),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts10))),
						compareSummaryTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts10))),
						compareSummary(900, 4980, [][]float64{{0.01, 1}, {0.9, 6}, {0.99, 8}}),
					},
				},
			}),
	}
	doCompare(t, "scrape-honorTimestamp-2", wantAttributes, m2, e2)

	m3 := resourceMetrics[2]
	// m1 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m3))

	e3 := []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pmetric.MetricTypeGauge),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts11))),
						compareDoubleValue(19),
					},
				},
			}),
		assertMetricPresent("http_requests_total",
			compareMetricType(pmetric.MetricTypeSum),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts7))),
						compareTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts12))),
						compareDoubleValue(100),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts8))),
						compareTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts13))),
						compareDoubleValue(5),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			}),
		assertMetricPresent("http_request_duration_seconds",
			compareMetricType(pmetric.MetricTypeHistogram),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts9))),
						compareHistogramTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts14))),
						compareHistogram(2500, 5000, []float64{0.05, 0.5, 1}, []uint64{1000, 500, 500, 500}),
					},
				},
			}),
		assertMetricPresent("rpc_duration_seconds",
			compareMetricType(pmetric.MetricTypeSummary),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts10))),
						compareSummaryTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(ts15))),
						compareSummary(1000, 5000, [][]float64{{0.01, 1}, {0.9, 5}, {0.99, 8}}),
					},
				},
			}),
	}
	doCompare(t, "scrape-honorTimestamp-3", wantAttributes, m3, e3)
}

func verifyHonorTimeStampsFalse(t *testing.T, td *testData, resourceMetrics []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]

	// m1 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m1))

	wantAttributes := td.attributes

	metrics1 := m1.ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)
	e1 := []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pmetric.MetricTypeGauge),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(19),
					},
				},
			}),
		assertMetricPresent("http_requests_total",
			compareMetricType(pmetric.MetricTypeSum),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts1),
						compareTimestamp(ts1),
						compareDoubleValue(100),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts1),
						compareTimestamp(ts1),
						compareDoubleValue(5),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			}),
		assertMetricPresent("http_request_duration_seconds",
			compareMetricType(pmetric.MetricTypeHistogram),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(ts1),
						compareHistogramTimestamp(ts1),
						compareHistogram(2500, 5000, []float64{0.05, 0.5, 1}, []uint64{1000, 500, 500, 500}),
					},
				},
			}),
		assertMetricPresent("rpc_duration_seconds",
			compareMetricType(pmetric.MetricTypeSummary),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(ts1),
						compareSummaryTimestamp(ts1),
						compareSummary(1000, 5000, [][]float64{{0.01, 1}, {0.9, 5}, {0.99, 8}}),
					},
				},
			}),
	}
	doCompare(t, "scrape-honorTimestamp-1", wantAttributes, m1, e1)

	m2 := resourceMetrics[1]
	// m2 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m2))

	metricsScrape2 := m2.ScopeMetrics().At(0).Metrics()
	ts2 := getTS(metricsScrape2)
	e2 := []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pmetric.MetricTypeGauge),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts2),
						compareDoubleValue(18),
					},
				},
			}),
		assertMetricPresent("http_requests_total",
			compareMetricType(pmetric.MetricTypeSum),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts2),
						compareTimestamp(ts2),
						compareDoubleValue(99),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts2),
						compareTimestamp(ts2),
						compareDoubleValue(3),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			}),
		assertMetricPresent("http_request_duration_seconds",
			compareMetricType(pmetric.MetricTypeHistogram),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(ts2),
						compareHistogramTimestamp(ts2),
						compareHistogram(2400, 4950, []float64{0.05, 0.5, 1}, []uint64{900, 500, 500, 500}),
					},
				},
			}),
		assertMetricPresent("rpc_duration_seconds",
			compareMetricType(pmetric.MetricTypeSummary),
			compareMetricUnit(""),
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(ts2),
						compareSummaryTimestamp(ts2),
						compareSummary(900, 4980, [][]float64{{0.01, 1}, {0.9, 6}, {0.99, 8}}),
					},
				},
			}),
	}
	doCompare(t, "scrape-honorTimestamp-2", wantAttributes, m2, e2)
}
