// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ucal

import (
	"testing"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type inMemoryRecorder struct {
	cpuUtilizations []CPUUtilization
}

func (r *inMemoryRecorder) record(_ pcommon.Timestamp, utilization CPUUtilization) {
	r.cpuUtilizations = append(r.cpuUtilizations, utilization)
}

func TestCpuUtilizationCalculator_Calculate(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name                 string
		now                  pcommon.Timestamp
		cpuTimes             []cpu.TimesStat
		previousCPUTimes     []cpu.TimesStat
		expectedUtilizations []CPUUtilization
		expectedError        error
	}{
		{
			name: "no previous times",
			cpuTimes: []cpu.TimesStat{
				{
					CPU:  "cpu0",
					User: 8260.4,
				},
			},
		},
		{
			name: "no delta time should return utilization=0",
			now:  1640097430772858000,
			previousCPUTimes: []cpu.TimesStat{
				{
					CPU:  "cpu0",
					User: 8259.4,
				},
			},
			cpuTimes: []cpu.TimesStat{
				{
					CPU:  "cpu0",
					User: 8259.4,
				},
			},
			expectedUtilizations: []CPUUtilization{
				{CPU: "cpu0"},
			},
		},
		{
			name: "invalid TimesStats",
			now:  1640097430772859000,
			previousCPUTimes: []cpu.TimesStat{
				{
					CPU:  "cpu5",
					User: 8259.4,
				},
			},
			cpuTimes: []cpu.TimesStat{
				{
					CPU:  "cpu6",
					User: 8260.4,
				},
			},
			expectedError: ErrTimeStatNotFound,
		},
		{
			name: "one cpu",
			now:  1640097435776827000,
			previousCPUTimes: []cpu.TimesStat{
				{
					CPU:    "cpu0",
					User:   8258.4,
					System: 6193.3,
					Idle:   34284.7,
				},
			},
			cpuTimes: []cpu.TimesStat{
				{
					CPU:    "cpu0",
					User:   8259.4,
					System: 6193.9,
					Idle:   34288.2,
				},
			},
			expectedUtilizations: []CPUUtilization{
				{
					CPU:    "cpu0",
					User:   0.19607,
					System: 0.11764,
					Idle:   0.68627,
				},
			},
		},
		{
			// On Linux, /proc/stat's user column already includes guest time and nice
			// already includes guest_nice (kernel increments both CPUTIME_USER and
			// CPUTIME_GUEST in account_guest_time). Adding Guest/GuestNice again in
			// totalCPU inflates the denominator and makes all utilizations too low.
			name: "guest time is not double-counted in total",
			now:  1640097435776827000,
			previousCPUTimes: []cpu.TimesStat{
				{
					CPU:   "cpu0",
					User:  10.0, // already includes Guest on Linux
					Idle:  5.0,
					Guest: 5.0, // already counted in User
				},
			},
			cpuTimes: []cpu.TimesStat{
				{
					CPU:   "cpu0",
					User:  20.0, // already includes Guest on Linux
					Idle:  10.0,
					Guest: 10.0, // already counted in User
				},
			},
			expectedUtilizations: []CPUUtilization{
				{
					CPU: "cpu0",
					// elapsed = (20+10)-(10+5) = 15, not 20 (which would double-count guest)
					User: 10.0 / 15.0, // ~0.66667
					Idle: 5.0 / 15.0,  // ~0.33333
				},
			},
		},
		{
			name: "multiple cpus unordered",
			now:  1640097435776827000,
			previousCPUTimes: []cpu.TimesStat{
				{
					CPU:    "cpu1",
					User:   528.3,
					System: 549.7,
					Idle:   47638.2,
				},
				{
					CPU:    "cpu0",
					User:   8258.4,
					System: 6193.3,
					Idle:   34284.7,
				},
			},
			cpuTimes: []cpu.TimesStat{
				{
					CPU:    "cpu0",
					User:   8259.4,
					System: 6193.9,
					Idle:   34288.2,
				},
				{
					CPU:    "cpu1",
					User:   528.4,
					System: 549.7,
					Idle:   47643.1,
				},
			},
			expectedUtilizations: []CPUUtilization{
				{
					CPU:    "cpu1",
					User:   0.02,
					System: 0,
					Idle:   0.98,
				},
				{
					CPU:    "cpu0",
					User:   0.19607,
					System: 0.11764,
					Idle:   0.68627,
				},
			},
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			recorder := inMemoryRecorder{}
			calculator := CPUUtilizationCalculator{
				previousCPUTimes: test.previousCPUTimes,
			}
			err := calculator.CalculateAndRecord(test.now, test.cpuTimes, recorder.record)
			assert.ErrorIs(t, err, test.expectedError)
			assert.Len(t, recorder.cpuUtilizations, len(test.expectedUtilizations))
			for idx, expectedUtilization := range test.expectedUtilizations {
				assert.Equal(t, expectedUtilization.CPU, recorder.cpuUtilizations[idx].CPU)
				assert.InDelta(t, expectedUtilization.System, recorder.cpuUtilizations[idx].System, 0.00001)
				assert.InDelta(t, expectedUtilization.User, recorder.cpuUtilizations[idx].User, 0.00001)
				assert.InDelta(t, expectedUtilization.Idle, recorder.cpuUtilizations[idx].Idle, 0.00001)
			}
		})
	}
}

func Test_cpuUtilization(t *testing.T) {
	timeStart := cpu.TimesStat{
		CPU:    "cpu0",
		User:   1.5,
		System: 2.7,
		Idle:   0.8,
	}
	timeEnd := cpu.TimesStat{
		CPU:    "cpu0",
		User:   2.7,
		System: 4.2,
		Idle:   3.1,
	}
	expectedUtilization := CPUUtilization{
		CPU:    "cpu0",
		User:   0.24,
		System: 0.3,
		Idle:   0.46,
	}

	actualUtilization := cpuUtilization(timeStart, timeEnd)
	assert.Equal(t, expectedUtilization.CPU, actualUtilization.CPU, "%+v", 0.00001)
	assert.InDelta(t, expectedUtilization.User, actualUtilization.User, 0.00001)
	assert.InDelta(t, expectedUtilization.System, actualUtilization.System, 0.00001)
	assert.InDelta(t, expectedUtilization.Idle, actualUtilization.Idle, 0.00001)
}

func Test_cpuTimeByCpu(t *testing.T) {
	testCases := []struct {
		name             string
		cpuNum           string
		times            []cpu.TimesStat
		expectedErr      error
		expectedTimeStat cpu.TimesStat
	}{
		{
			name:        "cpu does not exist",
			cpuNum:      "cpu9",
			times:       []cpu.TimesStat{{CPU: "cpu0"}, {CPU: "cpu1"}, {CPU: "cpu2"}},
			expectedErr: ErrTimeStatNotFound,
		},
		{
			name:             "cpu does exist",
			cpuNum:           "cpu1",
			times:            []cpu.TimesStat{{CPU: "cpu0"}, {CPU: "cpu1"}, {CPU: "cpu2"}},
			expectedTimeStat: cpu.TimesStat{CPU: "cpu1"},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			actualTimeStat, err := cpuTimeForCPU(test.cpuNum, test.times)
			assert.ErrorIs(t, err, test.expectedErr)
			assert.Equal(t, test.expectedTimeStat, actualTimeStat)
		})
	}
}
