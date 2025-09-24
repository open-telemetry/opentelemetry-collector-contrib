// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package cpuscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"

import (
	"github.com/prometheus/procfs"
	"github.com/shirou/gopsutil/v4/cpu"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/ucal"
)

func (s *cpuScraper) recordCPUTimeStateDataPoints(now pcommon.Timestamp, cpuTime cpu.TimesStat) {
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.User, cpuTime.CPU, metadata.AttributeStateUser)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.System, cpuTime.CPU, metadata.AttributeStateSystem)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Idle, cpuTime.CPU, metadata.AttributeStateIdle)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Irq, cpuTime.CPU, metadata.AttributeStateInterrupt)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Nice, cpuTime.CPU, metadata.AttributeStateNice)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Softirq, cpuTime.CPU, metadata.AttributeStateSoftirq)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Steal, cpuTime.CPU, metadata.AttributeStateSteal)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Iowait, cpuTime.CPU, metadata.AttributeStateWait)
}

func (s *cpuScraper) recordCPUUtilization(now pcommon.Timestamp, cpuUtilization ucal.CPUUtilization) {
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.User, cpuUtilization.CPU, metadata.AttributeStateUser)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.System, cpuUtilization.CPU, metadata.AttributeStateSystem)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Idle, cpuUtilization.CPU, metadata.AttributeStateIdle)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Irq, cpuUtilization.CPU, metadata.AttributeStateInterrupt)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Nice, cpuUtilization.CPU, metadata.AttributeStateNice)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Softirq, cpuUtilization.CPU, metadata.AttributeStateSoftirq)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Steal, cpuUtilization.CPU, metadata.AttributeStateSteal)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Iowait, cpuUtilization.CPU, metadata.AttributeStateWait)
}

func (*cpuScraper) getCPUInfo() ([]cpuInfo, error) {
	var cpuInfos []cpuInfo
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return nil, scrapererror.NewPartialScrapeError(err, metricsLen)
	}
	cInf, err := fs.CPUInfo()
	if err != nil {
		return nil, scrapererror.NewPartialScrapeError(err, metricsLen)
	}
	for i := range cInf {
		cInfo := &cInf[i]
		c := cpuInfo{
			frequency: cInfo.CPUMHz,
			processor: cInfo.Processor,
		}
		cpuInfos = append(cpuInfos, c)
	}
	return cpuInfos, nil
}
