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

package bcal

import (
	"testing"

	"github.com/shirou/gopsutil/v3/net"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type inMemoryRecorder struct {
	networkBandwidths []NetworkBandwidth
}

func (r *inMemoryRecorder) record(_ pcommon.Timestamp, bandwidth NetworkBandwidth) {
	r.networkBandwidths = append(r.networkBandwidths, bandwidth)
}

func TestNetworkBandwidthCalculator_Calculate(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name                     string
		now                      pcommon.Timestamp
		netIOCounters            []net.IOCountersStat
		previousNetIOCounters    []net.IOCountersStat
		expectedNetworkBandwidth []NetworkBandwidth
		expectedError            error
	}{
		{
			name: "no previous times",
			netIOCounters: []net.IOCountersStat{
				{
					Name:      "interface0",
					BytesRecv: 1234,
					BytesSent: 5678,
				},
			},
		},
		{
			name: "no delta time should return bandwidth=0",
			now:  2,
			previousNetIOCounters: []net.IOCountersStat{
				{
					Name:      "interface0",
					BytesRecv: 8259,
					BytesSent: 8259,
				},
			},
			netIOCounters: []net.IOCountersStat{
				{
					Name:      "interface0",
					BytesRecv: 8259,
					BytesSent: 8259,
				},
			},
			expectedNetworkBandwidth: []NetworkBandwidth{
				{Name: "interface0"},
			},
		},
		{
			name: "invalid TimesStats",
			now:  1640097430772859000,
			previousNetIOCounters: []net.IOCountersStat{
				{
					Name:      "interface5",
					BytesRecv: 8259,
				},
			},
			netIOCounters: []net.IOCountersStat{
				{
					Name:      "interface6",
					BytesRecv: 8260,
				},
			},
			expectedError: ErrIOCounterStatNotFound,
		},
		{
			name: "one interface",
			now:  1,
			previousNetIOCounters: []net.IOCountersStat{
				{
					Name:      "interface0",
					BytesRecv: 8258,
					BytesSent: 8234,
				},
			},
			netIOCounters: []net.IOCountersStat{
				{
					Name:      "interface0",
					BytesRecv: 8259,
					BytesSent: 8244,
				},
			},
			expectedNetworkBandwidth: []NetworkBandwidth{
				{
					Name:         "interface0",
					InboundRate:  1,
					OutboundRate: 10,
				},
			},
		},
		{
			name: "multiple interfaces unordered",
			now:  1,
			previousNetIOCounters: []net.IOCountersStat{
				{
					Name:      "interface1",
					BytesRecv: 528,
					BytesSent: 538,
				},
				{
					Name:      "interface0",
					BytesRecv: 510,
					BytesSent: 512,
				},
			},
			netIOCounters: []net.IOCountersStat{
				{
					Name:      "interface0",
					BytesRecv: 520,
					BytesSent: 528,
				},
				{
					Name:      "interface1",
					BytesRecv: 528,
					BytesSent: 549,
				},
			},
			expectedNetworkBandwidth: []NetworkBandwidth{
				{
					Name:         "interface1",
					InboundRate:  0,
					OutboundRate: 11,
				},
				{
					Name:         "interface0",
					InboundRate:  10,
					OutboundRate: 16,
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
			calculator := NetworkBandwidthCalculator{
				previousNetIOCounters:          test.previousNetIOCounters,
				previousNetIOCounterRecordTime: 1,
			}
			err := calculator.CalculateAndRecord(test.now, test.netIOCounters, recorder.record)
			assert.ErrorIs(t, err, test.expectedError)
			assert.Len(t, recorder.networkBandwidths, len(test.expectedNetworkBandwidth))
			for idx, expectedBandwidth := range test.expectedNetworkBandwidth {
				assert.Equal(t, expectedBandwidth.Name, recorder.networkBandwidths[idx].Name)
				assert.InDelta(t, expectedBandwidth.InboundRate, recorder.networkBandwidths[idx].InboundRate, 0.00001)
				assert.InDelta(t, expectedBandwidth.OutboundRate, recorder.networkBandwidths[idx].OutboundRate, 0.00001)
			}
		})
	}
}

func Test_NetworkBandwidth(t *testing.T) {

	timeStart := net.IOCountersStat{
		Name:      "interface0",
		BytesRecv: 1,
		BytesSent: 2,
	}
	timeEnd := net.IOCountersStat{
		Name:      "interface0",
		BytesRecv: 3,
		BytesSent: 4,
	}
	expectedUtilization := NetworkBandwidth{
		Name:         "interface0",
		InboundRate:  2,
		OutboundRate: 2,
	}

	getCurrentTime = func() float64 {
		return 2
	}

	actualUtilization := networkBandwidth(1, timeStart, timeEnd)
	assert.Equal(t, expectedUtilization.Name, actualUtilization.Name, 0.00001)
	assert.InDelta(t, expectedUtilization.InboundRate, actualUtilization.InboundRate, 0.00001)
	assert.InDelta(t, expectedUtilization.OutboundRate, actualUtilization.OutboundRate, 0.00001)

}

func Test_networkCounterForName(t *testing.T) {
	testCases := []struct {
		name             string
		Name             string
		times            []net.IOCountersStat
		expectedErr      error
		expectedTimeStat net.IOCountersStat
	}{
		{
			name:        "interface does not exist",
			Name:        "interface9",
			times:       []net.IOCountersStat{{Name: "interface0"}, {Name: "interface1"}, {Name: "interface2"}},
			expectedErr: ErrIOCounterStatNotFound,
		},
		{
			name:             "interface does exist",
			Name:             "interface1",
			times:            []net.IOCountersStat{{Name: "interface0"}, {Name: "interface1"}, {Name: "interface2"}},
			expectedTimeStat: net.IOCountersStat{Name: "interface1"},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			actualTimeStat, err := networkCounterForName(test.Name, test.times)
			assert.ErrorIs(t, err, test.expectedErr)
			assert.Equal(t, test.expectedTimeStat, actualTimeStat)
		})
	}
}
