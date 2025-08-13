// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !windows && !darwin && !freebsd

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"context"

	"github.com/shirou/gopsutil/v4/cpu"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/ucal"
)

func (s *processScraper) recordCPUTimeMetric(now pcommon.Timestamp, cpuTime *cpu.TimesStat) {
}

func (s *processScraper) recordCPUUtilization(now pcommon.Timestamp, cpuUtilization ucal.CPUUtilization) {
}

func getProcessName(context.Context, processHandle, string) (string, error) {
	return "", nil
}

func getProcessCgroup(ctx context.Context, proc processHandle) (string, error) {
	return "", nil
}

func getProcessExecutable(context.Context, processHandle) (string, error) {
	return "", nil
}

func getProcessCommand(context.Context, processHandle) (*commandMetadata, error) {
	return nil, nil
}
