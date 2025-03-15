// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/windows"
)

func getProcessHandlesInternalNew(ctx context.Context) (processHandles, error) {
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
			p, _ := process.NewProcess(int32(pe32.ProcessID))
			if p != nil {
				wrappedProcess := wrappedProcessHandle{
					Process: p,
					ppid:    int32(pe32.ParentProcessID),
					threads: int32(pe32.Threads),
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
