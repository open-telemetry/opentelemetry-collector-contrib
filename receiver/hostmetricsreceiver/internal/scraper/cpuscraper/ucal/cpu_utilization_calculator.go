// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ucal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/ucal"

import (
	"errors"
	"fmt"

	"github.com/shirou/gopsutil/v4/cpu"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

var ErrTimeStatNotFound = errors.New("cannot find TimesStat for cpu")

// CPUUtilization stores the utilization percents [0-1] for the different cpu states
type CPUUtilization struct {
	CPU     string
	User    float64
	System  float64
	Idle    float64
	Nice    float64
	Iowait  float64
	Irq     float64
	Softirq float64
	Steal   float64
}

// CPUUtilizationCalculator calculates the cpu utilization percents for the different cpu states
// It requires 2 []cpu.TimesStat and spend time to be able to calculate the difference
type CPUUtilizationCalculator struct {
	previousCPUTimes []cpu.TimesStat
	logger           *zap.Logger
}

func NewCPUUtilizationCalculator(logger *zap.Logger) *CPUUtilizationCalculator {
	c := &CPUUtilizationCalculator{logger: logger}
	if logger == nil {
		c.logger = zap.NewNop()
	}
	return c
}

// CalculateAndRecord calculates the cpu utilization for the different cpu states comparing previously
// stored []cpu.TimesStat and time.Time and current []cpu.TimesStat and current time.Time
// If no previous data is stored it will return empty slice of CPUUtilization and no error
func (c *CPUUtilizationCalculator) CalculateAndRecord(now pcommon.Timestamp, cpuTimes []cpu.TimesStat, recorder func(pcommon.Timestamp, CPUUtilization)) {
	// If this is not the first scrape, we are able to calculate the deltas
	// and report utilization values.
	if c.previousCPUTimes != nil {
		for _, previousCPUTime := range c.previousCPUTimes {
			// Check the CPU times from this scrape for the new times of the current CPU.
			currentCPUTime, err := cpuTimeForCPU(previousCPUTime.CPU, cpuTimes)

			// If the current scrape does not have a times entry for a previously seen CPU,
			// then we have to skip it. We can only report a utilization value for a core
			// if we have a time for it within the previous scrape interval. Our only choice
			// is to not report a value for that core this scrape.
			//
			// In some scenarios, such as CPU Hotplugging or LXC container reconfiguration,
			// CPU cores that used to be present won't be anymore. Sometimes the entire set
			// of CPUs changes, and in that scenario no points will end up being recorded
			// for this scrape.
			//
			// We opt just to log a warning so the user can see what happened, and continue.
			// We want this to be a warning rather than an error because this can be a perfectly
			// reasonable occurence and the Collector has no way to know if that's the case.
			if err != nil {
				c.logger.Warn(
					"could not get time for cpu, utilization will not be recorded",
					zap.Error(fmt.Errorf("%s: %w", previousCPUTime.CPU, err)),
				)
				continue
			}

			// If we have a previous time and current time for the CPU, record the delta
			// and record the metric for this core with the utilization calculation.
			recorder(now, cpuUtilization(previousCPUTime, currentCPUTime))
		}
	}

	// Update the previous scrape times. This has to be done every scrape. We need to ensure
	// we always report the same time delta in our utilization calculations for the lifetime
	// of this timeseries. So even if a core was missed for some reason, we can't hold it and
	// try to read later. We have to opt not to report it for this scrape, and maybe it comes
	// back in future scrapes or maybe it doesn't.
	c.previousCPUTimes = cpuTimes
}

// cpuUtilization calculates the difference between 2 cpu.TimesStat using spent time between them
func cpuUtilization(timeStart, timeEnd cpu.TimesStat) CPUUtilization {
	elapsedSeconds := totalCPU(timeEnd) - totalCPU(timeStart)
	if elapsedSeconds <= 0 {
		return CPUUtilization{CPU: timeStart.CPU}
	}
	return CPUUtilization{
		CPU:     timeStart.CPU,
		User:    (timeEnd.User - timeStart.User) / elapsedSeconds,
		System:  (timeEnd.System - timeStart.System) / elapsedSeconds,
		Idle:    (timeEnd.Idle - timeStart.Idle) / elapsedSeconds,
		Nice:    (timeEnd.Nice - timeStart.Nice) / elapsedSeconds,
		Iowait:  (timeEnd.Iowait - timeStart.Iowait) / elapsedSeconds,
		Irq:     (timeEnd.Irq - timeStart.Irq) / elapsedSeconds,
		Softirq: (timeEnd.Softirq - timeStart.Softirq) / elapsedSeconds,
		Steal:   (timeEnd.Steal - timeStart.Steal) / elapsedSeconds,
	}
}

// cpuTimeForCPU returns cpu.TimesStat from a slice of cpu.TimesStat based on CPU
// If CPU is not found and error will be returned
func cpuTimeForCPU(cpuNum string, times []cpu.TimesStat) (cpu.TimesStat, error) {
	for _, t := range times {
		if t.CPU == cpuNum {
			return t, nil
		}
	}
	return cpu.TimesStat{}, fmt.Errorf("cpu %s : %w", cpuNum, ErrTimeStatNotFound)
}

// totalCPU returns the total CPU time across all states. On Linux, /proc/stat's
// user column already includes guest time and nice already includes guest_nice
// (the kernel increments both CPUTIME_USER and CPUTIME_GUEST in account_guest_time:
// https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git/tree/kernel/sched/cputime.c#n150),
// so Guest and GuestNice must not be added again to avoid double-counting.
func totalCPU(c cpu.TimesStat) float64 {
	return c.User + c.System + c.Idle + c.Nice + c.Iowait + c.Irq +
		c.Softirq + c.Steal
}
