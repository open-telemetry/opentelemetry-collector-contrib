// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processscraper

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/common"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tilinna/clock"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/collector/scraper/scrapertest"
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
		mutateScraper       func(*processScraper)
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
			mutateMetricsConfig: func(_ *testing.T, ms *metadata.MetricsConfig) {
				ms.ProcessMemoryUtilization.Enabled = true
			},
		},
		{
			name: "Enable uptime",
			mutateMetricsConfig: func(_ *testing.T, ms *metadata.MetricsConfig) {
				ms.ProcessUptime.Enabled = true
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
			cfg := &Config{MetricsBuilderConfig: metricsBuilderConfig}
			envMap := common.EnvMap{
				common.HostProcEnvKey:    "/proc",
				common.HostSysEnvKey:     "/sys",
				common.HostEtcEnvKey:     "/etc",
				common.HostVarEnvKey:     "/var",
				common.HostRunEnvKey:     "/run",
				common.HostDevEnvKey:     "/dev",
				common.HostProcMountinfo: "",
			}
			ctx := context.WithValue(context.Background(), common.EnvKey, envMap)
			ctx = clock.Context(ctx, clock.NewMock(time.Unix(200, 0)))
			scraper, err := newProcessScraper(scrapertest.NewNopSettings(metadata.Type), cfg)
			if test.mutateScraper != nil {
				test.mutateScraper(scraper)
			}
			scraper.getProcessCreateTime = func(processHandle, context.Context) (int64, error) { return createTime, nil }
			require.NoError(t, err, "Failed to create process scraper: %v", err)
			err = scraper.start(ctx, componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize process scraper: %v", err)

			md, err := scraper.scrape(ctx)
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
				require.Positivef(t, noProcessesErrored, "Failed to scrape metrics - : error, but 0 failed process %v", err)
				require.Positivef(t, noProcessesScraped, "Failed to scrape metrics - : 0 successful scrapes %v", err)
			}

			require.Greater(t, md.ResourceMetrics().Len(), 1)
			assertValidProcessResourceAttributes(t, md.ResourceMetrics())
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
			if metricsBuilderConfig.Metrics.ProcessUptime.Enabled {
				assertUptimeMetricValid(t, md.ResourceMetrics())
			} else {
				assertMetricMissing(t, md.ResourceMetrics(), "process.uptime")
			}
			assertSameTimeStampForAllMetricsWithinResource(t, md.ResourceMetrics())
		})
	}
}

// assertValidProcessResourceAttributes will assert that each process resource contains at least
// the most important identifying attribute (the PID attribute) and only contains permissible
// resource attributes defined by the process scraper metadata.yaml/Process Semantic Conventions.
func assertValidProcessResourceAttributes(t *testing.T, resourceMetrics pmetric.ResourceMetricsSlice) {
	requiredResourceAttributes := []string{
		conventions.AttributeProcessPID,
	}
	permissibleResourceAttributes := []string{
		conventions.AttributeProcessPID,
		conventions.AttributeProcessExecutableName,
		conventions.AttributeProcessExecutablePath,
		conventions.AttributeProcessCommand,
		conventions.AttributeProcessCommandLine,
		conventions.AttributeProcessOwner,
		"process.parent_pid", // TODO: use this from conventions when it is available
	}
	for i := 0; i < resourceMetrics.Len(); i++ {
		attrs := resourceMetrics.At(i).Resource().Attributes().AsRaw()
		for _, attr := range requiredResourceAttributes {
			_, ok := attrs[attr]
			require.True(t, ok, "resource %s missing required attribute %s", attrs, attr)
		}
		for attr := range attrs {
			require.Contains(t, permissibleResourceAttributes, attr, "resource %s contained unknown attribute %s", attrs, attr)
		}
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
	startTime pcommon.Timestamp,
) {
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

func assertUptimeMetricValid(t *testing.T, resourceMetrics pmetric.ResourceMetricsSlice) {
	m := getMetric(t, "process.uptime", resourceMetrics)
	assert.Equal(t, "process.uptime", m.Name())

	for i := 0; i < m.Gauge().DataPoints().Len(); i++ {
		dp := m.Gauge().DataPoints().At(i)
		assert.Equal(t, float64(199.9), dp.DoubleValue(), "Must have an uptime of 199s")
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

	_, err := newProcessScraper(scrapertest.NewNopSettings(metadata.Type), &Config{Include: MatchConfig{Names: []string{"test"}}, MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()})
	require.Error(t, err)
	require.Regexp(t, "^error creating process include filters:", err.Error())

	_, err = newProcessScraper(scrapertest.NewNopSettings(metadata.Type), &Config{Exclude: MatchConfig{Names: []string{"test"}}, MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()})
	require.Error(t, err)
	require.Regexp(t, "^error creating process exclude filters:", err.Error())
}

func TestScrapeMetrics_GetProcessesError(t *testing.T) {
	skipTestOnUnsupportedOS(t)

	scraper, err := newProcessScraper(scrapertest.NewNopSettings(metadata.Type), &Config{MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()})
	require.NoError(t, err, "Failed to create process scraper: %v", err)

	scraper.getProcessHandles = func(context.Context) (processHandles, error) { return nil, errors.New("err1") }

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

func (p *processHandleMock) CgroupWithContext(ctx context.Context) (string, error) {
	args := p.MethodCalled("CgroupWithContext", ctx)
	return args.String(0), args.Error(1)
}

func (p *processHandleMock) NameWithContext(ctx context.Context) (ret string, err error) {
	args := p.MethodCalled("NameWithContext", ctx)
	return args.String(0), args.Error(1)
}

func (p *processHandleMock) ExeWithContext(ctx context.Context) (string, error) {
	args := p.MethodCalled("ExeWithContext", ctx)
	return args.String(0), args.Error(1)
}

func (p *processHandleMock) UsernameWithContext(ctx context.Context) (string, error) {
	args := p.MethodCalled("UsernameWithContext", ctx)
	return args.String(0), args.Error(1)
}

func (p *processHandleMock) CmdlineWithContext(ctx context.Context) (string, error) {
	args := p.MethodCalled("CmdlineWithContext", ctx)
	return args.String(0), args.Error(1)
}

func (p *processHandleMock) CmdlineSliceWithContext(ctx context.Context) ([]string, error) {
	args := p.MethodCalled("CmdlineSliceWithContext", ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (p *processHandleMock) TimesWithContext(ctx context.Context) (*cpu.TimesStat, error) {
	args := p.MethodCalled("TimesWithContext", ctx)
	return args.Get(0).(*cpu.TimesStat), args.Error(1)
}

func (p *processHandleMock) MemoryInfoWithContext(ctx context.Context) (*process.MemoryInfoStat, error) {
	args := p.MethodCalled("MemoryInfoWithContext", ctx)
	return args.Get(0).(*process.MemoryInfoStat), args.Error(1)
}

func (p *processHandleMock) MemoryPercentWithContext(ctx context.Context) (float32, error) {
	args := p.MethodCalled("MemoryPercentWithContext", ctx)
	return args.Get(0).(float32), args.Error(1)
}

func (p *processHandleMock) IOCountersWithContext(ctx context.Context) (*process.IOCountersStat, error) {
	args := p.MethodCalled("IOCountersWithContext", ctx)
	return args.Get(0).(*process.IOCountersStat), args.Error(1)
}

func (p *processHandleMock) NumThreadsWithContext(ctx context.Context) (int32, error) {
	args := p.MethodCalled("NumThreadsWithContext", ctx)
	return args.Get(0).(int32), args.Error(1)
}

func (p *processHandleMock) CreateTimeWithContext(ctx context.Context) (int64, error) {
	args := p.MethodCalled("CreateTimeWithContext", ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (p *processHandleMock) PpidWithContext(ctx context.Context) (int32, error) {
	args := p.MethodCalled("PpidWithContext", ctx)
	return args.Get(0).(int32), args.Error(1)
}

func (p *processHandleMock) PageFaultsWithContext(ctx context.Context) (*process.PageFaultsStat, error) {
	args := p.MethodCalled("PageFaultsWithContext", ctx)
	return args.Get(0).(*process.PageFaultsStat), args.Error(1)
}

func (p *processHandleMock) NumCtxSwitchesWithContext(ctx context.Context) (*process.NumCtxSwitchesStat, error) {
	args := p.MethodCalled("NumCtxSwitchesWithContext", ctx)
	return args.Get(0).(*process.NumCtxSwitchesStat), args.Error(1)
}

func (p *processHandleMock) NumFDsWithContext(ctx context.Context) (int32, error) {
	args := p.MethodCalled("NumFDsWithContext", ctx)
	return args.Get(0).(int32), args.Error(1)
}

func (p *processHandleMock) GetProcessHandleCountWithContext(ctx context.Context) (int64, error) {
	args := p.MethodCalled("GetProcessHandleCountWithContext", ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (p *processHandleMock) RlimitUsageWithContext(ctx context.Context, b bool) ([]process.RlimitStat, error) {
	args := p.MethodCalled("RlimitUsageWithContext", ctx, b)
	return args.Get(0).([]process.RlimitStat), args.Error(1)
}

func initDefaultsHandleMock(t mock.TestingT, handleMock *processHandleMock) {
	if !handleMock.IsMethodCallable(t, "UsernameWithContext", mock.Anything) {
		handleMock.On("UsernameWithContext", mock.Anything).Return("username", nil)
	}
	if !handleMock.IsMethodCallable(t, "NameWithContext", mock.Anything) {
		handleMock.On("NameWithContext", mock.Anything).Return("processname", nil)
	}
	if !handleMock.IsMethodCallable(t, "CmdlineWithContext", mock.Anything) {
		handleMock.On("CmdlineWithContext", mock.Anything).Return("cmdline", nil)
	}
	if !handleMock.IsMethodCallable(t, "CmdlineSliceWithContext", mock.Anything) {
		handleMock.On("CmdlineSliceWithContext", mock.Anything).Return([]string{"cmdline"}, nil)
	}
	if !handleMock.IsMethodCallable(t, "TimesWithContext", mock.Anything) {
		handleMock.On("TimesWithContext", mock.Anything).Return(&cpu.TimesStat{}, nil)
	}
	if !handleMock.IsMethodCallable(t, "MemoryInfoWithContext", mock.Anything) {
		handleMock.On("MemoryInfoWithContext", mock.Anything).Return(&process.MemoryInfoStat{}, nil)
	}
	if !handleMock.IsMethodCallable(t, "MemoryPercentWithContext", mock.Anything) {
		handleMock.On("MemoryPercentWithContext", mock.Anything).Return(float32(0), nil)
	}
	if !handleMock.IsMethodCallable(t, "IOCountersWithContext", mock.Anything) {
		handleMock.On("IOCountersWithContext", mock.Anything).Return(&process.IOCountersStat{}, nil)
	}
	if !handleMock.IsMethodCallable(t, "PpidWithContext", mock.Anything) {
		handleMock.On("PpidWithContext", mock.Anything).Return(int32(2), nil)
	}
	if !handleMock.IsMethodCallable(t, "NumThreadsWithContext", mock.Anything) {
		handleMock.On("NumThreadsWithContext", mock.Anything).Return(int32(0), nil)
	}
	if !handleMock.IsMethodCallable(t, "PageFaultsWithContext", mock.Anything) {
		handleMock.On("PageFaultsWithContext", mock.Anything).Return(&process.PageFaultsStat{}, nil)
	}
	if !handleMock.IsMethodCallable(t, "NumCtxSwitchesWithContext", mock.Anything) {
		handleMock.On("NumCtxSwitchesWithContext", mock.Anything).Return(&process.NumCtxSwitchesStat{}, nil)
	}
	if !handleMock.IsMethodCallable(t, "NumFDsWithContext", mock.Anything) {
		handleMock.On("NumFDsWithContext", mock.Anything).Return(int32(0), nil)
	}
	if !handleMock.IsMethodCallable(t, "GetProcessHandleCountWithContext", mock.Anything) {
		handleMock.On("GetProcessHandleCountWithContext", mock.Anything).Return(int64(0), nil)
	}
	if !handleMock.IsMethodCallable(t, "RlimitUsageWithContext", mock.Anything, mock.Anything) {
		handleMock.On("RlimitUsageWithContext", mock.Anything, mock.Anything).Return([]process.RlimitStat{}, nil)
	}
	if !handleMock.IsMethodCallable(t, "CreateTimeWithContext", mock.Anything) {
		handleMock.On("CreateTimeWithContext", mock.Anything).Return(time.Now().UnixMilli(), nil)
	}
	if !handleMock.IsMethodCallable(t, "ExeWithContext", mock.Anything) {
		handleMock.On("ExeWithContext", mock.Anything).Return("processname", nil)
	}
	if !handleMock.IsMethodCallable(t, "CgroupWithContext", mock.Anything) {
		handleMock.On("CgroupWithContext", mock.Anything).Return("cgroup", nil)
	}
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

			scraper, err := newProcessScraper(scrapertest.NewNopSettings(metadata.Type), config)
			require.NoError(t, err, "Failed to create process scraper: %v", err)
			err = scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize process scraper: %v", err)

			handles := make([]*processHandleMock, 0, len(test.names))
			for i, name := range test.names {
				handleMock := &processHandleMock{}
				handleMock.On("NameWithContext", mock.Anything).Return(name, nil)
				handleMock.On("ExeWithContext", mock.Anything).Return(name, nil)
				handleMock.On("CreateTimeWithContext", mock.Anything).Return(time.Now().UnixMilli()-test.upTimeMs[i], nil)
				initDefaultsHandleMock(t, handleMock)

				handles = append(handles, handleMock)
			}

			scraper.getProcessHandles = func(context.Context) (processHandles, error) {
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
	ms.ProcessUptime.Enabled = true
	if runtime.GOOS == "windows" {
		// Only Windows can produce this metric, do not fake it for other OSes.
		ms.ProcessHandles.Enabled = true
	}
}

func TestScrapeMetrics_ProcessErrors(t *testing.T) {
	skipTestOnUnsupportedOS(t)

	type testCase struct {
		name                string
		osFilter            []string
		nameError           error
		exeError            error
		cgroupError         error
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
		handleCountError    error
		rlimitError         error
		expectedError       string
	}

	testCases := []testCase{
		{
			name:          "Name Error",
			osFilter:      []string{"windows"},
			nameError:     errors.New("err1"),
			expectedError: `error reading process name for pid 1: err1`,
		},
		{
			name:     "Exe Error",
			osFilter: []string{"darwin"},
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
			name:        "Cgroup Error",
			osFilter:    []string{"darwin", "windows"},
			cgroupError: errors.New("err1"),
			expectedError: func() string {
				return `error reading process cgroup for pid 1: err1`
			}(),
		},
		{
			name:          "Cmdline Error",
			osFilter:      []string{"darwin"},
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
			expectedError:   `error reading create time for process "test" (pid 1): err4; error calculating uptime for process "test" (pid 1): err4`,
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
			osFilter:           []string{"darwin"},
			memoryPercentError: errors.New("err-mem-percent"),
			expectedError:      `error reading memory utilization for process "test" (pid 1): err-mem-percent`,
		},
		{
			name:            "IO Counters Error",
			osFilter:        []string{"darwin"},
			ioCountersError: errors.New("err7"),
			expectedError:   `error reading disk usage for process "test" (pid 1): err7`,
		},
		{
			name:           "Parent PID Error",
			osFilter:       []string{"darwin"},
			parentPidError: errors.New("err8"),
			expectedError:  `error reading parent pid for process "test" (pid 1): err8`,
		},
		{
			name:            "Page Faults Error",
			osFilter:        []string{"darwin"},
			pageFaultsError: errors.New("err-paging"),
			expectedError:   `error reading memory paging info for process "test" (pid 1): err-paging`,
		},
		{
			name:            "Thread count Error",
			osFilter:        []string{"darwin"},
			numThreadsError: errors.New("err8"),
			expectedError:   `error reading thread info for process "test" (pid 1): err8`,
		},
		{
			name:                "Context Switches Error",
			osFilter:            []string{"darwin"},
			numCtxSwitchesError: errors.New("err9"),
			expectedError:       `error reading context switch counts for process "test" (pid 1): err9`,
		},
		{
			name:          "File Descriptors Error",
			osFilter:      []string{"darwin"},
			numFDsError:   errors.New("err10"),
			expectedError: `error reading open file descriptor count for process "test" (pid 1): err10`,
		},
		{
			name:             "Handle Count Error",
			osFilter:         []string{"darwin", "linux"},
			handleCountError: errors.New("err-handle-count"),
			expectedError:    `error reading handle count for process "test" (pid 1): err-handle-count`,
		},
		{
			name:          "Signals Pending Error",
			osFilter:      []string{"darwin"},
			rlimitError:   errors.New("err-rlimit"),
			expectedError: `error reading pending signals for process "test" (pid 1): err-rlimit`,
		},
		{
			name:                "Multiple Errors",
			osFilter:            []string{"darwin"},
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
			handleCountError:    handleCountErrorIfSupportedOnPlatform(),
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
				handleCountErrorMessageIfSupportedOnPlatform() +
				`error reading pending signals for process "test" (pid 1): err-rlimit; ` +
				`error calculating uptime for process "test" (pid 1): err4`,
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
			for _, os := range test.osFilter {
				if os == runtime.GOOS {
					t.Skipf("skipping test %v on %v", test.name, runtime.GOOS)
				}
			}

			metricsBuilderConfig := metadata.DefaultMetricsBuilderConfig()
			if runtime.GOOS != "darwin" {
				enableOptionalMetrics(&metricsBuilderConfig.Metrics)
			} else {
				// disable darwin unsupported default metric
				metricsBuilderConfig.Metrics.ProcessUptime.Enabled = true
				metricsBuilderConfig.Metrics.ProcessDiskIo.Enabled = false
			}

			scraper, err := newProcessScraper(scrapertest.NewNopSettings(metadata.Type), &Config{MetricsBuilderConfig: metricsBuilderConfig})
			require.NoError(t, err, "Failed to create process scraper: %v", err)
			err = scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize process scraper: %v", err)

			username := "username"
			if test.usernameError != nil {
				username = ""
			}

			handleMock := &processHandleMock{}
			handleMock.On("NameWithContext", mock.Anything).Return("test", test.nameError)
			handleMock.On("ExeWithContext", mock.Anything).Return("test", test.exeError)
			handleMock.On("CgroupWithContext", mock.Anything).Return("test", test.cgroupError)
			handleMock.On("UsernameWithContext", mock.Anything).Return(username, test.usernameError)
			handleMock.On("CmdlineWithContext", mock.Anything).Return("cmdline", test.cmdlineError)
			handleMock.On("CmdlineSliceWithContext", mock.Anything).Return([]string{"cmdline"}, test.cmdlineError)
			handleMock.On("TimesWithContext", mock.Anything).Return(&cpu.TimesStat{}, test.timesError)
			handleMock.On("MemoryInfoWithContext", mock.Anything).Return(&process.MemoryInfoStat{}, test.memoryInfoError)
			handleMock.On("MemoryPercentWithContext", mock.Anything).Return(float32(0), test.memoryPercentError)
			handleMock.On("IOCountersWithContext", mock.Anything).Return(&process.IOCountersStat{}, test.ioCountersError)
			handleMock.On("CreateTimeWithContext", mock.Anything).Return(int64(0), test.createTimeError)
			handleMock.On("PpidWithContext", mock.Anything).Return(int32(2), test.parentPidError)
			handleMock.On("NumThreadsWithContext", mock.Anything).Return(int32(0), test.numThreadsError)
			handleMock.On("PageFaultsWithContext", mock.Anything).Return(&process.PageFaultsStat{}, test.pageFaultsError)
			handleMock.On("NumCtxSwitchesWithContext", mock.Anything).Return(&process.NumCtxSwitchesStat{}, test.numCtxSwitchesError)
			handleMock.On("NumFDsWithContext", mock.Anything).Return(int32(0), test.numFDsError)
			handleMock.On("GetProcessHandleCountWithContext", mock.Anything).Return(int64(0), test.handleCountError)
			handleMock.On("RlimitUsageWithContext", mock.Anything, mock.Anything).Return([]process.RlimitStat{
				{
					Resource: process.RLIMIT_SIGPENDING,
					Used:     0,
				},
			}, test.rlimitError)

			scraper.getProcessHandles = func(context.Context) (processHandles, error) {
				return &processHandlesMock{handles: []*processHandleMock{handleMock}}, nil
			}

			md, err := scraper.scrape(context.Background())

			// In darwin we get the exe path with Cmdline()
			executableError := test.exeError
			if runtime.GOOS == "darwin" {
				executableError = test.cmdlineError
			}

			expectedResourceMetricsLen, expectedMetricsLen := getExpectedLengthOfReturnedMetrics(test.nameError, executableError, test.timesError, test.memoryInfoError, test.memoryPercentError, test.ioCountersError, test.pageFaultsError, test.numThreadsError, test.numCtxSwitchesError, test.numFDsError, test.handleCountError, test.rlimitError, test.cgroupError, test.createTimeError)
			assert.Equal(t, expectedResourceMetricsLen, md.ResourceMetrics().Len())
			assert.Equal(t, expectedMetricsLen, md.MetricCount())

			assert.EqualError(t, err, test.expectedError)
			isPartial := scrapererror.IsPartialScrapeError(err)
			assert.True(t, isPartial)
			if isPartial {
				expectedFailures := getExpectedScrapeFailures(test.nameError, executableError, test.timesError, test.memoryInfoError, test.memoryPercentError, test.ioCountersError, test.pageFaultsError, test.numThreadsError, test.numCtxSwitchesError, test.numFDsError, test.handleCountError, test.rlimitError, test.cgroupError, test.createTimeError)
				var scraperErr scrapererror.PartialScrapeError
				require.ErrorAs(t, err, &scraperErr)
				assert.Equal(t, expectedFailures, scraperErr.Failed)
			}
		})
	}
}

func getExpectedLengthOfReturnedMetrics(nameError, exeError, timeError, memError, memPercentError, diskError, pageFaultsError, threadError, contextSwitchError, fileDescriptorError, handleCountError, rlimitError, cgroupError, uptimeError error) (int, int) {
	if runtime.GOOS == "windows" && exeError != nil {
		return 0, 0
	}
	if runtime.GOOS == "linux" && cgroupError != nil {
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
	if handleCountError == nil {
		expectedLen += handleCountMetricsLen
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
	if uptimeError == nil {
		expectedLen += uptimeMetricsLen
	}

	if expectedLen == 0 {
		return 0, 0
	}
	return 1, expectedLen
}

func getExpectedScrapeFailures(nameError, exeError, timeError, memError, memPercentError, diskError, pageFaultsError, threadError, contextSwitchError, fileDescriptorError error, handleCountError, rlimitError, cgroupError, uptimeError error) int {
	if runtime.GOOS == "windows" && exeError != nil {
		return 2
	}
	if runtime.GOOS == "linux" && cgroupError != nil {
		return 1
	}

	if nameError != nil || exeError != nil {
		return 1
	}
	_, expectedMetricsLen := getExpectedLengthOfReturnedMetrics(nameError, exeError, timeError, memError, memPercentError, diskError, pageFaultsError, threadError, contextSwitchError, fileDescriptorError, handleCountError, rlimitError, cgroupError, uptimeError)

	// excluding unsupported metrics from darwin 'metricsLen'
	if runtime.GOOS == "darwin" {
		darwinMetricsLen := cpuMetricsLen + memoryMetricsLen + uptimeMetricsLen
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
		skipTestCase         bool
		muteProcessNameError bool
		muteProcessExeError  bool
		muteProcessIOError   bool
		muteProcessUserError bool
		muteProcessAllErrors bool
		skipProcessNameError bool
		omitConfigField      bool
		expectedError        string
		expectedCount        int
	}

	testCases := []testCase{
		{
			name:                 "Process Name Error Muted And Process Exe Error Muted And Process IO Error Muted",
			muteProcessNameError: true,
			muteProcessExeError:  true,
			muteProcessIOError:   true,
			muteProcessUserError: true,
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
		{
			name:                 "Process User Error Muted",
			muteProcessUserError: true,
			skipProcessNameError: true,
			muteProcessExeError:  true,
			muteProcessNameError: true,
			expectedCount: func() int {
				if runtime.GOOS == "darwin" {
					// disk.io is not collected on darwin
					return 3
				}
				return 4
			}(),
		},
		{
			name:                 "Process User Error Unmuted",
			muteProcessUserError: false,
			skipProcessNameError: true,
			muteProcessExeError:  true,
			muteProcessNameError: true,
			expectedError: func() string {
				return fmt.Sprintf("error reading username for process \"processname\" (pid 1): %v", processNameError)
			}(),
			expectedCount: func() int {
				if runtime.GOOS == "darwin" {
					// disk.io is not collected on darwin
					return 3
				}
				return 4
			}(),
		},
		{
			name:                 "All Process Errors Muted",
			muteProcessNameError: false,
			muteProcessExeError:  false,
			muteProcessIOError:   false,
			muteProcessUserError: false,
			muteProcessAllErrors: true,
			expectedCount:        0,
		},
		{
			name:                 "Process User Error Enabled and All Process Errors Muted",
			muteProcessUserError: false,
			skipProcessNameError: true,
			muteProcessExeError:  true,
			muteProcessNameError: true,
			muteProcessAllErrors: true,
			expectedCount: func() int {
				if runtime.GOOS == "darwin" {
					// disk.io is not collected on darwin
					return 3
				}
				return 4
			}(),
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			if test.skipTestCase {
				t.Skipf("skipping test %v on %v", test.name, runtime.GOOS)
			}
			config := &Config{MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()}
			if !test.omitConfigField {
				config.MuteProcessNameError = test.muteProcessNameError
				config.MuteProcessExeError = test.muteProcessExeError
				config.MuteProcessIOError = test.muteProcessIOError
				config.MuteProcessUserError = test.muteProcessUserError
				config.MuteProcessAllErrors = test.muteProcessAllErrors
			}
			scraper, err := newProcessScraper(scrapertest.NewNopSettings(metadata.Type), config)
			require.NoError(t, err, "Failed to create process scraper: %v", err)
			err = scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize process scraper: %v", err)

			handleMock := &processHandleMock{}
			if !test.skipProcessNameError {
				handleMock.On("NameWithContext", mock.Anything).Return("test", processNameError)
				handleMock.On("ExeWithContext", mock.Anything).Return("test", processNameError)
				handleMock.On("CmdlineWithContext", mock.Anything).Return("test", processNameError)
			} else {
				handleMock.On("UsernameWithContext", mock.Anything).Return("processname", processNameError)
				handleMock.On("NameWithContext", mock.Anything).Return("processname", nil)
				handleMock.On("CreateTimeWithContext", mock.Anything).Return(time.Now().UnixMilli(), nil)
			}
			initDefaultsHandleMock(t, handleMock)

			if config.MuteProcessIOError {
				handleMock.On("IOCountersWithContext", mock.Anything).Return("test", errors.New("permission denied"))
			}

			scraper.getProcessHandles = func(context.Context) (processHandles, error) {
				return &processHandlesMock{handles: []*processHandleMock{handleMock}}, nil
			}
			md, err := scraper.scrape(context.Background())

			assert.Equal(t, test.expectedCount, md.MetricCount())

			if (config.MuteProcessNameError && config.MuteProcessExeError && config.MuteProcessUserError) || config.MuteProcessAllErrors {
				assert.NoError(t, err)
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
	handleMock.On("UsernameWithContext", mock.Anything).Return("username", nil)
	handleMock.On("CmdlineWithContext", mock.Anything).Return("cmdline", nil)
	handleMock.On("CmdlineSliceWithContext", mock.Anything).Return([]string{"cmdline"}, nil)
	handleMock.On("TimesWithContext", mock.Anything).Return(&cpu.TimesStat{}, &ProcessReadError{})
	handleMock.On("MemoryInfoWithContext", mock.Anything).Return(&process.MemoryInfoStat{}, &ProcessReadError{})
	handleMock.On("IOCountersWithContext", mock.Anything).Return(&process.IOCountersStat{}, &ProcessReadError{})
	handleMock.On("NumThreadsWithContext", mock.Anything).Return(int32(0), &ProcessReadError{})
	handleMock.On("NumCtxSwitchesWithContext", mock.Anything).Return(&process.NumCtxSwitchesStat{}, &ProcessReadError{})
	handleMock.On("NumFDsWithContext", mock.Anything).Return(int32(0), &ProcessReadError{})
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

		scraper, err := newProcessScraper(scrapertest.NewNopSettings(metadata.Type), config)
		require.NoError(t, err, "Failed to create process scraper: %v", err)
		err = scraper.start(context.Background(), componenttest.NewNopHost())
		require.NoError(t, err, "Failed to initialize process scraper: %v", err)

		handleMock := newErroringHandleMock()
		handleMock.On("CgroupWithContext", mock.Anything).Return("test", nil)
		handleMock.On("NameWithContext", mock.Anything).Return("test", nil)
		handleMock.On("ExeWithContext", mock.Anything).Return("test", nil)
		handleMock.On("CreateTimeWithContext", mock.Anything).Return(time.Now().UnixMilli(), nil)
		handleMock.On("PpidWithContext", mock.Anything).Return(int32(2), nil)

		scraper.getProcessHandles = func(context.Context) (processHandles, error) {
			return &processHandlesMock{handles: []*processHandleMock{handleMock}}, nil
		}
		md, err := scraper.scrape(context.Background())

		assert.Zero(t, md.MetricCount())
		assert.NoError(t, err)
	})
}

func TestScrapeMetrics_CpuUtilizationWhenCpuTimesIsDisabled(t *testing.T) {
	skipTestOnUnsupportedOS(t)

	testCases := []struct {
		name                  string
		processCPUTimes       bool
		processCPUUtilization bool
		expectedMetricCount   int
		expectedMetricNames   []string
	}{
		{
			name:                  "process.cpu.time enabled, process.cpu.utilization disabled",
			processCPUTimes:       true,
			processCPUUtilization: false,
			expectedMetricCount:   1,
			expectedMetricNames:   []string{"process.cpu.time"},
		},
		{
			name:                  "process.cpu.time disabled, process.cpu.utilization enabled",
			processCPUTimes:       false,
			processCPUUtilization: true,
			expectedMetricCount:   1,
			expectedMetricNames:   []string{"process.cpu.utilization"},
		},
		{
			name:                  "process.cpu.time enabled, process.cpu.utilization enabled",
			processCPUTimes:       true,
			processCPUUtilization: true,
			expectedMetricCount:   2,
			expectedMetricNames:   []string{"process.cpu.time", "process.cpu.utilization"},
		},
	}

	for i := range testCases {
		testCase := testCases[i]
		t.Run(testCase.name, func(t *testing.T) {
			metricsBuilderConfig := metadata.DefaultMetricsBuilderConfig()

			metricsBuilderConfig.Metrics.ProcessCPUTime.Enabled = testCase.processCPUTimes
			metricsBuilderConfig.Metrics.ProcessCPUUtilization.Enabled = testCase.processCPUUtilization

			// disable some default metrics for easy assertion
			metricsBuilderConfig.Metrics.ProcessMemoryUsage.Enabled = false
			metricsBuilderConfig.Metrics.ProcessMemoryVirtual.Enabled = false
			metricsBuilderConfig.Metrics.ProcessDiskIo.Enabled = false

			config := &Config{MetricsBuilderConfig: metricsBuilderConfig}

			scraper, err := newProcessScraper(scrapertest.NewNopSettings(metadata.Type), config)
			require.NoError(t, err, "Failed to create process scraper: %v", err)
			err = scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize process scraper: %v", err)

			handleMock := &processHandleMock{}
			handleMock.On("NameWithContext", mock.Anything).Return("test", nil)
			handleMock.On("ExeWithContext", mock.Anything).Return("test", nil)
			handleMock.On("CreateTimeWithContext", mock.Anything).Return(time.Now().UnixMilli(), nil)
			initDefaultsHandleMock(t, handleMock)

			scraper.getProcessHandles = func(context.Context) (processHandles, error) {
				return &processHandlesMock{handles: []*processHandleMock{handleMock}}, nil
			}

			// scrape the first time
			_, err = scraper.scrape(context.Background())
			assert.NoError(t, err)

			// scrape second time to get utilization
			md, err := scraper.scrape(context.Background())
			assert.NoError(t, err)

			for k := 0; k < md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len(); k++ {
				fmt.Println(md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(k).Name())
			}
			assert.Equal(t, testCase.expectedMetricCount, md.MetricCount())
			for metricIdx, expectedMetricName := range testCase.expectedMetricNames {
				assert.Equal(t, expectedMetricName, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(metricIdx).Name())
			}
		})
	}
}

func handleCountErrorIfSupportedOnPlatform() error {
	if handleCountMetricsLen > 0 {
		return errors.New("error-handle-count")
	}

	return nil
}

func handleCountErrorMessageIfSupportedOnPlatform() string {
	if handleCountErr := handleCountErrorIfSupportedOnPlatform(); handleCountErr != nil {
		return fmt.Errorf("error reading handle count for process \"test\" (pid 1): %w; ", handleCountErr).Error()
	}

	return ""
}
