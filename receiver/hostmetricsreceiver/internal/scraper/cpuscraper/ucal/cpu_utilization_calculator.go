// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ucal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/ucal"

import (
	"errors"
	"fmt"

	"github.com/shirou/gopsutil/v3/cpu"
	"go.opentelemetry.io/collector/pdata/pcommon"
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
}

// CalculateAndRecord calculates the cpu utilization for the different cpu states comparing previously
// stored []cpu.TimesStat and time.Time and current []cpu.TimesStat and current time.Time
// If no previous data is stored it will return empty slice of CPUUtilization and no error
func (c *CPUUtilizationCalculator) CalculateAndRecord(now pcommon.Timestamp, cpuTimes []cpu.TimesStat, recorder func(pcommon.Timestamp, CPUUtilization)) error {
	if c.previousCPUTimes != nil {
		for _, previousCPUTime := range c.previousCPUTimes {
			currentCPUTime, err := cpuTimeForCPU(previousCPUTime.CPU, cpuTimes)
			if err != nil {
				return fmt.Errorf("getting time for cpu %s: %w", previousCPUTime.CPU, err)
			}
			recorder(now, cpuUtilization(previousCPUTime, currentCPUTime))
		}
	}
	c.previousCPUTimes = cpuTimes

	return nil
}

// cpuUtilization calculates the difference between 2 cpu.TimesStat using spent time between them
func cpuUtilization(timeStart cpu.TimesStat, timeEnd cpu.TimesStat) CPUUtilization {
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

// Copied from cpu.TimesStat.Total(), since that func is deprecated.
func totalCPU(c cpu.TimesStat) float64 {
	total := c.User + c.System + c.Idle + c.Nice + c.Iowait + c.Irq +
		c.Softirq + c.Steal + c.Guest + c.GuestNice

	return total
}
