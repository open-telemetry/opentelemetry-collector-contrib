// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecscontainermetrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/internal/awsecscontainermetrics"

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"go.uber.org/zap"
)

// getContainerMetrics generate ECS Container metrics from Container stats
func getContainerMetrics(stats *ContainerStats, logger *zap.Logger) ECSMetrics {
	m := ECSMetrics{}

	if stats.Memory != nil {
		m.MemoryUsage = aws.ToUint64(stats.Memory.Usage)
		m.MemoryMaxUsage = aws.ToUint64(stats.Memory.MaxUsage)
		m.MemoryLimit = aws.ToUint64(stats.Memory.Limit)

		if stats.Memory.Stats != nil {
			m.MemoryUtilized = (aws.ToUint64(stats.Memory.Usage) - stats.Memory.Stats["cache"]) / bytesInMiB
		}
	} else {
		logger.Debug("Nil memory stats found for docker container:" + stats.Name)
	}

	if stats.CPU != nil && stats.CPU.CPUUsage != nil &&
		stats.PreviousCPU != nil && stats.PreviousCPU.CPUUsage != nil {
		numOfCores := (uint64)(len(stats.CPU.CPUUsage.PerCPUUsage))
		timeDiffSinceLastRead := (float64)(stats.Read.Sub(stats.PreviousRead).Nanoseconds())

		cpuUsageInVCpu := float64(0)
		if timeDiffSinceLastRead > 0 {
			cpuDelta := (float64)(aws.ToUint64(stats.CPU.CPUUsage.TotalUsage) - aws.ToUint64(stats.PreviousCPU.CPUUsage.TotalUsage))
			cpuUsageInVCpu = cpuDelta / timeDiffSinceLastRead
		}
		cpuUtilized := cpuUsageInVCpu * 100

		m.CPUTotalUsage = aws.ToUint64(stats.CPU.CPUUsage.TotalUsage)
		m.CPUUsageInKernelmode = aws.ToUint64(stats.CPU.CPUUsage.UsageInKernelmode)
		m.CPUUsageInUserMode = aws.ToUint64(stats.CPU.CPUUsage.UsageInUserMode)
		m.NumOfCPUCores = numOfCores
		m.CPUOnlineCpus = aws.ToUint64(stats.CPU.OnlineCpus)
		m.SystemCPUUsage = aws.ToUint64(stats.CPU.SystemCPUUsage)
		m.CPUUsageInVCPU = cpuUsageInVCpu
		m.CPUUtilized = cpuUtilized
	} else {
		logger.Debug("Nil CPUUsage stats found for docker container:" + stats.Name)
	}

	if stats.NetworkRate != nil {
		m.NetworkRateRxBytesPerSecond = aws.ToFloat64(stats.NetworkRate.RxBytesPerSecond)
		m.NetworkRateTxBytesPerSecond = aws.ToFloat64(stats.NetworkRate.TxBytesPerSecond)
	} else {
		logger.Debug("Nil NetworkRate stats found for docker container:" + stats.Name)
	}

	if stats.Network != nil {
		netStatArray := getNetworkStats(stats.Network)

		m.NetworkRxBytes = netStatArray[0]
		m.NetworkRxPackets = netStatArray[1]
		m.NetworkRxErrors = netStatArray[2]
		m.NetworkRxDropped = netStatArray[3]

		m.NetworkTxBytes = netStatArray[4]
		m.NetworkTxPackets = netStatArray[5]
		m.NetworkTxErrors = netStatArray[6]
		m.NetworkTxDropped = netStatArray[7]
	} else {
		logger.Debug("Nil Network stats found for docker container:" + stats.Name)
	}

	if stats.Disk != nil {
		storageReadBytes, storageWriteBytes := extractStorageUsage(stats.Disk)

		m.StorageReadBytes = storageReadBytes
		m.StorageWriteBytes = storageWriteBytes
	}

	return m
}

// Followed ECS Agent calculations
// https://github.com/aws/amazon-ecs-agent/blob/1ebf0604c13013596cfd4eb239574a85890b13e8/agent/stats/utils.go#L30
func getNetworkStats(stats map[string]NetworkStats) [8]uint64 {
	var netStatArray [8]uint64
	for _, netStat := range stats {
		netStatArray[0] += aws.ToUint64(netStat.RxBytes)
		netStatArray[1] += aws.ToUint64(netStat.RxPackets)
		netStatArray[2] += aws.ToUint64(netStat.RxErrors)
		netStatArray[3] += aws.ToUint64(netStat.RxDropped)

		netStatArray[4] += aws.ToUint64(netStat.TxBytes)
		netStatArray[5] += aws.ToUint64(netStat.TxPackets)
		netStatArray[6] += aws.ToUint64(netStat.TxErrors)
		netStatArray[7] += aws.ToUint64(netStat.TxDropped)
	}
	return netStatArray
}

// Followed ECS Agent calculations
// https://github.com/aws/amazon-ecs-agent/blob/1ebf0604c13013596cfd4eb239574a85890b13e8/agent/stats/utils_unix.go#L48
func extractStorageUsage(stats *DiskStats) (uint64, uint64) {
	var readBytes, writeBytes uint64
	if stats == nil {
		return uint64(0), uint64(0)
	}

	for _, blockStat := range stats.IoServiceBytesRecursives {
		switch op := blockStat.Op; op {
		case "Read":
			readBytes += aws.ToUint64(blockStat.Value)
		case "Write":
			writeBytes += aws.ToUint64(blockStat.Value)

		default:
			// ignoring "Async", "Total", "Sum", etc
			continue
		}
	}
	return readBytes, writeBytes
}

func aggregateTaskMetrics(taskMetrics *ECSMetrics, conMetrics ECSMetrics) {
	taskMetrics.MemoryUsage += conMetrics.MemoryUsage
	taskMetrics.MemoryMaxUsage += conMetrics.MemoryMaxUsage
	taskMetrics.MemoryLimit += conMetrics.MemoryLimit
	taskMetrics.MemoryReserved += conMetrics.MemoryReserved
	taskMetrics.MemoryUtilized += conMetrics.MemoryUtilized

	taskMetrics.CPUTotalUsage += conMetrics.CPUTotalUsage
	taskMetrics.CPUUsageInKernelmode += conMetrics.CPUUsageInKernelmode
	taskMetrics.CPUUsageInUserMode += conMetrics.CPUUsageInUserMode
	taskMetrics.NumOfCPUCores += conMetrics.NumOfCPUCores
	taskMetrics.CPUOnlineCpus += conMetrics.CPUOnlineCpus
	taskMetrics.SystemCPUUsage += conMetrics.SystemCPUUsage
	taskMetrics.CPUReserved += conMetrics.CPUReserved
	taskMetrics.CPUUsageInVCPU += conMetrics.CPUUsageInVCPU

	taskMetrics.NetworkRateRxBytesPerSecond += conMetrics.NetworkRateRxBytesPerSecond
	taskMetrics.NetworkRateTxBytesPerSecond += conMetrics.NetworkRateTxBytesPerSecond

	taskMetrics.NetworkRxBytes += conMetrics.NetworkRxBytes
	taskMetrics.NetworkRxPackets += conMetrics.NetworkRxPackets
	taskMetrics.NetworkRxErrors += conMetrics.NetworkRxErrors
	taskMetrics.NetworkRxDropped += conMetrics.NetworkRxDropped

	taskMetrics.NetworkTxBytes += conMetrics.NetworkTxBytes
	taskMetrics.NetworkTxPackets += conMetrics.NetworkTxPackets
	taskMetrics.NetworkTxErrors += conMetrics.NetworkTxErrors
	taskMetrics.NetworkTxDropped += conMetrics.NetworkTxDropped

	taskMetrics.StorageReadBytes += conMetrics.StorageReadBytes
	taskMetrics.StorageWriteBytes += conMetrics.StorageWriteBytes
}
