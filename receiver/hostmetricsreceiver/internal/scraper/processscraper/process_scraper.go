// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/common"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/process"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/handlecount"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/ucal"
)

const (
	cpuMetricsLen               = 1
	memoryMetricsLen            = 2
	memoryUtilizationMetricsLen = 1
	diskMetricsLen              = 1
	pagingMetricsLen            = 1
	threadMetricsLen            = 1
	contextSwitchMetricsLen     = 1
	fileDescriptorMetricsLen    = 1
	handleMetricsLen            = 1
	signalMetricsLen            = 1

	metricsLen = cpuMetricsLen + memoryMetricsLen + diskMetricsLen + memoryUtilizationMetricsLen + pagingMetricsLen + threadMetricsLen + contextSwitchMetricsLen + fileDescriptorMetricsLen + signalMetricsLen
)

// scraper for Process Metrics
type scraper struct {
	settings           receiver.Settings
	config             *Config
	mb                 *metadata.MetricsBuilder
	includeFS          filterset.FilterSet
	excludeFS          filterset.FilterSet
	scrapeProcessDelay time.Duration
	ucals              map[int32]*ucal.CPUUtilizationCalculator
	logicalCores       int

	// for mocking
	getProcessCreateTime func(p processHandle, ctx context.Context) (int64, error)
	getProcessHandles    func(context.Context) (processHandles, error)

	handleCountManager handlecount.Manager
}

// newProcessScraper creates a Process Scraper
func newProcessScraper(settings receiver.Settings, cfg *Config) (*scraper, error) {
	scraper := &scraper{
		settings:             settings,
		config:               cfg,
		getProcessCreateTime: processHandle.CreateTimeWithContext,
		getProcessHandles:    getProcessHandlesInternal,
		scrapeProcessDelay:   cfg.ScrapeProcessDelay,
		ucals:                make(map[int32]*ucal.CPUUtilizationCalculator),
		handleCountManager:   handlecount.NewManager(),
	}

	var err error

	if len(cfg.Include.Names) > 0 {
		scraper.includeFS, err = filterset.CreateFilterSet(cfg.Include.Names, &cfg.Include.Config)
		if err != nil {
			return nil, fmt.Errorf("error creating process include filters: %w", err)
		}
	}

	if len(cfg.Exclude.Names) > 0 {
		scraper.excludeFS, err = filterset.CreateFilterSet(cfg.Exclude.Names, &cfg.Exclude.Config)
		if err != nil {
			return nil, fmt.Errorf("error creating process exclude filters: %w", err)
		}
	}

	logicalCores, err := cpu.Counts(true)
	if err != nil {
		return nil, fmt.Errorf("error getting number of logical cores: %w", err)
	}

	scraper.logicalCores = logicalCores

	return scraper, nil
}

func (s *scraper) start(context.Context, component.Host) error {
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings)
	return nil
}

func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	var errs scrapererror.ScrapeErrors

	// If the boot time cache featuregate is disabled, this will refresh the
	// cached boot time value for use in the current scrape. This functionally
	// replicates the previous functionality in all but the most extreme
	// cases of boot time changing in the middle of a scrape.
	if !bootTimeCacheFeaturegate.IsEnabled() {
		host.EnableBootTimeCache(false)
		_, err := host.BootTimeWithContext(ctx)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf(`retrieving boot time failed with error "%w", using cached boot time`, err))
		}
		host.EnableBootTimeCache(true)
	}

	data, err := s.getProcessMetadata()
	if err != nil {
		var partialErr scrapererror.PartialScrapeError
		if !errors.As(err, &partialErr) {
			return pmetric.NewMetrics(), err
		}

		errs.AddPartial(partialErr.Failed, partialErr)
	}

	presentPIDs := make(map[int32]struct{}, len(data))
	ctx = context.WithValue(ctx, common.EnvKey, s.config.EnvMap)

	for _, md := range data {
		presentPIDs[md.pid] = struct{}{}

		now := pcommon.NewTimestampFromTime(time.Now())

		if err = s.scrapeAndAppendCPUTimeMetric(ctx, now, md.handle, md.pid); err != nil {
			errs.AddPartial(cpuMetricsLen, fmt.Errorf("error reading cpu times for process %q (pid %v): %w", md.executable.name, md.pid, err))
		}

		if err = s.scrapeAndAppendMemoryUsageMetrics(ctx, now, md.handle); err != nil {
			errs.AddPartial(memoryMetricsLen, fmt.Errorf("error reading memory info for process %q (pid %v): %w", md.executable.name, md.pid, err))
		}

		if err = s.scrapeAndAppendMemoryUtilizationMetric(ctx, now, md.handle); err != nil {
			errs.AddPartial(memoryUtilizationMetricsLen, fmt.Errorf("error reading memory utilization for process %q (pid %v): %w", md.executable.name, md.pid, err))
		}

		if err = s.scrapeAndAppendDiskMetrics(ctx, now, md.handle); err != nil && !s.config.MuteProcessIOError {
			errs.AddPartial(diskMetricsLen, fmt.Errorf("error reading disk usage for process %q (pid %v): %w", md.executable.name, md.pid, err))
		}

		if err = s.scrapeAndAppendPagingMetric(ctx, now, md.handle); err != nil {
			errs.AddPartial(pagingMetricsLen, fmt.Errorf("error reading memory paging info for process %q (pid %v): %w", md.executable.name, md.pid, err))
		}

		if err = s.scrapeAndAppendThreadsMetrics(ctx, now, md.handle); err != nil {
			errs.AddPartial(threadMetricsLen, fmt.Errorf("error reading thread info for process %q (pid %v): %w", md.executable.name, md.pid, err))
		}

		if err = s.scrapeAndAppendContextSwitchMetrics(ctx, now, md.handle); err != nil {
			errs.AddPartial(contextSwitchMetricsLen, fmt.Errorf("error reading context switch counts for process %q (pid %v): %w", md.executable.name, md.pid, err))
		}

		if err = s.scrapeAndAppendOpenFileDescriptorsMetric(ctx, now, md.handle); err != nil {
			errs.AddPartial(fileDescriptorMetricsLen, fmt.Errorf("error reading open file descriptor count for process %q (pid %v): %w", md.executable.name, md.pid, err))
		}

		if err = s.scrapeAndAppendHandlesMetric(ctx, now, int64(md.pid)); err != nil {
			errs.AddPartial(handleMetricsLen, fmt.Errorf("error reading handle count for process %q (pid %v): %w", md.executable.name, md.pid, err))
		}

		if err = s.scrapeAndAppendSignalsPendingMetric(ctx, now, md.handle); err != nil {
			errs.AddPartial(signalMetricsLen, fmt.Errorf("error reading pending signals for process %q (pid %v): %w", md.executable.name, md.pid, err))
		}

		s.mb.EmitForResource(metadata.WithResource(md.buildResource(s.mb.NewResourceBuilder())),
			metadata.WithStartTimeOverride(pcommon.Timestamp(md.createTime*1e6)))
	}

	// Cleanup any [ucal.CPUUtilizationCalculator]s for PIDs that are no longer present
	for pid := range s.ucals {
		if _, ok := presentPIDs[pid]; !ok {
			delete(s.ucals, pid)
		}
	}

	if s.config.MuteProcessAllErrors {
		return s.mb.Emit(), nil
	}

	return s.mb.Emit(), errs.Combine()
}

// getProcessMetadata returns a slice of processMetadata, including handles,
// for all currently running processes. If errors occur obtaining information
// for some processes, an error will be returned, but any processes that were
// successfully obtained will still be returned.
func (s *scraper) getProcessMetadata() ([]*processMetadata, error) {
	ctx := context.WithValue(context.Background(), common.EnvKey, s.config.EnvMap)
	handles, err := s.getProcessHandles(ctx)
	if err != nil {
		return nil, err
	}

	var errs scrapererror.ScrapeErrors

	if err := s.refreshHandleCounts(); err != nil {
		errs.Add(err)
	}

	data := make([]*processMetadata, 0, handles.Len())
	for i := 0; i < handles.Len(); i++ {
		pid := handles.Pid(i)
		handle := handles.At(i)

		exe, err := getProcessExecutable(ctx, handle)
		if err != nil {
			if !s.config.MuteProcessExeError {
				errs.AddPartial(1, fmt.Errorf("error reading process executable for pid %v: %w", pid, err))
			}
		}

		name, err := getProcessName(ctx, handle, exe)
		if err != nil {
			if !s.config.MuteProcessNameError {
				errs.AddPartial(1, fmt.Errorf("error reading process name for pid %v: %w", pid, err))
			}
			continue
		}
		cgroup, err := getProcessCgroup(ctx, handle)
		if err != nil {
			if !s.config.MuteProcessCgroupError {
				errs.AddPartial(1, fmt.Errorf("error reading process cgroup for pid %v: %w", pid, err))
			}
			continue
		}

		executable := &executableMetadata{name: name, path: exe, cgroup: cgroup}

		// filter processes by name
		if (s.includeFS != nil && !s.includeFS.Matches(executable.name)) ||
			(s.excludeFS != nil && s.excludeFS.Matches(executable.name)) {
			continue
		}

		command, err := getProcessCommand(ctx, handle)
		if err != nil {
			errs.AddPartial(0, fmt.Errorf("error reading command for process %q (pid %v): %w", executable.name, pid, err))
		}

		username, err := handle.UsernameWithContext(ctx)
		if err != nil {
			if !s.config.MuteProcessUserError {
				errs.AddPartial(0, fmt.Errorf("error reading username for process %q (pid %v): %w", executable.name, pid, err))
			}
		}

		createTime, err := s.getProcessCreateTime(handle, ctx)
		if err != nil {
			errs.AddPartial(0, fmt.Errorf("error reading create time for process %q (pid %v): %w", executable.name, pid, err))
			// set the start time to now to avoid including this when a scrape_process_delay is set
			createTime = time.Now().UnixMilli()
		}
		if s.scrapeProcessDelay.Milliseconds() > (time.Now().UnixMilli() - createTime) {
			continue
		}

		parentPid, err := parentPid(ctx, handle, pid)
		if err != nil {
			errs.AddPartial(0, fmt.Errorf("error reading parent pid for process %q (pid %v): %w", executable.name, pid, err))
		}

		md := &processMetadata{
			pid:        pid,
			parentPid:  parentPid,
			executable: executable,
			command:    command,
			username:   username,
			handle:     handle,
			createTime: createTime,
		}

		data = append(data, md)
	}

	return data, errs.Combine()
}

func (s *scraper) scrapeAndAppendCPUTimeMetric(ctx context.Context, now pcommon.Timestamp, handle processHandle, pid int32) error {
	if !s.config.MetricsBuilderConfig.Metrics.ProcessCPUTime.Enabled && !s.config.MetricsBuilderConfig.Metrics.ProcessCPUUtilization.Enabled {
		return nil
	}

	times, err := handle.TimesWithContext(ctx)
	if err != nil {
		return err
	}

	if s.config.MetricsBuilderConfig.Metrics.ProcessCPUTime.Enabled {
		s.recordCPUTimeMetric(now, times)
	}

	if !s.config.MetricsBuilderConfig.Metrics.ProcessCPUUtilization.Enabled {
		return nil
	}

	if _, ok := s.ucals[pid]; !ok {
		s.ucals[pid] = &ucal.CPUUtilizationCalculator{}
	}

	err = s.ucals[pid].CalculateAndRecord(now, s.logicalCores, times, s.recordCPUUtilization)
	return err
}

func (s *scraper) scrapeAndAppendMemoryUsageMetrics(ctx context.Context, now pcommon.Timestamp, handle processHandle) error {
	if !(s.config.MetricsBuilderConfig.Metrics.ProcessMemoryUsage.Enabled || s.config.MetricsBuilderConfig.Metrics.ProcessMemoryVirtual.Enabled) {
		return nil
	}

	mem, err := handle.MemoryInfoWithContext(ctx)
	if err != nil {
		return err
	}

	s.mb.RecordProcessMemoryUsageDataPoint(now, int64(mem.RSS))
	s.mb.RecordProcessMemoryVirtualDataPoint(now, int64(mem.VMS))
	return nil
}

func (s *scraper) scrapeAndAppendMemoryUtilizationMetric(ctx context.Context, now pcommon.Timestamp, handle processHandle) error {
	if !s.config.MetricsBuilderConfig.Metrics.ProcessMemoryUtilization.Enabled {
		return nil
	}

	memoryPercent, err := handle.MemoryPercentWithContext(ctx)
	if err != nil {
		return err
	}

	s.mb.RecordProcessMemoryUtilizationDataPoint(now, float64(memoryPercent))

	return nil
}

func (s *scraper) scrapeAndAppendDiskMetrics(ctx context.Context, now pcommon.Timestamp, handle processHandle) error {
	if !(s.config.MetricsBuilderConfig.Metrics.ProcessDiskIo.Enabled || s.config.MetricsBuilderConfig.Metrics.ProcessDiskOperations.Enabled) || runtime.GOOS == "darwin" {
		return nil
	}

	io, err := handle.IOCountersWithContext(ctx)
	if err != nil {
		if s.config.MuteProcessIOError {
			return nil
		}
		return err
	}

	s.mb.RecordProcessDiskIoDataPoint(now, int64(io.ReadBytes), metadata.AttributeDirectionRead)
	s.mb.RecordProcessDiskIoDataPoint(now, int64(io.WriteBytes), metadata.AttributeDirectionWrite)
	s.mb.RecordProcessDiskOperationsDataPoint(now, int64(io.ReadCount), metadata.AttributeDirectionRead)
	s.mb.RecordProcessDiskOperationsDataPoint(now, int64(io.WriteCount), metadata.AttributeDirectionWrite)

	return nil
}

func (s *scraper) scrapeAndAppendPagingMetric(ctx context.Context, now pcommon.Timestamp, handle processHandle) error {
	if !s.config.MetricsBuilderConfig.Metrics.ProcessPagingFaults.Enabled {
		return nil
	}

	pageFaultsStat, err := handle.PageFaultsWithContext(ctx)
	if err != nil {
		return err
	}

	s.mb.RecordProcessPagingFaultsDataPoint(now, int64(pageFaultsStat.MajorFaults), metadata.AttributePagingFaultTypeMajor)
	s.mb.RecordProcessPagingFaultsDataPoint(now, int64(pageFaultsStat.MinorFaults), metadata.AttributePagingFaultTypeMinor)

	return nil
}

func (s *scraper) scrapeAndAppendThreadsMetrics(ctx context.Context, now pcommon.Timestamp, handle processHandle) error {
	if !s.config.MetricsBuilderConfig.Metrics.ProcessThreads.Enabled {
		return nil
	}
	threads, err := handle.NumThreadsWithContext(ctx)
	if err != nil {
		return err
	}
	s.mb.RecordProcessThreadsDataPoint(now, int64(threads))

	return nil
}

func (s *scraper) scrapeAndAppendContextSwitchMetrics(ctx context.Context, now pcommon.Timestamp, handle processHandle) error {
	if !s.config.MetricsBuilderConfig.Metrics.ProcessContextSwitches.Enabled {
		return nil
	}

	contextSwitches, err := handle.NumCtxSwitchesWithContext(ctx)

	if err != nil {
		return err
	}

	s.mb.RecordProcessContextSwitchesDataPoint(now, contextSwitches.Involuntary, metadata.AttributeContextSwitchTypeInvoluntary)
	s.mb.RecordProcessContextSwitchesDataPoint(now, contextSwitches.Voluntary, metadata.AttributeContextSwitchTypeVoluntary)

	return nil
}

func (s *scraper) scrapeAndAppendOpenFileDescriptorsMetric(ctx context.Context, now pcommon.Timestamp, handle processHandle) error {
	if !s.config.MetricsBuilderConfig.Metrics.ProcessOpenFileDescriptors.Enabled {
		return nil
	}

	fds, err := handle.NumFDsWithContext(ctx)

	if err != nil {
		return err
	}

	s.mb.RecordProcessOpenFileDescriptorsDataPoint(now, int64(fds))

	return nil
}

func (s *scraper) refreshHandleCounts() error {
	if !s.config.MetricsBuilderConfig.Metrics.ProcessHandles.Enabled {
		return nil
	}

	return s.handleCountManager.Refresh()
}

func (s *scraper) scrapeAndAppendHandlesMetric(_ context.Context, now pcommon.Timestamp, pid int64) error {
	if !s.config.MetricsBuilderConfig.Metrics.ProcessHandles.Enabled {
		return nil
	}

	count, err := s.handleCountManager.GetProcessHandleCount(pid)
	if err != nil {
		return err
	}

	s.mb.RecordProcessHandlesDataPoint(now, int64(count))

	return nil
}

func (s *scraper) scrapeAndAppendSignalsPendingMetric(ctx context.Context, now pcommon.Timestamp, handle processHandle) error {
	if !s.config.MetricsBuilderConfig.Metrics.ProcessSignalsPending.Enabled {
		return nil
	}

	rlimitStats, err := handle.RlimitUsageWithContext(ctx, true)
	if err != nil {
		return err
	}

	for _, rlimitStat := range rlimitStats {
		if rlimitStat.Resource == process.RLIMIT_SIGPENDING {
			s.mb.RecordProcessSignalsPendingDataPoint(now, int64(rlimitStat.Used))
			break
		}
	}

	return nil
}
