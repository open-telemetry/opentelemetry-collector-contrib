// Copyright The OpenTelemetry Authors
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

var (
	previousCPUUsage = CPUUsage{
		TotalUsage:        &v,
		UsageInKernelmode: &v,
		UsageInUserMode:   &v,
		PerCPUUsage:       percpu,
	}

	previousCPUStats = CPUStats{
		CPUUsage:       &previousCPUUsage,
		OnlineCpus:     &v,
		SystemCPUUsage: &v,
		CPUUtilized:    &v,
		CPUReserved:    &v,
	}
)

func TestGetContainerMetricsAllValid(t *testing.T) {
	containerStats = ContainerStats{
		Name:         "test",
		ID:           "001",
		Read:         time.Now(),
		PreviousRead: time.Now().Add(-10 * time.Second),
		Memory:       &mem,
		Disk:         &disk,
		Network:      net,
		NetworkRate:  &netRate,
		CPU:          &cpuStats,
		PreviousCPU:  &previousCPUStats,
	}

	containerMetrics := getContainerMetrics(&containerStats, logger)
	require.NotNil(t, containerMetrics)

	require.EqualValues(t, v, containerMetrics.MemoryUsage)
	require.EqualValues(t, floatZero, containerMetrics.MemoryUtilized)

	require.EqualValues(t, v, containerMetrics.CPUTotalUsage)
	require.EqualValues(t, v, containerMetrics.CPUUsageInKernelmode)
	require.EqualValues(t, v, containerMetrics.CPUUsageInUserMode)

	require.EqualValues(t, f, containerMetrics.NetworkRateRxBytesPerSecond)
	require.EqualValues(t, f, containerMetrics.NetworkRateTxBytesPerSecond)

	require.EqualValues(t, v, containerMetrics.NetworkRxBytes)
	require.EqualValues(t, v, containerMetrics.NetworkTxBytes)

	require.EqualValues(t, v, containerMetrics.StorageReadBytes)
	require.EqualValues(t, v, containerMetrics.StorageWriteBytes)
}

func TestGetContainerMetricsMissingMemory(t *testing.T) {
	containerStats = ContainerStats{
		Name:         "test",
		ID:           "001",
		Read:         time.Now(),
		PreviousRead: time.Now().Add(-10 * time.Second),
		Memory:       nil,
		Disk:         &disk,
		Network:      net,
		NetworkRate:  &netRate,
		CPU:          &cpuStats,
		PreviousCPU:  &previousCPUStats,
	}

	containerMetrics := getContainerMetrics(&containerStats, logger)
	require.NotNil(t, containerMetrics)

	require.EqualValues(t, 0, containerMetrics.MemoryUsage)
	require.EqualValues(t, floatZero, containerMetrics.MemoryUtilized)

	require.EqualValues(t, v, containerMetrics.CPUTotalUsage)
	require.EqualValues(t, v, containerMetrics.CPUUsageInKernelmode)
	require.EqualValues(t, v, containerMetrics.CPUUsageInUserMode)

	require.EqualValues(t, f, containerMetrics.NetworkRateRxBytesPerSecond)
	require.EqualValues(t, f, containerMetrics.NetworkRateTxBytesPerSecond)

	require.EqualValues(t, v, containerMetrics.NetworkRxBytes)
	require.EqualValues(t, v, containerMetrics.NetworkTxBytes)

	require.EqualValues(t, v, containerMetrics.StorageReadBytes)
	require.EqualValues(t, v, containerMetrics.StorageWriteBytes)
}

func TestGetContainerMetricsMissingCpu(t *testing.T) {
	containerStats = ContainerStats{
		Name:         "test",
		ID:           "001",
		Read:         time.Now(),
		PreviousRead: time.Now().Add(-10 * time.Second),
		Memory:       &mem,
		Disk:         &disk,
		Network:      net,
		NetworkRate:  &netRate,
		CPU:          nil,
		PreviousCPU:  &previousCPUStats,
	}

	containerMetrics := getContainerMetrics(&containerStats, logger)
	require.NotNil(t, containerMetrics)

	require.EqualValues(t, v, containerMetrics.MemoryUsage)
	require.EqualValues(t, floatZero, containerMetrics.MemoryUtilized)

	require.EqualValues(t, 0, containerMetrics.CPUTotalUsage)
	require.EqualValues(t, 0, containerMetrics.CPUUsageInKernelmode)
	require.EqualValues(t, 0, containerMetrics.CPUUsageInUserMode)

	require.EqualValues(t, f, containerMetrics.NetworkRateRxBytesPerSecond)
	require.EqualValues(t, f, containerMetrics.NetworkRateTxBytesPerSecond)

	require.EqualValues(t, v, containerMetrics.NetworkRxBytes)
	require.EqualValues(t, v, containerMetrics.NetworkTxBytes)

	require.EqualValues(t, v, containerMetrics.StorageReadBytes)
	require.EqualValues(t, v, containerMetrics.StorageWriteBytes)
}

func TestGetContainerMetricsMissingNetworkRate(t *testing.T) {
	containerStats = ContainerStats{
		Name:         "test",
		ID:           "001",
		Read:         time.Now(),
		PreviousRead: time.Now().Add(-10 * time.Second),
		Memory:       &mem,
		Disk:         &disk,
		Network:      net,
		NetworkRate:  nil,
		CPU:          &cpuStats,
		PreviousCPU:  &previousCPUStats,
	}

	containerMetrics := getContainerMetrics(&containerStats, logger)
	require.NotNil(t, containerMetrics)

	require.EqualValues(t, v, containerMetrics.MemoryUsage)
	require.EqualValues(t, floatZero, containerMetrics.MemoryUtilized)

	require.EqualValues(t, v, containerMetrics.CPUTotalUsage)
	require.EqualValues(t, v, containerMetrics.CPUUsageInKernelmode)
	require.EqualValues(t, v, containerMetrics.CPUUsageInUserMode)

	require.EqualValues(t, floatZero, containerMetrics.NetworkRateRxBytesPerSecond)
	require.EqualValues(t, floatZero, containerMetrics.NetworkRateTxBytesPerSecond)

	require.EqualValues(t, v, containerMetrics.NetworkRxBytes)
	require.EqualValues(t, v, containerMetrics.NetworkTxBytes)

	require.EqualValues(t, v, containerMetrics.StorageReadBytes)
	require.EqualValues(t, v, containerMetrics.StorageWriteBytes)
}

func TestGetContainerMetricsMissingNetworkAndDisk(t *testing.T) {
	containerStats = ContainerStats{
		Name:         "test",
		ID:           "001",
		Read:         time.Now(),
		PreviousRead: time.Now().Add(-10 * time.Second),
		Memory:       &mem,
		Disk:         nil,
		Network:      nil,
		NetworkRate:  &netRate,
		CPU:          &cpuStats,
		PreviousCPU:  &previousCPUStats,
	}

	containerMetrics := getContainerMetrics(&containerStats, logger)
	require.NotNil(t, containerMetrics)

	require.EqualValues(t, v, containerMetrics.MemoryUsage)
	require.EqualValues(t, floatZero, containerMetrics.MemoryUtilized)

	require.EqualValues(t, v, containerMetrics.CPUTotalUsage)
	require.EqualValues(t, v, containerMetrics.CPUUsageInKernelmode)
	require.EqualValues(t, v, containerMetrics.CPUUsageInUserMode)

	require.EqualValues(t, f, containerMetrics.NetworkRateRxBytesPerSecond)
	require.EqualValues(t, f, containerMetrics.NetworkRateTxBytesPerSecond)

	require.EqualValues(t, 0, containerMetrics.NetworkRxBytes)
	require.EqualValues(t, 0, containerMetrics.NetworkTxBytes)

	require.EqualValues(t, 0, containerMetrics.StorageReadBytes)
	require.EqualValues(t, 0, containerMetrics.StorageWriteBytes)
}

func TestGetContainerMetricsMissingMemoryStats(t *testing.T) {
	containerStats = ContainerStats{
		Name:         "test",
		ID:           "001",
		Read:         time.Now(),
		PreviousRead: time.Now().Add(-10 * time.Second),
		Memory:       &mem,
		Disk:         &disk,
		Network:      net,
		NetworkRate:  &netRate,
		CPU:          &cpuStats,
		PreviousCPU:  &previousCPUStats,
	}

	containerStats.Memory.Stats = nil

	containerMetrics := getContainerMetrics(&containerStats, logger)
	require.NotNil(t, containerMetrics)

	require.EqualValues(t, v, containerMetrics.MemoryUsage)
	require.EqualValues(t, floatZero, containerMetrics.MemoryUtilized)

	require.EqualValues(t, v, containerMetrics.CPUTotalUsage)
	require.EqualValues(t, v, containerMetrics.CPUUsageInKernelmode)
	require.EqualValues(t, v, containerMetrics.CPUUsageInUserMode)

	require.EqualValues(t, f, containerMetrics.NetworkRateRxBytesPerSecond)
	require.EqualValues(t, f, containerMetrics.NetworkRateTxBytesPerSecond)

	require.EqualValues(t, v, containerMetrics.NetworkRxBytes)
	require.EqualValues(t, v, containerMetrics.NetworkTxBytes)

	require.EqualValues(t, v, containerMetrics.StorageReadBytes)
	require.EqualValues(t, v, containerMetrics.StorageWriteBytes)
}

func TestAggregateTaskMetrics(t *testing.T) {
	containerStats = ContainerStats{
		Name:         "test",
		ID:           "001",
		Read:         time.Now(),
		PreviousRead: time.Now().Add(-10 * time.Second),
		Memory:       &mem,
		Disk:         &disk,
		Network:      net,
		NetworkRate:  &netRate,
		CPU:          &cpuStats,
		PreviousCPU:  &previousCPUStats,
	}

	containerMetrics := getContainerMetrics(&containerStats, logger)
	require.NotNil(t, containerMetrics)

	taskMetrics := ECSMetrics{}
	aggregateTaskMetrics(&taskMetrics, containerMetrics)
	require.EqualValues(t, v, taskMetrics.MemoryUsage)
	require.EqualValues(t, v, taskMetrics.MemoryMaxUsage)
	require.EqualValues(t, v, taskMetrics.StorageReadBytes)
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
