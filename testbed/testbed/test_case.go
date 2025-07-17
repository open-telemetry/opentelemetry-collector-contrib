// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testbed // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestCase defines a running test case.
type TestCase struct {
	t *testing.T

	// Directory where test case results and logs will be written.
	resultDir string

	// does not write out results when set to true
	skipResults bool

	// Resource spec for agent.
	resourceSpec ResourceSpec

	// Agent process.
	agentProc OtelcolRunner

	receiver DataReceiver

	LoadGenerator LoadGenerator
	MockBackend   *MockBackend
	validator     TestCaseValidator

	startTime time.Time

	// errorSignal indicates an error in the test case execution, e.g. process execution
	// failure or exceeding resource consumption, etc. The actual error message is already
	// logged, this is only an indicator on which you can wait to be informed.
	errorSignal       chan struct{}
	errorSignalCloser *sync.Once
	// Duration is the requested duration of the tests. Configured via TESTBED_DURATION
	// env variable and defaults to 15 seconds if env variable is unspecified.
	Duration       time.Duration
	doneSignal     chan struct{}
	errorCause     string
	resultsSummary TestResultsSummary

	// decision makes mockbackend return permanent/non-permament errors at random basis
	decision decisionFunc
}

const (
	mibibyte            = 1024 * 1024
	testcaseDurationVar = "TESTCASE_DURATION"
)

// NewTestCase creates a new TestCase. It expects agent-config.yaml in the specified directory.
func NewTestCase(
	t *testing.T,
	dataProvider DataProvider,
	sender DataSender,
	receiver DataReceiver,
	agentProc OtelcolRunner,
	validator TestCaseValidator,
	resultsSummary TestResultsSummary,
	opts ...TestCaseOption,
) *TestCase {
	loadGenerator, err := NewLoadGenerator(dataProvider, sender)
	require.NoError(t, err, "Cannot create generator")
	return NewLoadGeneratorTestCase(t, loadGenerator, receiver, agentProc, validator, resultsSummary, opts...)
}

func NewLoadGeneratorTestCase(t *testing.T, loadGenerator LoadGenerator, receiver DataReceiver, agentProc OtelcolRunner, validator TestCaseValidator, resultsSummary TestResultsSummary, opts ...TestCaseOption) *TestCase {
	tc := TestCase{
		t:                 t,
		errorSignal:       make(chan struct{}),
		errorSignalCloser: &sync.Once{},
		doneSignal:        make(chan struct{}),
		startTime:         time.Now(),
		LoadGenerator:     loadGenerator,
		receiver:          receiver,
		agentProc:         agentProc,
		validator:         validator,
		resultsSummary:    resultsSummary,
		decision:          func() error { return nil },
	}

	// Get requested test case duration from env variable.
	duration := os.Getenv(testcaseDurationVar)
	if duration == "" {
		duration = "15s"
	}
	var err error
	tc.Duration, err = time.ParseDuration(duration)
	if err != nil {
		log.Fatalf("Invalid "+testcaseDurationVar+": %v. Expecting a valid duration string.", duration)
	}

	// Apply all provided options.
	for _, opt := range opts {
		opt(&tc)
	}

	// Prepare directory for results.
	tc.resultDir, err = filepath.Abs(path.Join("results", t.Name()))
	require.NoErrorf(t, err, "Cannot resolve %s", t.Name())
	require.NoErrorf(t, os.MkdirAll(tc.resultDir, os.ModePerm), "Cannot create directory %s", tc.resultDir)

	// Set default resource check period.
	tc.resourceSpec.ResourceCheckPeriod = 3 * time.Second
	if tc.Duration < tc.resourceSpec.ResourceCheckPeriod {
		// Resource check period should not be longer than entire test duration.
		tc.resourceSpec.ResourceCheckPeriod = tc.Duration
	}

	tc.MockBackend = NewMockBackend(tc.ComposeTestResultFileName("backend.log"), receiver)
	tc.MockBackend.WithDecisionFunc(tc.decision)

	go tc.logStats()

	return &tc
}

func (tc *TestCase) ComposeTestResultFileName(fileName string) string {
	fileName, err := filepath.Abs(path.Join(tc.resultDir, fileName))
	require.NoError(tc.t, err, "Cannot resolve %s", fileName)
	return fileName
}

// StartAgent starts the agent and redirects its standard output and standard error
// to "agent.log" file located in the test directory.
func (tc *TestCase) StartAgent(args ...string) {
	logFileName := tc.ComposeTestResultFileName("agent.log")

	startParams := StartParams{
		Name:         "Agent",
		LogFilePath:  logFileName,
		CmdArgs:      args,
		resourceSpec: &tc.resourceSpec,
	}
	if err := tc.agentProc.Start(startParams); err != nil {
		tc.indicateError(err)
		return
	}

	// Start watching resource consumption.
	go func() {
		if err := tc.agentProc.WatchResourceConsumption(); err != nil {
			tc.indicateError(err)
		}
	}()

	tc.WaitFor(tc.LoadGenerator.IsReady, "LoadGenerator isn't ready")
}

// StopAgent stops agent process.
func (tc *TestCase) StopAgent() {
	if _, err := tc.agentProc.Stop(); err != nil {
		tc.indicateError(err)
	}
}

// StartLoad starts the load generator and redirects its standard output and standard error
// to "load-generator.log" file located in the test directory.
func (tc *TestCase) StartLoad(options LoadOptions) {
	tc.LoadGenerator.Start(options)
}

// StopLoad stops load generator.
func (tc *TestCase) StopLoad() {
	tc.LoadGenerator.Stop()
}

// StartBackend starts the specified backend type.
func (tc *TestCase) StartBackend() {
	require.NoError(tc.t, tc.MockBackend.Start(), "Cannot start backend")
}

// StopBackend stops the backend.
func (tc *TestCase) StopBackend() {
	tc.MockBackend.Stop()
}

// EnableRecording enables recording of all data received by MockBackend.
func (tc *TestCase) EnableRecording() {
	tc.MockBackend.EnableRecording()
}

// AgentMemoryInfo returns raw memory info struct about the agent
// as returned by github.com/shirou/gopsutil/process
func (tc *TestCase) AgentMemoryInfo() (uint32, uint32, error) {
	stat, err := tc.agentProc.GetProcessMon().MemoryInfo()
	if err != nil {
		return 0, 0, err
	}
	return uint32(stat.RSS / mibibyte), uint32(stat.VMS / mibibyte), nil
}

// Stop stops the load generator, the agent and the backend.
func (tc *TestCase) Stop() {
	// Stop monitoring the agent
	close(tc.doneSignal)

	// Stop all components
	tc.StopLoad()
	tc.StopAgent()
	tc.StopBackend()

	if tc.skipResults {
		return
	}

	// Report test results
	tc.validator.RecordResults(tc)
}

// ValidateData validates data received by mock backend against what was generated and sent to the collector
// instance(s) under test by the ProviderSender.
func (tc *TestCase) ValidateData() {
	select {
	case <-tc.errorSignal:
		// Error is already signaled and recorded. Validating data is pointless.
		return
	default:
	}

	tc.validator.Validate(tc)
}

// Sleep for specified duration or until error is signaled.
func (tc *TestCase) Sleep(d time.Duration) {
	select {
	case <-time.After(d):
	case <-tc.errorSignal:
	}
}

// WaitForN the specific condition for up to a specified duration. Records a test error
// if time is out and condition does not become true. If error is signaled
// while waiting the function will return false, but will not record additional
// test error (we assume that signaled error is already recorded in indicateError()).
func (tc *TestCase) WaitForN(cond func() bool, duration time.Duration, errMsg any) bool {
	startTime := time.Now()

	// Start with 5 ms waiting interval between condition re-evaluation.
	waitInterval := time.Millisecond * 5

	for {
		if cond() {
			return true
		}

		select {
		case <-time.After(waitInterval):
		case <-tc.errorSignal:
			return false
		}

		// Increase waiting interval exponentially up to 500 ms.
		if waitInterval < time.Millisecond*500 {
			waitInterval *= 2
		}

		if time.Since(startTime) > duration {
			// Waited too long
			tc.indicateError(fmt.Errorf("Time out waiting for %v", errMsg))
			return false
		}
	}
}

// WaitFor is like WaitForN but with a fixed duration of 10 seconds
func (tc *TestCase) WaitFor(cond func() bool, errMsg any) bool {
	return tc.WaitForN(cond, time.Second*10, errMsg)
}

func (tc *TestCase) indicateError(err error) {
	// Print for visibility but only set test error on first pass
	log.Print(err.Error())

	tc.errorSignalCloser.Do(func() {
		tc.t.Error(err.Error())

		tc.errorCause = err.Error()

		// Signal the error via channel
		close(tc.errorSignal)
	})
}

func (tc *TestCase) logStats() {
	t := time.NewTicker(tc.resourceSpec.ResourceCheckPeriod)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			tc.logStatsOnce()
		case <-tc.doneSignal:
			return
		}
	}
}

func (tc *TestCase) logStatsOnce() {
	log.Printf("%s | %s | %s",
		tc.agentProc.GetResourceConsumption(),
		tc.LoadGenerator.GetStats(),
		tc.MockBackend.GetStats())
}

// Used to search for text in agent.log
// It can be used to verify if we've hit QueuedRetry sender or memory limiter
func (tc *TestCase) AgentLogsContains(text string) bool {
	filename := tc.ComposeTestResultFileName("agent.log")
	cmd := exec.Command("cat", filename)
	grep := exec.Command("grep", "-E", text)

	pipe, err := cmd.StdoutPipe()
	defer func(pipe io.ReadCloser) {
		err = pipe.Close()
		if err != nil {
			panic(err)
		}
	}(pipe)
	grep.Stdin = pipe

	if err != nil {
		log.Printf("Error while searching %s in %s", text, tc.ComposeTestResultFileName("agent.log"))
		return false
	}

	err = cmd.Start()
	if err != nil {
		log.Print("Error while executing command: ", err.Error())
		return false
	}

	res, _ := grep.Output()
	return string(res) != ""
}
