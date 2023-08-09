// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"testing"

	"github.com/prometheus/common/model"
	promcfg "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
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

	testComponent(t, targets, false, false, "", func(cfg *promcfg.Config) {
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

	testComponent(t, targets, false, false, "", func(cfg *promcfg.Config) {
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

func verifyRenameMetric(t *testing.T, td *testData, resourceMetrics []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]

	// m1 has 2 metrics + 5 internal scraper metrics
	assert.Equal(t, 7, metricsCount(m1))

	wantAttributes := td.attributes

	metrics1 := m1.ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)
	e1 := []testExpectation{
		assertMetricPresent("foo",
			compareMetricType(pmetric.MetricTypeGauge),
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
			compareMetricType(pmetric.MetricTypeGauge),
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

func verifyRenameMetricKeepAction(t *testing.T, td *testData, resourceMetrics []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]

	// m1 has 1 metrics + 5 internal scraper metrics
	assert.Equal(t, 6, metricsCount(m1))

	wantAttributes := td.attributes

	metrics1 := m1.ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)
	e1 := []testExpectation{
		assertMetricPresent("rpc_duration_total",
			compareMetricType(pmetric.MetricTypeSum),
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
		assertMetricAbsent("redis_http_requests_total"),
	}
	doCompare(t, "scrape-metricRenameKeepAction-1", wantAttributes, m1, e1)
}

var renamingLabel = `
# HELP http_go_threads Number of OS threads created
# TYPE http_go_threads gauge
http_go_threads 19

# HELP http_connected_total connected clients
# TYPE http_connected_total counter
http_connected_total{url="localhost",status="ok"} 15.0

# HELP redis_http_requests_total Redis connected clients
# TYPE redis_http_requests_total counter
redis_http_requests_total{method="post",port="6380"} 10.0
redis_http_requests_total{job="sample-app",statusCode="200"} 12.0

# HELP rpc_duration_total RPC clients
# TYPE rpc_duration_total counter
rpc_duration_total{monitor="codeLab",host="local"} 100.0
rpc_duration_total{address="localhost:9090/metrics",contentType="application/json"} 120.0
`

func TestLabelRenaming(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: renamingLabel},
			},
			validateFunc: verifyRenameLabel,
		},
	}

	testComponent(t, targets, false, false, "", func(cfg *promcfg.Config) {
		for _, scrapeConfig := range cfg.ScrapeConfigs {
			scrapeConfig.MetricRelabelConfigs = []*relabel.Config{
				{
					// this config should add new label {foo="bar"} to all metrics'
					Regex:       relabel.MustNewRegexp("(.*)"),
					Action:      relabel.Replace,
					TargetLabel: "foo",
					Replacement: "bar",
				},
				{
					// this config should create new label {id="target1/metrics"}
					// using the value from capture group of matched regex
					SourceLabels: model.LabelNames{"address"},
					Regex:        relabel.MustNewRegexp(".*/(.*)"),
					Action:       relabel.Replace,
					TargetLabel:  "id",
					Replacement:  "$1",
				},
				{
					// this config creates a new label for metrics that has matched regex label.
					// They key of this new label will be as given in 'replacement'
					// and value will be of the matched regex label value.
					Regex:       relabel.MustNewRegexp("method(.*)"),
					Action:      relabel.LabelMap,
					Replacement: "bar$1",
				},
				{
					// this config should drop the matched regex label
					Regex:  relabel.MustNewRegexp("(url.*)"),
					Action: relabel.LabelDrop,
				},
			}
		}
	})

}

func verifyRenameLabel(t *testing.T, td *testData, resourceMetrics []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]

	// m1 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m1))

	wantAttributes := td.attributes

	metrics1 := m1.ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)
	e1 := []testExpectation{
		assertMetricPresent("http_go_threads",
			compareMetricType(pmetric.MetricTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(19),
						compareAttributes(map[string]string{"foo": "bar"}),
					},
				},
			}),
		assertMetricPresent("http_connected_total",
			compareMetricType(pmetric.MetricTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(15),
						compareAttributes(map[string]string{"foo": "bar", "status": "ok"}),
					},
				},
			}),
		assertMetricPresent("redis_http_requests_total",
			compareMetricType(pmetric.MetricTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(10),
						compareAttributes(map[string]string{"method": "post", "port": "6380", "bar": "post", "foo": "bar"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(12),
						// since honor_label bool in config is true by default,
						// Prometheus reserved keywords like "job" and "instance" should be prefixed by "exported_"
						compareAttributes(map[string]string{"exported_job": "sample-app", "statusCode": "200", "foo": "bar"}),
					},
				},
			}),
		assertMetricPresent("rpc_duration_total",
			compareMetricType(pmetric.MetricTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(100),
						compareAttributes(map[string]string{"monitor": "codeLab", "host": "local", "foo": "bar"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(120),
						compareAttributes(map[string]string{"address": "localhost:9090/metrics",
							"contentType": "application/json", "id": "metrics", "foo": "bar"}),
					},
				},
			}),
	}
	doCompare(t, "scrape-labelRename-1", wantAttributes, m1, e1)
}

func TestLabelRenamingKeepAction(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: renamingLabel},
			},
			validateFunc: verifyRenameLabelKeepAction,
		},
	}

	testComponent(t, targets, false, false, "", func(cfg *promcfg.Config) {
		for _, scrapeConfig := range cfg.ScrapeConfigs {
			scrapeConfig.MetricRelabelConfigs = []*relabel.Config{
				{
					// this config should keep only metric that matches the regex metric name, and drop the rest
					Regex: relabel.MustNewRegexp("__name__|__scheme__|__address__|" +
						"__metrics_path__|__scrape_interval__|instance|job|(m.*)"),
					Action: relabel.LabelKeep,
				},
			}
		}
	})

}

func verifyRenameLabelKeepAction(t *testing.T, td *testData, resourceMetrics []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]

	// m1 has 4 metrics + 5 internal scraper metrics
	assert.Equal(t, 9, metricsCount(m1))

	wantAttributes := td.attributes

	metrics1 := m1.ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)
	e1 := []testExpectation{
		assertMetricPresent("http_go_threads",
			compareMetricType(pmetric.MetricTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(19),
						assertAttributesAbsent(),
					},
				},
			}),
		assertMetricPresent("http_connected_total",
			compareMetricType(pmetric.MetricTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(15),
						assertAttributesAbsent(),
					},
				},
			}),
		assertMetricPresent("redis_http_requests_total",
			compareMetricType(pmetric.MetricTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(10),
						compareAttributes(map[string]string{"method": "post"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(12),
						assertAttributesAbsent(),
					},
				},
			}),
		assertMetricPresent("rpc_duration_total",
			compareMetricType(pmetric.MetricTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(100),
						compareAttributes(map[string]string{"monitor": "codeLab"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(120),
						assertAttributesAbsent(),
					},
				},
			}),
	}
	doCompare(t, "scrape-LabelRenameKeepAction-1", wantAttributes, m1, e1)
}
