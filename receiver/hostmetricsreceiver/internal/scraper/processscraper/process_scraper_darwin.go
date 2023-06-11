// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin
// +build darwin

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"regexp"

	"github.com/shirou/gopsutil/v3/cpu"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/ucal"
)

func (s *scraper) recordCPUTimeMetric(now pcommon.Timestamp, cpuTime *cpu.TimesStat) {
	s.mb.RecordProcessCPUTimeDataPoint(now, cpuTime.User, metadata.AttributeStateUser)
	s.mb.RecordProcessCPUTimeDataPoint(now, cpuTime.System, metadata.AttributeStateSystem)
	s.mb.RecordProcessCPUTimeDataPoint(now, cpuTime.Iowait, metadata.AttributeStateWait)
}

func (s *scraper) recordCPUUtilization(now pcommon.Timestamp, cpuUtilization ucal.CPUUtilization) {
	s.mb.RecordProcessCPUUtilizationDataPoint(now, cpuUtilization.User, metadata.AttributeStateUser)
	s.mb.RecordProcessCPUUtilizationDataPoint(now, cpuUtilization.System, metadata.AttributeStateSystem)
	s.mb.RecordProcessCPUUtilizationDataPoint(now, cpuUtilization.Iowait, metadata.AttributeStateWait)
}

func getProcessName(proc processHandle, _ string) (string, error) {
	name, err := proc.Name()
	if err != nil {
		return "", err
	}

	return name, nil
}

func getProcessCwd(proc processHandle) (string, error) {
	cwd, err := proc.Cwd()
	if err != nil {
		return "", err
	}

	return cwd, nil
}

func getProcessExecutable(proc processHandle) (string, error) {
	cmdline, err := proc.Cmdline()
	if err != nil {
		return "", err
	}
	regex := regexp.MustCompile(`^\S+`)
	exe := regex.FindString(cmdline)

	return exe, nil
}

func getProcessCommand(proc processHandle) (*commandMetadata, error) {
	cmdline, err := proc.Cmdline()
	if err != nil {
		return nil, err
	}

	command := &commandMetadata{command: cmdline, commandLine: cmdline}
	return command, nil

}
