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
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func convertToOTMetrics(prefix string, m ECSMetrics, labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue, timestamp *timestamppb.Timestamp) []*metricspb.Metric {

	return applyTimeStamp([]*metricspb.Metric{
		intGauge(prefix+"memory_usage", "Bytes", &m.MemoryUsage, labelKeys, labelValues),
		intGauge(prefix+"memory_maxusage", "Bytes", &m.MemoryMaxUsage, labelKeys, labelValues),
		intGauge(prefix+"memory_limit", "Bytes", &m.MemoryLimit, labelKeys, labelValues),
		intGauge(prefix+"memory_utilized", "MB", &m.MemoryUtilized, labelKeys, labelValues),
		intGauge(prefix+"memory_reserved", "MB", &m.MemoryReserved, labelKeys, labelValues),

		intGauge(prefix+"cpu_total_usage", "NanoSecond", &m.CPUTotalUsage, labelKeys, labelValues),
		intGauge(prefix+"cpu_usage_in_kernelmode", "NanoSecond", &m.CPUUsageInKernelmode, labelKeys, labelValues),
		intGauge(prefix+"cpu_usage_in_usermode", "NanoSecond", &m.CPUUsageInUserMode, labelKeys, labelValues),
		intGauge(prefix+"number_of_cpu_cores", "Count", &m.NumOfCPUCores, labelKeys, labelValues),
		intGauge(prefix+"online_cpus", "Count", &m.CPUOnlineCpus, labelKeys, labelValues),
		intGauge(prefix+"system_cpu_usage", "NanoSecond", &m.SystemCPUUsage, labelKeys, labelValues),
		doubleGauge(prefix+"cpu_utilized", "vCPU", &m.CPUUtilized, labelKeys, labelValues),
		doubleGauge(prefix+"cpu_reserved", "vCPU", &m.CPUReserved, labelKeys, labelValues),

		doubleGauge(prefix+"network_rate.rx_bytes_per_sec", "Bytes/Sec", &m.NetworkRateRxBytesPerSecond, labelKeys, labelValues),
		doubleGauge(prefix+"network_rate.tx_bytes_per_sec", "Bytes/Sec", &m.NetworkRateTxBytesPerSecond, labelKeys, labelValues),

		intGauge(prefix+"network_rx_bytes", "Bytes", &m.NetworkRxBytes, labelKeys, labelValues),
		intGauge(prefix+"network_rx_packets", "Bytes", &m.NetworkRxPackets, labelKeys, labelValues),
		intGauge(prefix+"network_rx_errors", "Bytes", &m.NetworkRxErrors, labelKeys, labelValues),
		intGauge(prefix+"network_rx_dropped", "Bytes", &m.NetworkRxDropped, labelKeys, labelValues),
		intGauge(prefix+"network_tx_bytes", "Bytes", &m.NetworkTxBytes, labelKeys, labelValues),
		intGauge(prefix+"network_tx_packets", "Bytes", &m.NetworkTxPackets, labelKeys, labelValues),
		intGauge(prefix+"network_tx_errors", "Bytes", &m.NetworkTxErrors, labelKeys, labelValues),
		intGauge(prefix+"network_tx_dropped", "Bytes", &m.NetworkTxDropped, labelKeys, labelValues),

		intGauge(prefix+"storage_read_bytes", "Bytes", &m.StorageReadBytes, labelKeys, labelValues),
		intGauge(prefix+"storage_write_bytes", "Bytes", &m.StorageWriteBytes, labelKeys, labelValues),
	}, timestamp)
}
