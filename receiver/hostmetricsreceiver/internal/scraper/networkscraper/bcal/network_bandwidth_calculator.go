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

package bcal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper/bcal"

import (
	"errors"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/net"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

var ErrIOCounterStatNotFound = errors.New("cannot find IOCounterStat for Interface")

var getCurrentTime = func() float64 {
	return float64(time.Now().UnixNano()) / float64(time.Second)
}

// NetworkBandwidth stores the network speed for the different network interfaces
type NetworkBandwidth struct {
	Name         string
	InboundRate  float64
	OutboundRate float64
}

// NetworkBandwidthCalculator calculates the network bandwidth percents for the different network interfaces
// It requires 2 []net.IOCountersStat and spend time to be able to calculate the difference
type NetworkBandwidthCalculator struct {
	previousNetIOCounters          []net.IOCountersStat
	previousNetIOCounterRecordTime float64
}

// CalculateAndRecord calculates the network bandwidth for the different interfaces comparing previously
// stored []net.IOCountersStat and time.Time and current []net.IOCountersStat and current time.Time
// If no previous data is stored it will return empty slice of NetworkBandwidth and no error
func (n *NetworkBandwidthCalculator) CalculateAndRecord(now pcommon.Timestamp, netIOCounters []net.IOCountersStat, recorder func(pcommon.Timestamp, NetworkBandwidth)) error {
	if n.previousNetIOCounters != nil {
		for _, previousNetIOCounter := range n.previousNetIOCounters {
			currentNetIOCounter, err := networkCounterForName(previousNetIOCounter.Name, netIOCounters)
			if err != nil {
				return fmt.Errorf("getting io count for interface %s: %w", previousNetIOCounter.Name, err)
			}
			recorder(now, networkBandwidth(n.previousNetIOCounterRecordTime, previousNetIOCounter, currentNetIOCounter))
		}
	}
	n.previousNetIOCounters = netIOCounters
	n.previousNetIOCounterRecordTime = getCurrentTime()

	return nil
}

// networkBandwidth calculates the difference between 2 net.IOCountersStat using spent time between them
func networkBandwidth(lastRecordTime float64, timeStart net.IOCountersStat, timeEnd net.IOCountersStat) NetworkBandwidth {
	elapsedSeconds := getCurrentTime() - lastRecordTime
	if elapsedSeconds <= 0 {
		return NetworkBandwidth{Name: timeStart.Name}
	}
	// fmt.Println("elapsed.............\n\n\n", elapsedSeconds)

	data := NetworkBandwidth{
		Name:         timeStart.Name,
		OutboundRate: (float64(timeEnd.BytesSent) - float64(timeStart.BytesSent)) / elapsedSeconds,
		InboundRate:  (float64(timeEnd.BytesRecv) - float64(timeStart.BytesRecv)) / elapsedSeconds,
	}
	return data
}

// networkCounterForName returns net.IOCountersStat from a slice of net.IOCountersStat based on Interface Name
// If NetIOCounter is not found and error will be returned
func networkCounterForName(interfaceName string, times []net.IOCountersStat) (net.IOCountersStat, error) {
	for _, t := range times {
		if t.Name == interfaceName {
			return t, nil
		}
	}
	return net.IOCountersStat{}, fmt.Errorf("interface %s : %w", interfaceName, ErrIOCounterStatNotFound)
}
