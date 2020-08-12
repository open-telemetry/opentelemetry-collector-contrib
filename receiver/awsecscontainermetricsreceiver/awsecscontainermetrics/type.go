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

type ContainerStats struct {
	Name string `json:"name"`
	Id   string `json:"id"`

	Memory      MemoryStats             `json:"memory_stats,omitempty"`
	Disk        DiskStats               `json:"blkio_stats,omitempty"`
	Network     map[string]NetworkStats `json:"networks,omitempty"`
	NetworkRate NetworkRateStats        `json:"network_rate_stats,omitempty"`
	CPU         CPUStats                `json:"cpu_stats,omitempty"`
}

type MemoryStats struct {
	// Memory usage.
	Usage *uint64 `json:"usage,omitempty"`

	// Memory max usage.
	MaxUsage *uint64 `json:"max_usage,omitempty"`

	// Memory limit.
	Limit *uint64 `json:"limit,omitempty"`
}

type DiskStats struct {
	IoServiceBytesRecursives []IoServiceBytesRecursive `json:"io_service_bytes_recursive,omitempty"`
}

type IoServiceBytesRecursive struct {
	Major *uint64 `json:"major,omitempty"`
	Minor *uint64 `json:"minor,omitempty"`
	Op    string  `json:"op,omitempty"`
	Value *uint64 `json:"value,omitempty"`
}

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

type NetworkRateStats struct {
	RxBytesPerSecond *float64 `json:"rx_bytes_per_sec,omitempty"`
	TxBytesPerSecond *float64 `json:"tx_bytes_per_sec,omitempty"`
}

type CPUUsage struct {
	TotalUsage        *uint64   `json:"total_usage,omitempty"`
	UsageInKernelmode *uint64   `json:"usage_in_kernelmode,omitempty"`
	UsageInUserMode   *uint64   `json:"usage_in_usermode,omitempty"`
	PerCpuUsage       []*uint64 `json:"percpu_usage,omitempty"`
}

type CPUStats struct {
	CpuUsage       CPUUsage `json:"cpu_usage,omitempty"`
	OnlineCpus     *uint64  `json:"online_cpus,omitempty"`
	SystemCpuUsage *uint64  `json:"system_cpu_usage,omitempty"`
}

type TaskStats struct {
	MemoryUsage    *uint64
	MemoryMaxUsage *uint64
	MemoryLimit    *uint64

	NetworkRateRxBytesPerSecond *float64
	NetworkRateTxBytesPerSecond *float64

	NetworkRxBytes   *uint64
	NetworkRxPackets *uint64
	NetworkRxErrors  *uint64
	NetworkRxDropped *uint64
	NetworkTxBytes   *uint64
	NetworkTxPackets *uint64
	NetworkTxErrors  *uint64
	NetworkTxDropped *uint64

	CPUTotalUsage        *uint64
	CPUUsageInKernelmode *uint64
	CPUUsageInUserMode   *uint64
	CPUOnlineCpus        *uint64
	SystemCPUUsage       *uint64
	NumOfCPUCores        *uint64

	StorageReadBytes  *uint64
	StorageWriteBytes *uint64
}
