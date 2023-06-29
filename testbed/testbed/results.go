// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testbed // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"time"
)

// TestResultsSummary defines the interface to record results of one category of testing.
type TestResultsSummary interface {
	// Init creates and open the file and write headers.
	Init(resultsDir string)
	// Add results for one test.
	Add(testName string, result interface{})
	// Save the total results and close the file.
	Save()
}

// BenchmarkResult holds the results of a benchmark to be stored by benchmark-action. See
// https://github.com/benchmark-action/github-action-benchmark#examples for more details on the
// format
type benchmarkResult struct {
	Name  string  `json:"name"`
	Unit  string  `json:"unit"`
	Value float64 `json:"value"`
	Range string  `json:"range,omitempty"`
	Extra string  `json:"extra,omitempty"`
}

// PerformResults implements the TestResultsSummary interface with fields suitable for reporting
// performance test results.
type PerformanceResults struct {
	resultsDir       string
	resultsFile      *os.File
	perTestResults   []*PerformanceTestResult
	benchmarkResults []*benchmarkResult
	totalDuration    time.Duration
}

// PerformanceTestResult reports the results of a single performance test.
type PerformanceTestResult struct {
	testName          string
	result            string
	duration          time.Duration
	cpuPercentageAvg  float64
	cpuPercentageMax  float64
	ramMibAvg         uint32
	ramMibMax         uint32
	sentSpanCount     uint64
	receivedSpanCount uint64
	errorCause        string
}

func (r *PerformanceResults) Init(resultsDir string) {
	r.resultsDir = resultsDir
	r.perTestResults = []*PerformanceTestResult{}
	r.benchmarkResults = []*benchmarkResult{}

	// Create resultsSummary file
	if err := os.MkdirAll(resultsDir, os.FileMode(0755)); err != nil {
		log.Fatal(err)
	}
	var err error
	r.resultsFile, err = os.Create(path.Join(r.resultsDir, "TESTRESULTS.md"))
	if err != nil {
		log.Fatal(err)
	}

	// Write the header
	_, _ = io.WriteString(r.resultsFile,
		"# Test PerformanceResults\n"+
			fmt.Sprintf("Started: %s\n\n", time.Now().Format(time.RFC1123Z))+
			"Test                                    |Result|Duration|CPU Avg%|CPU Max%|RAM Avg MiB|RAM Max MiB|Sent Items|Received Items|\n"+
			"----------------------------------------|------|-------:|-------:|-------:|----------:|----------:|---------:|-------------:|\n")
}

// Save the total results and close the file.
func (r *PerformanceResults) Save() {
	_, _ = io.WriteString(r.resultsFile,
		fmt.Sprintf("\nTotal duration: %.0fs\n", r.totalDuration.Seconds()))
	r.resultsFile.Close()
	r.saveBenchmarks()
}

// Add results for one test.
func (r *PerformanceResults) Add(_ string, result interface{}) {
	testResult, ok := result.(*PerformanceTestResult)
	if !ok {
		return
	}

	_, _ = io.WriteString(r.resultsFile,
		fmt.Sprintf("%-40s|%-6s|%7.0fs|%8.1f|%8.1f|%11d|%11d|%10d|%14d|%s\n",
			testResult.testName,
			testResult.result,
			testResult.duration.Seconds(),
			testResult.cpuPercentageAvg,
			testResult.cpuPercentageMax,
			testResult.ramMibAvg,
			testResult.ramMibMax,
			testResult.sentSpanCount,
			testResult.receivedSpanCount,
			testResult.errorCause,
		),
	)
	r.totalDuration += testResult.duration

	// individual benchmark results
	cpuChartName := fmt.Sprintf("%s - Cpu Percentage", testResult.testName)
	memoryChartName := fmt.Sprintf("%s - RAM (MiB)", testResult.testName)
	droppedSpansChartName := fmt.Sprintf("%s - Dropped Span Count", testResult.testName)

	r.benchmarkResults = append(r.benchmarkResults, &benchmarkResult{
		Name:  "cpu_percentage_avg",
		Value: testResult.cpuPercentageAvg,
		Unit:  "%",
		Extra: cpuChartName,
	})
	r.benchmarkResults = append(r.benchmarkResults, &benchmarkResult{
		Name:  "cpu_percentage_max",
		Value: testResult.cpuPercentageMax,
		Unit:  "%",
		Extra: cpuChartName,
	})
	r.benchmarkResults = append(r.benchmarkResults, &benchmarkResult{
		Name:  "ram_mib_avg",
		Value: float64(testResult.ramMibAvg),
		Unit:  "MiB",
		Extra: memoryChartName,
	})
	r.benchmarkResults = append(r.benchmarkResults, &benchmarkResult{
		Name:  "ram_mib_max",
		Value: float64(testResult.ramMibMax),
		Unit:  "MiB",
		Extra: memoryChartName,
	})
	r.benchmarkResults = append(r.benchmarkResults, &benchmarkResult{
		Name:  "dropped_span_count",
		Value: float64(testResult.sentSpanCount - testResult.receivedSpanCount),
		Unit:  "spans",
		Extra: droppedSpansChartName,
	})
}

// saveBenchmarks writes benchmarks to file as json to be stored by
// benchmark-action
func (r *PerformanceResults) saveBenchmarks() {
	path := path.Join(r.resultsDir, "benchmarks.json")
	j, _ := json.MarshalIndent(r.benchmarkResults, "", "  ")
	_ = os.WriteFile(path, j, 0644)
}

// CorrectnessResults implements the TestResultsSummary interface with fields suitable for reporting data translation
// correctness test results.
type CorrectnessResults struct {
	resultsDir             string
	resultsFile            *os.File
	perTestResults         []*CorrectnessTestResult
	totalAssertionFailures uint64
	totalDuration          time.Duration
}

// CorrectnessTestResult reports the results of a single correctness test.
type CorrectnessTestResult struct {
	testName                   string
	result                     string
	duration                   time.Duration
	sentSpanCount              uint64
	receivedSpanCount          uint64
	traceAssertionFailureCount uint64
	traceAssertionFailures     []*TraceAssertionFailure
}

type TraceAssertionFailure struct {
	typeName      string
	dataComboName string
	fieldPath     string
	expectedValue interface{}
	actualValue   interface{}
	sumCount      int
}

func (af TraceAssertionFailure) String() string {
	return fmt.Sprintf("%s/%s e=%#v a=%#v ", af.dataComboName, af.fieldPath, af.expectedValue, af.actualValue)
}

func (r *CorrectnessResults) Init(resultsDir string) {
	r.resultsDir = resultsDir
	r.perTestResults = []*CorrectnessTestResult{}

	// Create resultsSummary file
	if err := os.MkdirAll(resultsDir, os.FileMode(0755)); err != nil {
		log.Fatal(err)
	}
	var err error
	r.resultsFile, err = os.Create(path.Join(r.resultsDir, "CORRECTNESSRESULTS.md"))
	if err != nil {
		log.Fatal(err)
	}

	// Write the header
	_, _ = io.WriteString(r.resultsFile,
		"# Test Results\n"+
			fmt.Sprintf("Started: %s\n\n", time.Now().Format(time.RFC1123Z))+
			"Test                                    |Result|Duration|Sent Items|Received Items|Failure Count|Failures\n"+
			"----------------------------------------|------|-------:|---------:|-------------:|------------:|--------\n")
}

func (r *CorrectnessResults) Add(_ string, result interface{}) {
	testResult, ok := result.(*CorrectnessTestResult)
	if !ok {
		return
	}
	consolidated := consolidateAssertionFailures(testResult.traceAssertionFailures)
	failuresStr := ""
	for _, af := range consolidated {
		failuresStr = fmt.Sprintf("%s%s,%#v!=%#v,count=%d; ", failuresStr, af.fieldPath, af.expectedValue,
			af.actualValue, af.sumCount)
	}
	_, _ = io.WriteString(r.resultsFile,
		fmt.Sprintf("%-40s|%-6s|%7.0fs|%10d|%14d|%13d|%s\n",
			testResult.testName,
			testResult.result,
			testResult.duration.Seconds(),
			testResult.sentSpanCount,
			testResult.receivedSpanCount,
			testResult.traceAssertionFailureCount,
			failuresStr,
		),
	)
	r.perTestResults = append(r.perTestResults, testResult)
	r.totalAssertionFailures += testResult.traceAssertionFailureCount
	r.totalDuration += testResult.duration
}

func (r *CorrectnessResults) Save() {
	_, _ = io.WriteString(r.resultsFile,
		fmt.Sprintf("\nTotal assertion failures: %d\n", r.totalAssertionFailures))
	_, _ = io.WriteString(r.resultsFile,
		fmt.Sprintf("\nTotal duration: %.0fs\n", r.totalDuration.Seconds()))
	r.resultsFile.Close()
}

func consolidateAssertionFailures(failures []*TraceAssertionFailure) map[string]*TraceAssertionFailure {
	afMap := make(map[string]*TraceAssertionFailure)
	for _, f := range failures {
		summary := afMap[f.fieldPath]
		if summary == nil {
			summary = &TraceAssertionFailure{
				typeName:      f.typeName,
				dataComboName: f.dataComboName + "...",
				fieldPath:     f.fieldPath,
				expectedValue: f.expectedValue,
				actualValue:   f.actualValue,
			}
			afMap[f.fieldPath] = summary
		}
		summary.sumCount++
	}
	return afMap
}
