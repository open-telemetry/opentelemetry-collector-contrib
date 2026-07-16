// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ucal

import (
	"time"

	"github.com/shirou/gopsutil/v4/net"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

const bitsPerByte = 8

type LinkInfo struct {
	SpeedMbps             int64
	FullDuplex            bool
	Loopback              bool
	CapacityBitsPerSecond float64
}

type Utilization struct {
	Device         string
	Utilization    float64
	HasUtilization bool
}

type sample struct {
	timestamp pcommon.Timestamp
	bytesRecv uint64
	bytesSent uint64
}

type NetworkUtilizationCalculator struct {
	previous map[string]sample
}

func (c *NetworkUtilizationCalculator) Calculate(
	now pcommon.Timestamp,
	ioCounters []net.IOCountersStat,
	linkInfoForDevice func(string) (LinkInfo, error),
) ([]Utilization, float64, bool, error) {
	if c.previous == nil {
		c.previous = make(map[string]sample, len(ioCounters))
	}

	results := make([]Utilization, 0, len(ioCounters))
	utilizationSum := 0.0
	monitoredInterfaces := 0
	hasUtilization := false
	next := make(map[string]sample, len(ioCounters))

	for _, io := range ioCounters {
		current := sample{
			timestamp: now,
			bytesRecv: io.BytesRecv,
			bytesSent: io.BytesSent,
		}
		next[io.Name] = current

		linkInfo, err := linkInfoForDevice(io.Name)
		if err != nil {
			c.previous = next
			return nil, 0, false, err
		}
		if linkInfo.Loopback {
			continue
		}

		monitoredInterfaces++

		capacityBitsPerSecond := linkInfo.capacityBitsPerSecond()
		if capacityBitsPerSecond <= 0 {
			continue
		}

		result := Utilization{Device: io.Name}

		previous, ok := c.previous[io.Name]
		if !ok || now <= previous.timestamp ||
			io.BytesRecv < previous.bytesRecv ||
			io.BytesSent < previous.bytesSent {
			results = append(results, result)
			continue
		}

		elapsedSeconds := time.Duration(now - previous.timestamp).Seconds()
		if elapsedSeconds <= 0 {
			results = append(results, result)
			continue
		}

		bytesPerSecond := float64(io.BytesRecv-previous.bytesRecv+io.BytesSent-previous.bytesSent) / elapsedSeconds
		utilization := bytesPerSecond * bitsPerByte / capacityBitsPerSecond
		utilizationSum += utilization
		hasUtilization = true
		result.Utilization = utilization
		result.HasUtilization = true
		results = append(results, result)
	}

	c.previous = next
	if monitoredInterfaces == 0 || !hasUtilization {
		return results, 0, false, nil
	}
	return results, utilizationSum / float64(monitoredInterfaces), true, nil
}

func (l LinkInfo) capacityBitsPerSecond() float64 {
	if l.CapacityBitsPerSecond > 0 {
		return l.CapacityBitsPerSecond
	}
	capacityBitsPerSecond := float64(l.SpeedMbps) * 1_000_000
	if l.FullDuplex {
		capacityBitsPerSecond *= 2
	}
	return capacityBitsPerSecond
}
