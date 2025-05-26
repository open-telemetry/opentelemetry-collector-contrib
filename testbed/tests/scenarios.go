// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tests // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/tests"

// This file defines parametrized test scenarios and makes them public so that they can be
// also used by tests in custom builds of Collector (e.g. Collector Contrib).

import (
	"fmt"
	"math/rand/v2"
	"path"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

var (
	batchRegex                                           = regexp.MustCompile(` batch_index=(\S+) `)
	itemRegex                                            = regexp.MustCompile(` item_index=(\S+) `)
	performanceResultsSummary testbed.TestResultsSummary = &testbed.PerformanceResults{}
)

type ProcessorNameAndConfigBody struct {
	Name string
	Body string
}

// createConfigYaml creates a collector config file that corresponds to the
// sender and receiver used in the test and returns the config file name.
// Map of processor names to their configs. Config is in YAML and must be
// indented by 2 spaces. Processors will be placed between batch and queue for traces
// pipeline. For metrics pipeline these will be sole processors.
func createConfigYaml(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	resultDir string,
	processors []ProcessorNameAndConfigBody,
	extensions map[string]string,
) string {
	// Create a config. Note that our DataSender is used to generate a config for Collector's
	// receiver and our DataReceiver is used to generate a config for Collector's exporter.
	// This is because our DataSender sends to Collector's receiver and our DataReceiver
	// receives from Collector's exporter.

	// Prepare extra processor config section and comma-separated list of extra processor
	// names to use in corresponding "processors" settings.
	processorsSections := ""
	processorsList := ""
	if len(processors) > 0 {
		first := true
		for i := range processors {
			processorsSections += processors[i].Body + "\n"
			if !first {
				processorsList += ","
			}
			processorsList += processors[i].Name
			first = false
		}
	}

	// Prepare extra extension config section and comma-separated list of extra extension
	// names to use in corresponding "extensions" settings.
	extensionsSections := ""
	extensionsList := ""
	if len(extensions) > 0 {
		first := true
		for name, cfg := range extensions {
			extensionsSections += cfg + "\n"
			if !first {
				extensionsList += ","
			}
			extensionsList += name
			first = false
		}
	}

	// Set pipeline based on DataSender type
	var pipeline string
	switch sender.(type) {
	case testbed.TraceDataSender:
		pipeline = "traces"
	case testbed.MetricDataSender:
		pipeline = "metrics"
	case testbed.LogDataSender:
		pipeline = "logs"
	default:
		t.Error("Invalid DataSender type")
	}

	format := `
receivers:%v
exporters:%v
processors:
  %s

extensions:
  pprof:
    save_to_file: %v/cpu.prof
  %s

service:
  extensions: [pprof, %s]
  pipelines:
    %s:
      receivers: [%v]
      processors: [%s]
      exporters: [%v]
`

	// Put corresponding elements into the config template to generate the final config.
	return fmt.Sprintf(
		format,
		sender.GenConfigYAMLStr(),
		receiver.GenConfigYAMLStr(),
		processorsSections,
		resultDir,
		extensionsSections,
		extensionsList,
		pipeline,
		sender.ProtocolName(),
		processorsList,
		receiver.ProtocolName(),
	)
}

// Scenario10kItemsPerSecond runs 10k data items/sec test using specified sender and receiver protocols.
func Scenario10kItemsPerSecond(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	resourceSpec testbed.ResourceSpec,
	resultsSummary testbed.TestResultsSummary,
	processors []ProcessorNameAndConfigBody,
	extensions map[string]string,
	loadOptions *testbed.LoadOptions,
) {
	resultDir, err := filepath.Abs(path.Join("results", t.Name()))
	require.NoError(t, err)

	if loadOptions == nil {
		loadOptions = &testbed.LoadOptions{
			ItemsPerBatch: 100,
			Parallel:      1,
		}
	}
	loadOptions.DataItemsPerSecond = 10_000

	agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))

	configStr := createConfigYaml(t, sender, receiver, resultDir, processors, extensions)
	configCleanup, err := agentProc.PrepareConfig(t, configStr)
	require.NoError(t, err)
	defer configCleanup()

	dataProvider := testbed.NewPerfTestDataProvider(*loadOptions)
	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		agentProc,
		&testbed.PerfTestValidator{},
		resultsSummary,
		testbed.WithResourceLimits(resourceSpec),
	)
	t.Cleanup(tc.Stop)

	tc.StartBackend()
	tc.StartAgent()

	tc.StartLoad(*loadOptions)

	tc.WaitFor(func() bool { return tc.LoadGenerator.DataItemsSent() > 0 }, "load generator started")

	tc.Sleep(tc.Duration)

	tc.StopLoad()

	tc.WaitFor(func() bool { return tc.LoadGenerator.DataItemsSent() == tc.MockBackend.DataItemsReceived() },
		"all data items received")

	tc.ValidateData()
}

// Scenario10kItemsPerSecondAlternateBackend runs 10k data items/sec test using specified sender and receiver protocols.
// The only difference from Scenario10kItemsPerSecond is that this method can be used to specify a different backend. This
// is useful when testing components for which there is no exporter that emits the same format as the receiver format.
func Scenario10kItemsPerSecondAlternateBackend(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	backend testbed.DataReceiver,
	resourceSpec testbed.ResourceSpec,
	resultsSummary testbed.TestResultsSummary,
	processors []ProcessorNameAndConfigBody,
	extensions map[string]string,
) {
	resultDir, err := filepath.Abs(path.Join("results", t.Name()))
	require.NoError(t, err)

	options := testbed.LoadOptions{
		DataItemsPerSecond: 10_000,
		ItemsPerBatch:      100,
		Parallel:           1,
	}
	agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))

	configStr := createConfigYaml(t, sender, receiver, resultDir, processors, extensions)
	fmt.Println(configStr)
	configCleanup, err := agentProc.PrepareConfig(t, configStr)
	require.NoError(t, err)
	defer configCleanup()

	dataProvider := testbed.NewPerfTestDataProvider(options)
	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		agentProc,
		&testbed.PerfTestValidator{},
		resultsSummary,
		testbed.WithResourceLimits(resourceSpec),
	)
	t.Cleanup(tc.Stop)

	// for some scenarios, the mockbackend isn't the same as the receiver
	// therefore, the backend must be initialized with the correct receiver
	tc.MockBackend = testbed.NewMockBackend(tc.ComposeTestResultFileName("backend.log"), backend)

	tc.StartBackend()
	tc.StartAgent()

	tc.StartLoad(options)
	tc.WaitFor(func() bool { return tc.LoadGenerator.DataItemsSent() > 0 }, "load generator started")

	tc.Sleep(tc.Duration)

	tc.StopLoad()
	tc.WaitFor(func() bool { return tc.LoadGenerator.DataItemsSent() == tc.MockBackend.DataItemsReceived() },
		"all data items received")

	tc.ValidateData()
}

// TestCase for Scenario1kSPSWithAttrs func.
type TestCase struct {
	attrCount      int
	attrSizeByte   int
	expectedMaxCPU uint32
	expectedMaxRAM uint32
	resultsSummary testbed.TestResultsSummary
}

func genRandByteString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = byte(rand.IntN(128))
	}
	return string(b)
}

// Scenario1kSPSWithAttrs runs a performance test at 1k sps with specified span attributes
// and test options.
func Scenario1kSPSWithAttrs(t *testing.T, args []string, tests []TestCase, processors []ProcessorNameAndConfigBody, extensions map[string]string) {
	for _, test := range tests {
		t.Run(fmt.Sprintf("%d*%dbytes", test.attrCount, test.attrSizeByte), func(t *testing.T) {
			options := constructLoadOptions(test)

			agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))

			// Prepare results dir.
			resultDir, err := filepath.Abs(path.Join("results", t.Name()))
			require.NoError(t, err)

			// Create sender and receiver on available ports.
			sender := testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
			receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))

			// Prepare config.
			configStr := createConfigYaml(t, sender, receiver, resultDir, processors, extensions)
			configCleanup, err := agentProc.PrepareConfig(t, configStr)
			require.NoError(t, err)
			defer configCleanup()

			tc := testbed.NewTestCase(
				t,
				testbed.NewPerfTestDataProvider(options),
				sender,
				receiver,
				agentProc,
				&testbed.PerfTestValidator{},
				test.resultsSummary,
				testbed.WithResourceLimits(testbed.ResourceSpec{ExpectedMaxCPU: test.expectedMaxCPU, ExpectedMaxRAM: test.expectedMaxRAM}),
			)
			defer tc.Stop()

			tc.StartBackend()
			tc.StartAgent(args...)

			tc.StartLoad(options)
			tc.Sleep(tc.Duration)
			tc.StopLoad()

			tc.WaitFor(func() bool { return tc.LoadGenerator.DataItemsSent() > 0 }, "load generator started")
			tc.WaitFor(func() bool { return tc.LoadGenerator.DataItemsSent() == tc.MockBackend.DataItemsReceived() },
				"all spans received")

			tc.ValidateData()
		})
	}
}

// Structure used for TestTraceNoBackend10kSPS.
// Defines RAM usage range for defined processor type.
type processorConfig struct {
	Name string
	// slice of processor structs with their names and config YAML to use.
	Processor           []ProcessorNameAndConfigBody
	ExpectedMaxRAM      uint32
	ExpectedMinFinalRAM uint32
}

func ScenarioTestTraceNoBackend10kSPS(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	resourceSpec testbed.ResourceSpec,
	resultsSummary testbed.TestResultsSummary,
	configuration processorConfig,
) {
	resultDir, err := filepath.Abs(path.Join("results", t.Name()))
	require.NoError(t, err)

	options := testbed.LoadOptions{DataItemsPerSecond: 10000, ItemsPerBatch: 10}
	agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))
	configStr := createConfigYaml(t, sender, receiver, resultDir, configuration.Processor, nil)
	configCleanup, err := agentProc.PrepareConfig(t, configStr)
	require.NoError(t, err)
	defer configCleanup()

	dataProvider := testbed.NewPerfTestDataProvider(options)
	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		agentProc,
		&testbed.PerfTestValidator{},
		resultsSummary,
		testbed.WithResourceLimits(resourceSpec),
	)

	t.Cleanup(tc.Stop)

	tc.StartBackend()
	tc.StartAgent()
	tc.StartLoad(options)

	tc.Sleep(tc.Duration)

	rss, _, err := tc.AgentMemoryInfo()
	require.NoError(t, err)
	assert.Less(t, configuration.ExpectedMinFinalRAM, rss)
}

func ScenarioSendingQueuesFull(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	loadOptions testbed.LoadOptions,
	resourceSpec testbed.ResourceSpec,
	sleepTime int,
	resultsSummary testbed.TestResultsSummary,
	processors []ProcessorNameAndConfigBody,
	extensions map[string]string,
) {
	resultDir, err := filepath.Abs(path.Join("results", t.Name()))
	require.NoError(t, err)

	agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))

	configStr := createConfigYaml(t, sender, receiver, resultDir, processors, extensions)
	configCleanup, err := agentProc.PrepareConfig(t, configStr)
	require.NoError(t, err)
	defer configCleanup()
	dataProvider := testbed.NewPerfTestDataProvider(loadOptions)
	dataChannel := make(chan bool)
	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		agentProc,
		&testbed.LogPresentValidator{
			LogBody: "sending queue is full",
			Present: true,
		},
		resultsSummary,
		testbed.WithResourceLimits(resourceSpec),
		testbed.WithDecisionFunc(func() error { return testbed.GenerateNonPernamentErrorUntil(dataChannel) }),
	)

	t.Cleanup(tc.Stop)

	tc.MockBackend.EnableRecording()

	tc.StartBackend()
	tc.StartAgent()
	tc.StartLoad(loadOptions)

	tc.WaitForN(func() bool { return tc.LoadGenerator.DataItemsSent() > 0 }, time.Second*time.Duration(sleepTime), "load generator started")

	// searchFunc checks for "sending queue is full" communicate and sends the signal to GenerateNonPernamentErrorUntil
	// to generate only successes from that time on
	tc.WaitForN(func() bool {
		logFound := tc.AgentLogsContains("sending queue is full")
		if !logFound {
			dataChannel <- true
			return false
		}
		tc.WaitFor(func() bool { return tc.MockBackend.DataItemsReceived() == 0 }, "no data successfully received before an error")
		close(dataChannel)
		return logFound
	}, time.Second*time.Duration(sleepTime), "sending queue errors present")

	// check if data started to be received successfully
	tc.WaitForN(func() bool {
		return tc.MockBackend.DataItemsReceived() > 0
	}, time.Second*time.Duration(sleepTime), "data started to be successfully received")

	tc.WaitForN(func() bool {
		// get IDs from logs to retry
		logsToRetry := getLogsID(tc.MockBackend.LogsToRetry)

		// get IDs from logs received successfully
		successfulLogs := getLogsID(tc.MockBackend.ReceivedLogs)

		// check if all the logs to retry were actually retried
		logsWereRetried := allElementsExistInSlice(logsToRetry, successfulLogs)
		return logsWereRetried
	}, time.Second*time.Duration(sleepTime), "all logs were retried successfully")

	tc.StopLoad()
	tc.StopAgent()
	tc.ValidateData()
}

func ScenarioSendingQueuesNotFull(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	loadOptions testbed.LoadOptions,
	resourceSpec testbed.ResourceSpec,
	sleepTime int,
	resultsSummary testbed.TestResultsSummary,
	processors []ProcessorNameAndConfigBody,
	extensions map[string]string,
) {
	resultDir, err := filepath.Abs(path.Join("results", t.Name()))
	require.NoError(t, err)

	agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))

	configStr := createConfigYaml(t, sender, receiver, resultDir, processors, extensions)
	configCleanup, err := agentProc.PrepareConfig(t, configStr)
	require.NoError(t, err)
	defer configCleanup()
	dataProvider := testbed.NewPerfTestDataProvider(loadOptions)
	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		agentProc,
		&testbed.LogPresentValidator{
			LogBody: "sending queue is full",
			Present: false,
		},
		resultsSummary,
		testbed.WithResourceLimits(resourceSpec),
	)
	defer tc.Stop()

	tc.StartBackend()
	tc.StartAgent()

	tc.StartLoad(loadOptions)

	tc.Sleep(time.Second * time.Duration(sleepTime))

	tc.WaitFor(func() bool { return tc.LoadGenerator.DataItemsSent() > 0 }, "load generator started")

	tc.WaitForN(func() bool { return tc.LoadGenerator.DataItemsSent() == tc.MockBackend.DataItemsReceived() }, time.Second*time.Duration(sleepTime),
		"all spans received")

	tc.StopLoad()
	tc.StopAgent()
	tc.ValidateData()
}

func ScenarioLong(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	loadOptions testbed.LoadOptions,
	resultsSummary testbed.TestResultsSummary,
	sleepTime int,
	processors []ProcessorNameAndConfigBody,
) {
	resultDir, err := filepath.Abs(path.Join("results", t.Name()))
	require.NoError(t, err)

	agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))

	configStr := createConfigYaml(t, sender, receiver, resultDir, processors, nil)
	configCleanup, err := agentProc.PrepareConfig(t, configStr)
	require.NoError(t, err)
	defer configCleanup()
	dataProvider := testbed.NewPerfTestDataProvider(loadOptions)
	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		agentProc,
		&testbed.CorrectnessLogTestValidator{},
		resultsSummary,
	)
	t.Cleanup(tc.Stop)

	tc.StartBackend()
	tc.StartAgent()

	tc.StartLoad(loadOptions)

	tc.WaitFor(func() bool { return tc.LoadGenerator.DataItemsSent() > 0 }, "load generator started")

	tc.Sleep(time.Second * time.Duration(sleepTime))

	tc.StopLoad()

	tc.WaitForN(func() bool { return tc.LoadGenerator.DataItemsSent() == tc.MockBackend.DataItemsReceived() }, 60*time.Second, "all logs received")

	tc.ValidateData()
}

func ScenarioMemoryLimiterHit(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	loadOptions testbed.LoadOptions,
	resultsSummary testbed.TestResultsSummary,
	sleepTime int,
	processors []ProcessorNameAndConfigBody,
) {
	resultDir, err := filepath.Abs(path.Join("results", t.Name()))
	require.NoError(t, err)

	agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))

	configStr := createConfigYaml(t, sender, receiver, resultDir, processors, nil)
	configCleanup, err := agentProc.PrepareConfig(t, configStr)
	require.NoError(t, err)
	defer configCleanup()
	dataProvider := testbed.NewPerfTestDataProvider(loadOptions)
	dataChannel := make(chan bool)
	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		agentProc,
		&testbed.CorrectnessLogTestValidator{},
		resultsSummary,
		testbed.WithDecisionFunc(func() error { return testbed.GenerateNonPernamentErrorUntil(dataChannel) }),
	)
	t.Cleanup(tc.Stop)
	tc.MockBackend.EnableRecording()

	tc.StartBackend()
	tc.StartAgent()

	tc.StartLoad(loadOptions)

	tc.WaitFor(func() bool { return tc.LoadGenerator.DataItemsSent() > 0 }, "load generator started")

	var timer *time.Timer

	// check for "Memory usage is above soft limit"
	tc.WaitForN(func() bool {
		logFound := tc.AgentLogsContains("Memory usage is above soft limit. Refusing data.")
		if !logFound {
			dataChannel <- true
			return false
		}
		// Log found. But keep the collector under stress for 10 more seconds so it starts refusing data
		if timer == nil {
			timer = time.NewTimer(10 * time.Second)
		}
		select {
		case <-timer.C:
		default:
			return false
		}
		close(dataChannel)
		return logFound
	}, time.Second*time.Duration(sleepTime), "memory limit not hit")

	// check if data started to be received successfully
	tc.WaitForN(func() bool {
		return tc.MockBackend.DataItemsReceived() > 0
	}, time.Second*time.Duration(sleepTime), "data started to be successfully received")

	// stop sending any more data
	tc.StopLoad()

	tc.WaitForN(func() bool { return tc.LoadGenerator.DataItemsSent() == tc.MockBackend.DataItemsReceived() }, time.Second*time.Duration(sleepTime), "all logs received")

	tc.WaitForN(func() bool {
		// get IDs from logs to retry
		logsToRetry := getLogsID(tc.MockBackend.LogsToRetry)

		// get IDs from logs received successfully
		successfulLogs := getLogsID(tc.MockBackend.ReceivedLogs)

		// check if all the logs to retry were actually retried
		logsWereRetried := allElementsExistInSlice(logsToRetry, successfulLogs)
		return logsWereRetried
	}, time.Second*time.Duration(sleepTime), "all logs were retried successfully")

	tc.StopAgent()
	tc.ValidateData()
}

func constructLoadOptions(test TestCase) testbed.LoadOptions {
	options := testbed.LoadOptions{DataItemsPerSecond: 1000, ItemsPerBatch: 10}
	options.Attributes = make(map[string]string)

	// Generate attributes.
	for i := 0; i < test.attrCount; i++ {
		attrName := genRandByteString(rand.IntN(199) + 1)
		options.Attributes[attrName] = genRandByteString(rand.IntN(test.attrSizeByte*2-1) + 1)
	}
	return options
}

func getLogsID(logToRetry []plog.Logs) []string {
	var result []string
	for _, logElement := range logToRetry {
		logRecord := logElement.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
		for index := 0; index < logRecord.Len(); index++ {
			logObj := logRecord.At(index)
			itemIndex, batchIndex := extractIDFromLog(logObj)
			result = append(result, fmt.Sprintf("%s%s", batchIndex, itemIndex))
		}
	}
	return result
}

func allElementsExistInSlice(slice1, slice2 []string) bool {
	// Create a map to store elements of slice2 for efficient lookup
	elementMap := make(map[string]bool)

	// Populate the map with elements from slice2
	for _, element := range slice2 {
		elementMap[element] = true
	}

	// Check if all elements of slice1 exist in slice2
	for _, element := range slice1 {
		if _, exists := elementMap[element]; !exists {
			return false
		}
	}

	return true
}

// in case of filelog receiver, the batch_index and item_index are a part of log body.
// we use regex to extract them
func extractIDFromLog(log plog.LogRecord) (string, string) {
	var batch, item string
	match := batchRegex.FindStringSubmatch(log.Body().AsString())
	if len(match) == 2 {
		batch = match[0]
	}
	match = itemRegex.FindStringSubmatch(log.Body().AsString())
	if len(match) == 2 {
		batch = match[0]
	}
	// in case of otlp receiver, batch_index and item_index are part of attributes.
	if batchIndex, ok := log.Attributes().Get("batch_index"); ok {
		batch = batchIndex.AsString()
	}
	if itemIndex, ok := log.Attributes().Get("item_index"); ok {
		item = itemIndex.AsString()
	}
	return batch, item
}
