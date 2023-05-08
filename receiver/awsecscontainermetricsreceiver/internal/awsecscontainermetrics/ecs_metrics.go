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
