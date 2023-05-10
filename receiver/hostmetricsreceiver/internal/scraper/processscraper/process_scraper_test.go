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
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
)

func skipTestOnUnsupportedOS(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		t.Skipf("skipping test on %v", runtime.GOOS)
	}
}

func enableLinuxOnlyMetrics(ms *metadata.MetricsConfig) {
	if runtime.GOOS != "linux" {
		return
	}

	ms.ProcessPagingFaults.Enabled = true
	ms.ProcessContextSwitches.Enabled = true
	ms.ProcessOpenFileDescriptors.Enabled = true
	ms.ProcessSignalsPending.Enabled = true
}

func TestScrape(t *testing.T) {
	skipTestOnUnsupportedOS(t)
	type testCase struct {
		name                string
		mutateScraper       func(*scraper)
		mutateMetricsConfig func(*testing.T, *metadata.MetricsConfig)
	}
	testCases := []testCase{
		{
			name: "Default set of metrics",
		},
		{
			name: "Enable Linux-only metrics",
			mutateMetricsConfig: func(t *testing.T, ms *metadata.MetricsConfig) {
				if runtime.GOOS != "linux" {
					t.Skipf("skipping test on %v", runtime.GOOS)
				}

				enableLinuxOnlyMetrics(ms)
			},
		},
		{
			name: "Enable memory utilization",
			mutateMetricsConfig: func(t *testing.T, ms *metadata.MetricsConfig) {
				ms.ProcessMemoryUtilization.Enabled = true
			},
		},
	}

	const createTime = 100
	const expectedStartTime = 100 * 1e6

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			metricsBuilderConfig := metadata.DefaultMetricsBuilderConfig()
			if runtime.GOOS == "darwin" {
				// disable darwin unsupported default metric
				metricsBuilderConfig.Metrics.ProcessDiskIo.Enabled = false
			}

			if test.mutateMetricsConfig != nil {
				test.mutateMetricsConfig(t, &metricsBuilderConfig.Metrics)
			}
			scraper, err := newProcessScraper(receivertest.NewNopCreateSettings(), &Config{MetricsBuilderConfig: metricsBuilderConfig})
			if test.mutateScraper != nil {
				test.mutateScraper(scraper)
			}
			scraper.getProcessCreateTime = func(p processHandle) (int64, error) { return createTime, nil }
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
			if metricsBuilderConfig.Metrics.ProcessCPUUtilization.Enabled {
				assertCPUUtilizationMetricValid(t, md.ResourceMetrics(), expectedStartTime)
			} else {
				assertMetricMissing(t, md.ResourceMetrics(), "process.cpu.utilization")
			}
			assertMemoryUsageMetricValid(t, md.ResourceMetrics(), expectedStartTime)
			if metricsBuilderConfig.Metrics.ProcessDiskIo.Enabled {
				assertDiskIoMetricValid(t, md.ResourceMetrics(), expectedStartTime)
			}
			if metricsBuilderConfig.Metrics.ProcessDiskOperations.Enabled {
				assertDiskOperationsMetricValid(t, md.ResourceMetrics(), expectedStartTime)
			} else {
				assertMetricMissing(t, md.ResourceMetrics(), "process.disk.operations")
			}
			if metricsBuilderConfig.Metrics.ProcessPagingFaults.Enabled {
				assertPagingMetricValid(t, md.ResourceMetrics(), expectedStartTime)
			}
			if metricsBuilderConfig.Metrics.ProcessSignalsPending.Enabled {
				assertSignalsPendingMetricValid(t, md.ResourceMetrics(), expectedStartTime)
			}
			if metricsBuilderConfig.Metrics.ProcessMemoryUtilization.Enabled {
				assertMemoryUtilizationMetricValid(t, md.ResourceMetrics(), expectedStartTime)
			}
			if metricsBuilderConfig.Metrics.ProcessThreads.Enabled {
				assertThreadsCountValid(t, md.ResourceMetrics(), expectedStartTime)
			} else {
				assertMetricMissing(t, md.ResourceMetrics(), "process.threads")
			}
			if metricsBuilderConfig.Metrics.ProcessContextSwitches.Enabled {
				assertContextSwitchMetricValid(t, md.ResourceMetrics(), expectedStartTime)
			} else {
				assertMetricMissing(t, md.ResourceMetrics(), "process.context_switches")
			}
			if metricsBuilderConfig.Metrics.ProcessOpenFileDescriptors.Enabled {
				assertOpenFileDescriptorMetricValid(t, md.ResourceMetrics(), expectedStartTime)
			} else {
				assertMetricMissing(t, md.ResourceMetrics(), "process.open_file_descriptors")
			}
			assertSameTimeStampForAllMetricsWithinResource(t, md.ResourceMetrics())
		})
	}
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
		internal.AssertContainsAttribute(t, attr, "process.parent_pid")
	}
}

func assertCPUTimeMetricValid(t *testing.T, resourceMetrics pmetric.ResourceMetricsSlice, startTime pcommon.Timestamp) {
	cpuTimeMetric := getMetric(t, "process.cpu.time", resourceMetrics)
	assert.Equal(t, "process.cpu.time", cpuTimeMetric.Name())
	if startTime != 0 {
		internal.AssertSumMetricStartTimeEquals(t, cpuTimeMetric, startTime)
	}
	internal.AssertSumMetricHasAttributeValue(t, cpuTimeMetric, 0, "state",
		pcommon.NewValueStr(metadata.AttributeStateUser.String()))
	internal.AssertSumMetricHasAttributeValue(t, cpuTimeMetric, 1, "state",
		pcommon.NewValueStr(metadata.AttributeStateSystem.String()))
	if runtime.GOOS == "linux" {
		internal.AssertSumMetricHasAttributeValue(t, cpuTimeMetric, 2, "state",
			pcommon.NewValueStr(metadata.AttributeStateWait.String()))
	}
}

func assertCPUUtilizationMetricValid(t *testing.T, resourceMetrics pmetric.ResourceMetricsSlice, startTime pcommon.Timestamp) {
	metric := getMetric(t, "process.cpu.utilization", resourceMetrics)
	if startTime != 0 {
		internal.AssertGaugeMetricStartTimeEquals(t, metric, startTime)
	}

	internal.AssertGaugeMetricHasAttributeValue(t, metric, 0, "state", pcommon.NewValueStr(metadata.AttributeStateUser.String()))
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 1, "state", pcommon.NewValueStr(metadata.AttributeStateSystem.String()))
	if runtime.GOOS == "linux" {
		internal.AssertGaugeMetricHasAttributeValue(t, metric, 2, "state", pcommon.NewValueStr(metadata.AttributeStateWait.String()))
	}
}

func assertMemoryUsageMetricValid(t *testing.T, resourceMetrics pmetric.ResourceMetricsSlice, startTime pcommon.Timestamp) {
	physicalMemUsageMetric := getMetric(t, "process.memory.usage", resourceMetrics)
	assert.Equal(t, "process.memory.usage", physicalMemUsageMetric.Name())
	virtualMemUsageMetric := getMetric(t, "process.memory.virtual", resourceMetrics)
	assert.Equal(t, "process.memory.virtual", virtualMemUsageMetric.Name())

	if startTime != 0 {
		internal.AssertSumMetricStartTimeEquals(t, physicalMemUsageMetric, startTime)
		internal.AssertSumMetricStartTimeEquals(t, virtualMemUsageMetric, startTime)
	}
}

func assertPagingMetricValid(t *testing.T, resourceMetrics pmetric.ResourceMetricsSlice, startTime pcommon.Timestamp) {
	pagingFaultsMetric := getMetric(t, "process.paging.faults", resourceMetrics)
	internal.AssertSumMetricHasAttributeValue(t, pagingFaultsMetric, 0, "type", pcommon.NewValueStr(metadata.AttributePagingFaultTypeMajor.String()))
	internal.AssertSumMetricHasAttributeValue(t, pagingFaultsMetric, 1, "type", pcommon.NewValueStr(metadata.AttributePagingFaultTypeMinor.String()))

	if startTime != 0 {
		internal.AssertSumMetricStartTimeEquals(t, pagingFaultsMetric, startTime)
	}
}

func assertSignalsPendingMetricValid(t *testing.T, resourceMetrics pmetric.ResourceMetricsSlice, startTime pcommon.Timestamp) {
	signalsPendingMetric := getMetric(t, "process.signals_pending", resourceMetrics)
	if startTime != 0 {
		internal.AssertSumMetricStartTimeEquals(t, signalsPendingMetric, startTime)
	}
}

func assertMemoryUtilizationMetricValid(t *testing.T, resourceMetrics pmetric.ResourceMetricsSlice, startTime pcommon.Timestamp) {
	memoryUtilizationMetric := getMetric(t, "process.memory.utilization", resourceMetrics)

	if startTime != 0 {
		internal.AssertGaugeMetricStartTimeEquals(t, memoryUtilizationMetric, startTime)
	}
}

func assertThreadsCountValid(t *testing.T, resourceMetrics pmetric.ResourceMetricsSlice, startTime pcommon.Timestamp) {
	for _, metricName := range []string{"process.threads"} {
		threadsMetric := getMetric(t, metricName, resourceMetrics)
		if startTime != 0 {
			internal.AssertSumMetricStartTimeEquals(t, threadsMetric, startTime)
		}
	}
}

func assertMetricMissing(t *testing.T, resourceMetrics pmetric.ResourceMetricsSlice, expectedMetricName string) {
	for i := 0; i < resourceMetrics.Len(); i++ {
		metrics := getMetricSlice(t, resourceMetrics.At(i))
		for j := 0; j < metrics.Len(); j++ {
			metric := metrics.At(j)
			if metric.Name() == expectedMetricName {
				require.Fail(t, fmt.Sprintf("metric with name %s should not be present", expectedMetricName))
			}
		}
	}
}

func assertDiskIoMetricValid(t *testing.T, resourceMetrics pmetric.ResourceMetricsSlice,
	startTime pcommon.Timestamp) {
	diskIoMetric := getMetric(t, "process.disk.io", resourceMetrics)
	if startTime != 0 {
		internal.AssertSumMetricStartTimeEquals(t, diskIoMetric, startTime)
	}
	internal.AssertSumMetricHasAttributeValue(t, diskIoMetric, 0, "direction",
		pcommon.NewValueStr(metadata.AttributeDirectionRead.String()))
	internal.AssertSumMetricHasAttributeValue(t, diskIoMetric, 1, "direction",
		pcommon.NewValueStr(metadata.AttributeDirectionWrite.String()))
}

func assertDiskOperationsMetricValid(t *testing.T, resourceMetrics pmetric.ResourceMetricsSlice, startTime pcommon.Timestamp) {
	diskOperationsMetric := getMetric(t, "process.disk.operations", resourceMetrics)
	if startTime != 0 {
		internal.AssertSumMetricStartTimeEquals(t, diskOperationsMetric, startTime)
	}
	internal.AssertSumMetricHasAttributeValue(t, diskOperationsMetric, 0, "direction",
		pcommon.NewValueStr(metadata.AttributeDirectionRead.String()))
	internal.AssertSumMetricHasAttributeValue(t, diskOperationsMetric, 1, "direction",
		pcommon.NewValueStr(metadata.AttributeDirectionWrite.String()))
}

func assertContextSwitchMetricValid(t *testing.T, resourceMetrics pmetric.ResourceMetricsSlice, startTime pcommon.Timestamp) {
	contextSwitchMetric := getMetric(t, "process.context_switches", resourceMetrics)
	assert.Equal(t, "process.context_switches", contextSwitchMetric.Name())

	if startTime != 0 {
		internal.AssertSumMetricStartTimeEquals(t, contextSwitchMetric, startTime)
	}

	internal.AssertSumMetricHasAttributeValue(t, contextSwitchMetric, 0, "type",
		pcommon.NewValueStr(metadata.AttributeContextSwitchTypeInvoluntary.String()))
	internal.AssertSumMetricHasAttributeValue(t, contextSwitchMetric, 1, "type",
		pcommon.NewValueStr(metadata.AttributeContextSwitchTypeVoluntary.String()))
}

func assertOpenFileDescriptorMetricValid(t *testing.T, resourceMetrics pmetric.ResourceMetricsSlice, startTime pcommon.Timestamp) {
	openFileDescriptorsMetric := getMetric(t, "process.open_file_descriptors", resourceMetrics)
	assert.Equal(t, "process.open_file_descriptors", openFileDescriptorsMetric.Name())

	if startTime != 0 {
		internal.AssertSumMetricStartTimeEquals(t, openFileDescriptorsMetric, startTime)
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

	_, err := newProcessScraper(receivertest.NewNopCreateSettings(), &Config{Include: MatchConfig{Names: []string{"test"}}, MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()})
	require.Error(t, err)
	require.Regexp(t, "^error creating process include filters:", err.Error())

	_, err = newProcessScraper(receivertest.NewNopCreateSettings(), &Config{Exclude: MatchConfig{Names: []string{"test"}}, MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()})
	require.Error(t, err)
	require.Regexp(t, "^error creating process exclude filters:", err.Error())
}

func TestScrapeMetrics_GetProcessesError(t *testing.T) {
	skipTestOnUnsupportedOS(t)

	scraper, err := newProcessScraper(receivertest.NewNopCreateSettings(), &Config{MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()})
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

func (p *processHandleMock) Percent(time.Duration) (float64, error) {
	args := p.MethodCalled("Percent")
	return args.Get(0).(float64), args.Error(1)
}

func (p *processHandleMock) MemoryInfo() (*process.MemoryInfoStat, error) {
	args := p.MethodCalled("MemoryInfo")
	return args.Get(0).(*process.MemoryInfoStat), args.Error(1)
}

func (p *processHandleMock) MemoryPercent() (float32, error) {
	args := p.MethodCalled("MemoryPercent")
	return args.Get(0).(float32), args.Error(1)
}

func (p *processHandleMock) IOCounters() (*process.IOCountersStat, error) {
	args := p.MethodCalled("IOCounters")
	return args.Get(0).(*process.IOCountersStat), args.Error(1)
}

func (p *processHandleMock) NumThreads() (int32, error) {
	args := p.MethodCalled("NumThreads")
	return args.Get(0).(int32), args.Error(1)
}

func (p *processHandleMock) CreateTime() (int64, error) {
	args := p.MethodCalled("CreateTime")
	return args.Get(0).(int64), args.Error(1)
}

func (p *processHandleMock) Parent() (*process.Process, error) {
	args := p.MethodCalled("Parent")
	return args.Get(0).(*process.Process), args.Error(1)
}

func (p *processHandleMock) PageFaults() (*process.PageFaultsStat, error) {
	args := p.MethodCalled("PageFaults")
	return args.Get(0).(*process.PageFaultsStat), args.Error(1)
}

func (p *processHandleMock) NumCtxSwitches() (*process.NumCtxSwitchesStat, error) {
	args := p.MethodCalled("NumCtxSwitches")
	return args.Get(0).(*process.NumCtxSwitchesStat), args.Error(1)
}

func (p *processHandleMock) NumFDs() (int32, error) {
	args := p.MethodCalled("NumFDs")
	return args.Get(0).(int32), args.Error(1)
}

func (p *processHandleMock) RlimitUsage(gatherUsed bool) ([]process.RlimitStat, error) {
	args := p.MethodCalled("RlimitUsage")
	return args.Get(0).([]process.RlimitStat), args.Error(1)
}

func newDefaultHandleMock() *processHandleMock {
	handleMock := &processHandleMock{}
	handleMock.On("Username").Return("username", nil)
	handleMock.On("Cmdline").Return("cmdline", nil)
	handleMock.On("CmdlineSlice").Return([]string{"cmdline"}, nil)
	handleMock.On("Times").Return(&cpu.TimesStat{}, nil)
	handleMock.On("Percent").Return(float64(0), nil)
	handleMock.On("MemoryInfo").Return(&process.MemoryInfoStat{}, nil)
	handleMock.On("MemoryPercent").Return(float32(0), nil)
	handleMock.On("IOCounters").Return(&process.IOCountersStat{}, nil)
	handleMock.On("Parent").Return(&process.Process{Pid: 2}, nil)
	handleMock.On("NumThreads").Return(int32(0), nil)
	handleMock.On("PageFaults").Return(&process.PageFaultsStat{}, nil)
	handleMock.On("NumCtxSwitches").Return(&process.NumCtxSwitchesStat{}, nil)
	handleMock.On("NumFDs").Return(int32(0), nil)
	handleMock.On("RlimitUsage").Return([]process.RlimitStat{}, nil)
	return handleMock
}

func TestScrapeMetrics_Filtered(t *testing.T) {
	skipTestOnUnsupportedOS(t)

	type testCase struct {
		name               string
		names              []string
		include            []string
		exclude            []string
		upTimeMs           []int64
		scrapeProcessDelay string
		expectedNames      []string
	}

	testCases := []testCase{
		{
			name:               "No Filter",
			names:              []string{"test1", "test2"},
			include:            []string{"test*"},
			upTimeMs:           []int64{5000, 5000},
			scrapeProcessDelay: "0s",
			expectedNames:      []string{"test1", "test2"},
		},
		{
			name:               "Include All",
			names:              []string{"test1", "test2"},
			include:            []string{"test*"},
			upTimeMs:           []int64{5000, 5000},
			scrapeProcessDelay: "0s",
			expectedNames:      []string{"test1", "test2"},
		},
		{
			name:               "Include One",
			names:              []string{"test1", "test2"},
			include:            []string{"test1"},
			upTimeMs:           []int64{5000, 5000},
			scrapeProcessDelay: "0s",
			expectedNames:      []string{"test1"},
		},
		{
			name:               "Exclude All",
			names:              []string{"test1", "test2"},
			exclude:            []string{"test*"},
			upTimeMs:           []int64{5000, 5000},
			scrapeProcessDelay: "0s",
			expectedNames:      []string{},
		},
		{
			name:               "Include & Exclude",
			names:              []string{"test1", "test2"},
			include:            []string{"test*"},
			exclude:            []string{"test2"},
			upTimeMs:           []int64{5000, 5000},
			scrapeProcessDelay: "0s",
			expectedNames:      []string{"test1"},
		},
		{
			name:               "Scrape Process Delay Keep One",
			names:              []string{"test1", "test2"},
			include:            []string{"test*"},
			upTimeMs:           []int64{5000, 50000},
			scrapeProcessDelay: "10s",
			expectedNames:      []string{"test2"},
		},
		{
			name:               "Scrape Process Delay Keep Both",
			names:              []string{"test1", "test2"},
			include:            []string{"test*"},
			upTimeMs:           []int64{50000, 50000},
			scrapeProcessDelay: "10s",
			expectedNames:      []string{"test1", "test2"},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scrapeProcessDelay, _ := time.ParseDuration(test.scrapeProcessDelay)
			metricsBuilderConfig := metadata.DefaultMetricsBuilderConfig()
			enableLinuxOnlyMetrics(&metricsBuilderConfig.Metrics)

			config := &Config{
				MetricsBuilderConfig: metricsBuilderConfig,
				ScrapeProcessDelay:   scrapeProcessDelay,
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

			scraper, err := newProcessScraper(receivertest.NewNopCreateSettings(), config)
			require.NoError(t, err, "Failed to create process scraper: %v", err)
			err = scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize process scraper: %v", err)

			handles := make([]*processHandleMock, 0, len(test.names))
			for i, name := range test.names {
				handleMock := newDefaultHandleMock()
				handleMock.On("Name").Return(name, nil)
				handleMock.On("Exe").Return(name, nil)
				handleMock.On("CreateTime").Return(time.Now().UnixMilli()-test.upTimeMs[i], nil)
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
				assert.Equal(t, expectedName, name.Str())
			}
		})
	}
}

func enableOptionalMetrics(ms *metadata.MetricsConfig) {
	ms.ProcessMemoryUtilization.Enabled = true
	ms.ProcessThreads.Enabled = true
	ms.ProcessPagingFaults.Enabled = true
	ms.ProcessContextSwitches.Enabled = true
	ms.ProcessOpenFileDescriptors.Enabled = true
	ms.ProcessSignalsPending.Enabled = true
}

func TestScrapeMetrics_ProcessErrors(t *testing.T) {
	skipTestOnUnsupportedOS(t)

	type testCase struct {
		name                string
		osFilter            string
		nameError           error
		exeError            error
		usernameError       error
		cmdlineError        error
		timesError          error
		memoryInfoError     error
		memoryPercentError  error
		ioCountersError     error
		createTimeError     error
		parentPidError      error
		pageFaultsError     error
		numThreadsError     error
		numCtxSwitchesError error
		numFDsError         error
		rlimitError         error
		expectedError       string
	}

	testCases := []testCase{
		{
			name:          "Name Error",
			osFilter:      "windows",
			nameError:     errors.New("err1"),
			expectedError: `error reading process name for pid 1: err1`,
		},
		{
			name:     "Exe Error",
			osFilter: "darwin",
			exeError: errors.New("err1"),
			expectedError: func() string {
				if runtime.GOOS == "windows" {
					return `error reading process executable for pid 1: err1; ` +
						`error reading process name for pid 1: executable path is empty`
				}
				return `error reading process executable for pid 1: err1`
			}(),
		},
		{
			name:          "Cmdline Error",
			osFilter:      "darwin",
			cmdlineError:  errors.New("err2"),
			expectedError: `error reading command for process "test" (pid 1): err2`,
		},
		{
			name:          "Username Error",
			usernameError: errors.New("err3"),
			expectedError: `error reading username for process "test" (pid 1): err3`,
		},
		{
			name:            "Create Time Error",
			createTimeError: errors.New("err4"),
			expectedError:   `error reading create time for process "test" (pid 1): err4`,
		},
		{
			name:          "Times Error",
			timesError:    errors.New("err5"),
			expectedError: `error reading cpu times for process "test" (pid 1): err5`,
		},
		{
			name:            "Memory Info Error",
			memoryInfoError: errors.New("err6"),
			expectedError:   `error reading memory info for process "test" (pid 1): err6`,
		},
		{
			name:               "Memory Percent Error",
			osFilter:           "darwin",
			memoryPercentError: errors.New("err-mem-percent"),
			expectedError:      `error reading memory utilization for process "test" (pid 1): err-mem-percent`,
		},
		{
			name:            "IO Counters Error",
			osFilter:        "darwin",
			ioCountersError: errors.New("err7"),
			expectedError:   `error reading disk usage for process "test" (pid 1): err7`,
		},
		{
			name:           "Parent PID Error",
			osFilter:       "darwin",
			parentPidError: errors.New("err8"),
			expectedError:  `error reading parent pid for process "test" (pid 1): err8`,
		},
		{
			name:            "Page Faults Error",
			osFilter:        "darwin",
			pageFaultsError: errors.New("err-paging"),
			expectedError:   `error reading memory paging info for process "test" (pid 1): err-paging`,
		},
		{
			name:            "Thread count Error",
			osFilter:        "darwin",
			numThreadsError: errors.New("err8"),
			expectedError:   `error reading thread info for process "test" (pid 1): err8`,
		},
		{
			name:                "Context Switches Error",
			osFilter:            "darwin",
			numCtxSwitchesError: errors.New("err9"),
			expectedError:       `error reading context switch counts for process "test" (pid 1): err9`,
		},
		{
			name:          "File Descriptors Error",
			osFilter:      "darwin",
			numFDsError:   errors.New("err10"),
			expectedError: `error reading open file descriptor count for process "test" (pid 1): err10`,
		},
		{
			name:          "Signals Pending Error",
			osFilter:      "darwin",
			rlimitError:   errors.New("err-rlimit"),
			expectedError: `error reading pending signals for process "test" (pid 1): err-rlimit`,
		},
		{
			name:                "Multiple Errors",
			osFilter:            "darwin",
			cmdlineError:        errors.New("err2"),
			usernameError:       errors.New("err3"),
			createTimeError:     errors.New("err4"),
			timesError:          errors.New("err5"),
			memoryInfoError:     errors.New("err6"),
			memoryPercentError:  errors.New("err-mem-percent"),
			ioCountersError:     errors.New("err7"),
			pageFaultsError:     errors.New("err-paging"),
			numThreadsError:     errors.New("err8"),
			numCtxSwitchesError: errors.New("err9"),
			numFDsError:         errors.New("err10"),
			rlimitError:         errors.New("err-rlimit"),
			expectedError: `error reading command for process "test" (pid 1): err2; ` +
				`error reading username for process "test" (pid 1): err3; ` +
				`error reading create time for process "test" (pid 1): err4; ` +
				`error reading cpu times for process "test" (pid 1): err5; ` +
				`error reading memory info for process "test" (pid 1): err6; ` +
				`error reading memory utilization for process "test" (pid 1): err-mem-percent; ` +
				`error reading disk usage for process "test" (pid 1): err7; ` +
				`error reading memory paging info for process "test" (pid 1): err-paging; ` +
				`error reading thread info for process "test" (pid 1): err8; ` +
				`error reading context switch counts for process "test" (pid 1): err9; ` +
				`error reading open file descriptor count for process "test" (pid 1): err10; ` +
				`error reading pending signals for process "test" (pid 1): err-rlimit`,
		},
	}

	if runtime.GOOS == "darwin" {
		darwinTestCase := testCase{
			name:          "Darwin Cmdline Error",
			cmdlineError:  errors.New("err2"),
			expectedError: `error reading process executable for pid 1: err2; error reading command for process "test" (pid 1): err2`,
		}
		testCases = append(testCases, darwinTestCase)
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			if test.osFilter == runtime.GOOS {
				t.Skipf("skipping test %v on %v", test.name, runtime.GOOS)
			}

			metricsBuilderConfig := metadata.DefaultMetricsBuilderConfig()
			if runtime.GOOS != "darwin" {
				enableOptionalMetrics(&metricsBuilderConfig.Metrics)
			} else {
				// disable darwin unsupported default metric
				metricsBuilderConfig.Metrics.ProcessDiskIo.Enabled = false
			}

			scraper, err := newProcessScraper(receivertest.NewNopCreateSettings(), &Config{MetricsBuilderConfig: metricsBuilderConfig})
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
			handleMock.On("Percent").Return(float64(0), nil)
			handleMock.On("MemoryInfo").Return(&process.MemoryInfoStat{}, test.memoryInfoError)
			handleMock.On("MemoryPercent").Return(float32(0), test.memoryPercentError)
			handleMock.On("IOCounters").Return(&process.IOCountersStat{}, test.ioCountersError)
			handleMock.On("CreateTime").Return(int64(0), test.createTimeError)
			handleMock.On("Parent").Return(&process.Process{Pid: 2}, test.parentPidError)
			handleMock.On("NumThreads").Return(int32(0), test.numThreadsError)
			handleMock.On("PageFaults").Return(&process.PageFaultsStat{}, test.pageFaultsError)
			handleMock.On("NumCtxSwitches").Return(&process.NumCtxSwitchesStat{}, test.numCtxSwitchesError)
			handleMock.On("NumFDs").Return(int32(0), test.numFDsError)
			handleMock.On("RlimitUsage").Return([]process.RlimitStat{
				{
					Resource: process.RLIMIT_SIGPENDING,
					Used:     0,
				},
			}, test.rlimitError)

			scraper.getProcessHandles = func() (processHandles, error) {
				return &processHandlesMock{handles: []*processHandleMock{handleMock}}, nil
			}

			md, err := scraper.scrape(context.Background())

			// In darwin we get the exe path with Cmdline()
			executableError := test.exeError
			if runtime.GOOS == "darwin" {
				executableError = test.cmdlineError
			}

			expectedResourceMetricsLen, expectedMetricsLen := getExpectedLengthOfReturnedMetrics(test.nameError, executableError, test.timesError, test.memoryInfoError, test.memoryPercentError, test.ioCountersError, test.pageFaultsError, test.numThreadsError, test.numCtxSwitchesError, test.numFDsError, test.rlimitError)
			assert.Equal(t, expectedResourceMetricsLen, md.ResourceMetrics().Len())
			assert.Equal(t, expectedMetricsLen, md.MetricCount())

			assert.EqualError(t, err, test.expectedError)
			isPartial := scrapererror.IsPartialScrapeError(err)
			assert.True(t, isPartial)
			if isPartial {
				expectedFailures := getExpectedScrapeFailures(test.nameError, executableError, test.timesError, test.memoryInfoError, test.memoryPercentError, test.ioCountersError, test.pageFaultsError, test.numThreadsError, test.numCtxSwitchesError, test.numFDsError, test.rlimitError)
				var scraperErr scrapererror.PartialScrapeError
				require.ErrorAs(t, err, &scraperErr)
				assert.Equal(t, expectedFailures, scraperErr.Failed)
			}
		})
	}
}

func getExpectedLengthOfReturnedMetrics(nameError, exeError, timeError, memError, memPercentError, diskError, pageFaultsError, threadError, contextSwitchError, fileDescriptorError, rlimitError error) (int, int) {
	if runtime.GOOS == "windows" && exeError != nil {
		return 0, 0
	}

	if nameError != nil {
		return 0, 0
	}

	expectedLen := 0
	if timeError == nil {
		expectedLen += cpuMetricsLen
	}
	if memError == nil {
		expectedLen += memoryMetricsLen
	}
	if memPercentError == nil && runtime.GOOS != "darwin" {
		expectedLen += memoryUtilizationMetricsLen
	}
	if diskError == nil && runtime.GOOS != "darwin" {
		expectedLen += diskMetricsLen
	}
	if pageFaultsError == nil && runtime.GOOS != "darwin" {
		expectedLen += pagingMetricsLen
	}
	if rlimitError == nil && runtime.GOOS != "darwin" {
		expectedLen += signalMetricsLen
	}
	if threadError == nil && runtime.GOOS != "darwin" {
		expectedLen += threadMetricsLen
	}
	if contextSwitchError == nil && runtime.GOOS != "darwin" {
		expectedLen += contextSwitchMetricsLen
	}
	if fileDescriptorError == nil && runtime.GOOS != "darwin" {
		expectedLen += fileDescriptorMetricsLen
	}

	if expectedLen == 0 {
		return 0, 0
	}
	return 1, expectedLen
}

func getExpectedScrapeFailures(nameError, exeError, timeError, memError, memPercentError, diskError, pageFaultsError, threadError, contextSwitchError, fileDescriptorError error, rlimitError error) int {
	if runtime.GOOS == "windows" && exeError != nil {
		return 2
	}

	if nameError != nil || exeError != nil {
		return 1
	}
	_, expectedMetricsLen := getExpectedLengthOfReturnedMetrics(nameError, exeError, timeError, memError, memPercentError, diskError, pageFaultsError, threadError, contextSwitchError, fileDescriptorError, rlimitError)

	// excluding unsupported metrics from darwin 'metricsLen'
	if runtime.GOOS == "darwin" {
		darwinMetricsLen := cpuMetricsLen + memoryMetricsLen
		return darwinMetricsLen - expectedMetricsLen
	}

	return metricsLen - expectedMetricsLen
}

func TestScrapeMetrics_MuteErrorFlags(t *testing.T) {
	skipTestOnUnsupportedOS(t)

	processNameError := errors.New("err1")
	processEmptyExeError := errors.New("executable path is empty")

	type testCase struct {
		name                 string
		muteProcessNameError bool
		muteProcessExeError  bool
		muteProcessIOError   bool
		omitConfigField      bool
		expectedError        string
	}

	testCases := []testCase{
		{
			name:                 "Process Name Error Muted And Process Exe Error Muted And Process IO Error Muted",
			muteProcessNameError: true,
			muteProcessExeError:  true,
			muteProcessIOError:   true,
		},
		{
			name:                 "Process Name Error Muted And Process Exe Error Enabled And Process IO Error Muted",
			muteProcessNameError: true,
			muteProcessExeError:  false,
			muteProcessIOError:   true,
			expectedError:        fmt.Sprintf("error reading process executable for pid 1: %v", processNameError),
		},
		{
			name:                 "Process Name Error Enabled And Process Exe Error Muted And Process IO Error Muted",
			muteProcessNameError: false,
			muteProcessExeError:  true,
			muteProcessIOError:   true,
			expectedError: func() string {
				if runtime.GOOS == "windows" {
					return fmt.Sprintf("error reading process name for pid 1: %v", processEmptyExeError)
				}
				return fmt.Sprintf("error reading process name for pid 1: %v", processNameError)
			}(),
		},
		{
			name:                 "Process Name Error Enabled And Process Exe Error Enabled And Process IO Error Muted",
			muteProcessNameError: false,
			muteProcessExeError:  false,
			muteProcessIOError:   true,
			expectedError: func() string {
				if runtime.GOOS == "windows" {
					return fmt.Sprintf("error reading process executable for pid 1: %v; ", processNameError) +
						fmt.Sprintf("error reading process name for pid 1: %v", processEmptyExeError)
				}
				return fmt.Sprintf("error reading process executable for pid 1: %v; ", processNameError) +
					fmt.Sprintf("error reading process name for pid 1: %v", processNameError)
			}(),
		},
		{
			name:            "Process Name Error Default (Enabled) And Process Exe Error Default (Enabled) And Process IO Error Default (Enabled)",
			omitConfigField: true,
			expectedError: func() string {
				if runtime.GOOS == "windows" {
					return fmt.Sprintf("error reading process executable for pid 1: %v; ", processNameError) +
						fmt.Sprintf("error reading process name for pid 1: %v", processEmptyExeError)
				}
				return fmt.Sprintf("error reading process executable for pid 1: %v; ", processNameError) +
					fmt.Sprintf("error reading process name for pid 1: %v", processNameError)
			}(),
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			config := &Config{MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()}
			if !test.omitConfigField {
				config.MuteProcessNameError = test.muteProcessNameError
				config.MuteProcessExeError = test.muteProcessExeError
				config.MuteProcessIOError = test.muteProcessIOError
			}
			scraper, err := newProcessScraper(receivertest.NewNopCreateSettings(), config)
			require.NoError(t, err, "Failed to create process scraper: %v", err)
			err = scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize process scraper: %v", err)

			handleMock := &processHandleMock{}
			handleMock.On("Name").Return("test", processNameError)
			handleMock.On("Exe").Return("test", processNameError)
			handleMock.On("Cmdline").Return("test", processNameError)

			if config.MuteProcessIOError {
				handleMock.On("IOCounters").Return("test", errors.New("permission denied"))
			}

			scraper.getProcessHandles = func() (processHandles, error) {
				return &processHandlesMock{handles: []*processHandleMock{handleMock}}, nil
			}
			md, err := scraper.scrape(context.Background())

			assert.Zero(t, md.MetricCount())

			if config.MuteProcessNameError && config.MuteProcessExeError {
				assert.Nil(t, err)
			} else {
				assert.EqualError(t, err, test.expectedError)
			}
		})
	}
}

type ProcessReadError struct{}

func (m *ProcessReadError) Error() string {
	return "unable to read data"
}

func newErroringHandleMock() *processHandleMock {
	handleMock := &processHandleMock{}
	handleMock.On("Username").Return("username", nil)
	handleMock.On("Cmdline").Return("cmdline", nil)
	handleMock.On("CmdlineSlice").Return([]string{"cmdline"}, nil)
	handleMock.On("Times").Return(&cpu.TimesStat{}, &ProcessReadError{})
	handleMock.On("Percent").Return(float64(0), nil)
	handleMock.On("MemoryInfo").Return(&process.MemoryInfoStat{}, &ProcessReadError{})
	handleMock.On("IOCounters").Return(&process.IOCountersStat{}, &ProcessReadError{})
	handleMock.On("NumThreads").Return(int32(0), &ProcessReadError{})
	handleMock.On("NumCtxSwitches").Return(&process.NumCtxSwitchesStat{}, &ProcessReadError{})
	handleMock.On("NumFDs").Return(int32(0), &ProcessReadError{})
	return handleMock
}

func TestScrapeMetrics_DontCheckDisabledMetrics(t *testing.T) {
	skipTestOnUnsupportedOS(t)

	metricsBuilderConfig := metadata.DefaultMetricsBuilderConfig()

	metricsBuilderConfig.Metrics.ProcessCPUTime.Enabled = false
	metricsBuilderConfig.Metrics.ProcessDiskIo.Enabled = false
	metricsBuilderConfig.Metrics.ProcessDiskOperations.Enabled = false
	metricsBuilderConfig.Metrics.ProcessMemoryUsage.Enabled = false
	metricsBuilderConfig.Metrics.ProcessMemoryVirtual.Enabled = false

	t.Run("Metrics don't log errors when disabled", func(t *testing.T) {
		config := &Config{MetricsBuilderConfig: metricsBuilderConfig}

		scraper, err := newProcessScraper(receivertest.NewNopCreateSettings(), config)
		require.NoError(t, err, "Failed to create process scraper: %v", err)
		err = scraper.start(context.Background(), componenttest.NewNopHost())
		require.NoError(t, err, "Failed to initialize process scraper: %v", err)

		handleMock := newErroringHandleMock()
		handleMock.On("Name").Return("test", nil)
		handleMock.On("Exe").Return("test", nil)
		handleMock.On("CreateTime").Return(time.Now().UnixMilli(), nil)
		handleMock.On("Parent").Return(&process.Process{Pid: 2}, nil)

		scraper.getProcessHandles = func() (processHandles, error) {
			return &processHandlesMock{handles: []*processHandleMock{handleMock}}, nil
		}
		md, err := scraper.scrape(context.Background())

		assert.Zero(t, md.MetricCount())
		assert.Nil(t, err)
	})
}
