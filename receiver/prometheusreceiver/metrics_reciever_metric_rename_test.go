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
	"testing"

	"github.com/prometheus/common/model"
	promcfg "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/pkg/relabel"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

var renameMetric = `
# HELP http_go_threads Number of OS threads created
# TYPE http_go_threads gauge
http_go_threads 19

# HELP http_connected_total connected clients
# TYPE http_connected_total counter
http_connected_total{method="post",port="6380"} 15.0

# HELP redis_http_requests_total Redis connected clients
# TYPE redis_http_requests_total counter
redis_http_requests_total{method="post",port="6380"} 10.0
redis_http_requests_total{method="post",port="6381"} 12.0

# HELP rpc_duration_total RPC clients
# TYPE rpc_duration_total counter
rpc_duration_total{method="post",port="6380"} 100.0
rpc_duration_total{method="post",port="6381"} 120.0
`

// TestMetricRenaming validates the 'Replace' and 'Drop' actions of metric renaming config
// Renaming metric config converts any metric type to Gauge double.
// And usage of renaming metric on complex types like histogram or summary will lead to undefined results and hence not tested here
func TestMetricRenaming(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: renameMetric},
			},
			validateFunc: verifyRenameMetric,
		},
	}

	testComponent(t, targets, false, "", func(cfg *promcfg.Config) {
		for _, scrapeConfig := range cfg.ScrapeConfigs {
			scrapeConfig.MetricRelabelConfigs = []*relabel.Config{
				{
					// this config should replace the matching regex metric name with 'foo'
					SourceLabels: model.LabelNames{"__name__"},
					Regex:        relabel.MustNewRegexp("http_.*"),
					Action:       relabel.Replace,
					TargetLabel:  "__name__",
					Replacement:  "foo",
				},
				{
					// this config should omit 'redis_' from the matching regex metric name
					SourceLabels: model.LabelNames{"__name__"},
					Regex:        relabel.MustNewRegexp("redis_(.*)"),
					Action:       relabel.Replace,
					TargetLabel:  "__name__",
					Replacement:  "$1",
				},
				{
					// this config should drop the metric that matches the regex metric name
					SourceLabels: model.LabelNames{"__name__"},
					Regex:        relabel.MustNewRegexp("rpc_(.*)"),
					Action:       relabel.Drop,
					Replacement:  relabel.DefaultRelabelConfig.Replacement,
				},
			}
		}
	})
}

// TestMetricRenaming validates the 'Keep' action of metric renaming config
func TestMetricRenamingKeepAction(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: renameMetric},
			},
			validateFunc: verifyRenameMetricKeepAction,
		},
	}

	testComponent(t, targets, false, "", func(cfg *promcfg.Config) {
		for _, scrapeConfig := range cfg.ScrapeConfigs {
			scrapeConfig.MetricRelabelConfigs = []*relabel.Config{
				{
					// this config should keep only the metric that matches the regex metric name, and drop the rest
					SourceLabels: model.LabelNames{"__name__"},
					Regex:        relabel.MustNewRegexp("rpc_(.*)"),
					Action:       relabel.Keep,
					Replacement:  relabel.DefaultRelabelConfig.Replacement,
				},
			}
		}
	})

}

func verifyRenameMetric(t *testing.T, td *testData, resourceMetrics []*pdata.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]

	// m1 has 2 metrics + 5 internal scraper metrics
	assert.Equal(t, 7, metricsCount(m1))

	wantAttributes := td.attributes

	metrics1 := m1.InstrumentationLibraryMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)
	e1 := []testExpectation{
		assertMetricPresent("foo",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(19),
					},
				},
				{
					// renaming config converts any metric type to untyped metric, which then gets converted to gauge double type by metric builder
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(15),
						compareAttributes(map[string]string{"method": "post", "port": "6380"}),
					},
				},
			}),
		// renaming config converts any metric type to untyped metric, which then gets converted to gauge double type by metric builder
		assertMetricPresent("http_requests_total",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(10),
						compareAttributes(map[string]string{"method": "post", "port": "6380"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(12),
						compareAttributes(map[string]string{"method": "post", "port": "6381"}),
					},
				},
			}),
		assertMetricAbsent("rpc_duration_total"),
	}
	doCompare(t, "scrape-metricRename-1", wantAttributes, m1, e1)
}

func verifyRenameMetricKeepAction(t *testing.T, td *testData, resourceMetrics []*pdata.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]

	// m1 has 1 metrics + 5 internal scraper metrics
	assert.Equal(t, 6, metricsCount(m1))

	wantAttributes := td.attributes

	metrics1 := m1.InstrumentationLibraryMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)
	e1 := []testExpectation{
		assertMetricPresent("rpc_duration_total",
			compareMetricType(pdata.MetricDataTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareStartTimestamp(ts1),
						compareDoubleValue(100),
						compareAttributes(map[string]string{"method": "post", "port": "6380"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(120),
						compareAttributes(map[string]string{"method": "post", "port": "6381"}),
					},
				},
			}),
		assertMetricAbsent("http_go_threads"),
		assertMetricAbsent("http_connected_total"),
		assertMetricAbsent("redis_http_request_total"),
	}
	doCompare(t, "scrape-metricRenameKeepAction-1", wantAttributes, m1, e1)
}
