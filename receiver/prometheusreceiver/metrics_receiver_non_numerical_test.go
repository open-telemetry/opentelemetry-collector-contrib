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
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

var infPage1 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads +Inf

# HELP redis_connected_clients Redis connected clients
redis_connected_clients{name="rough-snowflake-web",port="6380"} -Inf

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} +Inf

# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{quantile="0.01"} +Inf
rpc_duration_seconds{quantile="0.9"} +Inf
rpc_duration_seconds{quantile="0.99"} +Inf
rpc_duration_seconds_sum 5000
rpc_duration_seconds_count 1000
`

func TestInfValues(t *testing.T) {
	// 1. setup input data
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: infPage1},
			},
			validateFunc: verifyInfValues,
		},
	}
	testComponent(t, targets, false, "")
}

func verifyInfValues(t *testing.T, td *testData, resourceMetrics []*pdata.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]

	// m1 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m1))

	wantAttributes := td.attributes

	metrics1 := m1.InstrumentationLibraryMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)
	e1 := []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(math.Inf(1)),
					},
				},
			}),
		assertMetricPresent("redis_connected_clients",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareAttributes(map[string]string{"name": "rough-snowflake-web", "port": "6380"}),
						compareDoubleValue(math.Inf(-1)),
					},
				},
			}),
		assertMetricPresent("http_requests_total",
			compareMetricType(pdata.MetricDataTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts1),
						compareTimestamp(ts1),
						compareDoubleValue(math.Inf(1)),
						compareAttributes(map[string]string{"method": "post", "code": "200"}),
					},
				},
			}),
		assertMetricPresent("rpc_duration_seconds",
			compareMetricType(pdata.MetricDataTypeSummary),
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(ts1),
						compareSummaryTimestamp(ts1),
						compareSummary(1000, 5000, [][]float64{{0.01, math.Inf(1)}, {0.9, math.Inf(1)}, {0.99, math.Inf(1)}}),
					},
				},
			}),
	}
	doCompare(t, "scrape-InfValues-1", wantAttributes, m1, e1)
}
