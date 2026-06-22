// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package cpuscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/procfs"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/tklauser/go-sysconf"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	hostmetricsmetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/precision"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/cputicks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/ucal"
)

// USER_HZ: hardcoded at 100 in the Linux kernel since 2.6 for /proc/stat ABI stability.
const defaultTicksPerSecond = 100

// tickReader reads per-CPU tick counts from the operating system.
// Defined at the consumer for testability.
type tickReader interface {
	ReadAll() ([]cputicks.Stat, error)
	TicksPerSecond() uint64
}

func clockTicksPerSecond() uint64 {
	clkTck, err := sysconf.Sysconf(sysconf.SC_CLK_TCK)
	if err != nil || clkTck <= 0 {
		return defaultTicksPerSecond
	}
	return uint64(clkTck)
}

func newCPUEmitter(cfg *Config) func(context.Context, pcommon.Timestamp, *metadata.MetricsBuilder) error {
	if hostmetricsmetadata.ReceiverHostmetricsreceiverUseCPUTicksFeatureGate.IsEnabled() {
		return newCputicksEmitter(cputicks.NewReader(cfg.rootPath, clockTicksPerSecond()))
	}
	return newGopsutilEmitter(cpu.TimesWithContext)
}

func newCputicksEmitter(reader tickReader) func(context.Context, pcommon.Timestamp, *metadata.MetricsBuilder) error {
	tickDuration := time.Second / time.Duration(reader.TicksPerSecond())
	var prevTicks map[string]cputicks.Stat
	return func(_ context.Context, now pcommon.Timestamp, mb *metadata.MetricsBuilder) error {
		ticks, err := reader.ReadAll()
		if err != nil {
			return err
		}

		recordTickTimes(now, ticks, tickDuration, mb)

		if prevTicks != nil {
			currTicks := make(map[string]cputicks.Stat, len(ticks))
			for _, t := range ticks {
				currTicks[t.CPU] = t
			}
			for _, prev := range prevTicks {
				curr, ok := currTicks[prev.CPU]
				if !ok {
					return fmt.Errorf("getting ticks for cpu %s: %w", prev.CPU, ucal.ErrTimeStatNotFound)
				}
				recordTickUtilization(now, prev, curr, mb)
			}
		}
		prevTicks = make(map[string]cputicks.Stat, len(ticks))
		for _, t := range ticks {
			prevTicks[t.CPU] = t
		}
		return nil
	}
}

func recordTickTimes(now pcommon.Timestamp, ticks []cputicks.Stat, tickDuration time.Duration, mb *metadata.MetricsBuilder) {
	for _, t := range ticks {
		mb.RecordSystemCPUTimeDataPoint(now, precision.Scale(t.User, tickDuration), t.CPU, metadata.AttributeStateUser)
		mb.RecordSystemCPUTimeDataPoint(now, precision.Scale(t.System, tickDuration), t.CPU, metadata.AttributeStateSystem)
		mb.RecordSystemCPUTimeDataPoint(now, precision.Scale(t.Idle, tickDuration), t.CPU, metadata.AttributeStateIdle)
		mb.RecordSystemCPUTimeDataPoint(now, precision.Scale(t.Irq, tickDuration), t.CPU, metadata.AttributeStateInterrupt)
		mb.RecordSystemCPUTimeDataPoint(now, precision.Scale(t.Nice, tickDuration), t.CPU, metadata.AttributeStateNice)
		mb.RecordSystemCPUTimeDataPoint(now, precision.Scale(t.Softirq, tickDuration), t.CPU, metadata.AttributeStateSoftirq)
		mb.RecordSystemCPUTimeDataPoint(now, precision.Scale(t.Steal, tickDuration), t.CPU, metadata.AttributeStateSteal)
		mb.RecordSystemCPUTimeDataPoint(now, precision.Scale(t.Iowait, tickDuration), t.CPU, metadata.AttributeStateWait)
	}
}

func recordTickUtilization(now pcommon.Timestamp, prev, curr cputicks.Stat, mb *metadata.MetricsBuilder) {
	deltaTotal := curr.Total() - prev.Total()
	if deltaTotal == 0 {
		recordZeroUtilization(now, curr.CPU, mb)
		return
	}
	mb.RecordSystemCPUUtilizationDataPoint(now, precision.Ratio(curr.User-prev.User, deltaTotal), curr.CPU, metadata.AttributeStateUser)
	mb.RecordSystemCPUUtilizationDataPoint(now, precision.Ratio(curr.System-prev.System, deltaTotal), curr.CPU, metadata.AttributeStateSystem)
	mb.RecordSystemCPUUtilizationDataPoint(now, precision.Ratio(curr.Idle-prev.Idle, deltaTotal), curr.CPU, metadata.AttributeStateIdle)
	mb.RecordSystemCPUUtilizationDataPoint(now, precision.Ratio(curr.Irq-prev.Irq, deltaTotal), curr.CPU, metadata.AttributeStateInterrupt)
	mb.RecordSystemCPUUtilizationDataPoint(now, precision.Ratio(curr.Nice-prev.Nice, deltaTotal), curr.CPU, metadata.AttributeStateNice)
	mb.RecordSystemCPUUtilizationDataPoint(now, precision.Ratio(curr.Softirq-prev.Softirq, deltaTotal), curr.CPU, metadata.AttributeStateSoftirq)
	mb.RecordSystemCPUUtilizationDataPoint(now, precision.Ratio(curr.Steal-prev.Steal, deltaTotal), curr.CPU, metadata.AttributeStateSteal)
	mb.RecordSystemCPUUtilizationDataPoint(now, precision.Ratio(curr.Iowait-prev.Iowait, deltaTotal), curr.CPU, metadata.AttributeStateWait)
}

func recordZeroUtilization(now pcommon.Timestamp, cpuName string, mb *metadata.MetricsBuilder) {
	for _, state := range []metadata.AttributeState{
		metadata.AttributeStateUser,
		metadata.AttributeStateSystem,
		metadata.AttributeStateIdle,
		metadata.AttributeStateInterrupt,
		metadata.AttributeStateNice,
		metadata.AttributeStateSoftirq,
		metadata.AttributeStateSteal,
		metadata.AttributeStateWait,
	} {
		mb.RecordSystemCPUUtilizationDataPoint(now, 0, cpuName, state)
	}
}

func recordCPUTimeStateDataPoints(now pcommon.Timestamp, cpuTime cpu.TimesStat, mb *metadata.MetricsBuilder) {
	mb.RecordSystemCPUTimeDataPoint(now, cpuTime.User, cpuTime.CPU, metadata.AttributeStateUser)
	mb.RecordSystemCPUTimeDataPoint(now, cpuTime.System, cpuTime.CPU, metadata.AttributeStateSystem)
	mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Idle, cpuTime.CPU, metadata.AttributeStateIdle)
	mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Irq, cpuTime.CPU, metadata.AttributeStateInterrupt)
	mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Nice, cpuTime.CPU, metadata.AttributeStateNice)
	mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Softirq, cpuTime.CPU, metadata.AttributeStateSoftirq)
	mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Steal, cpuTime.CPU, metadata.AttributeStateSteal)
	mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Iowait, cpuTime.CPU, metadata.AttributeStateWait)
}

func recordCPUUtilization(now pcommon.Timestamp, cpuUtilization ucal.CPUUtilization, mb *metadata.MetricsBuilder) {
	mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.User, cpuUtilization.CPU, metadata.AttributeStateUser)
	mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.System, cpuUtilization.CPU, metadata.AttributeStateSystem)
	mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Idle, cpuUtilization.CPU, metadata.AttributeStateIdle)
	mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Irq, cpuUtilization.CPU, metadata.AttributeStateInterrupt)
	mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Nice, cpuUtilization.CPU, metadata.AttributeStateNice)
	mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Softirq, cpuUtilization.CPU, metadata.AttributeStateSoftirq)
	mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Steal, cpuUtilization.CPU, metadata.AttributeStateSteal)
	mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Iowait, cpuUtilization.CPU, metadata.AttributeStateWait)
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
