// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/process"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
)

// processMetadata stores process related metadata along
// with the process handle, and provides a function to
// initialize a pcommon.Resource with the metadata

type processMetadata struct {
	pid        int32
	parentPid  int32
	executable *executableMetadata
	command    *commandMetadata
	username   string
	handle     processHandle
	createTime int64
}

type executableMetadata struct {
	name string
	path string
}

type commandMetadata struct {
	command          string
	commandLine      string
	commandLineSlice []string
}

func (m *processMetadata) resourceOptions() []metadata.ResourceMetricsOption {
	opts := make([]metadata.ResourceMetricsOption, 0, 6)
	opts = append(opts,
		metadata.WithProcessPid(int64(m.pid)),
		metadata.WithProcessParentPid(int64(m.parentPid)),
		metadata.WithProcessExecutableName(m.executable.name),
		metadata.WithProcessExecutablePath(m.executable.path),
	)
	if m.command != nil {
		opts = append(opts, metadata.WithProcessCommand(m.command.command))
		if m.command.commandLineSlice != nil {
			// TODO insert slice here once this is supported by the data model
			// (see https://github.com/open-telemetry/opentelemetry-collector/pull/1142)
			opts = append(opts, metadata.WithProcessCommandLine(strings.Join(m.command.commandLineSlice, " ")))
		} else {
			opts = append(opts, metadata.WithProcessCommandLine(m.command.commandLine))
		}
	}
	if m.username != "" {
		opts = append(opts, metadata.WithProcessOwner(m.username))
	}
	return opts
}

// processHandles provides a wrapper around []*process.Process
// to support testing

type processHandles interface {
	Pid(index int) int32
	At(index int) processHandle
	Len() int
}

type processHandle interface {
	Name() (string, error)
	Exe() (string, error)
	Username() (string, error)
	Cmdline() (string, error)
	CmdlineSlice() ([]string, error)
	Times() (*cpu.TimesStat, error)
	Percent(time.Duration) (float64, error)
	MemoryInfo() (*process.MemoryInfoStat, error)
	MemoryPercent() (float32, error)
	IOCounters() (*process.IOCountersStat, error)
	NumThreads() (int32, error)
	CreateTime() (int64, error)
	Parent() (*process.Process, error)
	Ppid() (int32, error)
	PageFaults() (*process.PageFaultsStat, error)
	NumCtxSwitches() (*process.NumCtxSwitchesStat, error)
	NumFDs() (int32, error)
	// If gatherUsed is true, the currently used value will be gathered and added to the resulting RlimitStat.
	RlimitUsage(gatherUsed bool) ([]process.RlimitStat, error)
}

type gopsProcessHandles struct {
	handles []*process.Process
}

func (p *gopsProcessHandles) Pid(index int) int32 {
	return p.handles[index].Pid
}

func (p *gopsProcessHandles) At(index int) processHandle {
	return p.handles[index]
}

func (p *gopsProcessHandles) Len() int {
	return len(p.handles)
}

func getProcessHandlesInternal() (processHandles, error) {
	processes, err := process.Processes()
	if err != nil {
		return nil, err
	}

	return &gopsProcessHandles{handles: processes}, nil
}

func parentPid(handle processHandle, pid int32) (int32, error) {
	// special case for pid 0 and pid 1 in darwin
	if pid == 0 || (pid == 1 && runtime.GOOS == "darwin") {
		return 0, nil
	}

	return handle.Ppid()
}
