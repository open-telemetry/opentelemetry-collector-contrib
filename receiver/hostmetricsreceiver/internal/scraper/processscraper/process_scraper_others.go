// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !windows && !darwin
// +build !linux,!windows,!darwin

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"github.com/shirou/gopsutil/v3/cpu"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/ucal"
)

func (s *scraper) recordCPUTimeMetric(now pcommon.Timestamp, cpuTime *cpu.TimesStat) {}

func (s *scraper) recordCPUUtilization(now pcommon.Timestamp, cpuUtilization ucal.CPUUtilization) {}

func getProcessName(processHandle, string) (string, error) {
	return nil, nil
}

func getProcessCwd(proc processHandle) (string, error) {
	return nil, nil
}

func getProcessExecutable(processHandle) (string, error) {
	return nil, nil
}

func getProcessCommand(processHandle) (*commandMetadata, error) {
	return nil, nil
}
