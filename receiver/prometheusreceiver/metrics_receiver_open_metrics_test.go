// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func verifyPositiveTarget(t *testing.T, _ *testData, mds []pmetric.ResourceMetrics) {
	require.Greater(t, len(mds), 0, "At least one resource metric should be present")
	metrics := getMetrics(mds[0])
	assertUp(t, 1, metrics)
}

// Test open metrics positive test cases
func TestOpenMetricsPositive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/10148")
	}
	targetsMap := getOpenMetricsTestData(false)
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

	testComponent(t, targets, false, false, "")
}

func verifyNegativeTarget(t *testing.T, td *testData, mds []pmetric.ResourceMetrics) {
	// failing negative tests are skipped since prometheus scrape package is currently not fully
	// compatible with OpenMetrics tests and successfully scrapes some invalid metrics
	// see: https://github.com/prometheus/prometheus/issues/9699
	if _, ok := skippedTests[td.name]; ok {
		t.Skip("skipping failing negative OpenMetrics parser tests")
	}

	require.Greater(t, len(mds), 0, "At least one resource metric should be present")
	metrics := getMetrics(mds[0])
	assertUp(t, 0, metrics)
}

// Test open metrics negative test cases
func TestOpenMetricsNegative(t *testing.T) {

	targetsMap := getOpenMetricsTestData(true)
	var targets []*testData
	for k, v := range targetsMap {
		testData := &testData{
			name: k,
			pages: []mockPrometheusResponse{
				{code: 200, data: v, useOpenMetrics: true},
			},
			validateFunc:    verifyNegativeTarget,
			validateScrapes: true,
		}
		targets = append(targets, testData)
	}

	testComponent(t, targets, false, false, "")
}

// reads test data from testdata/openmetrics directory
func getOpenMetricsTestData(negativeTestsOnly bool) map[string]string {
	testDir, err := os.Open(testDir)
	if err != nil {
		log.Fatalf("failed opening openmetrics test directory")
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
		if negativeTestsOnly && !strings.Contains(testName, "bad") {
			continue
		} else if !negativeTestsOnly && strings.Contains(testName, "bad") {
			continue
		}
		if testData, err := readTestCase(testName); err == nil {
			targetsData[testName] = testData
		}
	}
	return targetsData
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

	testComponent(t, targets, false, false, "")

}

func verifyInfoStatesetMetrics(t *testing.T, td *testData, resourceMetrics []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]

	// m1 has 2 metrics + 5 internal scraper metrics
	assert.Equal(t, 7, metricsCount(m1))

	wantAttributes := td.attributes

	metrics1 := m1.ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)
	e1 := []testExpectation{
		assertMetricPresent("foo",
			compareMetricIsMonotonic(false),
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
			}),
		assertMetricPresent("bar",
			compareMetricIsMonotonic(false),
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
			}),
	}
	doCompare(t, "scrape-infostatesetmetrics-1", wantAttributes, m1, e1)
}
