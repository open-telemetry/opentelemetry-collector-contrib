// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheusreceiver

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

var staleNaNsPage1 = `
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

var (
	totalScrapes = 10
)

// TestStaleNaNs validates that staleness marker gets generated when the timeseries is no longer present
func TestStaleNaNs(t *testing.T) {
	mockResponses := make([]mockPrometheusResponse, 0)
	for i := 0; i < totalScrapes; i++ {
		if i%2 == 0 {
			mockResponses = append(mockResponses, mockPrometheusResponse{
				code: 200,
				data: staleNaNsPage1,
			})
		} else {
			mockResponses = append(mockResponses, mockPrometheusResponse{
				code: 500,
				data: "",
			})
		}
	}
	targets := []*testData{
		{
			name:            "target1",
			pages:           mockResponses,
			validateFunc:    verifyStaleNaNs,
			validateScrapes: true,
		},
	}
	testComponent(t, targets, false, "")
}

func verifyStaleNaNs(t *testing.T, td *testData, resourceMetrics []*pdata.ResourceMetrics) {
	verifyNumTotalScrapeResults(t, td, resourceMetrics)
	metrics1 := resourceMetrics[0].InstrumentationLibraryMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)
	for i := 0; i < totalScrapes; i++ {
		if i%2 == 0 {
			verifyStaleNaNPage1SuccessfulScrape(t, td, resourceMetrics[i], &ts1, i+1)
		} else {
			verifyStaleNanPage1FirstFailedScrape(t, td, resourceMetrics[i], &ts1, i+1)
		}
	}
}

func verifyStaleNaNPage1SuccessfulScrape(t *testing.T, td *testData, resourceMetric *pdata.ResourceMetrics, startTimestamp *pdata.Timestamp, iteration int) {
	// m1 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(resourceMetric))
	wantAttributes := td.attributes // should want attribute be part of complete target or each scrape?
	metrics1 := resourceMetric.InstrumentationLibraryMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)
	e1 := []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(19),
					},
				},
			}),
		assertMetricPresent("http_requests_total",
			compareMetricType(pdata.MetricDataTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(*startTimestamp),
						compareTimestamp(ts1),
						compareDoubleValue(100),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(*startTimestamp),
						compareTimestamp(ts1),
						compareDoubleValue(5),
						compareAttributes(map[string]string{"method": "post", "code": "400"}),
					},
				},
			}),
		assertMetricPresent("http_request_duration_seconds",
			compareMetricType(pdata.MetricDataTypeHistogram),
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						// TODO: #6360 Prometheus Receiver Issue- start_timestamp are incorrect
						// for Summary and Histogram metrics after a failed scrape
						//compareHistogramStartTimestamp(*startTimestamp),
						compareHistogramTimestamp(ts1),
						compareHistogram(2500, 5000, []uint64{1000, 500, 500, 500}),
					},
				},
			}),
		assertMetricPresent("rpc_duration_seconds",
			compareMetricType(pdata.MetricDataTypeSummary),
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						// TODO: #6360 Prometheus Receiver Issue- start_timestamp are incorrect
						// for Summary and Histogram metrics after a failed scrape
						//compareSummaryStartTimestamp(*startTimestamp),
						compareSummaryTimestamp(ts1),
						compareSummary(1000, 5000, [][]float64{{0.01, 1}, {0.9, 5}, {0.99, 8}}),
					},
				},
			}),
	}
	doCompare(t, fmt.Sprintf("validScrape-scrape-%d", iteration), wantAttributes, resourceMetric, e1)
}

func verifyStaleNanPage1FirstFailedScrape(t *testing.T, td *testData, resourceMetric *pdata.ResourceMetrics, startTimestamp *pdata.Timestamp, iteration int) {
	// m1 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(resourceMetric))
	wantAttributes := td.attributes
	allMetrics := getMetrics(resourceMetric)
	assertUp(t, 0, allMetrics)

	// TODO: Issue #6376. Remove this skip once OTLP format is directly used in Prometheus Receiver Metric Builder.
	if true {
		t.Log(`Skipping the datapoint flag check for staleness markers, as the current receiver doesnt yet set the flag true for staleNaNs`)
		return
	}
	metrics1 := resourceMetric.InstrumentationLibraryMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)
	e1 := []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						assertNumberPointFlagNoRecordedValue(),
					},
				},
			}),
		assertMetricPresent("http_requests_total",
			compareMetricType(pdata.MetricDataTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(*startTimestamp),
						compareTimestamp(ts1),
						assertNumberPointFlagNoRecordedValue(),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(*startTimestamp),
						compareTimestamp(ts1),
						assertNumberPointFlagNoRecordedValue(),
					},
				},
			}),
		assertMetricPresent("http_request_duration_seconds",
			compareMetricType(pdata.MetricDataTypeHistogram),
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(*startTimestamp),
						compareHistogramTimestamp(ts1),
						assertHistogramPointFlagNoRecordedValue(),
					},
				},
			}),
		assertMetricPresent("rpc_duration_seconds",
			compareMetricType(pdata.MetricDataTypeSummary),
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(*startTimestamp),
						compareSummaryTimestamp(ts1),
						assertSummaryPointFlagNoRecordedValue(),
					},
				},
			}),
	}
	doCompare(t, fmt.Sprintf("failedScrape-scrape-%d", iteration), wantAttributes, resourceMetric, e1)
}
