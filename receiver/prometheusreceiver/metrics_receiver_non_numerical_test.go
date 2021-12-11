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

	"github.com/prometheus/prometheus/pkg/value"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

// Prometheus gauge metric can be set to NaN, a use case could be when value 0 is not representable
// Prometheus summary metric quantiles can have NaN after getting expired
var normalNaNsPage1 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads NaN

# HELP redis_connected_clients Redis connected clients
redis_connected_clients{name="rough-snowflake-web",port="6380"} NaN 

# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{quantile="0.01"} NaN
rpc_duration_seconds{quantile="0.9"} NaN
rpc_duration_seconds{quantile="0.99"} NaN
rpc_duration_seconds_sum 5000
rpc_duration_seconds_count 1000
`

// TestNormalNaNs validates the output of receiver when testdata contains NaN values
func TestNormalNaNs(t *testing.T) {
	// 1. setup input data
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: normalNaNsPage1},
			},
			validateFunc: verifyNormalNaNs,
		},
	}
	testComponent(t, targets, false, "")
}

func verifyNormalNaNs(t *testing.T, td *testData, resourceMetrics []*pdata.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]

	// m1 has 3 metrics + 5 internal scraper metrics
	assert.Equal(t, 8, metricsCount(m1))

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
						assertNormalNan(),
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
						assertNormalNan(),
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
						compareSummary(1000, 5000, [][]float64{{0.01, math.Float64frombits(value.NormalNaN)},
							{0.9, math.Float64frombits(value.NormalNaN)}, {0.99, math.Float64frombits(value.NormalNaN)}}),
					},
				},
			}),
	}
	doCompare(t, "scrape-NormalNaN-1", wantAttributes, m1, e1)
}
