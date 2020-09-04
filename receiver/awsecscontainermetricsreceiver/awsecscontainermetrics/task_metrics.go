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
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

func taskMetricsCreator(prefix string, stats *TaskStats, taskLimit Limit, labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue) []*metricspb.Metric {
	return applyCurrentTime([]*metricspb.Metric{
		intGauge(prefix+"memory.usage", "Bytes", stats.Memory.Usage, labelKeys, labelValues),
		intGauge(prefix+"memory.maxusage", "MB", stats.Memory.MaxUsage, labelKeys, labelValues),
		intGauge(prefix+"memory.limit", "MB", stats.Memory.Limit, labelKeys, labelValues),
		intGauge(prefix+"memory.utilized", "MB", stats.Memory.MemoryUtilized, labelKeys, labelValues),
		intGauge(prefix+"memory.reserved", "MB", taskLimit.Memory, labelKeys, labelValues),

		doubleGauge(prefix+"network_rate.rx_bytes_per_sec", "Bytes/Sec", stats.NetworkRate.RxBytesPerSecond, labelKeys, labelValues),
		doubleGauge(prefix+"network_rate.tx_bytes_per_sec", "Bytes/Sec", stats.NetworkRate.TxBytesPerSecond, labelKeys, labelValues),

		intGauge(prefix+"network.rx_bytes", "Bytes", stats.Network.RxBytes, labelKeys, labelValues),
		intGauge(prefix+"network.rx_packets", "Bytes", stats.Network.RxPackets, labelKeys, labelValues),
		intGauge(prefix+"network.rx_errors", "Bytes", stats.Network.RxErrors, labelKeys, labelValues),
		intGauge(prefix+"network.rx_dropped", "Bytes", stats.Network.RxDropped, labelKeys, labelValues),
		intGauge(prefix+"network.tx_bytes", "Bytes", stats.Network.TxBytes, labelKeys, labelValues),
		intGauge(prefix+"network.tx_packets", "Bytes", stats.Network.TxPackets, labelKeys, labelValues),
		intGauge(prefix+"network.tx_errors", "Bytes", stats.Network.TxErrors, labelKeys, labelValues),
		intGauge(prefix+"network.tx_dropped", "Bytes", stats.Network.TxDropped, labelKeys, labelValues),

		// intGauge(prefix+"disk.storage_read_bytes", "Bytes", stats.StorageReadBytes, labelKeys, labelValues),
		// intGauge(prefix+"disk.storage_write_bytes", "Bytes", stats.StorageWriteBytes, labelKeys, labelValues),
		// intGauge(prefix+"cpu.total_usage", "Count", stats.CPUTotalUsage, labelKeys, labelValues),
		// intGauge(prefix+"cpu.usage_in_kernelmode", "Count", stats.CPUUsageInKernelmode, labelKeys, labelValues),
		// intGauge(prefix+"cpu.usage_in_usermode", "Count", stats.CPUUsageInUserMode, labelKeys, labelValues),
		// intGauge(prefix+"cpu.number_of_cores", "Count", stats.NumOfCPUCores, labelKeys, labelValues),
		// intGauge(prefix+"cpu.online_cpus", "Count", stats.CPUOnlineCpus, labelKeys, labelValues),
		// intGauge(prefix+"cpu.system_cpu_usage", "Count", stats.SystemCPUUsage, labelKeys, labelValues),
		// doubleGauge(prefix+"cpu.cpu_reserved", "vCPU", taskLimit.CPU, labelKeys, labelValues),
	}, time.Now())
}
