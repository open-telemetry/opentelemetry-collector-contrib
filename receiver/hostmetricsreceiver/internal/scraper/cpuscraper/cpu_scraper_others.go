// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package cpuscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"

import (
	"context"

	"github.com/shirou/gopsutil/v4/cpu"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/ucal"
)

func newCPUEmitter(_ *Config) func(context.Context, pcommon.Timestamp, *metadata.MetricsBuilder) error {
	return newGopsutilEmitter(cpu.TimesWithContext)
}

func recordCPUTimeStateDataPoints(now pcommon.Timestamp, cpuTime cpu.TimesStat, mb *metadata.MetricsBuilder) {
	mb.RecordSystemCPUTimeDataPoint(now, cpuTime.User, cpuTime.CPU, metadata.AttributeStateUser)
	mb.RecordSystemCPUTimeDataPoint(now, cpuTime.System, cpuTime.CPU, metadata.AttributeStateSystem)
	mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Idle, cpuTime.CPU, metadata.AttributeStateIdle)
	mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Irq, cpuTime.CPU, metadata.AttributeStateInterrupt)
}

func recordCPUUtilization(now pcommon.Timestamp, cpuUtilization ucal.CPUUtilization, mb *metadata.MetricsBuilder) {
	mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.User, cpuUtilization.CPU, metadata.AttributeStateUser)
	mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.System, cpuUtilization.CPU, metadata.AttributeStateSystem)
	mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Idle, cpuUtilization.CPU, metadata.AttributeStateIdle)
	mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Irq, cpuUtilization.CPU, metadata.AttributeStateInterrupt)
}

func (*cpuScraper) getCPUInfo() ([]cpuInfo, error) {
	var cpuInfos []cpuInfo
	return cpuInfos, nil
}
