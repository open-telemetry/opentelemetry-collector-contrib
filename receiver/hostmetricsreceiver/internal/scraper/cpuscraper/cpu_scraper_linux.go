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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/precision"
	hostmetricsmetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/ucal"
)

func newCPUEmitter() func(context.Context, pcommon.Timestamp, *metadata.MetricsBuilder) error {
	if hostmetricsmetadata.ReceiverHostmetricsreceiverUseCPUTicksFeatureGate.IsEnabled() {
		return newCputicksEmitter(cpu.NewExLinux().TimesWithContext)
	}
	return newGopsutilEmitter(cpu.TimesWithContext)
}

func newCputicksEmitter(readTicks func(context.Context, bool) ([]cpu.ExTimesStat, error)) func(context.Context, pcommon.Timestamp, *metadata.MetricsBuilder) error {
	tickDuration := time.Duration(float64(time.Second) / cpu.ClocksPerSec)
	var prevTicks map[string]cpu.ExTimesStat
	return func(ctx context.Context, now pcommon.Timestamp, mb *metadata.MetricsBuilder) error {
		ticks, err := readTicks(ctx, true)
		if err != nil {
			return err
		}

		recordTickTimes(now, ticks, tickDuration, mb)

		if prevTicks != nil {
			currTicks := make(map[string]cpu.ExTimesStat, len(ticks))
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
		prevTicks = make(map[string]cpu.ExTimesStat, len(ticks))
		for _, t := range ticks {
			prevTicks[t.CPU] = t
		}
		return nil
	}
}

// tickDelta returns 0 when a counter decreases. The kernel can report a lower
// value than the previous read, in particular for iowait, and uint64
// subtraction would wrap around to a huge delta.
func tickDelta(curr, prev uint64) uint64 {
	if curr < prev {
		return 0
	}
	return curr - prev
}

func tickDeltas(prev, curr cpu.ExTimesStat) cpu.ExTimesStat {
	return cpu.ExTimesStat{
		CPU:     curr.CPU,
		User:    tickDelta(curr.User, prev.User),
		Nice:    tickDelta(curr.Nice, prev.Nice),
		System:  tickDelta(curr.System, prev.System),
		Idle:    tickDelta(curr.Idle, prev.Idle),
		Iowait:  tickDelta(curr.Iowait, prev.Iowait),
		Irq:     tickDelta(curr.Irq, prev.Irq),
		Softirq: tickDelta(curr.Softirq, prev.Softirq),
		Steal:   tickDelta(curr.Steal, prev.Steal),
	}
}

func recordTickTimes(now pcommon.Timestamp, ticks []cpu.ExTimesStat, tickDuration time.Duration, mb *metadata.MetricsBuilder) {
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

func recordTickUtilization(now pcommon.Timestamp, prev, curr cpu.ExTimesStat, mb *metadata.MetricsBuilder) {
	d := tickDeltas(prev, curr)
	deltaTotal := d.User + d.Nice + d.System + d.Idle + d.Iowait + d.Irq + d.Softirq + d.Steal
	if deltaTotal == 0 {
		recordZeroUtilization(now, curr.CPU, mb)
		return
	}
	total := float64(deltaTotal)
	mb.RecordSystemCPUUtilizationDataPoint(now, float64(d.User)/total, curr.CPU, metadata.AttributeStateUser)
	mb.RecordSystemCPUUtilizationDataPoint(now, float64(d.System)/total, curr.CPU, metadata.AttributeStateSystem)
	mb.RecordSystemCPUUtilizationDataPoint(now, float64(d.Idle)/total, curr.CPU, metadata.AttributeStateIdle)
	mb.RecordSystemCPUUtilizationDataPoint(now, float64(d.Irq)/total, curr.CPU, metadata.AttributeStateInterrupt)
	mb.RecordSystemCPUUtilizationDataPoint(now, float64(d.Nice)/total, curr.CPU, metadata.AttributeStateNice)
	mb.RecordSystemCPUUtilizationDataPoint(now, float64(d.Softirq)/total, curr.CPU, metadata.AttributeStateSoftirq)
	mb.RecordSystemCPUUtilizationDataPoint(now, float64(d.Steal)/total, curr.CPU, metadata.AttributeStateSteal)
	mb.RecordSystemCPUUtilizationDataPoint(now, float64(d.Iowait)/total, curr.CPU, metadata.AttributeStateWait)
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
