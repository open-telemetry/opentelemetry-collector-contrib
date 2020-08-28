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

func taskMetrics(prefix string, stats *TaskStats, taskLimit Limit, labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue) []*metricspb.Metric {
	return applyCurrentTime([]*metricspb.Metric{
		intGauge(prefix+"memory.usage", "Bytes", stats.MemoryUsage, labelKeys, labelValues),
		intGauge(prefix+"memory.maxusage", "MB", stats.MemoryMaxUsage, labelKeys, labelValues),
		intGauge(prefix+"memory.limit", "MB", stats.MemoryLimit, labelKeys, labelValues),
		intGauge(prefix+"memory.utilized", "MB", stats.MemoryUtilized, labelKeys, labelValues),
		intGauge(prefix+"memory.reserved", "MB", taskLimit.Memory, labelKeys, labelValues),
		doubleGauge(prefix+"network_rate.rx_bytes_per_sec", "Bytes/Sec", stats.NetworkRateRxBytesPerSecond, labelKeys, labelValues),
		doubleGauge(prefix+"network_rate.tx_bytes_per_sec", "Bytes/Sec", stats.NetworkRateTxBytesPerSecond, labelKeys, labelValues),
		intGauge(prefix+"network.rx_bytes", "Bytes", stats.NetworkRxBytes, labelKeys, labelValues),
		intGauge(prefix+"network.rx_packets", "Bytes", stats.NetworkRxPackets, labelKeys, labelValues),
		intGauge(prefix+"network.rx_errors", "Bytes", stats.NetworkRxErrors, labelKeys, labelValues),
		intGauge(prefix+"network.rx_dropped", "Bytes", stats.NetworkRxDropped, labelKeys, labelValues),
		intGauge(prefix+"network.tx_bytes", "Bytes", stats.NetworkTxBytes, labelKeys, labelValues),
		intGauge(prefix+"network.tx_packets", "Bytes", stats.NetworkTxPackets, labelKeys, labelValues),
		intGauge(prefix+"network.tx_errors", "Bytes", stats.NetworkTxErrors, labelKeys, labelValues),
		intGauge(prefix+"network.tx_dropped", "Bytes", stats.NetworkTxDropped, labelKeys, labelValues),
		intGauge(prefix+"disk.storage_read_bytes", "Bytes", stats.StorageReadBytes, labelKeys, labelValues),
		intGauge(prefix+"disk.storage_write_bytes", "Bytes", stats.StorageWriteBytes, labelKeys, labelValues),
		intGauge(prefix+"cpu.total_usage", "Count", stats.CPUTotalUsage, labelKeys, labelValues),
		intGauge(prefix+"cpu.usage_in_kernelmode", "Count", stats.CPUUsageInKernelmode, labelKeys, labelValues),
		intGauge(prefix+"cpu.usage_in_usermode", "Count", stats.CPUUsageInUserMode, labelKeys, labelValues),
		intGauge(prefix+"cpu.number_of_cores", "Count", stats.NumOfCPUCores, labelKeys, labelValues),
		intGauge(prefix+"cpu.online_cpus", "Count", stats.CPUOnlineCpus, labelKeys, labelValues),
		intGauge(prefix+"cpu.system_cpu_usage", "Count", stats.SystemCPUUsage, labelKeys, labelValues),
		doubleGauge(prefix+"cpu.cpu_reserved", "vCPU", taskLimit.CPU, labelKeys, labelValues),

		// memUsageMetric(prefix, stats.MemoryUsage),
		// memMaxUsageMetric(prefix, stats.MemoryMaxUsage),
		// memLimitMetric(prefix, stats.MemoryLimit),
		// memUtilizedMetric(prefix, stats.MemoryUtilized),
		// rxBytesPerSecond(prefix, stats.NetworkRateRxBytesPerSecond),
		// txBytesPerSecond(prefix, stats.NetworkRateTxBytesPerSecond),
		// rxBytes(prefix, stats.NetworkRxBytes),
		// rxPackets(prefix, stats.NetworkRxPackets),
		// rxErrors(prefix, stats.NetworkRxErrors),
		// rxDropped(prefix, stats.NetworkRxDropped),
		// txBytes(prefix, stats.NetworkTxBytes),
		// txPackets(prefix, stats.NetworkTxPackets),
		// txErrors(prefix, stats.NetworkTxErrors),
		// txDropped(prefix, stats.NetworkTxDropped),
		// totalUsageMetric(prefix, stats.CPUTotalUsage),
		// usageInKernelMode(prefix, stats.CPUUsageInKernelmode),
		// usageInUserMode(prefix, stats.CPUUsageInUserMode),
		// numberOfCores(prefix, stats.NumOfCPUCores),
		// onlineCpus(prefix, stats.CPUOnlineCpus),
		// systemCpuUsage(prefix, stats.SystemCPUUsage),
		// storageReadBytes(prefix, stats.StorageReadBytes),
		// storageWriteBytes(prefix, stats.StorageWriteBytes),
		// memReserved(prefix, taskLimit.Memory),
		// cpuReserved(prefix, taskLimit.CPU),
	}, time.Now())
}
