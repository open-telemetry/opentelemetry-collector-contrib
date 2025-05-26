// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/shirou/gopsutil/v4/process"
	"go.opentelemetry.io/collector/featuregate"
	"golang.org/x/sys/windows"
)

var useNewGetProcessHandles = featuregate.GlobalRegistry().MustRegister(
	"hostmetrics.process.onWindowsUseNewGetProcesses",
	featuregate.StageBeta,
	featuregate.WithRegisterDescription("If disabled, the scraper will use the legacy implementation to retrieve process handles."),
)

func getGopsutilProcessHandles(ctx context.Context) (processHandles, error) {
	if !useNewGetProcessHandles.IsEnabled() {
		return getGopsutilProcessHandlesLegacy(ctx)
	}

	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("could not create snapshot: %w", err)
	}
	defer func() {
		_ = windows.CloseHandle(snap)
	}()

	var pe32 windows.ProcessEntry32
	pe32.Size = uint32(unsafe.Sizeof(pe32))
	if err = windows.Process32First(snap, &pe32); err != nil {
		return nil, fmt.Errorf("could not get first process: %w", err)
	}

	wrappedProcesses := make([]wrappedProcessHandle, 0, 64)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			// Ignoring any errors here to keep same behavior as the legacy implementation
			// based on the `process.ProcessesWithContext` from the `gopsutil` package.
			p, _ := process.NewProcess(int32(pe32.ProcessID))
			if p != nil {
				wrappedProcess := wrappedProcessHandle{
					Process:           p,
					parentPid:         int32(pe32.ParentProcessID),
					initialNumThreads: int32(pe32.Threads),
					flags:             flagParentPidSet | flagUseInitialNumThreadsOnce,
				}
				wrappedProcesses = append(wrappedProcesses, wrappedProcess)
			}
		}

		if err = windows.Process32Next(snap, &pe32); err != nil {
			break
		}
	}

	return &gopsProcessHandles{handles: wrappedProcesses}, nil
}

func getGopsutilProcessHandlesLegacy(ctx context.Context) (processHandles, error) {
	processes, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}
	wrapped := make([]wrappedProcessHandle, len(processes))
	for i, p := range processes {
		wrapped[i] = wrappedProcessHandle{
			Process: p,
		}
	}

	return &gopsProcessHandles{handles: wrapped}, nil
}
