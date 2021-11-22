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
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
)

const testDir = "./testdata/openmetrics/"

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
	// negative tests are skipped since prometheus scrape package is currently not fully
	// compatible with OpenMetrics tests and successfully scrapes some invalid metrics
	// see: https://github.com/prometheus/prometheus/issues/9699
	t.Skip("skipping negative OpenMetrics parser tests")

	require.Greater(t, len(mds), 0, "At least one resource metric should be present")
	metrics := getMetrics(mds[0])
	assertUp(t, 0, metrics)
}

// Test open metrics negative test cases
func TestOpenMetricsNegative(t *testing.T) {
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

//reads test data from testdata/openmetrics directory
func getOpenMetricsTestData(negativeTestsOnly bool) map[string]string {
	testDir, err := os.Open(testDir)
	if err != nil {
		log.Fatalf("failed opening openmetrics test directory")
	}
	defer testDir.Close()

	//read all test file names in testdata/openmetrics
	testList, _ := testDir.Readdirnames(0)

	targetsData := make(map[string]string)
	for _, testName := range testList {
		//ignore hidden files
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
	filePath := fmt.Sprintf("%s/%s/metrics", testDir, testName)
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("failed opening file: %s", filePath)
		return "", err
	}
	return string(content), nil
}
