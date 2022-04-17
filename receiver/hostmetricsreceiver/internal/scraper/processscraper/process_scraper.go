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

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
)

const (
	cpuMetricsLen            = 1
	memoryMetricsLen         = 2
	diskMetricsLen           = 1
	aggregateChildMetricsLen = 1

	metricsLen = cpuMetricsLen + memoryMetricsLen + diskMetricsLen
)

// scraper for Process Metrics
type scraper struct {
	config    *Config
	mb        *metadata.MetricsBuilder
	filterSet *processFilterSet

	// for mocking
	bootTime             func() (uint64, error)
	getProcessHandles    func() (processHandles, error)
	getProcessExecutable func(processHandle) (*executableMetadata, error)
	getProcessCommand    func(processHandle) (*commandMetadata, error)
}

// newProcessScraper creates a Process Scraper
func newProcessScraper(cfg *Config) (*scraper, error) {
	scraper := &scraper{config: cfg, bootTime: host.BootTime, getProcessHandles: getProcessHandlesInternal}
	var err error

	scraper.filterSet, err = createFilters(cfg.Filters)
	if err != nil {
		return nil, fmt.Errorf("error creating process filters: %w", err)
	}

	scraper.getProcessExecutable = getProcessExecutable
	scraper.getProcessCommand = getProcessCommand

	return scraper, nil
}

func (s *scraper) start(context.Context, component.Host) error {
	bootTime, err := s.bootTime()
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.Metrics, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return nil
}

func (s *scraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	var errs scrapererror.ScrapeErrors

	metadata, err := s.getProcessMetadata()
	if err != nil {
		partialErr, isPartial := err.(scrapererror.PartialScrapeError)
		if !isPartial {
			return pmetric.NewMetrics(), err
		}

		errs.AddPartial(partialErr.Failed, partialErr)
	}

	for pid, md := range metadata {
		// sanity test, this should never happen
		if md.parent == nil {
			errLen := cpuMetricsLen + memoryMetricsLen + diskMetricsLen
			if s.config.AggregateChildMetrics {
				errLen += aggregateChildMetricsLen
			}
			errs.AddPartial(errLen, fmt.Errorf("error reading child metrics, parent %d not found", pid))
			continue
		}

		now := pdata.NewTimestampFromTime(time.Now())
		if err = s.scrapeAndAppendCPUTimeMetric(now, md.parent.handle, md.children); err != nil {
			errs.AddPartial(cpuMetricsLen, fmt.Errorf("error reading cpu times for process %q (pid %v): %w", md.parent.executable.name, md.parent.pid, err))
		}

		if err = s.scrapeAndAppendMemoryUsageMetrics(now, md.parent.handle, md.children); err != nil {
			errs.AddPartial(memoryMetricsLen, fmt.Errorf("error reading memory info for process %q (pid %v): %w", md.parent.executable.name, md.parent.pid, err))
		}

		if err = s.scrapeAndAppendDiskIOMetric(now, md.parent.handle, md.children); err != nil {
			errs.AddPartial(diskMetricsLen, fmt.Errorf("error reading disk usage for process %q (pid %v): %w", md.parent.executable.name, md.parent.pid, err))
		}

		if s.config.AggregateChildMetrics {
			s.mb.RecordProcessChildCountDataPoint(now, int64(len(md.children)))
		}

		s.mb.EmitForResource(md.parent.resourceOptions()...)
	}

	return s.mb.Emit(), errs.Combine()
}

// getProcessMetadata returns a slice of processMetadata, including handles,
// for all currently running processes. If errors occur obtaining information
// for some processes, an error will be returned, but any processes that were
// successfully obtained will still be returned.
func (s *scraper) getProcessMetadata() (map[int32]*hierarchicalMetadata, error) {
	handles, err := s.getProcessHandles()
	if err != nil {
		return nil, err
	}

	var errs scrapererror.ScrapeErrors

	metadata := make(map[int32]*hierarchicalMetadata)

	for i := 0; i < handles.Len(); i++ {
		pid := handles.Pid(i)

		// filter by pid
		matches := s.filterSet.includePid(pid)
		if len(matches) == 0 {
			continue
		}
		handle := handles.At(i)

		executable, err := s.getProcessExecutable(handle)
		if err != nil {
			if !s.config.MuteProcessNameError {
				errs.AddPartial(1, fmt.Errorf("error reading process name for pid %v: %w", pid, err))
			}
			continue
		}
		// filter by executable
		matches = s.filterSet.includeExecutable(executable.name, executable.path, matches)
		if len(matches) == 0 {
			continue
		}

		command, err := s.getProcessCommand(handle)
		if err != nil {
			errs.AddPartial(0, fmt.Errorf("error reading command for process %q (pid %v): %w", executable.name, pid, err))
			//create an empty command to run includeCommand against
			command = &commandMetadata{}
		}
		// filter by command
		commandLine := strings.Join(command.commandLineSlice, " ")
		matches = s.filterSet.includeCommand(command.command, commandLine, matches)
		if len(matches) == 0 {
			continue
		}

		username, err := handle.Username()
		if err != nil {
			errs.AddPartial(0, fmt.Errorf("error reading username for process %q (pid %v): %w", executable.name, pid, err))
		}
		// filter by user
		matches = s.filterSet.includeOwner(username, matches)
		if len(matches) == 0 {
			continue
		}

		if s.config.AggregateChildMetrics {
			parentPid, err := handles.ParentPid(i)
			if err != nil {
				errs.AddPartial(0, fmt.Errorf("error reading parent pid for process %q (pid %v): %w", executable.name, pid, err))
				// if we were unable to find the parent, store the data against the process itself rather than aggregating
				// it in the parent
				parentPid = pid
			}

			if _, ok := metadata[parentPid]; !ok {
				metadata[parentPid] = &hierarchicalMetadata{
					children: make([]processHandle, 0),
				}
			}

			// if this process is the parent process
			if pid == parentPid {
				metadata[pid].parent = &processMetadata{
					pid:        pid,
					executable: executable,
					command:    command,
					username:   username,
					handle:     handle,
				}
			} else {
				metadata[parentPid].children = append(metadata[parentPid].children, handle)
			}
		} else {
			metadata[pid] = &hierarchicalMetadata{
				parent: &processMetadata{
					pid:        pid,
					executable: executable,
					command:    command,
					username:   username,
					handle:     handle,
				},
				children: make([]processHandle, 0),
			}
		}
	}

	return metadata, errs.Combine()
}

func (s *scraper) scrapeAndAppendCPUTimeMetric(now pdata.Timestamp, parent processHandle, children []processHandle) error {
	var allTimes []*cpu.TimesStat

	times, err := parent.Times()

	if err != nil {
		return err
	}
	allTimes = append(allTimes, times)

	for _, val := range children {
		times, err = val.Times()
		if err != nil {
			return err
		}
		allTimes = append(allTimes, times)
	}
	s.recordCPUTimeMetric(now, allTimes)
	return nil
}

func (s *scraper) scrapeAndAppendMemoryUsageMetrics(now pdata.Timestamp, parent processHandle, children []processHandle) error {
	mem, err := parent.MemoryInfo()
	if err != nil {
		return err
	}

	memRSS := mem.RSS
	memVMS := mem.VMS

	for _, val := range children {
		mem, err = val.MemoryInfo()
		if err != nil {
			return err
		}
		memRSS += mem.RSS
		memVMS += mem.VMS
	}

	s.mb.RecordProcessMemoryPhysicalUsageDataPoint(now, int64(memRSS))
	s.mb.RecordProcessMemoryVirtualUsageDataPoint(now, int64(memVMS))
	return nil
}

func (s *scraper) scrapeAndAppendDiskIOMetric(now pdata.Timestamp, parent processHandle, children []processHandle) error {
	io, err := parent.IOCounters()
	if err != nil {
		return err
	}

	readBytes := io.ReadBytes
	writeBytes := io.WriteBytes

	for _, val := range children {
		io, err = val.IOCounters()
		if err != nil {
			return err
		}

		readBytes += io.ReadBytes
		writeBytes += io.WriteBytes
	}

	s.mb.RecordProcessDiskIoDataPoint(now, int64(readBytes), metadata.AttributeDirection.Read)
	s.mb.RecordProcessDiskIoDataPoint(now, int64(writeBytes), metadata.AttributeDirection.Write)
	return nil
}
