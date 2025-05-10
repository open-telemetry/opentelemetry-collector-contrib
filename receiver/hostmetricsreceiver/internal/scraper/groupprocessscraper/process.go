// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupprocessscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/groupprocessscraper"

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/groupprocessscraper/internal/metadata"
)

// processMetadata stores process related metadata along
// with the process handle, and provides a function to
// initialize a pcommon.Resource with the metadata

type processMetadata struct {
	pid             int32
	parentPid       int32
	executable      *executableMetadata
	command         *commandMetadata
	username        string
	handle          processHandle
	createTime      int64
	groupConfigName string
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

func (s *scraper) buildGroupResource(rb *metadata.ResourceBuilder, groupName string) pcommon.Resource {
	rb.SetGroupName(groupName)
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
	return p.handles[index]
}

func (p *gopsProcessHandles) Len() int {
	return len(p.handles)
}

type wrappedProcessHandle struct {
	*process.Process
}

func (p wrappedProcessHandle) CgroupWithContext(ctx context.Context) (string, error) {
	pid := p.Process.Pid
	statPath := getEnvWithContext(ctx, string(common.HostProcEnvKey), "/proc", strconv.Itoa(int(pid)), "cgroup")
	contents, err := os.ReadFile(statPath)
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(string(contents), "\n"), nil
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

func getProcessHandlesInternal(ctx context.Context) (processHandles, error) {
	processes, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}
	wrapped := make([]wrappedProcessHandle, len(processes))
	for i, p := range processes {
		wrapped[i] = wrappedProcessHandle{Process: p}
	}

	return &gopsProcessHandles{handles: wrapped}, nil
}

func parentPid(ctx context.Context, handle processHandle, pid int32) (int32, error) {
	// special case for pid 0 and pid 1 in darwin
	if pid == 0 || (pid == 1 && runtime.GOOS == "darwin") {
		return 0, nil
	}

	return handle.PpidWithContext(ctx)
}
