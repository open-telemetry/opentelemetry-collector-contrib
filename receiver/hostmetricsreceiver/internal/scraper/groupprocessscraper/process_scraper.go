// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupprocessscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/groupprocessscraper"

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/shirou/gopsutil/v4/common"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/groupprocessscraper/internal/handlecount"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/groupprocessscraper/internal/metadata"
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
	signalMetricsLen            = 1

	metricsLen = cpuMetricsLen + memoryMetricsLen + diskMetricsLen + memoryUtilizationMetricsLen + pagingMetricsLen + threadMetricsLen + contextSwitchMetricsLen + fileDescriptorMetricsLen + signalMetricsLen
)

// scraper for Process Metrics
type scraper struct {
	settings           receiver.Settings
	config             *Config
	mb                 *metadata.MetricsBuilder
	scrapeProcessDelay time.Duration
	logicalCores       int

	matchGroupFS    map[string]map[string]filterset.FilterSet
	matchGroupNames map[string][]string

	// for mocking
	getProcessCreateTime func(p processHandle, ctx context.Context) (int64, error)
	getProcessHandles    func(context.Context) (processHandles, error)

	handleCountManager handlecount.Manager
}

// newGroupProcessScraper creates a Process Scraper
func newGroupProcessScraper(settings receiver.Settings, cfg *Config) (*scraper, error) {
	scraper := &scraper{
		settings:             settings,
		config:               cfg,
		getProcessCreateTime: processHandle.CreateTimeWithContext,
		getProcessHandles:    getProcessHandlesInternal,
		scrapeProcessDelay:   cfg.ScrapeProcessDelay,
		handleCountManager:   handlecount.NewManager(),
		matchGroupFS:         make(map[string]map[string]filterset.FilterSet), // Initialize the map
		matchGroupNames:      make(map[string][]string),                       // Initialize the map for cmdline names
	}

	var err error

	for _, gc := range cfg.GroupConfig {

		scraper.matchGroupFS[gc.GroupName] = make(map[string]filterset.FilterSet)

		if len(gc.Comm.Names) > 0 {
			commFS, err := filterset.CreateFilterSet(gc.Comm.Names, &filterset.Config{MatchType: filterset.MatchType(gc.Comm.MatchType)})
			if err != nil {
				return nil, fmt.Errorf("error creating comm filter set: %w", err)
			}
			scraper.matchGroupFS[gc.GroupName]["comm"] = commFS
		}

		if len(gc.Exe.Names) > 0 {
			exeFS, err := filterset.CreateFilterSet(gc.Exe.Names, &filterset.Config{MatchType: filterset.MatchType(gc.Exe.MatchType)})
			if err != nil {
				return nil, fmt.Errorf("error creating exe filter set: %w", err)
			}
			scraper.matchGroupFS[gc.GroupName]["exe"] = exeFS
		}

		if len(gc.Cmdline.Names) > 0 {
			cmdlineFS, err := filterset.CreateFilterSet(gc.Cmdline.Names, &filterset.Config{MatchType: filterset.MatchType(gc.Cmdline.MatchType)})
			if err != nil {
				return nil, fmt.Errorf("error creating cmdline filter set: %w", err)
			}
			scraper.matchGroupFS[gc.GroupName]["cmdline"] = cmdlineFS
			scraper.matchGroupNames[gc.GroupName] = gc.Cmdline.Names
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

	for groupName, processMetadataList := range data {
		var totalCPUPercent float64
		var totalMemoryPercent float32
		var totalProcessCount int64
		var totalThreadCount int64
		var totalOpenFDCount int64

		for _, md := range processMetadataList {
			presentPIDs[md.pid] = struct{}{}
			cpuPercent, err := md.handle.PercentWithContext(ctx, time.Second)
			if err != nil {
				errs.AddPartial(cpuMetricsLen, fmt.Errorf("error reading cpu percent for process %q (pid %v): %w", md.executable.name, md.pid, err))
				continue
			}
			totalCPUPercent += cpuPercent

			memPercent, err := md.handle.MemoryPercentWithContext(ctx)
			if err != nil {
				errs.AddPartial(memoryMetricsLen, fmt.Errorf("error reading memory percent for process %q (pid %v): %w", md.executable.name, md.pid, err))
				continue
			}
			totalMemoryPercent += memPercent

			threads, err := md.handle.NumThreadsWithContext(ctx)
			if err != nil {
				errs.AddPartial(threadMetricsLen, fmt.Errorf("error reading thread info for process %q (pid %v): %w", md.executable.name, md.pid, err))
				continue
			}
			totalThreadCount += int64(threads)

			fds, err := md.handle.NumFDsWithContext(ctx)
			if err != nil {
				errs.AddPartial(fileDescriptorMetricsLen, fmt.Errorf("error reading open file descriptor count for process %q (pid %v): %w", md.executable.name, md.pid, err))
				continue
			}
			totalOpenFDCount += int64(fds)

			totalProcessCount++
		}

		// Normalize the total CPU percentage by dividing by the number of logical cores
		totalCPUPercent /= float64(s.logicalCores)

		now := pcommon.NewTimestampFromTime(time.Now())
		s.mb.RecordProcessCPUPercentDataPoint(now, totalCPUPercent, metadata.AttributeStateTotal)
		s.mb.RecordProcessMemoryPercentDataPoint(now, float64(totalMemoryPercent))
		s.mb.RecordProcessCountDataPoint(now, totalProcessCount)
		s.mb.RecordProcessThreadsDataPoint(now, totalThreadCount)
		s.mb.RecordProcessOpenFileDescriptorsDataPoint(now, totalOpenFDCount)

		s.mb.EmitForResource(metadata.WithResource(s.buildGroupResource(s.mb.NewResourceBuilder(), groupName)))
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
func (s *scraper) getProcessMetadata() (map[string][]*processMetadata, error) {
	ctx := context.WithValue(context.Background(), common.EnvKey, s.config.EnvMap)
	handles, err := s.getProcessHandles(ctx)
	if err != nil {
		return nil, err
	}

	var errs scrapererror.ScrapeErrors

	//data := make([]*processMetadata, 0, handles.Len())
	data := make(map[string][]*processMetadata)
	assignedPIDs := make(map[int32]struct{})

	for i := 0; i < handles.Len(); i++ {
		pid := handles.Pid(i)

		// Check and skip if PID is already assigned to a group
		if _, exists := assignedPIDs[pid]; exists {
			continue
		}

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

		command, err := getProcessCommand(ctx, handle)
		if err != nil {
			errs.AddPartial(0, fmt.Errorf("error reading command for process %q (pid %v): %w", executable.name, pid, err))
		}

		groupConfigName := ""
		for _, gc := range s.config.GroupConfig {
			commMatched := true
			exeMatched := true
			cmdlineMatched := true

			if s.matchGroupFS[gc.GroupName]["comm"] == nil && s.matchGroupFS[gc.GroupName]["exe"] == nil && s.matchGroupFS[gc.GroupName]["cmdline"] == nil {
				continue
			}

			if s.matchGroupFS[gc.GroupName]["comm"] != nil {
				commMatched = matchesFilterSetString(s.matchGroupFS[gc.GroupName]["comm"], name)
			}
			if s.matchGroupFS[gc.GroupName]["exe"] != nil {
				exeMatched = matchesFilterSetString(s.matchGroupFS[gc.GroupName]["exe"], exe)
			}
			if s.matchGroupFS[gc.GroupName]["cmdline"] != nil {
				cmdlineMatched = matchesFilterSetSlice(s.matchGroupNames[gc.GroupName], command.commandLineSlice)
			}

			if commMatched && exeMatched && cmdlineMatched {
				groupConfigName = gc.GroupName
				break
			}
		}

		if groupConfigName == "" {
			continue
		}

		assignedPIDs[pid] = struct{}{} // Mark PID as assigned

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
			pid:             pid,
			parentPid:       parentPid,
			executable:      executable,
			command:         command,
			username:        username,
			handle:          handle,
			createTime:      createTime,
			groupConfigName: groupConfigName,
		}

		data[groupConfigName] = append(data[groupConfigName], md)
	}

	return data, errs.Combine()
}

func matchesFilterSetString(fs filterset.FilterSet, value string) bool {
	return fs.Matches(value)
}

func matchesFilterSetSlice(names []string, values []string) bool {
	for _, name := range names {
		matched := false
		re, err := regexp.Compile(name)
		if err != nil {
			continue // Skip invalid regex patterns
		}
		for _, value := range values {
			if re.MatchString(value) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
