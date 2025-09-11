// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"context"

	"github.com/shirou/gopsutil/v4/process"
)

func getGopsutilProcessHandles(ctx context.Context) (processHandles, error) {
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
