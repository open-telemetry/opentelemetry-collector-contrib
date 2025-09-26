// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"strings"
	"testing"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"
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

	testComponent(t, targets, nil, func(cfg *PromConfig) {
		cfg.GlobalConfig.ExternalLabels = labels.FromStrings("key", "value")
	})
}

func verifyExternalLabels(t *testing.T, td *testData, rms []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, rms)
	require.NotEmpty(t, rms, "At least one resource metric should be present")

	wantAttributes := td.attributes
	metrics1 := rms[0].ScopeMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()
	doCompare(t, "scrape-externalLabels", wantAttributes, rms[0], []metricExpectation{
		{
			"go_threads",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(19),
						compareAttributes(map[string]string{"key": "value"}),
					},
				},
			},
			nil,
		},
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
	require.NotEmpty(t, rms, "At least one resource metric should be present")

	want := td.attributes
	metrics1 := rms[0].ScopeMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()

	doCompare(t, "scrape-labelLimit", want, rms[0], []metricExpectation{
		{
			"test_gauge0",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(10),
						compareAttributes(map[string]string{"label1": "value1", "label2": "value2"}),
					},
				},
			},
			nil,
		},
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

	testComponent(t, targets, nil, func(cfg *PromConfig) {
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
	require.NotEmpty(t, rms, "At least one resource metric should be present")

	want := td.attributes
	metrics1 := rms[0].ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)

	e1 := []metricExpectation{
		{
			"test_counter0",
			pmetric.MetricTypeSum,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts1),
						compareTimestamp(ts1),
						compareDoubleValue(1),
						compareAttributes(map[string]string{"label1": "value1", "label2": "value2"}),
					},
				},
			},
			nil,
		},
		{
			"test_gauge0",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(10),
						compareAttributes(map[string]string{"label1": "value1", "label2": "value2"}),
					},
				},
			},
			nil,
		},
		{
			"test_histogram0",
			pmetric.MetricTypeHistogram,
			"",
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(ts1),
						compareHistogramTimestamp(ts1),
						compareHistogram(2500, 5000, []float64{0.1, 0.5, 1}, []uint64{1000, 500, 500, 500}),
						compareHistogramAttributes(map[string]string{"label1": "value1", "label2": "value2"}),
					},
				},
			},
			nil,
		},
		{
			"test_summary0",
			pmetric.MetricTypeSummary,
			"",
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(ts1),
						compareSummaryTimestamp(ts1),
						compareSummary(1000, 5000, [][]float64{{0.1, 1}, {0.5, 5}, {0.99, 8}}),
						compareSummaryAttributes(map[string]string{"label1": "value1", "label2": "value2"}),
					},
				},
			},
			nil,
		},
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

	testComponent(t, targets, nil, func(cfg *PromConfig) {
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

	testComponent(t, targets, nil, func(cfg *PromConfig) {
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
	require.NotEmpty(t, rms, "At least one resource metric should be present")

	want := td.attributes
	metrics1 := rms[0].ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)

	e1 := []metricExpectation{
		{
			"test_gauge0",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(19),
						compareAttributes(map[string]string{"id": "1"}),
					},
				},
			},
			nil,
		},
		{
			"test_counter0",
			pmetric.MetricTypeSum,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(100),
						compareAttributes(map[string]string{"id": "1"}),
					},
				},
			},
			nil,
		},
		{
			"test_histogram0",
			pmetric.MetricTypeHistogram,
			"",
			[]dataPointExpectation{
				{
					histogramPointComparator: []histogramPointComparator{
						compareHistogramStartTimestamp(ts1),
						compareHistogramTimestamp(ts1),
						compareHistogram(2500, 5000, []float64{0.1, 0.5, 1}, []uint64{1000, 500, 500, 500}),
						compareHistogramAttributes(map[string]string{"id": "1"}),
					},
				},
			},
			nil,
		},
		{
			"test_summary0",
			pmetric.MetricTypeSummary,
			"",
			[]dataPointExpectation{
				{
					summaryPointComparator: []summaryPointComparator{
						compareSummaryStartTimestamp(ts1),
						compareSummaryTimestamp(ts1),
						compareSummary(1000, 5000, [][]float64{{0.1, 1}, {0.5, 5}, {0.99, 8}}),
						compareSummaryAttributes(map[string]string{"id": "1"}),
					},
				},
			},
			nil,
		},
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
	require.NotEmpty(t, rms, "At least one resource metric should be present")

	want := td.attributes
	metrics1 := rms[0].ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)

	e1 := []metricExpectation{
		{
			"test_gauge0",
			pmetric.MetricTypeGauge,
			"",
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
			},
			nil,
		},
		{
			"test_counter0",
			pmetric.MetricTypeSum,
			"",
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
			},
			nil,
		},
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
	testComponent(t, targets, nil)
}

const honorLabelsTarget = `
# HELP test_gauge0 This is my gauge
# TYPE test_gauge0 gauge
test_gauge0{instance="hostname:8080",job="honor_labels_test",testLabel="value1"} 1
`

func verifyHonorLabelsFalse(t *testing.T, td *testData, rms []pmetric.ResourceMetrics) {
	want := td.attributes
	require.NotEmpty(t, rms, "At least one resource metric should be present")

	metrics1 := rms[0].ScopeMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()

	doCompare(t, "honor_labels_false", want, rms[0], []metricExpectation{
		{
			"test_gauge0",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(1),
						// job and instance labels must be prefixed with "exported_"
						compareAttributes(map[string]string{"exported_job": "honor_labels_test", "exported_instance": "hostname:8080", "testLabel": "value1"}),
					},
				},
			},
			nil,
		},
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
	require.NotEmpty(t, rms, "At least one resource metric should be present")

	want := td.attributes
	metrics1 := rms[0].ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)

	e1 := []metricExpectation{
		{
			"test_gauge0",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(19),
						compareAttributes(map[string]string{}),
					},
				},
			},
			nil,
		},
		{
			"test_counter0",
			pmetric.MetricTypeSum,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(100),
						compareAttributes(map[string]string{}),
					},
				},
			},
			nil,
		},
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
	testComponent(t, targets, nil)
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

	testComponent(t, targets, nil)
}

func verifyHonorLabelsTrue(t *testing.T, td *testData, rms []pmetric.ResourceMetrics) {
	require.NotEmpty(t, rms, "At least one resource metric should be present")

	// job and instance label values should be honored from honorLabelsTarget
	expectedResourceAttributes := pcommon.NewMap()
	td.attributes.CopyTo(expectedResourceAttributes)
	expectedResourceAttributes.PutStr("service.name", "honor_labels_test")
	expectedResourceAttributes.PutStr("service.instance.id", "hostname:8080")
	expectedResourceAttributes.PutStr("server.port", "8080")
	expectedResourceAttributes.PutStr("server.address", "hostname")

	expectedScrapeConfigAttributes := td.attributes

	// get the resource created for the scraped metric
	var resourceMetric pmetric.ResourceMetrics
	var scrapeConfigResourceMetrics pmetric.ResourceMetrics
	gotScrapeConfigMetrics, gotResourceMetrics := false, false
	for _, rm := range rms {
		serviceInstance, ok := rm.Resource().Attributes().Get(string(semconv.ServiceInstanceIDKey))
		require.True(t, ok)
		if serviceInstance.AsString() == "hostname:8080" {
			resourceMetric = rm
			gotResourceMetrics = true
		} else {
			scrapeConfigResourceMetrics = rm
			gotScrapeConfigMetrics = true
		}
		if gotResourceMetrics && gotScrapeConfigMetrics {
			break
		}
	}

	metrics1 := resourceMetric.ScopeMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()

	// check the scrape metrics of the resource created from the scrape config
	doCompare(t, "honor_labels_true", expectedScrapeConfigAttributes, scrapeConfigResourceMetrics, nil)

	// assert that the gauge metric has been retrieved correctly. This resource only contains the gauge and no scrape metrics,
	// so we directly check the gauge metric without the scrape metrics
	assertExpectedAttributes(t, expectedResourceAttributes, resourceMetric)
	assertExpectedMetrics(t, []metricExpectation{
		{
			"test_gauge0",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(1),
						compareAttributes(map[string]string{"testLabel": "value1"}),
					},
				},
			},
			nil,
		},
	}, resourceMetric, false, true)
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

	testComponent(t, targets, nil, func(cfg *PromConfig) {
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

	testComponent(t, targets, nil, func(cfg *PromConfig) {
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
	require.NotEmpty(t, rms, "At least one resource metric should be present")

	wantAttributes := td.attributes
	wantAttributes.PutStr("service.name", "not-target1")
	wantAttributes.PutStr("service.instance.id", "relabeled-instance")
	wantAttributes.PutStr("server.port", "")
	wantAttributes.PutStr("server.address", "relabeled-instance")

	metrics1 := rms[0].ScopeMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()

	// directly assert the expected attributes and the expected metrics,
	// as the scrape metrics which are checked within the doCompare function
	// are not included in this resourceMetric, which only contains the relabeled
	// metrics
	assertExpectedAttributes(t, wantAttributes, rms[0])
	assertExpectedMetrics(t, []metricExpectation{
		{
			"jvm_memory_bytes_used",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(100),
						compareAttributes(map[string]string{"area": "heap"}),
					},
				},
			},
			nil,
		},
	}, rms[0], false, true)
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

	testComponent(t, targets, nil)
}

func verifyTargetInfoResourceAttributes(t *testing.T, td *testData, rms []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, rms)
	require.NotEmpty(t, rms, "At least one resource metric should be present")

	wantAttributes := td.attributes
	wantAttributes.PutStr("foo", "bar")
	wantAttributes.PutStr("team", "infra")

	metrics1 := rms[0].ScopeMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()
	doCompare(t, "relabel-job-instance", wantAttributes, rms[0], []metricExpectation{
		{
			"jvm_memory_bytes_used",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(100),
						compareAttributes(map[string]string{"area": "heap"}),
					},
				},
			},
			nil,
		},
	})
}

const targetInstrumentationScopeInfoMetric = `
# HELP jvm_memory_bytes_used Used bytes of a given JVM memory area.
# TYPE jvm_memory_bytes_used gauge
jvm_memory_bytes_used{area="heap", otel_scope_name="fake.scope.name", otel_scope_version="v0.1.0", otel_scope_schema_url="https://opentelemetry.io/schemas/1.21.0"} 100
jvm_memory_bytes_used{area="heap", otel_scope_name="scope.with.attributes", otel_scope_version="v1.5.0"} 100
jvm_memory_bytes_used{area="heap"} 100
# TYPE otel_scope_info gauge
otel_scope_info{animal="bear", otel_scope_name="scope.with.attributes", otel_scope_version="v1.5.0"} 1
`

const targetInstrumentationScopeLabels = `
# HELP jvm_memory_bytes_used Used bytes of a given JVM memory area.
# TYPE jvm_memory_bytes_used gauge
jvm_memory_bytes_used{area="heap", otel_scope_name="fake.scope.name", otel_scope_version="v0.1.0", otel_scope_schema_url="https://opentelemetry.io/schemas/1.21.0"} 100
jvm_memory_bytes_used{area="heap", otel_scope_name="scope.with.attributes", otel_scope_version="v1.5.0", otel_scope_animal="bear", otel_scope_color="blue"} 200
jvm_memory_bytes_used{area="heap"} 100
`

const targetMixedScopeInfoAndLabels = `
# HELP jvm_memory_bytes_used Used bytes of a given JVM memory area.
# TYPE jvm_memory_bytes_used gauge
jvm_memory_bytes_used{area="heap", otel_scope_name="scope.from.labels", otel_scope_version="v1.0.0", otel_scope_animal="bear", otel_scope_environment="staging"} 100
jvm_memory_bytes_used{area="heap", otel_scope_name="scope.from.info", otel_scope_version="v2.0.0", otel_scope_animal="cat"} 200
jvm_memory_bytes_used{area="heap"} 300
# TYPE otel_scope_info gauge
otel_scope_info{otel_scope_name="scope.from.info", otel_scope_version="v2.0.0", team="backend", environment="production"} 1
`

const targetMixedWithConflicts = `
# HELP jvm_memory_bytes_used Used bytes of a given JVM memory area.
# TYPE jvm_memory_bytes_used gauge
jvm_memory_bytes_used{area="heap", otel_scope_name="conflicted.scope", otel_scope_version="v1.0.0", otel_scope_animal="bear", otel_scope_team="frontend"} 100
jvm_memory_bytes_used{area="heap"} 200
# TYPE otel_scope_info gauge
otel_scope_info{otel_scope_name="conflicted.scope", otel_scope_version="v1.0.0", animal="cat", team="backend", environment="production"} 1
`

// TestScopeAttributes tests scope attribute extraction behavior.
// Note: Some tests may fail until the implementation is updated to match the target behavior
// defined in the implementation plan (always extract otel_scope_ labels as scope attributes).
func TestScopeAttributes(t *testing.T) {
	t.Run("ScopeInfoMetric", func(t *testing.T) {
		// Test parsing otel_scope_info metric (feature gate disabled)
		defer testutil.SetFeatureGateForTest(t, internal.RemoveScopeInfoGate, false)()

		targets := []*testData{
			{
				name: "target1",
				pages: []mockPrometheusResponse{
					{code: 200, data: targetInstrumentationScopeInfoMetric},
				},
				validateFunc: verifyTargetWithScopeInfoMetric,
			},
		}

		testComponent(t, targets, nil)
	})

	t.Run("ScopeLabels/feature_enabled", func(t *testing.T) {
		// Test parsing otel_scope_ labels (feature gate enabled)
		defer testutil.SetFeatureGateForTest(t, internal.RemoveScopeInfoGate, true)()

		targets := []*testData{
			{
				name: "target1",
				pages: []mockPrometheusResponse{
					{code: 200, data: targetInstrumentationScopeLabels},
				},
				validateFunc: verifyTargetWithScopeLabels,
			},
		}

		testComponent(t, targets, nil)
	})

	t.Run("ScopeLabels/feature_disabled", func(t *testing.T) {
		// Test parsing otel_scope_ labels (feature gate disabled)
		defer testutil.SetFeatureGateForTest(t, internal.RemoveScopeInfoGate, false)()

		targets := []*testData{
			{
				name: "target1",
				pages: []mockPrometheusResponse{
					{code: 200, data: targetInstrumentationScopeLabels},
				},
				validateFunc: verifyTargetWithScopeLabels,
			},
		}

		testComponent(t, targets, nil)
	})

	t.Run("Mixed/ScopeInfoAndLabels/feature_disabled", func(t *testing.T) {
		// Test mixed scenario: both otel_scope_info and otel_scope_ labels (feature gate disabled)
		defer testutil.SetFeatureGateForTest(t, internal.RemoveScopeInfoGate, false)()

		targets := []*testData{
			{
				name: "target1",
				pages: []mockPrometheusResponse{
					{code: 200, data: targetMixedScopeInfoAndLabels},
				},
				validateFunc: verifyMixedScopeInfoAndLabels,
			},
		}

		testComponent(t, targets, nil)
	})

	t.Run("Mixed/ScopeInfoAndLabels/feature_enabled", func(t *testing.T) {
		// Test mixed scenario: both otel_scope_info and otel_scope_ labels (feature gate enabled)
		defer testutil.SetFeatureGateForTest(t, internal.RemoveScopeInfoGate, true)()

		targets := []*testData{
			{
				name: "target1",
				pages: []mockPrometheusResponse{
					{code: 200, data: targetMixedScopeInfoAndLabels},
				},
				validateFunc: verifyMixedScopeInfoAndLabelsFeatureEnabled,
			},
		}

		testComponent(t, targets, nil)
	})

	t.Run("Mixed/ConflictResolution/feature_disabled", func(t *testing.T) {
		// Test conflict resolution: otel_scope_ labels should win over otel_scope_info
		defer testutil.SetFeatureGateForTest(t, internal.RemoveScopeInfoGate, false)()

		targets := []*testData{
			{
				name: "target1",
				pages: []mockPrometheusResponse{
					{code: 200, data: targetMixedWithConflicts},
				},
				validateFunc: verifyMixedConflictResolution,
			},
		}

		testComponent(t, targets, nil)
	})
}

func verifyTargetWithScopeInfoMetric(t *testing.T, td *testData, rms []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, rms)
	require.NotEmpty(t, rms, "At least one resource metric should be present")

	sms := rms[0].ScopeMetrics()
	require.Equal(t, 3, sms.Len(), "Three scope metrics should be present")
	sms.Sort(func(a, b pmetric.ScopeMetrics) bool {
		return a.Scope().Name() < b.Scope().Name()
	})

	require.Equal(t, "fake.scope.name", sms.At(0).Scope().Name())
	require.Equal(t, "v0.1.0", sms.At(0).Scope().Version())
	require.Equal(t, "https://opentelemetry.io/schemas/1.21.0", sms.At(0).SchemaUrl())
	require.Equal(t, 0, sms.At(0).Scope().Attributes().Len())
	require.Equal(t, "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver", sms.At(1).Scope().Name())
	require.Empty(t, sms.At(1).SchemaUrl())
	require.Equal(t, 0, sms.At(1).Scope().Attributes().Len())
	require.Equal(t, "scope.with.attributes", sms.At(2).Scope().Name())
	require.Equal(t, "v1.5.0", sms.At(2).Scope().Version())
	require.Empty(t, sms.At(2).SchemaUrl())
	require.Equal(t, 1, sms.At(2).Scope().Attributes().Len())
	scopeAttrVal, found := sms.At(2).Scope().Attributes().Get("animal")
	require.True(t, found)
	require.Equal(t, "bear", scopeAttrVal.Str())

	// Check that otel_scope_name, otel_scope_version, and otel_scope_schema_url are dropped from metric data point attributes
	require.Equal(t, 1, sms.At(0).Metrics().Len())
	metric := sms.At(0).Metrics().At(0)
	dp := metric.Gauge().DataPoints().At(0)
	require.Equal(t, 1, dp.Attributes().Len(), "Should only have 'area' attribute")
	_, found = dp.Attributes().Get("area")
	require.True(t, found, "Should only have 'area' attribute")
}

func verifyTargetWithScopeLabels(t *testing.T, td *testData, rms []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, rms)
	require.NotEmpty(t, rms, "At least one resource metric should be present")

	sms := rms[0].ScopeMetrics()
	require.Equal(t, 3, sms.Len(), "Three scope metrics should be present in new mode")
	sms.Sort(func(a, b pmetric.ScopeMetrics) bool {
		return a.Scope().Name() < b.Scope().Name()
	})

	// Verify first scope: fake.scope.name (no attributes)
	require.Equal(t, "fake.scope.name", sms.At(0).Scope().Name())
	require.Equal(t, "v0.1.0", sms.At(0).Scope().Version())
	require.Equal(t, "https://opentelemetry.io/schemas/1.21.0", sms.At(0).SchemaUrl())
	require.Equal(t, 0, sms.At(0).Scope().Attributes().Len())

	// Verify second scope: default receiver scope (no attributes)
	require.Equal(t, "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver", sms.At(1).Scope().Name())
	require.Empty(t, sms.At(1).SchemaUrl())
	require.Equal(t, 0, sms.At(1).Scope().Attributes().Len())

	// Verify third scope: scope.with.attributes with attributes from otel_scope_ labels
	require.Equal(t, "scope.with.attributes", sms.At(2).Scope().Name())
	require.Equal(t, "v1.5.0", sms.At(2).Scope().Version())
	require.Empty(t, sms.At(2).SchemaUrl())
	require.Equal(t, 2, sms.At(2).Scope().Attributes().Len())
	animalVal, found := sms.At(2).Scope().Attributes().Get("animal")
	require.True(t, found)
	require.Equal(t, "bear", animalVal.Str())
	colorVal, found := sms.At(2).Scope().Attributes().Get("color")
	require.True(t, found)
	require.Equal(t, "blue", colorVal.Str())

	// Check that otel_scope_ labels are filtered from metric data point attributes
	require.Equal(t, 1, sms.At(2).Metrics().Len())
	metric := sms.At(2).Metrics().At(0)
	dp := metric.Gauge().DataPoints().At(0)
	require.Equal(t, 1, dp.Attributes().Len(), "Should only have 'area' attribute, otel_scope_ labels should be filtered")
	_, found = dp.Attributes().Get("area")
	require.True(t, found, "Should have 'area' attribute")
	_, found = dp.Attributes().Get("otel_scope_animal")
	require.False(t, found, "Should not have otel_scope_animal attribute in datapoint")
	_, found = dp.Attributes().Get("otel_scope_color")
	require.False(t, found, "Should not have otel_scope_color attribute in datapoint")

	// Verify scope grouping: Test that scope attributes are correctly extracted and grouped
	// scope.with.attributes should have 1 metric with 2 attributes (animal=bear, color=blue)
	scopeWithAttrs := sms.At(2).Scope()
	require.Equal(t, "scope.with.attributes", scopeWithAttrs.Name())
	require.Equal(t, 1, sms.At(2).Metrics().Len(), "scope.with.attributes should have 1 metric")
	require.Equal(t, 2, scopeWithAttrs.Attributes().Len(), "scope.with.attributes should have 2 attributes")

	// Verify that the default receiver scope has multiple metrics (including the one without otel_scope_ labels)
	defaultScope := sms.At(1)
	require.GreaterOrEqual(t, defaultScope.Metrics().Len(), 1, "default scope should have at least 1 metric")
	require.Equal(t, 0, defaultScope.Scope().Attributes().Len(), "default scope should have no attributes")
}

// verifyMixedScopeInfoAndLabels tests the target behavior where both otel_scope_info metrics
// and otel_scope_ labels are present, with feature gate that removes scope info parsing disabled.
// attributes from otel_scope_info should be merged with attributes from otel_scope_ labels when scope is the same.
func verifyMixedScopeInfoAndLabels(t *testing.T, td *testData, rms []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, rms)
	require.NotEmpty(t, rms, "At least one resource metric should be present")

	sms := rms[0].ScopeMetrics()
	require.Equal(t, 3, sms.Len(), "Three scope metrics should be present")
	sms.Sort(func(a, b pmetric.ScopeMetrics) bool {
		return a.Scope().Name() < b.Scope().Name()
	})

	// Verify default receiver scope (no attributes)
	// This one is used for the metric without any otel_scope_ labels
	require.Equal(t, "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver", sms.At(0).Scope().Name())
	require.Equal(t, 0, sms.At(0).Scope().Attributes().Len())

	// Verify scope.from.info: should have attributes from otel_scope_info, plus the one from otel_scope_ labels
	require.Equal(t, "scope.from.info", sms.At(1).Scope().Name())
	require.Equal(t, "v2.0.0", sms.At(1).Scope().Version())
	require.Equal(t, 3, sms.At(1).Scope().Attributes().Len())
	teamVal, found := sms.At(1).Scope().Attributes().Get("team")
	require.True(t, found)
	require.Equal(t, "backend", teamVal.Str())
	envVal, found := sms.At(1).Scope().Attributes().Get("environment")
	require.True(t, found)
	require.Equal(t, "production", envVal.Str())
	animalVal, found := sms.At(1).Scope().Attributes().Get("animal")
	require.True(t, found)
	require.Equal(t, "cat", animalVal.Str())

	// Verify scope.from.labels: should have attributes from otel_scope_ labels
	require.Equal(t, "scope.from.labels", sms.At(2).Scope().Name())
	require.Equal(t, "v1.0.0", sms.At(2).Scope().Version())
	require.Equal(t, 2, sms.At(2).Scope().Attributes().Len())
	animalVal, found = sms.At(2).Scope().Attributes().Get("animal")
	require.True(t, found)
	require.Equal(t, "bear", animalVal.Str())
	envVal, found = sms.At(2).Scope().Attributes().Get("environment")
	require.True(t, found)
	require.Equal(t, "staging", envVal.Str())

	// Verify that otel_scope_ labels are filtered from datapoint attributes
	for i := 0; i < sms.Len(); i++ {
		scope := sms.At(i)
		for j := 0; j < scope.Metrics().Len(); j++ {
			metric := scope.Metrics().At(j)
			if metric.Type() == pmetric.MetricTypeGauge {
				for k := 0; k < metric.Gauge().DataPoints().Len(); k++ {
					dp := metric.Gauge().DataPoints().At(k)
					dp.Attributes().Range(func(key string, _ pcommon.Value) bool {
						require.False(t, strings.HasPrefix(key, "otel_scope_"),
							"Found otel_scope_ prefixed attribute %s in datapoint - should be filtered", key)
						return true
					})
				}
			}
		}
	}
}

// verifyMixedScopeInfoAndLabelsFeatureEnabled tests the behavior when feature gate is enabled:
// otel_scope_ labels should be extracted as scope attributes, otel_scope_info should be treated as regular metric.
func verifyMixedScopeInfoAndLabelsFeatureEnabled(t *testing.T, td *testData, rms []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, rms)
	require.NotEmpty(t, rms, "At least one resource metric should be present")

	sms := rms[0].ScopeMetrics()
	require.Equal(t, 4, sms.Len(), "Four scope metrics should be present")
	sms.Sort(func(a, b pmetric.ScopeMetrics) bool {
		return a.Scope().Name() < b.Scope().Name()
	})

	// Verify default receiver scope (no attributes)
	// This one is used for the metric without any otel_scope_ labels
	require.Equal(t, "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver", sms.At(0).Scope().Name())
	require.Equal(t, 0, sms.At(0).Scope().Attributes().Len())

	// Verify scope.from.info(jvm_memory_bytes_used): should have only attributes from otel_scope_ labels (otel_scope_info is not processed)
	require.Equal(t, "scope.from.info", sms.At(1).Scope().Name())
	require.Equal(t, "v2.0.0", sms.At(1).Scope().Version())
	require.Equal(t, 1, sms.At(1).Scope().Attributes().Len(), "Should have only attributes from otel_scope_ labels when feature gate is enabled")
	animalVal, found := sms.At(1).Scope().Attributes().Get("animal")
	require.True(t, found)
	require.Equal(t, "cat", animalVal.Str())
	_, found = sms.At(1).Scope().Attributes().Get("team")
	require.False(t, found)
	_, found = sms.At(1).Scope().Attributes().Get("environment")
	require.False(t, found)

	// Verify scope.from.info(otel_scope_info): It should become a new scope, since it has different attributes from jvm_memory_bytes_used
	require.Equal(t, "scope.from.info", sms.At(2).Scope().Name())
	require.Equal(t, "v2.0.0", sms.At(2).Scope().Version())
	require.Equal(t, 0, sms.At(2).Scope().Attributes().Len())
	// Verify otel_scope_info has become a datapoint, and team and environment are now datapoint attributes
	require.Equal(t, 1, sms.At(2).Metrics().Len())
	metric := sms.At(2).Metrics().At(0)
	require.Equal(t, "otel_scope_info", metric.Name())
	require.Equal(t, pmetric.MetricTypeGauge, metric.Type())
	require.Equal(t, 1, metric.Gauge().DataPoints().Len())
	dp := metric.Gauge().DataPoints().At(0)
	require.Equal(t, 2, dp.Attributes().Len())
	teamVal, found := dp.Attributes().Get("team")
	require.True(t, found)
	require.Equal(t, "backend", teamVal.Str())
	envVal, found := dp.Attributes().Get("environment")
	require.True(t, found)
	require.Equal(t, "production", envVal.Str())

	// Verify scope.from.labels: should have attributes from otel_scope_ labels
	require.Equal(t, "scope.from.labels", sms.At(3).Scope().Name())
	require.Equal(t, "v1.0.0", sms.At(3).Scope().Version())
	require.Equal(t, 2, sms.At(3).Scope().Attributes().Len())
	animalVal, found = sms.At(3).Scope().Attributes().Get("animal")
	require.True(t, found)
	require.Equal(t, "bear", animalVal.Str())
	envVal, found = sms.At(3).Scope().Attributes().Get("environment")
	require.True(t, found)
	require.Equal(t, "staging", envVal.Str())

	// Verify that otel_scope_ labels are filtered from datapoint attributes
	for i := 0; i < sms.Len(); i++ {
		scope := sms.At(i)
		for j := 0; j < scope.Metrics().Len(); j++ {
			metric := scope.Metrics().At(j)
			if metric.Type() == pmetric.MetricTypeGauge && metric.Name() != "otel_scope_info" {
				for k := 0; k < metric.Gauge().DataPoints().Len(); k++ {
					dp := metric.Gauge().DataPoints().At(k)
					dp.Attributes().Range(func(key string, _ pcommon.Value) bool {
						require.False(t, strings.HasPrefix(key, "otel_scope_"),
							"Found otel_scope_ prefixed attribute %s in datapoint - should be filtered", key)
						return true
					})
				}
			}
		}
	}
}

// verifyMixedConflictResolution tests that otel_scope_ labels take precedence over otel_scope_info
// attributes when there are conflicts.
func verifyMixedConflictResolution(t *testing.T, td *testData, rms []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, rms)
	require.NotEmpty(t, rms, "At least one resource metric should be present")

	sms := rms[0].ScopeMetrics()
	require.Equal(t, 2, sms.Len(), "Two scope metrics should be present")
	sms.Sort(func(a, b pmetric.ScopeMetrics) bool {
		return a.Scope().Name() < b.Scope().Name()
	})

	// Verify conflicted.scope: otel_scope_ labels should win over otel_scope_info
	require.Equal(t, "conflicted.scope", sms.At(0).Scope().Name())
	require.Equal(t, "v1.0.0", sms.At(0).Scope().Version())
	require.Equal(t, 3, sms.At(0).Scope().Attributes().Len())

	// Verify otel_scope_ labels won (animal="bear", team="frontend")
	animalVal, found := sms.At(0).Scope().Attributes().Get("animal")
	require.True(t, found)
	require.Equal(t, "bear", animalVal.Str(), "otel_scope_ label should win over otel_scope_info")
	teamVal, found := sms.At(0).Scope().Attributes().Get("team")
	require.True(t, found)
	require.Equal(t, "frontend", teamVal.Str(), "otel_scope_ label should win over otel_scope_info")

	// Verify otel_scope_info attribute without conflict is preserved
	envVal, found := sms.At(0).Scope().Attributes().Get("environment")
	require.True(t, found)
	require.Equal(t, "production", envVal.Str(), "otel_scope_info attribute should be preserved when no conflict")

	// Verify default receiver scope
	require.Equal(t, "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver", sms.At(1).Scope().Name())
	require.Equal(t, 0, sms.At(1).Scope().Attributes().Len())

	// Verify that otel_scope_ labels are filtered from datapoint attributes
	conflictedScope := sms.At(0)
	for j := 0; j < conflictedScope.Metrics().Len(); j++ {
		metric := conflictedScope.Metrics().At(j)
		if metric.Type() == pmetric.MetricTypeGauge {
			for k := 0; k < metric.Gauge().DataPoints().Len(); k++ {
				dp := metric.Gauge().DataPoints().At(k)
				dp.Attributes().Range(func(key string, _ pcommon.Value) bool {
					require.False(t, strings.HasPrefix(key, "otel_scope_"),
						"Found otel_scope_ prefixed attribute %s in datapoint - should be filtered", key)
					return true
				})
			}
		}
	}
}
