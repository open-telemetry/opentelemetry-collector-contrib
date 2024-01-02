// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ucal

import (
	"testing"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
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
		currentReadTime     pcommon.Timestamp
		currentCPUStat      *cpu.TimesStat
		previousReadTime    pcommon.Timestamp
		previousCPUStat     *cpu.TimesStat
		expectedUtilization *CPUUtilization
	}{
		{
			name: "no previous times",
			currentCPUStat: &cpu.TimesStat{
				User: 8260.4,
			},
			expectedUtilization: &CPUUtilization{},
		},
		{
			name:             "no delta time should return utilization=0",
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
	}
	for _, test := range testCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			recorder := inMemoryRecorder{}
			calculator := CPUUtilizationCalculator{
				previousReadTime: test.previousReadTime,
				previousCPUStats: test.previousCPUStat,
			}
			err := calculator.CalculateAndRecord(test.currentReadTime, test.currentCPUStat, recorder.record)
			assert.NoError(t, err)
			assert.InDelta(t, test.expectedUtilization.System, recorder.cpuUtilization.System, 0.00001)
			assert.InDelta(t, test.expectedUtilization.User, recorder.cpuUtilization.User, 0.00001)
			assert.InDelta(t, test.expectedUtilization.Iowait, recorder.cpuUtilization.Iowait, 0.00001)
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

	actualUtilization := cpuUtilization(startStat, startTime, endStat, halfSecondLater)
	assert.InDelta(t, expectedUtilization.User, actualUtilization.User, 0.00001)
	assert.InDelta(t, expectedUtilization.System, actualUtilization.System, 0.00001)
	assert.InDelta(t, expectedUtilization.Iowait, actualUtilization.Iowait, 0.00001)

}
