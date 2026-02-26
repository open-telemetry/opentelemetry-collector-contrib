// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package cpuscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"

import (
	"github.com/shirou/gopsutil/v4/cpu"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/ucal"
)

func (s *cpuScraper) recordCPUTimeStateDataPoints(now pcommon.Timestamp, cpuTime cpu.TimesStat, socket, core string) {
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.User, cpuTime.CPU, metadata.AttributeStateUser, socket, core)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.System, cpuTime.CPU, metadata.AttributeStateSystem, socket, core)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Idle, cpuTime.CPU, metadata.AttributeStateIdle, socket, core)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Irq, cpuTime.CPU, metadata.AttributeStateInterrupt, socket, core)
}

func (s *cpuScraper) recordCPUUtilization(now pcommon.Timestamp, cpuUtilization ucal.CPUUtilization, socket, core string) {
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.User, cpuUtilization.CPU, metadata.AttributeStateUser, socket, core)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.System, cpuUtilization.CPU, metadata.AttributeStateSystem, socket, core)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Idle, cpuUtilization.CPU, metadata.AttributeStateIdle, socket, core)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Irq, cpuUtilization.CPU, metadata.AttributeStateInterrupt, socket, core)
}

func getCPUInfo() ([]cpuInfo, error) {
	var cpuInfos []cpuInfo
	return cpuInfos, nil
}
