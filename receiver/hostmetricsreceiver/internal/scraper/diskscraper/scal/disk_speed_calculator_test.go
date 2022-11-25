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

package scal

import (
	"testing"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type inMemoryRecorder struct {
	diskSpeeds []DiskSpeed
}

func (r *inMemoryRecorder) record(_ pcommon.Timestamp, bandwidth DiskSpeed) {
	r.diskSpeeds = append(r.diskSpeeds, bandwidth)
}

func TestDiskSpeedCalculator_Calculate(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name                   string
		now                    pcommon.Timestamp
		diskIOCounters         map[string]disk.IOCountersStat
		previousDiskIOCounters map[string]disk.IOCountersStat
		expectedDiskSpeed      []DiskSpeed
		expectedError          error
	}{
		{
			name: "no previous times",
			diskIOCounters: map[string]disk.IOCountersStat{
				"device0": {
					Name:       "device0",
					ReadBytes:  1234,
					WriteBytes: 5678,
				},
			},
		},
		{
			name: "no delta time should return bandwidth=0",
			now:  2,
			previousDiskIOCounters: map[string]disk.IOCountersStat{
				"device0": {
					Name:       "device0",
					ReadBytes:  8259,
					WriteBytes: 8259,
				},
			},
			diskIOCounters: map[string]disk.IOCountersStat{
				"device0": {
					Name:       "device0",
					ReadBytes:  8259,
					WriteBytes: 8259,
				},
			},
			expectedDiskSpeed: []DiskSpeed{
				{Name: "device0"},
			},
		},
		{
			name: "invalid TimesStats",
			now:  1640097430772859000,
			previousDiskIOCounters: map[string]disk.IOCountersStat{
				"device5": {
					Name:      "device5",
					ReadBytes: 8259,
				},
			},
			diskIOCounters: map[string]disk.IOCountersStat{
				"device6": {
					Name:      "device6",
					ReadBytes: 8260,
				},
			},
			expectedError: ErrIOCounterStatNotFound,
		},
		{
			name: "one device",
			now:  1,
			previousDiskIOCounters: map[string]disk.IOCountersStat{
				"device0": {
					Name:       "device0",
					ReadBytes:  8258,
					WriteBytes: 8234,
				},
			},
			diskIOCounters: map[string]disk.IOCountersStat{
				"device0": {
					Name:       "device0",
					ReadBytes:  8259,
					WriteBytes: 8244,
				},
			},
			expectedDiskSpeed: []DiskSpeed{
				{
					Name:       "device0",
					ReadSpeed:  1,
					WriteSpeed: 10,
				},
			},
		},
		{
			name: "multiple devices unordered",
			now:  1,
			previousDiskIOCounters: map[string]disk.IOCountersStat{
				"device1": {
					Name:       "device1",
					ReadBytes:  528,
					WriteBytes: 538,
				},
				"device0": {
					Name:       "device0",
					ReadBytes:  510,
					WriteBytes: 512,
				},
			},
			diskIOCounters: map[string]disk.IOCountersStat{
				"device0": {
					Name:       "device0",
					ReadBytes:  520,
					WriteBytes: 528,
				},
				"device1": {
					Name:       "device1",
					ReadBytes:  528,
					WriteBytes: 549,
				},
			},
			expectedDiskSpeed: []DiskSpeed{
				{
					Name:       "device1",
					ReadSpeed:  0,
					WriteSpeed: 11,
				},
				{
					Name:       "device0",
					ReadSpeed:  10,
					WriteSpeed: 16,
				},
			},
		},
	}

	getCurrentTime = func() float64 {
		return 2
	}

	for _, test := range testCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			recorder := inMemoryRecorder{}
			calculator := DiskSpeedCalculator{
				previousDiskIOCounters:          test.previousDiskIOCounters,
				previousDiskIOCounterRecordTime: 1,
			}
			err := calculator.CalculateAndRecord(test.now, test.diskIOCounters, recorder.record)
			assert.ErrorIs(t, err, test.expectedError)
			assert.Len(t, recorder.diskSpeeds, len(test.expectedDiskSpeed))
			for idx, expectedBandwidth := range test.expectedDiskSpeed {
				assert.Equal(t, expectedBandwidth.Name, recorder.diskSpeeds[idx].Name)
				assert.InDelta(t, expectedBandwidth.ReadSpeed, recorder.diskSpeeds[idx].ReadSpeed, 0.00001)
				assert.InDelta(t, expectedBandwidth.WriteSpeed, recorder.diskSpeeds[idx].WriteSpeed, 0.00001)
			}
		})
	}
}

func Test_DiskSpeed(t *testing.T) {

	timeStart := disk.IOCountersStat{
		Name:       "device0",
		ReadBytes:  1,
		WriteBytes: 2,
	}
	timeEnd := disk.IOCountersStat{
		Name:       "device0",
		ReadBytes:  3,
		WriteBytes: 4,
	}
	expectedUtilization := DiskSpeed{
		Name:       "device0",
		ReadSpeed:  2,
		WriteSpeed: 2,
	}

	getCurrentTime = func() float64 {
		return 2
	}

	actualUtilization := diskSpeed(1, timeStart, timeEnd)
	assert.Equal(t, expectedUtilization.Name, actualUtilization.Name, 0.00001)
	assert.InDelta(t, expectedUtilization.ReadSpeed, actualUtilization.ReadSpeed, 0.00001)
	assert.InDelta(t, expectedUtilization.WriteSpeed, actualUtilization.WriteSpeed, 0.00001)

}

func Test_diskCounterForDeviceName(t *testing.T) {
	testCases := []struct {
		name             string
		Name             string
		times            map[string]disk.IOCountersStat
		expectedErr      error
		expectedTimeStat disk.IOCountersStat
	}{
		{
			name: "device does not exist",
			Name: "device9",
			times: map[string]disk.IOCountersStat{
				"device0": {Name: "device0"},
				"device1": {Name: "device1"},
				"device2": {Name: "device2"}},
			expectedErr: ErrIOCounterStatNotFound,
		},
		{
			name: "device does exist",
			Name: "device1",
			times: map[string]disk.IOCountersStat{
				"device0": {Name: "device0"},
				"device1": {Name: "device1"},
				"device2": {Name: "device2"}},
			expectedTimeStat: disk.IOCountersStat{Name: "device1"},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			actualTimeStat, err := diskCounterForDeviceName(test.Name, test.times)
			assert.ErrorIs(t, err, test.expectedErr)
			assert.Equal(t, test.expectedTimeStat, actualTimeStat)
		})
	}
}
