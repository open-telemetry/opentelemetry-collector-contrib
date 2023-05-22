// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ucal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/ucal"

import (
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// CPUUtilization stores the utilization percents [0-1] for the different cpu states
type CPUUtilization struct {
	User   float64
	System float64
	Iowait float64
}

// CPUUtilizationCalculator calculates the cpu utilization percents for the different cpu states
// It requires 2 []cpu.TimesStat and spend time to be able to calculate the difference
type CPUUtilizationCalculator struct {
	previousCPUStats *cpu.TimesStat
	previousReadTime pcommon.Timestamp
}

// CalculateAndRecord calculates the cpu utilization for the different cpu states comparing previously
// stored []cpu.TimesStat and time.Time and current []cpu.TimesStat and current time.Time
// If no previous data is stored it will return empty slice of CPUUtilization and no error
func (c *CPUUtilizationCalculator) CalculateAndRecord(now pcommon.Timestamp, currentCPUStats *cpu.TimesStat, recorder func(pcommon.Timestamp, CPUUtilization)) error {
	if c.previousCPUStats != nil {
		recorder(now, cpuUtilization(c.previousCPUStats, c.previousReadTime, currentCPUStats, now))
	}
	c.previousCPUStats = currentCPUStats
	c.previousReadTime = now

	return nil
}

// cpuUtilization calculates the difference between 2 cpu.TimesStat using spent time between them
func cpuUtilization(startStats *cpu.TimesStat, startTime pcommon.Timestamp, endStats *cpu.TimesStat, endTime pcommon.Timestamp) CPUUtilization {
	elapsedTime := time.Duration(endTime - startTime).Seconds()
	if elapsedTime <= 0 {
		return CPUUtilization{}
	}
	return CPUUtilization{
		User:   (endStats.User - startStats.User) / elapsedTime,
		System: (endStats.System - startStats.System) / elapsedTime,
		Iowait: (endStats.Iowait - startStats.Iowait) / elapsedTime,
	}
}
