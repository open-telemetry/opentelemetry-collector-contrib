// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin || freebsd

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"context"
	"regexp"

	"github.com/shirou/gopsutil/v4/cpu"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/ucal"
)

func (s *processScraper) recordCPUTimeMetric(now pcommon.Timestamp, cpuTime *cpu.TimesStat) {
	s.mb.RecordProcessCPUTimeDataPoint(now, cpuTime.User, metadata.AttributeStateUser, metadata.AttributeCPUModeUser)
	s.mb.RecordProcessCPUTimeDataPoint(now, cpuTime.System, metadata.AttributeStateSystem, metadata.AttributeCPUModeSystem)
	s.mb.RecordProcessCPUTimeDataPoint(now, cpuTime.Iowait, metadata.AttributeStateWait, metadata.AttributeCPUModeIowait)
}

func (s *processScraper) recordCPUUtilization(now pcommon.Timestamp, cpuUtilization ucal.CPUUtilization) {
	s.mb.RecordProcessCPUUtilizationDataPoint(now, cpuUtilization.User, metadata.AttributeStateUser, metadata.AttributeCPUModeUser)
	s.mb.RecordProcessCPUUtilizationDataPoint(now, cpuUtilization.System, metadata.AttributeStateSystem, metadata.AttributeCPUModeSystem)
	s.mb.RecordProcessCPUUtilizationDataPoint(now, cpuUtilization.Iowait, metadata.AttributeStateWait, metadata.AttributeCPUModeIowait)
}

func getProcessName(ctx context.Context, proc processHandle, _ string) (string, error) {
	name, err := proc.NameWithContext(ctx)
	if err != nil {
		return "", err
	}

	return name, nil
}

func getProcessCgroup(_ context.Context, _ processHandle) (string, error) {
	return "", nil
}

func getProcessExecutable(ctx context.Context, proc processHandle) (string, error) {
	cmdline, err := proc.CmdlineWithContext(ctx)
	if err != nil {
		return "", err
	}
	regex := regexp.MustCompile(`^\S+`)
	exe := regex.FindString(cmdline)

	return exe, nil
}

func getProcessCommand(ctx context.Context, proc processHandle) (*commandMetadata, error) {
	cmdline, err := proc.CmdlineWithContext(ctx)
	if err != nil {
		return nil, err
	}

	command := &commandMetadata{command: cmdline, commandLine: cmdline}
	return command, nil
}
