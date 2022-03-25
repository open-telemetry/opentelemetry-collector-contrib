// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ucal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/ucal"

import (
	"errors"
	"fmt"

	"github.com/shirou/gopsutil/v3/cpu"
	"go.opentelemetry.io/collector/model/pdata"
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
func (c *CPUUtilizationCalculator) CalculateAndRecord(now pdata.Timestamp, cpuTimes []cpu.TimesStat, recorder func(pdata.Timestamp, CPUUtilization)) error {
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
	totalSeconds := timeEnd.Total() - timeStart.Total()
	if totalSeconds <= 0 {
		return CPUUtilization{CPU: timeStart.CPU}
	}
	return CPUUtilization{
		CPU:     timeStart.CPU,
		User:    (timeEnd.User - timeStart.User) / totalSeconds,
		System:  (timeEnd.System - timeStart.System) / totalSeconds,
		Idle:    (timeEnd.Idle - timeStart.Idle) / totalSeconds,
		Nice:    (timeEnd.Nice - timeStart.Nice) / totalSeconds,
		Iowait:  (timeEnd.Iowait - timeStart.Iowait) / totalSeconds,
		Irq:     (timeEnd.Irq - timeStart.Irq) / totalSeconds,
		Softirq: (timeEnd.Softirq - timeStart.Softirq) / totalSeconds,
		Steal:   (timeEnd.Steal - timeStart.Steal) / totalSeconds,
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
