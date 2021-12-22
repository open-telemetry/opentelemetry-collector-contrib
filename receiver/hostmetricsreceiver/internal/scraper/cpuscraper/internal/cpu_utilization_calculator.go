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

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal"

import (
	"errors"
	"fmt"

	"github.com/shirou/gopsutil/v3/cpu"
	"go.opentelemetry.io/collector/model/pdata"
)

var ErrInvalidElapsed = errors.New("invalid elapsed seconds")
var ErrDifferentCPUs = errors.New("cannot compare times from different cpus")
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
	previousTime     pdata.Timestamp
	previousCPUTimes []cpu.TimesStat
}

// Calculate calculates the cpu utilization for the different cpu states comparing previously
// stored []cpu.TimesStat and time.Time and current []cpu.TimesStat and current time.Time
// If no previous data is stored it will return empty slice of CPUUtilization and no error
func (c *CPUUtilizationCalculator) Calculate(now pdata.Timestamp, cpuTimes []cpu.TimesStat) ([]CPUUtilization, error) {
	var utilizationByCPU []CPUUtilization
	elapsedSeconds := now.AsTime().Sub(c.previousTime.AsTime()).Seconds()

	if c.previousCPUTimes != nil {
		for _, previousCPUTime := range c.previousCPUTimes {
			currentCPUTime, err := cpuTimeForCPU(previousCPUTime.CPU, cpuTimes)
			if err != nil {
				return nil, fmt.Errorf("getting time for cpu %s: %w", previousCPUTime.CPU, err)
			}
			utilization, err := cpuUtilization(previousCPUTime, currentCPUTime, elapsedSeconds)
			if err != nil {
				return nil, fmt.Errorf("getting utilization for cpu %s: %w", previousCPUTime.CPU, err)
			}
			utilizationByCPU = append(utilizationByCPU, utilization)
		}
	}
	c.previousCPUTimes = cpuTimes
	c.previousTime = now

	return utilizationByCPU, nil
}

// cpuUtilization calculates the difference between 2 cpu.TimesStat using spent time between them
// If no time was spent between TimesStat an error will be returned
// If TimesStats do not belog to same CPU an error will be returned
func cpuUtilization(timeStart cpu.TimesStat, timeEnd cpu.TimesStat, elapsedSeconds float64) (CPUUtilization, error) {
	if elapsedSeconds <= 0 {
		return CPUUtilization{}, fmt.Errorf("%f: %w", elapsedSeconds, ErrInvalidElapsed)
	}
	if timeStart.CPU != timeEnd.CPU {
		return CPUUtilization{}, fmt.Errorf("%s <> %s: %w", timeStart.CPU, timeEnd.CPU, ErrDifferentCPUs)
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
	}, nil
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
