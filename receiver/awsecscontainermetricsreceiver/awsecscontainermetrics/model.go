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

	Memory      MemoryStats      `json:"memory_stats,omitempty"`
	NetworkRate NetworkRateStats `json:"network_rate_stats,omitempty"`
	Disk        DiskStats        `json:"blkio_stats,omitempty"`
}

type MemoryStats struct {
	// Memory usage.
	Usage *uint64 `json:"usage,omitempty"`

	// Memory max usage.
	MaxUsage *uint64 `json:"max_usage,omitempty"`

	// Memory limit.
	Limit *uint64 `json:"limit,omitempty"`
}

type NetworkRateStats struct {
	RxBytesPerSecond *float64 `json:"rx_bytes_per_sec,omitempty"`
	TxBytesPerSecond *float64 `json:"tx_bytes_per_sec,omitempty"`
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
