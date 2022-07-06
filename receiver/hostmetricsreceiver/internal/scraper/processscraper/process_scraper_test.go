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

package processscraper

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"testing"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.opentelemetry.io/collector/service/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
)

func skipTestOnUnsupportedOS(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skipf("skipping test on %v", runtime.GOOS)
	}
}

func TestScrape(t *testing.T) {
	skipTestOnUnsupportedOS(t)
	type testCase struct {
		name                                       string
		removeDirectionAttributeFeatureGateEnabled bool
	}
	testCases := []testCase{
		{
			name: "Standard",
		},
		{
			name: "Standard with direction removed",
			removeDirectionAttributeFeatureGateEnabled: true,
		},
	}

	const bootTime = 100
	const expectedStartTime = 100 * 1e9

	originalVal := featuregate.GetRegistry().IsEnabled(removeDirectionAttributeFeatureGateID)
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			featuregate.GetRegistry().Apply(map[string]bool{removeDirectionAttributeFeatureGateID: test.removeDirectionAttributeFeatureGateEnabled})

			scraper, err := newProcessScraper(componenttest.NewNopReceiverCreateSettings(), &Config{Metrics: metadata.DefaultMetricsSettings()})
			scraper.bootTime = func() (uint64, error) { return bootTime, nil }
			require.NoError(t, err, "Failed to create process scraper: %v", err)
			err = scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize process scraper: %v", err)

			md, err := scraper.scrape(context.Background())

			// may receive some partial errors as a result of attempting to:
			// a) read native system processes on Windows (e.g. Registry process)
			// b) read info on processes that have just terminated
			//
			// so validate that we have at some errors & some valid data
			if err != nil {
				require.True(t, scrapererror.IsPartialScrapeError(err))
				noProcessesScraped := md.ResourceMetrics().Len()
				var scraperErr scrapererror.PartialScrapeError
				require.ErrorAs(t, err, &scraperErr)
				noProcessesErrored := scraperErr.Failed
				require.Lessf(t, 0, noProcessesErrored, "Failed to scrape metrics - : error, but 0 failed process %v", err)
				require.Lessf(t, 0, noProcessesScraped, "Failed to scrape metrics - : 0 successful scrapes %v", err)
			}

			require.Greater(t, md.ResourceMetrics().Len(), 1)
			assertProcessResourceAttributesExist(t, md.ResourceMetrics())
			assertCPUTimeMetricValid(t, md.ResourceMetrics(), expectedStartTime)
			assertMemoryUsageMetricValid(t, md.ResourceMetrics(), expectedStartTime)
			assertDiskIOMetricValid(t, md.ResourceMetrics(), expectedStartTime, test.removeDirectionAttributeFeatureGateEnabled)
			assertSameTimeStampForAllMetricsWithinResource(t, md.ResourceMetrics())
		})
	}
	featuregate.GetRegistry().Apply(map[string]bool{removeDirectionAttributeFeatureGateID: originalVal})
}

func assertProcessResourceAttributesExist(t *testing.T, resourceMetrics pmetric.ResourceMetricsSlice) {
	for i := 0; i < resourceMetrics.Len(); i++ {
		attr := resourceMetrics.At(0).Resource().Attributes()
		internal.AssertContainsAttribute(t, attr, conventions.AttributeProcessPID)
		internal.AssertContainsAttribute(t, attr, conventions.AttributeProcessExecutableName)
		internal.AssertContainsAttribute(t, attr, conventions.AttributeProcessExecutablePath)
		internal.AssertContainsAttribute(t, attr, conventions.AttributeProcessCommand)
		internal.AssertContainsAttribute(t, attr, conventions.AttributeProcessCommandLine)
		internal.AssertContainsAttribute(t, attr, conventions.AttributeProcessOwner)
	}
}

func assertCPUTimeMetricValid(t *testing.T, resourceMetrics pmetric.ResourceMetricsSlice, startTime pcommon.Timestamp) {
	cpuTimeMetric := getMetric(t, "process.cpu.time", resourceMetrics)
	assert.Equal(t, "process.cpu.time", cpuTimeMetric.Name())
	if startTime != 0 {
		internal.AssertSumMetricStartTimeEquals(t, cpuTimeMetric, startTime)
	}
	internal.AssertSumMetricHasAttributeValue(t, cpuTimeMetric, 0, "state",
		pcommon.NewValueString(metadata.AttributeStateUser.String()))
	internal.AssertSumMetricHasAttributeValue(t, cpuTimeMetric, 1, "state",
		pcommon.NewValueString(metadata.AttributeStateSystem.String()))
	if runtime.GOOS == "linux" {
		internal.AssertSumMetricHasAttributeValue(t, cpuTimeMetric, 2, "state",
			pcommon.NewValueString(metadata.AttributeStateWait.String()))
	}
}

func assertMemoryUsageMetricValid(t *testing.T, resourceMetrics pmetric.ResourceMetricsSlice, startTime pcommon.Timestamp) {
	physicalMemUsageMetric := getMetric(t, "process.memory.physical_usage", resourceMetrics)
	assert.Equal(t, "process.memory.physical_usage", physicalMemUsageMetric.Name())
	virtualMemUsageMetric := getMetric(t, "process.memory.virtual_usage", resourceMetrics)
	assert.Equal(t, "process.memory.virtual_usage", virtualMemUsageMetric.Name())

	if startTime != 0 {
		internal.AssertSumMetricStartTimeEquals(t, physicalMemUsageMetric, startTime)
		internal.AssertSumMetricStartTimeEquals(t, virtualMemUsageMetric, startTime)
	}
}

func assertDiskIOMetricValid(t *testing.T, resourceMetrics pmetric.ResourceMetricsSlice, startTime pcommon.Timestamp, removeDirection bool) {
	if removeDirection {
		for _, metricName := range []string{"process.disk.io.read", "process.disk.io.write"} {
			diskIOMetric := getMetric(t, metricName, resourceMetrics)
			assert.Equal(t, metricName, diskIOMetric.Name())
			if startTime != 0 {
				internal.AssertSumMetricStartTimeEquals(t, diskIOMetric, startTime)
			}
		}
	} else {
		diskIOMetric := getMetric(t, "process.disk.io", resourceMetrics)
		assert.Equal(t, "process.disk.io", diskIOMetric.Name())
		if startTime != 0 {
			internal.AssertSumMetricStartTimeEquals(t, diskIOMetric, startTime)
		}
		internal.AssertSumMetricHasAttributeValue(t, diskIOMetric, 0, "direction",
			pcommon.NewValueString(metadata.AttributeDirectionRead.String()))
		internal.AssertSumMetricHasAttributeValue(t, diskIOMetric, 1, "direction",
			pcommon.NewValueString(metadata.AttributeDirectionWrite.String()))
	}
}

func assertSameTimeStampForAllMetricsWithinResource(t *testing.T, resourceMetrics pmetric.ResourceMetricsSlice) {
	for i := 0; i < resourceMetrics.Len(); i++ {
		ilms := resourceMetrics.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			internal.AssertSameTimeStampForAllMetrics(t, ilms.At(j).Metrics())
		}
	}
}

func getMetric(t *testing.T, expectedMetricName string, rms pmetric.ResourceMetricsSlice) pmetric.Metric {
	for i := 0; i < rms.Len(); i++ {
		metrics := getMetricSlice(t, rms.At(i))
		for j := 0; j < metrics.Len(); j++ {
			metric := metrics.At(j)
			if metric.Name() == expectedMetricName {
				return metric
			}
		}
	}

	require.Fail(t, fmt.Sprintf("no metric with name %s was returned", expectedMetricName))
	return pmetric.NewMetric()
}

func getMetricSlice(t *testing.T, rm pmetric.ResourceMetrics) pmetric.MetricSlice {
	ilms := rm.ScopeMetrics()
	require.Equal(t, 1, ilms.Len())
	return ilms.At(0).Metrics()
}

func TestScrapeMetrics_NewError(t *testing.T) {
	skipTestOnUnsupportedOS(t)

	_, err := newProcessScraper(componenttest.NewNopReceiverCreateSettings(), &Config{Include: MatchConfig{Names: []string{"test"}}, Metrics: metadata.DefaultMetricsSettings()})
	require.Error(t, err)
	require.Regexp(t, "^error creating process include filters:", err.Error())

	_, err = newProcessScraper(componenttest.NewNopReceiverCreateSettings(), &Config{Exclude: MatchConfig{Names: []string{"test"}}, Metrics: metadata.DefaultMetricsSettings()})
	require.Error(t, err)
	require.Regexp(t, "^error creating process exclude filters:", err.Error())
}

func TestScrapeMetrics_GetProcessesError(t *testing.T) {
	skipTestOnUnsupportedOS(t)

	scraper, err := newProcessScraper(componenttest.NewNopReceiverCreateSettings(), &Config{Metrics: metadata.DefaultMetricsSettings()})
	require.NoError(t, err, "Failed to create process scraper: %v", err)

	scraper.getProcessHandles = func() (processHandles, error) { return nil, errors.New("err1") }

	err = scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err, "Failed to initialize process scraper: %v", err)

	md, err := scraper.scrape(context.Background())
	assert.EqualError(t, err, "err1")
	assert.Equal(t, 0, md.ResourceMetrics().Len())
	assert.False(t, scrapererror.IsPartialScrapeError(err))
}

type processHandlesMock struct {
	handles []*processHandleMock
}

func (p *processHandlesMock) Pid(int) int32 {
	return 1
}

func (p *processHandlesMock) At(index int) processHandle {
	return p.handles[index]
}

func (p *processHandlesMock) Len() int {
	return len(p.handles)
}

type processHandleMock struct {
	mock.Mock
}

func (p *processHandleMock) Name() (ret string, err error) {
	args := p.MethodCalled("Name")
	return args.String(0), args.Error(1)
}

func (p *processHandleMock) Exe() (string, error) {
	args := p.MethodCalled("Exe")
	return args.String(0), args.Error(1)
}

func (p *processHandleMock) Username() (string, error) {
	args := p.MethodCalled("Username")
	return args.String(0), args.Error(1)
}

func (p *processHandleMock) Cmdline() (string, error) {
	args := p.MethodCalled("Cmdline")
	return args.String(0), args.Error(1)
}

func (p *processHandleMock) CmdlineSlice() ([]string, error) {
	args := p.MethodCalled("CmdlineSlice")
	return args.Get(0).([]string), args.Error(1)
}

func (p *processHandleMock) Times() (*cpu.TimesStat, error) {
	args := p.MethodCalled("Times")
	return args.Get(0).(*cpu.TimesStat), args.Error(1)
}

func (p *processHandleMock) MemoryInfo() (*process.MemoryInfoStat, error) {
	args := p.MethodCalled("MemoryInfo")
	return args.Get(0).(*process.MemoryInfoStat), args.Error(1)
}

func (p *processHandleMock) IOCounters() (*process.IOCountersStat, error) {
	args := p.MethodCalled("IOCounters")
	return args.Get(0).(*process.IOCountersStat), args.Error(1)
}

func newDefaultHandleMock() *processHandleMock {
	handleMock := &processHandleMock{}
	handleMock.On("Username").Return("username", nil)
	handleMock.On("Cmdline").Return("cmdline", nil)
	handleMock.On("CmdlineSlice").Return([]string{"cmdline"}, nil)
	handleMock.On("Times").Return(&cpu.TimesStat{}, nil)
	handleMock.On("MemoryInfo").Return(&process.MemoryInfoStat{}, nil)
	handleMock.On("IOCounters").Return(&process.IOCountersStat{}, nil)
	return handleMock
}

func TestScrapeMetrics_Filtered(t *testing.T) {
	skipTestOnUnsupportedOS(t)

	type testCase struct {
		name          string
		names         []string
		include       []string
		exclude       []string
		expectedNames []string
	}

	testCases := []testCase{
		{
			name:          "No Filter",
			names:         []string{"test1", "test2"},
			include:       []string{"test*"},
			expectedNames: []string{"test1", "test2"},
		},
		{
			name:          "Include All",
			names:         []string{"test1", "test2"},
			include:       []string{"test*"},
			expectedNames: []string{"test1", "test2"},
		},
		{
			name:          "Include One",
			names:         []string{"test1", "test2"},
			include:       []string{"test1"},
			expectedNames: []string{"test1"},
		},
		{
			name:          "Exclude All",
			names:         []string{"test1", "test2"},
			exclude:       []string{"test*"},
			expectedNames: []string{},
		},
		{
			name:          "Include & Exclude",
			names:         []string{"test1", "test2"},
			include:       []string{"test*"},
			exclude:       []string{"test2"},
			expectedNames: []string{"test1"},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			config := &Config{
				Metrics: metadata.DefaultMetricsSettings(),
			}

			if len(test.include) > 0 {
				config.Include = MatchConfig{
					Names:  test.include,
					Config: filterset.Config{MatchType: filterset.Regexp},
				}
			}
			if len(test.exclude) > 0 {
				config.Exclude = MatchConfig{
					Names:  test.exclude,
					Config: filterset.Config{MatchType: filterset.Regexp},
				}
			}

			scraper, err := newProcessScraper(componenttest.NewNopReceiverCreateSettings(), config)
			require.NoError(t, err, "Failed to create process scraper: %v", err)
			err = scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize process scraper: %v", err)

			handles := make([]*processHandleMock, 0, len(test.names))
			for _, name := range test.names {
				handleMock := newDefaultHandleMock()
				handleMock.On("Name").Return(name, nil)
				handleMock.On("Exe").Return(name, nil)
				handles = append(handles, handleMock)
			}

			scraper.getProcessHandles = func() (processHandles, error) {
				return &processHandlesMock{handles: handles}, nil
			}

			md, err := scraper.scrape(context.Background())
			require.NoError(t, err)

			assert.Equal(t, len(test.expectedNames), md.ResourceMetrics().Len())
			for i, expectedName := range test.expectedNames {
				rm := md.ResourceMetrics().At(i)
				name, _ := rm.Resource().Attributes().Get(conventions.AttributeProcessExecutableName)
				assert.Equal(t, expectedName, name.StringVal())
			}
		})
	}
}

func TestScrapeMetrics_ProcessErrors(t *testing.T) {
	skipTestOnUnsupportedOS(t)

	type testCase struct {
		name            string
		osFilter        string
		nameError       error
		exeError        error
		usernameError   error
		cmdlineError    error
		timesError      error
		memoryInfoError error
		ioCountersError error
		expectedError   string
	}

	testCases := []testCase{
		{
			name:          "Name Error",
			osFilter:      "windows",
			nameError:     errors.New("err1"),
			expectedError: `error reading process name for pid 1: err1`,
		},
		{
			name:          "Exe Error",
			exeError:      errors.New("err1"),
			expectedError: `error reading process name for pid 1: err1`,
		},
		{
			name:          "Cmdline Error",
			cmdlineError:  errors.New("err2"),
			expectedError: `error reading command for process "test" (pid 1): err2`,
		},
		{
			name:          "Username Error",
			usernameError: errors.New("err3"),
			expectedError: `error reading username for process "test" (pid 1): err3`,
		},
		{
			name:          "Times Error",
			timesError:    errors.New("err4"),
			expectedError: `error reading cpu times for process "test" (pid 1): err4`,
		},
		{
			name:            "Memory Info Error",
			memoryInfoError: errors.New("err5"),
			expectedError:   `error reading memory info for process "test" (pid 1): err5`,
		},
		{
			name:            "IO Counters Error",
			ioCountersError: errors.New("err6"),
			expectedError:   `error reading disk usage for process "test" (pid 1): err6`,
		},
		{
			name:            "Multiple Errors",
			cmdlineError:    errors.New("err2"),
			usernameError:   errors.New("err3"),
			timesError:      errors.New("err4"),
			memoryInfoError: errors.New("err5"),
			ioCountersError: errors.New("err6"),
			expectedError: `error reading command for process "test" (pid 1): err2; ` +
				`error reading username for process "test" (pid 1): err3; ` +
				`error reading cpu times for process "test" (pid 1): err4; ` +
				`error reading memory info for process "test" (pid 1): err5; ` +
				`error reading disk usage for process "test" (pid 1): err6`,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			if test.osFilter == runtime.GOOS {
				t.Skipf("skipping test %v on %v", test.name, runtime.GOOS)
			}

			scraper, err := newProcessScraper(componenttest.NewNopReceiverCreateSettings(), &Config{Metrics: metadata.DefaultMetricsSettings()})
			require.NoError(t, err, "Failed to create process scraper: %v", err)
			err = scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize process scraper: %v", err)

			username := "username"
			if test.usernameError != nil {
				username = ""
			}

			handleMock := &processHandleMock{}
			handleMock.On("Name").Return("test", test.nameError)
			handleMock.On("Exe").Return("test", test.exeError)
			handleMock.On("Username").Return(username, test.usernameError)
			handleMock.On("Cmdline").Return("cmdline", test.cmdlineError)
			handleMock.On("CmdlineSlice").Return([]string{"cmdline"}, test.cmdlineError)
			handleMock.On("Times").Return(&cpu.TimesStat{}, test.timesError)
			handleMock.On("MemoryInfo").Return(&process.MemoryInfoStat{}, test.memoryInfoError)
			handleMock.On("IOCounters").Return(&process.IOCountersStat{}, test.ioCountersError)

			scraper.getProcessHandles = func() (processHandles, error) {
				return &processHandlesMock{handles: []*processHandleMock{handleMock}}, nil
			}

			md, err := scraper.scrape(context.Background())

			expectedResourceMetricsLen, expectedMetricsLen := getExpectedLengthOfReturnedMetrics(test.nameError, test.exeError, test.timesError, test.memoryInfoError, test.ioCountersError)
			assert.Equal(t, expectedResourceMetricsLen, md.ResourceMetrics().Len())
			assert.Equal(t, expectedMetricsLen, md.MetricCount())

			assert.EqualError(t, err, test.expectedError)
			isPartial := scrapererror.IsPartialScrapeError(err)
			assert.True(t, isPartial)
			if isPartial {
				expectedFailures := getExpectedScrapeFailures(test.nameError, test.exeError, test.timesError, test.memoryInfoError, test.ioCountersError)
				var scraperErr scrapererror.PartialScrapeError
				require.ErrorAs(t, err, &scraperErr)
				assert.Equal(t, expectedFailures, scraperErr.Failed)
			}
		})
	}
}

func getExpectedLengthOfReturnedMetrics(nameError, exeError, timeError, memError, diskError error) (int, int) {
	if nameError != nil || exeError != nil {
		return 0, 0
	}

	expectedLen := 0
	if timeError == nil {
		expectedLen += cpuMetricsLen
	}
	if memError == nil {
		expectedLen += memoryMetricsLen
	}
	if diskError == nil {
		expectedLen += diskMetricsLen
	}

	if expectedLen == 0 {
		return 0, 0
	}
	return 1, expectedLen
}

func getExpectedScrapeFailures(nameError, exeError, timeError, memError, diskError error) int {
	if nameError != nil || exeError != nil {
		return 1
	}
	_, expectedMetricsLen := getExpectedLengthOfReturnedMetrics(nameError, exeError, timeError, memError, diskError)
	return metricsLen - expectedMetricsLen
}

func TestScrapeMetrics_MuteProcessNameError(t *testing.T) {
	skipTestOnUnsupportedOS(t)

	processNameError := errors.New("err1")

	type testCase struct {
		name                 string
		muteProcessNameError bool
		omitConfigField      bool
		expectedError        string
	}

	testCases := []testCase{
		{
			name:                 "Process Name Error Muted",
			muteProcessNameError: true,
		},
		{
			name:                 "Process Name Error Enabled",
			muteProcessNameError: false,
			expectedError:        fmt.Sprintf("error reading process name for pid 1: %v", processNameError),
		},
		{
			name:            "Process Name Error Default (Enabled)",
			omitConfigField: true,
			expectedError:   fmt.Sprintf("error reading process name for pid 1: %v", processNameError),
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			config := &Config{Metrics: metadata.DefaultMetricsSettings()}
			if !test.omitConfigField {
				config.MuteProcessNameError = test.muteProcessNameError
			}
			scraper, err := newProcessScraper(componenttest.NewNopReceiverCreateSettings(), config)
			require.NoError(t, err, "Failed to create process scraper: %v", err)
			err = scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize process scraper: %v", err)

			handleMock := &processHandleMock{}
			handleMock.On("Name").Return("test", processNameError)
			handleMock.On("Exe").Return("test", processNameError)

			scraper.getProcessHandles = func() (processHandles, error) {
				return &processHandlesMock{handles: []*processHandleMock{handleMock}}, nil
			}
			md, err := scraper.scrape(context.Background())

			assert.Zero(t, md.MetricCount())
			if config.MuteProcessNameError {
				assert.Nil(t, err)
			} else {
				assert.EqualError(t, err, test.expectedError)
			}
		})
	}
}
