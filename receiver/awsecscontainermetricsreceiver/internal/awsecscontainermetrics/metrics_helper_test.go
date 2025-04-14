// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

	require.Equal(t, v, containerMetrics.MemoryUsage)
	require.EqualValues(t, floatZero, containerMetrics.MemoryUtilized)

	require.Equal(t, v, containerMetrics.CPUTotalUsage)
	require.Equal(t, v, containerMetrics.CPUUsageInKernelmode)
	require.Equal(t, v, containerMetrics.CPUUsageInUserMode)

	require.Equal(t, f, containerMetrics.NetworkRateRxBytesPerSecond)
	require.Equal(t, f, containerMetrics.NetworkRateTxBytesPerSecond)

	require.Equal(t, v, containerMetrics.NetworkRxBytes)
	require.Equal(t, v, containerMetrics.NetworkTxBytes)

	require.Equal(t, v, containerMetrics.StorageReadBytes)
	require.Equal(t, v, containerMetrics.StorageWriteBytes)
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

	require.Equal(t, v, containerMetrics.CPUTotalUsage)
	require.Equal(t, v, containerMetrics.CPUUsageInKernelmode)
	require.Equal(t, v, containerMetrics.CPUUsageInUserMode)

	require.Equal(t, f, containerMetrics.NetworkRateRxBytesPerSecond)
	require.Equal(t, f, containerMetrics.NetworkRateTxBytesPerSecond)

	require.Equal(t, v, containerMetrics.NetworkRxBytes)
	require.Equal(t, v, containerMetrics.NetworkTxBytes)

	require.Equal(t, v, containerMetrics.StorageReadBytes)
	require.Equal(t, v, containerMetrics.StorageWriteBytes)
}

func TestGetContainerDereferenceCheck(t *testing.T) {
	tests := []struct {
		memoryStats  *MemoryStats
		testName     string
		cpuStats     *CPUStats
		networkRate  *NetworkRateStats
		prevCPUStats *CPUStats
	}{
		{
			memoryStats: &MemoryStats{
				Usage:          nil,
				MaxUsage:       nil,
				Limit:          nil,
				MemoryReserved: nil,
				MemoryUtilized: nil,
				Stats:          nil,
			},
			testName:     "nil memory stats values",
			cpuStats:     &cpuStats,
			networkRate:  &netRate,
			prevCPUStats: &previousCPUStats,
		},
		{
			memoryStats: &mem,
			testName:    "nil cpuStats values",
			cpuStats: &CPUStats{
				CPUUsage:       &cpuUsage,
				OnlineCpus:     nil,
				SystemCPUUsage: nil,
				CPUReserved:    nil,
				CPUUtilized:    nil,
			},
			networkRate:  &netRate,
			prevCPUStats: &previousCPUStats,
		},
		{
			memoryStats: &mem,
			testName:    "nil cpuUsage and CPUstats values",
			cpuStats: &CPUStats{
				CPUUsage: &CPUUsage{
					TotalUsage:        nil,
					UsageInKernelmode: nil,
					UsageInUserMode:   nil,
					PerCPUUsage:       nil,
				},
				OnlineCpus:     nil,
				SystemCPUUsage: nil,
				CPUReserved:    nil,
				CPUUtilized:    nil,
			},
			networkRate:  &netRate,
			prevCPUStats: &previousCPUStats,
		},
		{
			memoryStats: &mem,
			testName:    "nil network rate values",
			cpuStats:    &cpuStats,
			networkRate: &NetworkRateStats{
				RxBytesPerSecond: nil,
				TxBytesPerSecond: nil,
			},
			prevCPUStats: &previousCPUStats,
		},
		{
			memoryStats: &mem,
			testName:    "nil prev cpu stats values",
			cpuStats:    &cpuStats,
			networkRate: &netRate,
			prevCPUStats: &CPUStats{
				CPUUsage: &CPUUsage{
					TotalUsage:        nil,
					UsageInKernelmode: nil,
					UsageInUserMode:   nil,
					PerCPUUsage:       nil,
				},
				OnlineCpus:     nil,
				SystemCPUUsage: nil,
				CPUReserved:    nil,
				CPUUtilized:    nil,
			},
		},
		{
			memoryStats: &mem,
			testName:    "nil prevcpu.cpuUsage stats values",
			cpuStats:    &cpuStats,
			networkRate: &netRate,
			prevCPUStats: &CPUStats{
				CPUUsage:       nil,
				OnlineCpus:     nil,
				SystemCPUUsage: nil,
				CPUReserved:    nil,
				CPUUtilized:    nil,
			},
		},
		{
			memoryStats: &mem,
			testName:    "nil cpu.cpuUsage",
			cpuStats: &CPUStats{
				CPUUsage:       nil,
				OnlineCpus:     nil,
				SystemCPUUsage: nil,
				CPUReserved:    nil,
				CPUUtilized:    nil,
			},
			networkRate:  &netRate,
			prevCPUStats: &previousCPUStats,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			containerStats = ContainerStats{
				Name:         "test",
				ID:           "001",
				Read:         time.Now(),
				PreviousRead: time.Now().Add(-10 * time.Second),
				Memory:       test.memoryStats,
				Disk:         &disk,
				Network:      net,
				NetworkRate:  test.networkRate,
				CPU:          test.cpuStats,
				PreviousCPU:  test.prevCPUStats,
			}

			require.NotPanics(t, func() {
				getContainerMetrics(&containerStats, logger)
			})
		})
	}
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

	require.Equal(t, v, containerMetrics.MemoryUsage)
	require.EqualValues(t, floatZero, containerMetrics.MemoryUtilized)

	require.EqualValues(t, 0, containerMetrics.CPUTotalUsage)
	require.EqualValues(t, 0, containerMetrics.CPUUsageInKernelmode)
	require.EqualValues(t, 0, containerMetrics.CPUUsageInUserMode)

	require.Equal(t, f, containerMetrics.NetworkRateRxBytesPerSecond)
	require.Equal(t, f, containerMetrics.NetworkRateTxBytesPerSecond)

	require.Equal(t, v, containerMetrics.NetworkRxBytes)
	require.Equal(t, v, containerMetrics.NetworkTxBytes)

	require.Equal(t, v, containerMetrics.StorageReadBytes)
	require.Equal(t, v, containerMetrics.StorageWriteBytes)
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

	require.Equal(t, v, containerMetrics.MemoryUsage)
	require.EqualValues(t, floatZero, containerMetrics.MemoryUtilized)

	require.Equal(t, v, containerMetrics.CPUTotalUsage)
	require.Equal(t, v, containerMetrics.CPUUsageInKernelmode)
	require.Equal(t, v, containerMetrics.CPUUsageInUserMode)

	require.Equal(t, floatZero, containerMetrics.NetworkRateRxBytesPerSecond)
	require.Equal(t, floatZero, containerMetrics.NetworkRateTxBytesPerSecond)

	require.Equal(t, v, containerMetrics.NetworkRxBytes)
	require.Equal(t, v, containerMetrics.NetworkTxBytes)

	require.Equal(t, v, containerMetrics.StorageReadBytes)
	require.Equal(t, v, containerMetrics.StorageWriteBytes)
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

	require.Equal(t, v, containerMetrics.MemoryUsage)
	require.EqualValues(t, floatZero, containerMetrics.MemoryUtilized)

	require.Equal(t, v, containerMetrics.CPUTotalUsage)
	require.Equal(t, v, containerMetrics.CPUUsageInKernelmode)
	require.Equal(t, v, containerMetrics.CPUUsageInUserMode)

	require.Equal(t, f, containerMetrics.NetworkRateRxBytesPerSecond)
	require.Equal(t, f, containerMetrics.NetworkRateTxBytesPerSecond)

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

	require.Equal(t, v, containerMetrics.MemoryUsage)
	require.EqualValues(t, floatZero, containerMetrics.MemoryUtilized)

	require.Equal(t, v, containerMetrics.CPUTotalUsage)
	require.Equal(t, v, containerMetrics.CPUUsageInKernelmode)
	require.Equal(t, v, containerMetrics.CPUUsageInUserMode)

	require.Equal(t, f, containerMetrics.NetworkRateRxBytesPerSecond)
	require.Equal(t, f, containerMetrics.NetworkRateTxBytesPerSecond)

	require.Equal(t, v, containerMetrics.NetworkRxBytes)
	require.Equal(t, v, containerMetrics.NetworkTxBytes)

	require.Equal(t, v, containerMetrics.StorageReadBytes)
	require.Equal(t, v, containerMetrics.StorageWriteBytes)
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
	require.Equal(t, v, taskMetrics.MemoryUsage)
	require.Equal(t, v, taskMetrics.MemoryMaxUsage)
	require.Equal(t, v, taskMetrics.StorageReadBytes)
}

func TestExtractStorageUsage(t *testing.T) {
	v := uint64(100)
	v2 := uint64(200)
	disk := &DiskStats{
		IoServiceBytesRecursives: []IoServiceBytesRecursive{
			{Op: "Read", Value: &v},
			{Op: "Write", Value: &v},
			{Op: "Total", Value: &v},

			{Op: "Read", Value: &v2},
			{Op: "Write", Value: &v2},
			{Op: "Total", Value: &v2},
		},
	}
	read, write := extractStorageUsage(disk)

	require.Equal(t, v+v2, read)
	require.Equal(t, v+v2, write)

	read, write = extractStorageUsage(nil)
	v = uint64(0)
	require.Equal(t, v, read)
	require.Equal(t, v, write)
}

func TestExtractStorageUsageDereferenceCheck(t *testing.T) {
	disk := &DiskStats{
		IoServiceBytesRecursives: []IoServiceBytesRecursive{
			{Op: "Read", Value: nil},
			{Op: "Write", Value: nil},
			{Op: "Total", Value: nil},
		},
	}

	require.NotPanics(t, func() {
		extractStorageUsage(disk)
	})
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

func TestGetNetworkStatsDereferenceCheck(t *testing.T) {
	stats := make(map[string]NetworkStats)
	stats["eth0"] = NetworkStats{
		RxBytes:   nil,
		RxPackets: nil,
		RxErrors:  nil,
		RxDropped: nil,
		TxBytes:   nil,
		TxPackets: nil,
		TxErrors:  nil,
		TxDropped: nil,
	}

	require.NotPanics(t, func() {
		getNetworkStats(stats)
	})
}
