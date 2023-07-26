// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"testing"

	promcfg "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const targetExternalLabels = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 19`

func TestExternalLabels(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: targetExternalLabels},
			},
			validateFunc: verifyExternalLabels,
		},
	}

	testComponent(t, targets, false, false, "", func(cfg *promcfg.Config) {
		cfg.GlobalConfig.ExternalLabels = labels.FromStrings("key", "value")
	})
}

func verifyExternalLabels(t *testing.T, td *testData, rms []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, rms)
	require.Greater(t, len(rms), 0, "At least one resource metric should be present")

	wantAttributes := td.attributes
	metrics1 := rms[0].ScopeMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()
	doCompare(t, "scrape-externalLabels", wantAttributes, rms[0], []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pmetric.MetricTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(19),
						compareAttributes(map[string]string{"key": "value"}),
					},
				},
			}),
	})
}

const targetLabelLimit1 = `
# HELP test_gauge0 This is my gauge
# TYPE test_gauge0 gauge
test_gauge0{label1="value1",label2="value2"} 10
`

func verifyLabelLimitTarget1(t *testing.T, td *testData, rms []pmetric.ResourceMetrics) {
	// each sample in the scraped metrics is within the configured label_limit, scrape should be successful
	verifyNumValidScrapeResults(t, td, rms)
	require.Greater(t, len(rms), 0, "At least one resource metric should be present")

	want := td.attributes
	metrics1 := rms[0].ScopeMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()

	doCompare(t, "scrape-labelLimit", want, rms[0], []testExpectation{
		assertMetricPresent("test_gauge0",
			compareMetricType(pmetric.MetricTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(10),
						compareAttributes(map[string]string{"label1": "value1", "label2": "value2"}),
					},
				},
			},
		),
	})
}

const targetLabelLimit2 = `
# HELP test_gauge0 This is my gauge
# TYPE test_gauge0 gauge
test_gauge0{label1="value1",label2="value2",label3="value3"} 10
`

func verifyFailedScrape(t *testing.T, _ *testData, rms []pmetric.ResourceMetrics) {
	// Scrape should be unsuccessful since limit is exceeded in target2
	for _, rm := range rms {
		metrics := getMetrics(rm)
		assertUp(t, 0, metrics)
	}
}

func TestLabelLimitConfig(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: targetLabelLimit1},
			},
			validateFunc: verifyLabelLimitTarget1,
		},
		{
			name: "target2",
			pages: []mockPrometheusResponse{
				{code: 200, data: targetLabelLimit2},
			},
			validateFunc: verifyFailedScrape,
		},
	}

	testComponent(t, targets, false, false, "", func(cfg *promcfg.Config) {
		// set label limit in scrape_config
		for _, scrapeCfg := range cfg.ScrapeConfigs {
			scrapeCfg.LabelLimit = 5
		}
	})
}

const targetLabelLimits1 = `
# HELP test_gauge0 This is my gauge
# TYPE test_gauge0 gauge
test_gauge0{label1="value1",label2="value2"} 10

# HELP test_counter0 This is my counter
# TYPE test_counter0 counter
test_counter0{label1="value1",label2="value2"} 1

# HELP test_histogram0 This is my histogram
# TYPE test_histogram0 histogram
test_histogram0_bucket{label1="value1",label2="value2",le="0.1"} 1000
test_histogram0_bucket{label1="value1",label2="value2",le="0.5"} 1500
test_histogram0_bucket{label1="value1",label2="value2",le="1"} 2000
test_histogram0_bucket{label1="value1",label2="value2",le="+Inf"} 2500
test_histogram0_sum{label1="value1",label2="value2"} 5000
test_histogram0_count{label1="value1",label2="value2"} 2500

# HELP test_summary0 This is my summary
# TYPE test_summary0 summary
test_summary0{label1="value1",label2="value2",quantile="0.1"} 1
test_summary0{label1="value1",label2="value2",quantile="0.5"} 5
test_summary0{label1="value1",label2="value2",quantile="0.99"} 8
test_summary0_sum{label1="value1",label2="value2"} 5000
test_summary0_count{label1="value1",label2="value2"} 1000
`

func verifyLabelConfigTarget1(t *testing.T, td *testData, rms []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, rms)
	require.Greater(t, len(rms), 0, "At least one resource metric should be present")

	want := td.attributes
	metrics1 := rms[0].ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)

	e1 := []testExpectation{
		assertMetricPresent("test_counter0",
			compareMetricType(pmetric.MetricTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts1),
						compareTimestamp(ts1),
						compareDoubleValue(1),
						compareAttributes(map[string]string{"label1": "value1", "label2": "value2"}),
					},
				},
			}),
		assertMetricPresent("test_gauge0",
			compareMetricType(pmetric.MetricTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(10),
						compareAttributes(map[string]string{"label1": "value1", "label2": "value2"}),
					},
				},
			}),
		assertMetricPresent("test_histogram0",
			compareMetricType(pmetric.MetricTypeHistogram),
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(ts1),
						compareHistogramTimestamp(ts1),
						compareHistogram(2500, 5000, []uint64{1000, 500, 500, 500}),
						compareHistogramAttributes(map[string]string{"label1": "value1", "label2": "value2"}),
					},
				},
			}),
		assertMetricPresent("test_summary0",
			compareMetricType(pmetric.MetricTypeSummary),
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(ts1),
						compareSummaryTimestamp(ts1),
						compareSummary(1000, 5000, [][]float64{{0.1, 1}, {0.5, 5}, {0.99, 8}}),
						compareSummaryAttributes(map[string]string{"label1": "value1", "label2": "value2"}),
					},
				},
			}),
	}
	doCompare(t, "scrape-label-config-test", want, rms[0], e1)
}

const targetLabelNameLimit = `
# HELP test_gauge0 This is my gauge
# TYPE test_gauge0 gauge
test_gauge0{label1="value1",labelNameExceedingLimit="value2"} 10

# HELP test_counter0 This is my counter
# TYPE test_counter0 counter
test_counter0{label1="value1",label2="value2"} 1
`

func TestLabelNameLimitConfig(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: targetLabelLimits1},
			},
			validateFunc: verifyLabelConfigTarget1,
		},
		{
			name: "target2",
			pages: []mockPrometheusResponse{
				{code: 200, data: targetLabelNameLimit},
			},
			validateFunc: verifyFailedScrape,
		},
	}

	testComponent(t, targets, false, false, "", func(cfg *promcfg.Config) {
		// set label limit in scrape_config
		for _, scrapeCfg := range cfg.ScrapeConfigs {
			scrapeCfg.LabelNameLengthLimit = 20
		}
	})
}

const targetLabelValueLimit = `
# HELP test_gauge0 This is my gauge
# TYPE test_gauge0 gauge
test_gauge0{label1="value1",label2="label-value-exceeding-limit"} 10

# HELP test_counter0 This is my counter
# TYPE test_counter0 counter
test_counter0{label1="value1",label2="value2"} 1
`

func TestLabelValueLimitConfig(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: targetLabelLimits1},
			},
			validateFunc: verifyLabelConfigTarget1,
		},
		{
			name: "target2",
			pages: []mockPrometheusResponse{
				{code: 200, data: targetLabelValueLimit},
			},
			validateFunc: verifyFailedScrape,
		},
	}

	testComponent(t, targets, false, false, "", func(cfg *promcfg.Config) {
		// set label name limit in scrape_config
		for _, scrapeCfg := range cfg.ScrapeConfigs {
			scrapeCfg.LabelValueLengthLimit = 25
		}
	})
}

// for all metric types, testLabel has empty value
const emptyLabelValuesTarget1 = `
# HELP test_gauge0 This is my gauge
# TYPE test_gauge0 gauge
test_gauge0{id="1",testLabel=""} 19

# HELP test_counter0 This is my counter
# TYPE test_counter0 counter
test_counter0{id="1",testLabel=""} 100

# HELP test_histogram0 This is my histogram
# TYPE test_histogram0 histogram
test_histogram0_bucket{id="1",testLabel="",le="0.1"} 1000
test_histogram0_bucket{id="1",testLabel="",le="0.5"} 1500
test_histogram0_bucket{id="1",testLabel="",le="1"} 2000
test_histogram0_bucket{id="1",testLabel="",le="+Inf"} 2500
test_histogram0_sum{id="1",testLabel=""} 5000
test_histogram0_count{id="1",testLabel=""} 2500

# HELP test_summary0 This is my summary
# TYPE test_summary0 summary
test_summary0{id="1",testLabel="",quantile="0.1"} 1
test_summary0{id="1",testLabel="",quantile="0.5"} 5
test_summary0{id="1",testLabel="",quantile="0.99"} 8
test_summary0_sum{id="1",testLabel=""} 5000
test_summary0_count{id="1",testLabel=""} 1000
`

func verifyEmptyLabelValuesTarget1(t *testing.T, td *testData, rms []pmetric.ResourceMetrics) {
	require.Greater(t, len(rms), 0, "At least one resource metric should be present")

	want := td.attributes
	metrics1 := rms[0].ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)

	e1 := []testExpectation{
		assertMetricPresent("test_gauge0",
			compareMetricType(pmetric.MetricTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(19),
						compareAttributes(map[string]string{"id": "1"}),
					},
				},
			}),
		assertMetricPresent("test_counter0",
			compareMetricType(pmetric.MetricTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(100),
						compareAttributes(map[string]string{"id": "1"}),
					},
				},
			}),
		assertMetricPresent("test_histogram0",
			compareMetricType(pmetric.MetricTypeHistogram),
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(ts1),
						compareHistogramTimestamp(ts1),
						compareHistogram(2500, 5000, []uint64{1000, 500, 500, 500}),
						compareHistogramAttributes(map[string]string{"id": "1"}),
					},
				},
			}),
		assertMetricPresent("test_summary0",
			compareMetricType(pmetric.MetricTypeSummary),
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(ts1),
						compareSummaryTimestamp(ts1),
						compareSummary(1000, 5000, [][]float64{{0.1, 1}, {0.5, 5}, {0.99, 8}}),
						compareSummaryAttributes(map[string]string{"id": "1"}),
					},
				},
			}),
	}
	doCompare(t, "scrape-empty-label-values-1", want, rms[0], e1)
}

// target has two time series for both gauge and counter, only one time series has a value for the label "testLabel"
const emptyLabelValuesTarget2 = `
# HELP test_gauge0 This is my gauge.
# TYPE test_gauge0 gauge
test_gauge0{id="1",testLabel=""} 19
test_gauge0{id="2",testLabel="foobar"} 2

# HELP test_counter0 This is my counter
# TYPE test_counter0 counter
test_counter0{id="1",testLabel=""} 100
test_counter0{id="2",testLabel="foobar"} 110
`

func verifyEmptyLabelValuesTarget2(t *testing.T, td *testData, rms []pmetric.ResourceMetrics) {
	require.Greater(t, len(rms), 0, "At least one resource metric should be present")

	want := td.attributes
	metrics1 := rms[0].ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)

	e1 := []testExpectation{
		assertMetricPresent("test_gauge0",
			compareMetricType(pmetric.MetricTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(19),
						compareAttributes(map[string]string{"id": "1"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(2),
						compareAttributes(map[string]string{"id": "2", "testLabel": "foobar"}),
					},
				},
			}),
		assertMetricPresent("test_counter0",
			compareMetricType(pmetric.MetricTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(100),
						compareAttributes(map[string]string{"id": "1"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(110),
						compareAttributes(map[string]string{"id": "2", "testLabel": "foobar"}),
					},
				},
			}),
	}
	doCompare(t, "scrape-empty-label-values-2", want, rms[0], e1)
}

func TestEmptyLabelValues(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: emptyLabelValuesTarget1},
			},
			validateFunc: verifyEmptyLabelValuesTarget1,
		},
		{
			name: "target2",
			pages: []mockPrometheusResponse{
				{code: 200, data: emptyLabelValuesTarget2},
			},
			validateFunc: verifyEmptyLabelValuesTarget2,
		},
	}
	testComponent(t, targets, false, false, "")
}

const honorLabelsTarget = `
# HELP test_gauge0 This is my gauge
# TYPE test_gauge0 gauge
test_gauge0{instance="hostname:8080",job="honor_labels_test",testLabel="value1"} 1
`

func verifyHonorLabelsFalse(t *testing.T, td *testData, rms []pmetric.ResourceMetrics) {
	want := td.attributes
	require.Greater(t, len(rms), 0, "At least one resource metric should be present")

	metrics1 := rms[0].ScopeMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()

	doCompare(t, "honor_labels_false", want, rms[0], []testExpectation{
		assertMetricPresent("test_gauge0",
			compareMetricType(pmetric.MetricTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(1),
						// job and instance labels must be prefixed with "exported_"
						compareAttributes(map[string]string{"exported_job": "honor_labels_test", "exported_instance": "hostname:8080", "testLabel": "value1"}),
					},
				},
			}),
	})
}

// for all scalar metric types there are no labels
const emptyLabelsTarget1 = `
# HELP test_gauge0 This is my gauge
# TYPE test_gauge0 gauge
test_gauge0 19

# HELP test_counter0 This is my counter
# TYPE test_counter0 counter
test_counter0 100
`

func verifyEmptyLabelsTarget1(t *testing.T, td *testData, rms []pmetric.ResourceMetrics) {
	require.Greater(t, len(rms), 0, "At least one resource metric should be present")

	want := td.attributes
	metrics1 := rms[0].ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)

	e1 := []testExpectation{
		assertMetricPresent(
			"test_gauge0",
			compareMetricType(pmetric.MetricTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(19),
						compareAttributes(map[string]string{}),
					},
				},
			},
		),
		assertMetricPresent(
			"test_counter0",
			compareMetricType(pmetric.MetricTypeSum),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(100),
						compareAttributes(map[string]string{}),
					},
				},
			},
		),
	}
	doCompare(t, "scrape-empty-labels-1", want, rms[0], e1)
}

func TestEmptyLabels(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: emptyLabelsTarget1},
			},
			validateFunc: verifyEmptyLabelsTarget1,
		},
	}
	testComponent(t, targets, false, false, "")
}

func TestHonorLabelsFalseConfig(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: honorLabelsTarget},
			},
			validateFunc: verifyHonorLabelsFalse,
		},
	}

	testComponent(t, targets, false, false, "")
}

func verifyHonorLabelsTrue(t *testing.T, td *testData, rms []pmetric.ResourceMetrics) {
	require.Greater(t, len(rms), 0, "At least one resource metric should be present")

	// job and instance label values should be honored from honorLabelsTarget
	expectedAttributes := td.attributes
	expectedAttributes.PutStr("service.name", "honor_labels_test")
	expectedAttributes.PutStr("service.instance.id", "hostname:8080")
	expectedAttributes.PutStr("net.host.port", "8080")
	expectedAttributes.PutStr("net.host.name", "hostname")

	metrics1 := rms[0].ScopeMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()

	doCompare(t, "honor_labels_true", expectedAttributes, rms[0], []testExpectation{
		assertMetricPresent("test_gauge0",
			compareMetricType(pmetric.MetricTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(1),
						compareAttributes(map[string]string{"testLabel": "value1"}),
					},
				},
			}),
	})
}

func TestHonorLabelsTrueConfig(t *testing.T) {
	targets := []*testData{
		{
			name: "honor_labels_test",
			pages: []mockPrometheusResponse{
				{code: 200, data: honorLabelsTarget},
			},
			validateFunc: verifyHonorLabelsTrue,
		},
	}

	testComponent(t, targets, false, false, "", func(cfg *promcfg.Config) {
		// set label name limit in scrape_config
		for _, scrapeCfg := range cfg.ScrapeConfigs {
			scrapeCfg.HonorLabels = true
		}
	})
}

const targetRelabelJobInstance = `
# HELP jvm_memory_bytes_used Used bytes of a given JVM memory area.
# TYPE jvm_memory_bytes_used gauge
jvm_memory_bytes_used{area="heap"} 100
`

func TestRelabelJobInstance(t *testing.T) {
	targets := []*testData{
		{
			name:         "target1",
			relabeledJob: "not-target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: targetRelabelJobInstance},
			},
			validateFunc: verifyRelabelJobInstance,
		},
	}

	testComponent(t, targets, false, false, "", func(cfg *promcfg.Config) {
		for _, scrapeConfig := range cfg.ScrapeConfigs {
			scrapeConfig.MetricRelabelConfigs = []*relabel.Config{
				{
					// this config should replace the instance label with 'relabeled-instance'
					Action:      relabel.Replace,
					Regex:       relabel.MustNewRegexp("(.*)"),
					TargetLabel: "instance",
					Replacement: "relabeled-instance",
				},
				{
					// this config should replace the job label with 'not-target1'
					Action:      relabel.Replace,
					Regex:       relabel.MustNewRegexp("(.*)"),
					TargetLabel: "job",
					Replacement: "not-target1",
				},
			}
		}
	})
}

func verifyRelabelJobInstance(t *testing.T, td *testData, rms []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, rms)
	require.Greater(t, len(rms), 0, "At least one resource metric should be present")

	wantAttributes := td.attributes
	wantAttributes.PutStr("service.name", "not-target1")
	wantAttributes.PutStr("service.instance.id", "relabeled-instance")
	wantAttributes.PutStr("net.host.port", "")
	wantAttributes.PutStr("net.host.name", "relabeled-instance")

	metrics1 := rms[0].ScopeMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()
	doCompare(t, "relabel-job-instance", wantAttributes, rms[0], []testExpectation{
		assertMetricPresent("jvm_memory_bytes_used",
			compareMetricType(pmetric.MetricTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(100),
						compareAttributes(map[string]string{"area": "heap"}),
					},
				},
			}),
	})
}

const targetResourceAttsInTargetInfo = `
# HELP jvm_memory_bytes_used Used bytes of a given JVM memory area.
# TYPE jvm_memory_bytes_used gauge
jvm_memory_bytes_used{area="heap"} 100
# HELP target_info has the resource attributes
# TYPE target_info gauge
target_info{foo="bar", team="infra"} 1
`

func TestTargetInfoResourceAttributes(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: targetResourceAttsInTargetInfo},
			},
			validateFunc: verifyTargetInfoResourceAttributes,
		},
	}

	testComponent(t, targets, false, false, "")
}

func verifyTargetInfoResourceAttributes(t *testing.T, td *testData, rms []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, rms)
	require.Greater(t, len(rms), 0, "At least one resource metric should be present")

	wantAttributes := td.attributes
	wantAttributes.PutStr("foo", "bar")
	wantAttributes.PutStr("team", "infra")

	metrics1 := rms[0].ScopeMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()
	doCompare(t, "relabel-job-instance", wantAttributes, rms[0], []testExpectation{
		assertMetricPresent("jvm_memory_bytes_used",
			compareMetricType(pmetric.MetricTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(100),
						compareAttributes(map[string]string{"area": "heap"}),
					},
				},
			}),
	})
}
