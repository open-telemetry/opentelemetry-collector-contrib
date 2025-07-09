// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"

	"github.com/shirou/gopsutil/v4/cpu"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/ucal"
)

func (s *processScraper) recordCPUTimeMetric(now pcommon.Timestamp, cpuTime *cpu.TimesStat) {
	s.mb.RecordProcessCPUTimeDataPoint(now, cpuTime.User, metadata.AttributeStateUser)
	s.mb.RecordProcessCPUTimeDataPoint(now, cpuTime.System, metadata.AttributeStateSystem)
}

func (s *processScraper) recordCPUUtilization(now pcommon.Timestamp, cpuUtilization ucal.CPUUtilization) {
	s.mb.RecordProcessCPUUtilizationDataPoint(now, cpuUtilization.User, metadata.AttributeStateUser)
	s.mb.RecordProcessCPUUtilizationDataPoint(now, cpuUtilization.System, metadata.AttributeStateSystem)
}

func getProcessName(_ context.Context, _ processHandle, exePath string) (string, error) {
	if exePath == "" {
		return "", errors.New("executable path is empty")
	}

	return filepath.Base(exePath), nil
}

func getProcessCgroup(_ context.Context, _ processHandle) (string, error) {
	return "", nil
}

func getProcessExecutable(ctx context.Context, proc processHandle) (string, error) {
	exe, err := proc.ExeWithContext(ctx)
	if err != nil {
		return "", err
	}

	return exe, nil
}

// matches the first argument before an unquoted space or slash
var cmdRegex = regexp.MustCompile(`^((?:[^"]*?"[^"]*?")*?[^"]*?)(?:[ \/]|$)`)

func getProcessCommand(ctx context.Context, proc processHandle) (*commandMetadata, error) {
	cmdline, err := proc.CmdlineWithContext(ctx)
	if err != nil {
		return nil, err
	}

	cmd := cmdline
	match := cmdRegex.FindStringSubmatch(cmdline)
	if match != nil {
		cmd = match[1]
	}

	command := &commandMetadata{command: cmd, commandLine: cmdline}
	return command, nil
}
