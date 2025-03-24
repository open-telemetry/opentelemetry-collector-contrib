// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/common"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/process"
	"go.opentelemetry.io/collector/pdata/pcommon"

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
	name   string
	path   string
	cgroup string
}

type commandMetadata struct {
	command          string
	commandLine      string
	commandLineSlice []string
}

func (m *processMetadata) buildResource(rb *metadata.ResourceBuilder) pcommon.Resource {
	rb.SetProcessPid(int64(m.pid))
	rb.SetProcessParentPid(int64(m.parentPid))
	rb.SetProcessExecutableName(m.executable.name)
	rb.SetProcessExecutablePath(m.executable.path)
	rb.SetProcessCgroup(m.executable.cgroup)
	if m.command != nil {
		rb.SetProcessCommand(m.command.command)
		if m.command.commandLineSlice != nil {
			// TODO insert slice here once this is supported by the data model
			// (see https://github.com/open-telemetry/opentelemetry-collector/pull/1142)
			rb.SetProcessCommandLine(strings.Join(m.command.commandLineSlice, " "))
		} else {
			rb.SetProcessCommandLine(m.command.commandLine)
		}
	}
	if m.username != "" {
		rb.SetProcessOwner(m.username)
	}
	return rb.Emit()
}

// processHandles provides a wrapper around []*process.Process
// to support testing

type processHandles interface {
	Pid(index int) int32
	At(index int) processHandle
	Len() int
}

type processHandle interface {
	NameWithContext(context.Context) (string, error)
	ExeWithContext(context.Context) (string, error)
	UsernameWithContext(context.Context) (string, error)
	CmdlineWithContext(context.Context) (string, error)
	CmdlineSliceWithContext(context.Context) ([]string, error)
	TimesWithContext(context.Context) (*cpu.TimesStat, error)
	PercentWithContext(context.Context, time.Duration) (float64, error)
	MemoryInfoWithContext(context.Context) (*process.MemoryInfoStat, error)
	MemoryPercentWithContext(context.Context) (float32, error)
	IOCountersWithContext(context.Context) (*process.IOCountersStat, error)
	NumThreadsWithContext(context.Context) (int32, error)
	CreateTimeWithContext(context.Context) (int64, error)
	ParentWithContext(context.Context) (*process.Process, error)
	PpidWithContext(context.Context) (int32, error)
	PageFaultsWithContext(context.Context) (*process.PageFaultsStat, error)
	NumCtxSwitchesWithContext(context.Context) (*process.NumCtxSwitchesStat, error)
	NumFDsWithContext(context.Context) (int32, error)
	// If gatherUsed is true, the currently used value will be gathered and added to the resulting RlimitStat.
	RlimitUsageWithContext(ctx context.Context, gatherUsed bool) ([]process.RlimitStat, error)
	CgroupWithContext(ctx context.Context) (string, error)
}

type gopsProcessHandles struct {
	handles []wrappedProcessHandle
}

func (p *gopsProcessHandles) Pid(index int) int32 {
	return p.handles[index].Process.Pid
}

func (p *gopsProcessHandles) At(index int) processHandle {
	return &(p.handles[index])
}

func (p *gopsProcessHandles) Len() int {
	return len(p.handles)
}

const (
	flagParentPidSet             = 1 << 0
	flagUseInitialNumThreadsOnce = 1 << 1
)

type wrappedProcessHandle struct {
	*process.Process
	parentPid         int32
	initialNumThreads int32
	flags             uint8 // bitfield to track if fields are set
}

func (p *wrappedProcessHandle) CgroupWithContext(ctx context.Context) (string, error) {
	pid := p.Process.Pid
	statPath := getEnvWithContext(ctx, string(common.HostProcEnvKey), "/proc", strconv.Itoa(int(pid)), "cgroup")
	contents, err := os.ReadFile(statPath)
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(string(contents), "\n"), nil
}

func (p *wrappedProcessHandle) PpidWithContext(ctx context.Context) (int32, error) {
	if p.flags&flagParentPidSet != 0 {
		return p.parentPid, nil
	}

	parentPid, err := p.Process.PpidWithContext(ctx)
	if err != nil {
		return 0, err
	}

	p.parentPid = parentPid
	p.flags |= flagParentPidSet
	return parentPid, nil
}

func (p *wrappedProcessHandle) NumThreadsWithContext(ctx context.Context) (int32, error) {
	if p.flags&flagUseInitialNumThreadsOnce != 0 {
		// The number of threads can fluctuate so use the initially cached value only the first time.
		p.flags &^= flagUseInitialNumThreadsOnce
		return p.initialNumThreads, nil
	}

	numThreads, err := p.Process.NumThreadsWithContext(ctx)
	if err != nil {
		return 0, err
	}

	return numThreads, nil
}

// copied from gopsutil:
// GetEnvWithContext retrieves the environment variable key. If it does not exist it returns the default.
// The context may optionally contain a map superseding os.EnvKey.
func getEnvWithContext(ctx context.Context, key string, dfault string, combineWith ...string) string {
	var value string
	if env, ok := ctx.Value(common.EnvKey).(common.EnvMap); ok {
		value = env[common.EnvKeyType(key)]
	}
	if value == "" {
		value = os.Getenv(key)
	}
	if value == "" {
		value = dfault
	}
	segments := append([]string{value}, combineWith...)

	return filepath.Join(segments...)
}

func parentPid(ctx context.Context, handle processHandle, pid int32) (int32, error) {
	// special case for pid 0 and pid 1 in darwin
	if pid == 0 || (pid == 1 && runtime.GOOS == "darwin") {
		return 0, nil
	}

	return handle.PpidWithContext(ctx)
}
