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
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
)

const testDataUrl string = "https://raw.githubusercontent.com/OpenObservability/OpenMetrics/main/tests/urls.txt"

func verifyPositiveTarget(t *testing.T, _ *testData, mds []*pdata.ResourceMetrics) {
	require.Greater(t, len(mds), 0, "At least one resource metric should be present")
	metrics := getMetrics(mds[0])
	assertUp(t, 1, metrics)
}

// Test open metrics positive test cases
func TestOpenMetricsPositive(t *testing.T) {
	targetsMap := getOpenMetricsTestData(false)
	targets := make([]*testData, 0)
	for k, v := range targetsMap {
		testData := &testData{
			name: k,
			pages: []mockPrometheusResponse{
				{code: 200, data: v},
			},
			validateFunc: verifyPositiveTarget,
		}
		targets = append(targets, testData)
	}

	testComponent(t, targets, false, "", true)
}

func verifyNegativeTarget(t *testing.T, _ *testData, mds []*pdata.ResourceMetrics) {
	require.Greater(t, len(mds), 0, "At least one resource metric should be present")
	metrics := getMetrics(mds[0])
	assertUp(t, 0, metrics)
}

// Test open metrics negative test cases
func TestOpenMetricsNegative(t *testing.T) {
	// negative tests are skipped since prometheus scrape package is currently not fully
	// compatible with OpenMetrics tests and successfully scrapes some invalid metrics
	// see: https://github.com/prometheus/prometheus/issues/9699
	t.Skip("skipping negative OpenMetrics parser tests")

	targetsMap := getOpenMetricsTestData(true)
	targets := make([]*testData, 0)
	for k, v := range targetsMap {
		testData := &testData{
			name: k,
			pages: []mockPrometheusResponse{
				{code: 200, data: v},
			},
			validateFunc: verifyNegativeTarget,
		}
		targets = append(targets, testData)
	}

	testComponent(t, targets, false, "", true)
}

// maps each test name to the test data from OpenMetrics repository
func getOpenMetricsTestData(negativeTestsOnly bool) map[string]string {
	response, err := http.Get(testDataUrl)
	if err != nil {
		log.Fatal(err)
	}
	if response == nil || response.StatusCode != http.StatusOK {
		log.Fatal("Failed to get OpenMetrics test data")
	}
	defer response.Body.Close()

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	targetUrls := strings.Split(string(responseBody), "\n")
	targetsData := make(map[string]string)
	for _, targetUrl := range targetUrls {
		if negativeTestsOnly && !strings.Contains(targetUrl, "bad") || targetUrl == "" {
			continue
		} else if !negativeTestsOnly && strings.Contains(targetUrl, "bad") || targetUrl == "" {
			continue
		}

		if data, statusCode := getTestCase(targetUrl); statusCode == http.StatusOK {
			testName := strings.TrimPrefix(targetUrl, "https://raw.githubusercontent.com/OpenObservability/OpenMetrics/main/tests/testdata/parsers/")
			testName = strings.TrimSuffix(testName, "/metrics")
			targetsData[testName] = data
		} else {
			log.Printf("Failed to get test data from: %s", targetUrl)
		}
	}
	return targetsData
}

func getTestCase(url string) (string, int) {
	response, err := http.Get(url)

	if err != nil || response == nil || response.StatusCode != http.StatusOK {
		return "", response.StatusCode
	}
	defer response.Body.Close()

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}
	return string(responseBody), response.StatusCode
}
