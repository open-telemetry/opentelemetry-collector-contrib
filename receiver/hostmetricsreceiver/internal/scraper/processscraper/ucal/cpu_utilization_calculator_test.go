// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ucal

import (
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type inMemoryRecorder struct {
	cpuUtilization CPUUtilization
}

func (r *inMemoryRecorder) record(_ pcommon.Timestamp, utilization CPUUtilization) {
	r.cpuUtilization = utilization
}

func TestCpuUtilizationCalculator_Calculate(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name                string
		logicalCores        int
		currentReadTime     pcommon.Timestamp
		currentCPUStat      *cpu.TimesStat
		previousReadTime    pcommon.Timestamp
		previousCPUStat     *cpu.TimesStat
		expectedUtilization *CPUUtilization
		shouldError         bool
	}{
		{
			name:         "no previous times",
			logicalCores: 1,
			currentCPUStat: &cpu.TimesStat{
				User: 8260.4,
			},
			expectedUtilization: &CPUUtilization{},
		},
		{
			name:             "no delta time should return utilization=0",
			logicalCores:     1,
			previousReadTime: 1640097430772858000,
			currentReadTime:  1640097430772858000,
			previousCPUStat: &cpu.TimesStat{
				User: 8259.4,
			},
			currentCPUStat: &cpu.TimesStat{
				User: 8259.5,
			},
			expectedUtilization: &CPUUtilization{},
		},
		{
			name:             "one second time delta",
			logicalCores:     1,
			previousReadTime: 1640097430772858000,
			currentReadTime:  1640097431772858000,
			previousCPUStat: &cpu.TimesStat{
				User:   8258.4,
				System: 6193.3,
				Iowait: 34.201,
			},
			currentCPUStat: &cpu.TimesStat{
				User:   8258.5,
				System: 6193.6,
				Iowait: 34.202,
			},
			expectedUtilization: &CPUUtilization{
				User:   0.1,
				System: 0.3,
				Iowait: 0.001,
			},
		},
		{
			name:             "one second time delta, 2 logical cores, normalized",
			logicalCores:     2,
			previousReadTime: 1640097430772858000,
			currentReadTime:  1640097431772858000,
			previousCPUStat: &cpu.TimesStat{
				User:   8258.4,
				System: 6193.3,
				Iowait: 34.201,
			},
			currentCPUStat: &cpu.TimesStat{
				User:   8258.5,
				System: 6193.6,
				Iowait: 34.202,
			},
			expectedUtilization: &CPUUtilization{
				User:   0.05,
				System: 0.15,
				Iowait: 0.0005,
			},
		},
		{
			name:             "0 logical cores",
			logicalCores:     0,
			previousReadTime: 1640097430772858000,
			currentReadTime:  1640097431772858000,
			previousCPUStat: &cpu.TimesStat{
				User:   8258.4,
				System: 6193.3,
				Iowait: 34.201,
			},
			currentCPUStat: &cpu.TimesStat{
				User:   8258.5,
				System: 6193.6,
				Iowait: 34.202,
			},
			shouldError: true,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			recorder := inMemoryRecorder{}
			calculator := CPUUtilizationCalculator{
				previousReadTime: test.previousReadTime,
				previousCPUStats: test.previousCPUStat,
			}
			err := calculator.CalculateAndRecord(test.currentReadTime, test.logicalCores, test.currentCPUStat, recorder.record)
			if test.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.InDelta(t, test.expectedUtilization.System, recorder.cpuUtilization.System, 0.00001)
				assert.InDelta(t, test.expectedUtilization.User, recorder.cpuUtilization.User, 0.00001)
				assert.InDelta(t, test.expectedUtilization.Iowait, recorder.cpuUtilization.Iowait, 0.00001)
			}
		})
	}
}

func Test_cpuUtilization(t *testing.T) {
	startTime := pcommon.Timestamp(1640097435776827000)
	halfSecondLater := pcommon.Timestamp(uint64(startTime) + uint64(time.Second.Nanoseconds()/2))
	startStat := &cpu.TimesStat{
		User:   1.5,
		System: 2.702,
		Iowait: 0.888,
	}
	endStat := &cpu.TimesStat{
		User:   1.5124,
		System: 3.004,
		Iowait: 0.9,
	}
	expectedUtilization := CPUUtilization{
		User:   0.0248,
		System: 0.604,
		Iowait: 0.024,
	}

	actualUtilization := cpuUtilization(1, startStat, startTime, endStat, halfSecondLater)
	assert.InDelta(t, expectedUtilization.User, actualUtilization.User, 0.00001)
	assert.InDelta(t, expectedUtilization.System, actualUtilization.System, 0.00001)
	assert.InDelta(t, expectedUtilization.Iowait, actualUtilization.Iowait, 0.00001)
}
