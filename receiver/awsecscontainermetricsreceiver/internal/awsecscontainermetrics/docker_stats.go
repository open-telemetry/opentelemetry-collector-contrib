// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecscontainermetrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/internal/awsecscontainermetrics"

import "time"

const TaskStatsPath = "/task/stats"

// ContainerStats defines the structure for container stats
type ContainerStats struct {
	Name         string    `json:"name"`
	ID           string    `json:"id"`
	Read         time.Time `json:"read"`
	PreviousRead time.Time `json:"preread"`

	Memory      *MemoryStats            `json:"memory_stats,omitempty"`
	Disk        *DiskStats              `json:"blkio_stats,omitempty"`
	Network     map[string]NetworkStats `json:"networks,omitempty"`
	NetworkRate *NetworkRateStats       `json:"network_rate_stats,omitempty"`
	CPU         *CPUStats               `json:"cpu_stats,omitempty"`
	PreviousCPU *CPUStats               `json:"precpu_stats,omitempty"`
}

// MemoryStats defines the memory stats
type MemoryStats struct {
	Usage          *uint64 `json:"usage,omitempty"`
	MaxUsage       *uint64 `json:"max_usage,omitempty"`
	Limit          *uint64 `json:"limit,omitempty"`
	MemoryUtilized *uint64
	MemoryReserved *uint64
	Stats          map[string]uint64 `json:"stats,omitempty"`
}

// DiskStats defines the storage stats
type DiskStats struct {
	IoServiceBytesRecursives []IoServiceBytesRecursive `json:"io_service_bytes_recursive,omitempty"`
}

// IoServiceBytesRecursive defines the IO device stats
type IoServiceBytesRecursive struct {
	Major *uint64 `json:"major,omitempty"`
	Minor *uint64 `json:"minor,omitempty"`
	Op    string  `json:"op,omitempty"`
	Value *uint64 `json:"value,omitempty"`
}

// NetworkStats defines the network stats
type NetworkStats struct {
	RxBytes   *uint64 `json:"rx_bytes,omitempty"`
	RxPackets *uint64 `json:"rx_packets,omitempty"`
	RxErrors  *uint64 `json:"rx_errors,omitempty"`
	RxDropped *uint64 `json:"rx_dropped,omitempty"`
	TxBytes   *uint64 `json:"tx_bytes,omitempty"`
	TxPackets *uint64 `json:"tx_packets,omitempty"`
	TxErrors  *uint64 `json:"tx_errors,omitempty"`
	TxDropped *uint64 `json:"tx_dropped,omitempty"`
}

// NetworkRateStats doesn't come from docker stat. The rates are being calculated in ECS agent
type NetworkRateStats struct {
	RxBytesPerSecond *float64 `json:"rx_bytes_per_sec,omitempty"`
	TxBytesPerSecond *float64 `json:"tx_bytes_per_sec,omitempty"`
}

// CPUUsage defines raw Cpu usage
type CPUUsage struct {
	TotalUsage        *uint64   `json:"total_usage,omitempty"`
	UsageInKernelmode *uint64   `json:"usage_in_kernelmode,omitempty"`
	UsageInUserMode   *uint64   `json:"usage_in_usermode,omitempty"`
	PerCPUUsage       []*uint64 `json:"percpu_usage,omitempty"`
}

// CPUStats defines Cpu stats
type CPUStats struct {
	CPUUsage       *CPUUsage `json:"cpu_usage,omitempty"`
	OnlineCpus     *uint64   `json:"online_cpus,omitempty"`
	SystemCPUUsage *uint64   `json:"system_cpu_usage,omitempty"`
	CPUUtilized    *uint64
	CPUReserved    *uint64
}

// TaskStats defines the stats for a task
type TaskStats struct {
	Memory            MemoryStats
	NetworkRate       NetworkRateStats
	Network           NetworkStats
	CPU               CPUStats
	StorageReadBytes  *uint64
	StorageWriteBytes *uint64
}
