// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const testDir = "./testdata/openmetrics/"

var skippedTests = map[string]struct{}{
	"bad_clashing_names_0": {}, "bad_clashing_names_1": {}, "bad_clashing_names_2": {},
	"bad_counter_values_0": {}, "bad_counter_values_1": {}, "bad_counter_values_2": {},
	"bad_counter_values_3": {}, "bad_counter_values_5": {}, "bad_counter_values_6": {},
	"bad_counter_values_10": {}, "bad_counter_values_11": {}, "bad_counter_values_12": {},
	"bad_counter_values_13": {}, "bad_counter_values_14": {}, "bad_counter_values_15": {},
	"bad_counter_values_16": {}, "bad_counter_values_17": {}, "bad_counter_values_18": {},
	"bad_counter_values_19": {}, "bad_exemplars_on_unallowed_samples_2": {}, "bad_exemplar_timestamp_0": {},
	"bad_exemplar_timestamp_1": {}, "bad_exemplar_timestamp_2": {}, "bad_grouping_or_ordering_0": {},
	"bad_grouping_or_ordering_2": {}, "bad_grouping_or_ordering_3": {}, "bad_grouping_or_ordering_4": {},
	"bad_grouping_or_ordering_5": {}, "bad_grouping_or_ordering_6": {}, "bad_grouping_or_ordering_7": {},
	"bad_grouping_or_ordering_8": {}, "bad_grouping_or_ordering_9": {}, "bad_grouping_or_ordering_10": {},
	"bad_histograms_0": {}, "bad_histograms_1": {}, "bad_histograms_2": {}, "bad_histograms_3": {},
	"bad_histograms_6": {}, "bad_histograms_7": {}, "bad_histograms_8": {},
	"bad_info_and_stateset_values_0": {}, "bad_info_and_stateset_values_1": {}, "bad_metadata_in_wrong_place_0": {},
	"bad_metadata_in_wrong_place_1": {}, "bad_metadata_in_wrong_place_2": {},
	"bad_missing_or_invalid_labels_for_a_type_1": {}, "bad_missing_or_invalid_labels_for_a_type_3": {},
	"bad_missing_or_invalid_labels_for_a_type_4": {}, "bad_missing_or_invalid_labels_for_a_type_6": {},
	"bad_missing_or_invalid_labels_for_a_type_7": {}, "bad_repeated_metadata_0": {},
	"bad_repeated_metadata_1": {}, "bad_repeated_metadata_3": {}, "bad_stateset_info_values_0": {},
	"bad_stateset_info_values_1": {}, "bad_stateset_info_values_2": {}, "bad_stateset_info_values_3": {},
	"bad_timestamp_4": {}, "bad_timestamp_5": {}, "bad_timestamp_7": {}, "bad_unit_6": {}, "bad_unit_7": {},
	"bad_exemplars_on_unallowed_samples_0": {}, "bad_exemplars_on_unallowed_metric_types_0": {},
	"bad_exemplars_on_unallowed_samples_1": {}, "bad_exemplars_on_unallowed_metric_types_1": {},
	"bad_exemplars_on_unallowed_samples_3": {}, "bad_exemplars_on_unallowed_metric_types_2": {},
}

var positiveTestsWithoutSeries = []string{"null_byte", "empty_metadata"}

func verifyPositiveTarget(t *testing.T, td *testData, mds []pmetric.ResourceMetrics) {
	require.NotEmpty(t, mds, "At least one resource metric should be present")
	metrics := getMetrics(mds[0])
	assertUp(t, 1, metrics)
	if slices.Contains(positiveTestsWithoutSeries, td.name) {
		require.Equal(t, len(metrics), countScrapeMetrics(metrics, false))
	} else {
		require.Greater(t, len(metrics), countScrapeMetrics(metrics, false))
	}
	if len(mds) > 1 {
		// We expect a single scrape and the rest (if exists) to be stale (up==0).
		for _, m := range mds[1:] {
			metrics = getMetrics(m)
			assertUp(t, 0, metrics)
		}
	}
}

// Test open metrics positive test cases
func TestOpenMetricsPositive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/10148")
	}
	targetsMap := getOpenMetricsPositiveTestData()
	var targets []*testData
	for k, v := range targetsMap {
		testData := &testData{
			name: k,
			pages: []mockPrometheusResponse{
				{code: 200, data: v, useOpenMetrics: true},
			},
			validateFunc:    verifyPositiveTarget,
			validateScrapes: true,
		}
		targets = append(targets, testData)
	}

	testComponent(t, targets, nil)
}

func verifyFailTarget(t *testing.T, td *testData, mds []pmetric.ResourceMetrics) {
	// failing negative tests are skipped since prometheus scrape package is currently not fully
	// compatible with OpenMetrics tests and successfully scrapes some invalid metrics
	// see: https://github.com/prometheus/prometheus/issues/9699
	if _, ok := skippedTests[td.name]; ok {
		t.Skip("skipping failing negative OpenMetrics parser tests")
	}

	require.NotEmpty(t, mds, "At least one resource metric should be present")
	metrics := getMetrics(mds[0])
	assertUp(t, 0, metrics)
}

// Test open metrics negative test cases
func TestOpenMetricsFail(t *testing.T) {
	targetsMap := getOpenMetricsFailTestData()
	var targets []*testData
	for k, v := range targetsMap {
		testData := &testData{
			name: k,
			pages: []mockPrometheusResponse{
				{code: 200, data: v, useOpenMetrics: true},
			},
			validateFunc:    verifyFailTarget,
			validateScrapes: true,
		}
		targets = append(targets, testData)
	}

	testComponent(t, targets, nil)
}

func verifyInvalidTarget(t *testing.T, td *testData, mds []pmetric.ResourceMetrics) {
	// failing negative tests are skipped since prometheus scrape package is currently not fully
	// compatible with OpenMetrics tests and successfully scrapes some invalid metrics
	// see: https://github.com/prometheus/prometheus/issues/9699
	if _, ok := skippedTests[td.name]; ok {
		t.Skip("skipping failing negative OpenMetrics parser tests")
	}

	require.NotEmpty(t, mds, "At least one resource metric should be present")
	metrics := getMetrics(mds[0])

	// The Prometheus scrape parser accepted the sample, but the receiver dropped it due to incompatibility with the Otel schema.
	// In this case, we get just the default metrics.
	require.Equal(t, len(metrics), countScrapeMetrics(metrics, false))
}

func TestOpenMetricsInvalid(t *testing.T) {
	targetsMap := getOpenMetricsInvalidTestData()
	var targets []*testData
	for k, v := range targetsMap {
		testData := &testData{
			name: k,
			pages: []mockPrometheusResponse{
				{code: 200, data: v, useOpenMetrics: true},
			},
			validateFunc:    verifyInvalidTarget,
			validateScrapes: true,
		}
		targets = append(targets, testData)
	}

	testComponent(t, targets, nil)
}

// reads test data from testdata/openmetrics directory
func getOpenMetricsTestData(testNameFilterFunc func(testName string) bool) map[string]string {
	testDir, err := os.Open(testDir)
	if err != nil {
		log.Fatal("failed opening openmetrics test directory")
	}
	defer testDir.Close()

	// read all test file names in testdata/openmetrics
	testList, _ := testDir.Readdirnames(0)

	targetsData := make(map[string]string)
	for _, testName := range testList {
		// ignore hidden files
		if strings.HasPrefix(testName, ".") {
			continue
		}
		if !testNameFilterFunc(testName) {
			continue
		}
		if testData, err := readTestCase(testName); err == nil {
			targetsData[testName] = testData
		}
	}
	return targetsData
}

func getOpenMetricsPositiveTestData() map[string]string {
	return getOpenMetricsTestData(func(testName string) bool {
		return !strings.HasPrefix(testName, "bad") && !strings.HasPrefix(testName, "invalid")
	})
}

func getOpenMetricsFailTestData() map[string]string {
	return getOpenMetricsTestData(func(testName string) bool {
		return strings.HasPrefix(testName, "bad")
	})
}

func getOpenMetricsInvalidTestData() map[string]string {
	return getOpenMetricsTestData(func(testName string) bool {
		return strings.HasPrefix(testName, "invalid")
	})
}

func readTestCase(testName string) (string, error) {
	filePath := filepath.Join(testDir, testName, "metrics")
	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("failed opening file: %s", filePath)
		return "", err
	}
	return string(content), nil
}

// info and stateset metrics are converted to non-monotonic sums
var infoAndStatesetMetrics = `# TYPE foo info
foo_info{entity="controller",name="prettyname",version="8.2.7"} 1.0
foo_info{entity="replica",name="prettiername",version="8.1.9"} 1.0
# TYPE bar stateset
bar{entity="controller",foo="a"} 1.0
bar{entity="controller",foo="bb"} 0.0
bar{entity="controller",foo="ccc"} 0.0
bar{entity="replica",foo="a"} 1.0
bar{entity="replica",foo="bb"} 0.0
bar{entity="replica",foo="ccc"} 1.0
# EOF
`

// TestInfoStatesetMetrics validates the translation of info and stateset
// metrics
func TestInfoStatesetMetrics(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: infoAndStatesetMetrics, useOpenMetrics: true},
			},
			validateFunc:    verifyInfoStatesetMetrics,
			validateScrapes: true,
		},
	}

	testComponent(t, targets, nil)
}

func verifyInfoStatesetMetrics(t *testing.T, td *testData, resourceMetrics []pmetric.ResourceMetrics) {
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
						compareDoubleValue(1.0),
						compareAttributes(map[string]string{"entity": "controller", "name": "prettyname", "version": "8.2.7"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(1.0),
						compareAttributes(map[string]string{"entity": "replica", "name": "prettiername", "version": "8.1.9"}),
					},
				},
			},
			compareMetricIsMonotonic(false),
		},
		{
			"bar",
			pmetric.MetricTypeSum,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(1.0),
						compareAttributes(map[string]string{"entity": "controller", "foo": "a"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(0.0),
						compareAttributes(map[string]string{"entity": "controller", "foo": "bb"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(0.0),
						compareAttributes(map[string]string{"entity": "controller", "foo": "ccc"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(1.0),
						compareAttributes(map[string]string{"entity": "replica", "foo": "a"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(0.0),
						compareAttributes(map[string]string{"entity": "replica", "foo": "bb"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(1.0),
						compareAttributes(map[string]string{"entity": "replica", "foo": "ccc"}),
					},
				},
			},
			compareMetricIsMonotonic(false),
		},
	}
	doCompare(t, "scrape-infostatesetmetrics-1", wantAttributes, m1, e1)
}

var counterPayload = `# TYPE test_counter counter
test_counter_total 12
test_counter_created 1.55555e+09
`

// shuffle the order of _created and _total lines
// with an orphaned _created line {foo=bar3} that should not produce a Sum datapoint
var counterMultiPayload = `# TYPE test_counter counter
test_counter_created{foo="bar2"} 1000
test_counter_total{foo="bar1"} 12
test_counter_created{foo="bar1"} 1.55555e+09
test_counter_created{foo="bar3"} 100
test_counter_total{foo="bar2"} 100
`

var counterReversePayload = `# TYPE test_counter counter
test_counter_created 1.55555e+09
test_counter_total 12
`

var counterNoValue = `# TYPE test_counter counter
test_counter_created 1.55555e+09
`

var summaryPayload = `# TYPE test_summary summary
test_summary_count 1
test_summary_sum 2
test_summary_created 1.55555e+09
test_summary{quantile="0.5"} 0.7
test_summary{quantile="1"} 0.8
`

var histogramPayload = `# TYPE test_histogram histogram
test_histogram_bucket{le="0.0"} 0.0
test_histogram_bucket{le="10.0"} 15.0
test_histogram_bucket{le="+Inf"} 50.0
test_histogram_count 50.0
test_histogram_sum 123.3
test_histogram_created 1.55555e+09
`

func formPage(payloads ...string) string {
	sb := strings.Builder{}
	for _, payload := range payloads {
		sb.Write([]byte(payload))
	}
	sb.Write([]byte("# EOF\n"))
	return sb.String()
}

func TestCreatedMetric(t *testing.T) {
	tests := []testData{
		{
			name: "counter",
			pages: []mockPrometheusResponse{
				{code: 200, data: formPage(counterPayload)},
			},
			validateFunc: verifyCreatedTimeMetric(verifyOpts{true, false, false, nil}),
		},

		{
			name: "counter reversed",
			pages: []mockPrometheusResponse{
				{code: 200, data: formPage(counterReversePayload)},
			},
			validateFunc: verifyCreatedTimeMetric(verifyOpts{true, false, false, nil}),
		},
		{
			name: "counter multiple series",
			pages: []mockPrometheusResponse{
				{code: 200, data: formPage(counterMultiPayload)},
			},
			validateFunc: verifyCreatedTimeMetric(verifyOpts{true, false, false, func(t *testing.T, metrics []pmetric.Metric) {
				for _, metric := range metrics {
					if metric.Name() != "test_counter_total" {
						continue
					}
					require.Equal(t, 2, metric.Sum().DataPoints().Len())
					var first, second pmetric.NumberDataPoint
					if v, k := metric.Sum().DataPoints().At(0).Attributes().Get("foo"); k && v.AsString() == "bar1" {
						first = metric.Sum().DataPoints().At(0)
						second = metric.Sum().DataPoints().At(1)
					} else {
						first = metric.Sum().DataPoints().At(1)
						second = metric.Sum().DataPoints().At(0)
					}
					fooValue, _ := first.Attributes().Get("foo")
					require.Equal(t, "bar1", fooValue.AsString())
					require.Equal(t, expectedST, first.StartTimestamp())
					require.Equal(t, 12.0, first.DoubleValue())
					fooValue, _ = second.Attributes().Get("foo")
					require.Equal(t, "bar2", fooValue.AsString())
					require.Equal(t, pcommon.Timestamp(time.Unix(1000, 0).UnixNano()), second.StartTimestamp())
					require.Equal(t, 100.0, second.DoubleValue())
					return
				}
				t.Fatalf("did not find expected metric")
			}}),
		},
		{
			name: "counter no value",
			pages: []mockPrometheusResponse{
				{code: 200, data: formPage(counterNoValue)},
			},
			validateFunc: verifyCreatedTimeMetric(verifyOpts{false, false, false, nil}),
		},
		{
			name: "all",
			pages: []mockPrometheusResponse{
				{code: 200, data: formPage(counterPayload, summaryPayload, histogramPayload)},
			},
			validateFunc: verifyCreatedTimeMetric(verifyOpts{true, true, true, nil}),
		},
	}
	for _, useOM := range []bool{false, true} {
		for i := range tests {
			testCopy := tests[i]
			testCopy.pages = make([]mockPrometheusResponse, len(tests[i].pages))
			copy(testCopy.pages, tests[i].pages)
			t.Run(fmt.Sprintf("%s with useOpenMetrics=%v", testCopy.name, useOM), func(t *testing.T) {
				t.Parallel()
				for i := range testCopy.pages {
					testCopy.pages[i].useOpenMetrics = useOM
				}
				testComponent(t, []*testData{&testCopy}, nil)
			})
		}
	}
}

type verifyOpts struct {
	wantCounter   bool
	wantSummary   bool
	wantHisto     bool
	checkOverride func(t *testing.T, metrics []pmetric.Metric)
}

const ctMetricValueAsSecFloat = 1.55555e+09

var (
	expectedSTSecsPart  = int64(ctMetricValueAsSecFloat)
	expectedSTNanosPart = int64((ctMetricValueAsSecFloat - float64(expectedSTSecsPart)) * float64(time.Second))
	expectedST          = pcommon.Timestamp(expectedSTSecsPart*int64(time.Second) + expectedSTNanosPart)
)

func verifyCreatedTimeMetric(o verifyOpts) func(t *testing.T, _ *testData, mds []pmetric.ResourceMetrics) {
	return func(t *testing.T, _ *testData, mds []pmetric.ResourceMetrics) {
		wantCounter, wantSummary, wantHisto := o.wantCounter, o.wantSummary, o.wantHisto
		require.NotEmpty(t, mds, "At least one resource metric should be present")
		metrics := getMetrics(mds[0])
		assertUp(t, 1, metrics)

		if !wantCounter && !wantSummary && !wantHisto {
			require.Equal(t, len(metrics), countScrapeMetrics(metrics, false))
			return
		}
		require.Greater(t, len(metrics), countScrapeMetrics(metrics, false))

		if len(mds) > 1 {
			// We expect a single scrape and the rest (if exists) to be stale (up==0).
			for _, m := range mds[1:] {
				metrics = getMetrics(m)
				assertUp(t, 0, metrics)
			}
		}

		if o.checkOverride != nil {
			o.checkOverride(t, metrics)
			return
		}

		var (
			seenHisto   bool
			seenSummary bool
			seenCounter bool
		)
		// inspect the scraped metric
		for _, m := range metrics {
			name := m.Name()
			if !strings.HasPrefix(name, "test_") {
				continue
			}
			switch name {
			case "test_histogram":
				h := m.Histogram()
				require.NotNil(t, h)
				require.Equal(t, expectedST, h.DataPoints().At(0).StartTimestamp())
				seenHisto = true
			case "test_summary":
				s := m.Summary()
				require.NotNil(t, s)
				require.Equal(t, expectedST, s.DataPoints().At(0).StartTimestamp())
				seenSummary = true
			case "test_counter_total":
				s := m.Sum()
				require.NotNil(t, s)
				require.Equal(t, expectedST, s.DataPoints().At(0).StartTimestamp())
				seenCounter = true
			default:
				require.Fail(t, "unexpected metric name", name)
			}
		}

		msgFn := func(seen bool) string {
			if seen {
				return "scraped metric contains %s which is not expected"
			}
			return "scraped metric does not contain expected %s"
		}
		if wantHisto != seenHisto {
			require.Fail(t, fmt.Sprintf(msgFn(seenHisto), "histogram test_histogram"))
		}

		if wantSummary != seenSummary {
			require.Fail(t, fmt.Sprintf(msgFn(seenSummary), "summary test_summary"))
		}
		if wantCounter != seenCounter {
			require.Fail(t, fmt.Sprintf(msgFn(seenCounter), "counter test_counter_total"))
		}
	}
}
