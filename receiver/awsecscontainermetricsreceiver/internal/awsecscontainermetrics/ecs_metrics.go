// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecscontainermetrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/internal/awsecscontainermetrics"

// ECSMetrics defines the structure container/task level metrics
type ECSMetrics struct {
	MemoryUsage    uint64
	MemoryMaxUsage uint64
	MemoryLimit    uint64
	MemoryUtilized uint64
	MemoryReserved uint64

	CPUTotalUsage        uint64
	CPUUsageInKernelmode uint64
	CPUUsageInUserMode   uint64
	CPUOnlineCpus        uint64
	SystemCPUUsage       uint64
	NumOfCPUCores        uint64
	CPUReserved          float64
	CPUUtilized          float64
	CPUUsageInVCPU       float64

	NetworkRateRxBytesPerSecond float64
	NetworkRateTxBytesPerSecond float64

	NetworkRxBytes   uint64
	NetworkRxPackets uint64
	NetworkRxErrors  uint64
	NetworkRxDropped uint64
	NetworkTxBytes   uint64
	NetworkTxPackets uint64
	NetworkTxErrors  uint64
	NetworkTxDropped uint64

	StorageReadBytes  uint64
	StorageWriteBytes uint64
}
