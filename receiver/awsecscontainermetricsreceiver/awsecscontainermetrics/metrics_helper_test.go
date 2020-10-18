// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsecscontainermetrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetContainerAndTaskMetrics(t *testing.T) {
	v := uint64(1)
	f := float64(1.0)
	floatZero := float64(0)

	memStats := make(map[string]uint64)
	memStats["cache"] = v

	mem := MemoryStats{
		Usage:          &v,
		MaxUsage:       &v,
		Limit:          &v,
		MemoryReserved: &v,
		MemoryUtilized: &v,
		Stats:          memStats,
	}

	disk := DiskStats{
		IoServiceBytesRecursives: []IoServiceBytesRecursive{
			{Op: "Read", Value: &v},
			{Op: "Write", Value: &v},
			{Op: "Total", Value: &v},
		},
	}

	net := make(map[string]NetworkStats)
	net["eth0"] = NetworkStats{
		RxBytes:   &v,
		RxPackets: &v,
		RxErrors:  &v,
		RxDropped: &v,
		TxBytes:   &v,
		TxPackets: &v,
		TxErrors:  &v,
		TxDropped: &v,
	}

	netRate := NetworkRateStats{
		RxBytesPerSecond: &f,
		TxBytesPerSecond: &f,
	}

	percpu := []*uint64{&v, &v}
	cpuUsage := CPUUsage{
		TotalUsage:        &v,
		UsageInKernelmode: &v,
		UsageInUserMode:   &v,
		PerCPUUsage:       percpu,
	}

	cpuStats := CPUStats{
		CPUUsage:       cpuUsage,
		OnlineCpus:     &v,
		SystemCPUUsage: &v,
		CPUUtilized:    &v,
		CPUReserved:    &v,
	}

	previousCPUUsage := CPUUsage{
		TotalUsage:        &v,
		UsageInKernelmode: &v,
		UsageInUserMode:   &v,
		PerCPUUsage:       percpu,
	}

	previousCPUStats := CPUStats{
		CPUUsage:       previousCPUUsage,
		OnlineCpus:     &v,
		SystemCPUUsage: &v,
		CPUUtilized:    &v,
		CPUReserved:    &v,
	}

	containerStats := ContainerStats{
		Name:         "test",
		ID:           "001",
		Read:         time.Now(),
		PreviousRead: time.Now().Add(-10 * time.Second),
		Memory:       mem,
		Disk:         disk,
		Network:      net,
		NetworkRate:  netRate,
		CPU:          cpuStats,
		PreviousCPU:  previousCPUStats,
	}

	containerMetrics := getContainerMetrics(containerStats)
	require.NotNil(t, containerMetrics)

	taskMetrics := ECSMetrics{}
	aggregateTaskMetrics(&taskMetrics, containerMetrics)
	require.EqualValues(t, v, taskMetrics.MemoryUsage)
	require.EqualValues(t, v, taskMetrics.MemoryMaxUsage)
	require.EqualValues(t, v, taskMetrics.StorageReadBytes)

	containerStats = ContainerStats{
		Name:        "test",
		ID:          "001",
		Memory:      mem,
		Disk:        disk,
		Network:     net,
		NetworkRate: NetworkRateStats{},
		CPU:         cpuStats,
	}
	containerMetrics = getContainerMetrics(containerStats)
	require.NotNil(t, containerMetrics)

	require.EqualValues(t, floatZero, containerMetrics.NetworkRateRxBytesPerSecond)
	require.EqualValues(t, floatZero, containerMetrics.NetworkRateTxBytesPerSecond)
}

func TestExtractStorageUsage(t *testing.T) {
	v := uint64(100)
	disk := &DiskStats{
		IoServiceBytesRecursives: []IoServiceBytesRecursive{
			{Op: "Read", Value: &v},
			{Op: "Write", Value: &v},
			{Op: "Total", Value: &v},
		},
	}
	read, write := extractStorageUsage(disk)

	require.EqualValues(t, v, read)
	require.EqualValues(t, v, write)

	read, write = extractStorageUsage(nil)
	v = uint64(0)
	require.EqualValues(t, v, read)
	require.EqualValues(t, v, write)
}

func TestGetNetworkStats(t *testing.T) {
	v := uint64(100)
	stats := make(map[string]NetworkStats)
	stats["eth0"] = NetworkStats{
		RxBytes:   &v,
		RxPackets: &v,
		RxErrors:  &v,
		RxDropped: &v,
		TxBytes:   &v,
		TxPackets: &v,
		TxErrors:  &v,
		TxDropped: &v,
	}

	netArray := getNetworkStats(stats)

	sum := uint64(0)
	for _, v := range netArray {
		sum += v
	}
	require.EqualValues(t, 800, sum)
}
