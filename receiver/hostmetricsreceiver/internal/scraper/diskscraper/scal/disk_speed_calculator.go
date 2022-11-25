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

package scal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper/scal"

import (
	"errors"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/disk"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

var ErrIOCounterStatNotFound = errors.New("cannot find IOCounterStat for Device")

var getCurrentTime = func() float64 {
	return float64(time.Now().UnixNano()) / float64(time.Second)
}

// DiskSpeed stores the network speed for the different network interfaces
type DiskSpeed struct {
	Name       string
	ReadSpeed  float64
	WriteSpeed float64
}

// DiskSpeedCalculator calculates the disk speed percents for the different network interfaces
// It requires 2 []disk.IOCountersStat and spend time to be able to calculate the difference
type DiskSpeedCalculator struct {
	previousDiskIOCounters          map[string]disk.IOCountersStat
	previousDiskIOCounterRecordTime float64
}

// CalculateAndRecord calculates the disk speed for the different interfaces comparing previously
// stored []disk.IOCountersStat and time.Time and current []disk.IOCountersStat and current time.Time
// If no previous data is stored it will return empty slice of DiskSpeed and no error
func (n *DiskSpeedCalculator) CalculateAndRecord(now pcommon.Timestamp, diskIOCounters map[string]disk.IOCountersStat, recorder func(pcommon.Timestamp, DiskSpeed)) error {
	if n.previousDiskIOCounters != nil {
		for _, previousDiskIOCounter := range n.previousDiskIOCounters {
			currentNetIOCounter, err := diskCounterForDeviceName(previousDiskIOCounter.Name, diskIOCounters)
			if err != nil {
				return fmt.Errorf("getting io count for interface %s: %w", previousDiskIOCounter.Name, err)
			}
			recorder(now, diskSpeed(n.previousDiskIOCounterRecordTime, previousDiskIOCounter, currentNetIOCounter))
		}
	}
	n.previousDiskIOCounters = diskIOCounters
	n.previousDiskIOCounterRecordTime = getCurrentTime()

	return nil
}

// diskSpeed calculates the difference between 2 disk.IOCountersStat using spent time between them
func diskSpeed(lastRecordTime float64, timeStart disk.IOCountersStat, timeEnd disk.IOCountersStat) DiskSpeed {
	elapsedSeconds := getCurrentTime() - lastRecordTime
	if elapsedSeconds <= 0 {
		return DiskSpeed{Name: timeStart.Name}
	}
	// fmt.Println("elapsed.............\n\n\n", elapsedSeconds)

	data := DiskSpeed{
		Name:       timeStart.Name,
		WriteSpeed: (float64(timeEnd.WriteBytes) - float64(timeStart.WriteBytes)) / elapsedSeconds,
		ReadSpeed:  (float64(timeEnd.ReadBytes) - float64(timeStart.ReadBytes)) / elapsedSeconds,
	}
	return data
}

// diskCounterForDevice returns disk.IOCountersStat from a slice of disk.IOCountersStat based on Device Name
// If NetIOCounter is not found and error will be returned
func diskCounterForDeviceName(deviceName string, diskIOCountersMap map[string]disk.IOCountersStat) (disk.IOCountersStat, error) {
	if val, ok := diskIOCountersMap[deviceName]; ok {
		return val, nil
	}
	return disk.IOCountersStat{}, fmt.Errorf("device %s : %w", deviceName, ErrIOCounterStatNotFound)
}
