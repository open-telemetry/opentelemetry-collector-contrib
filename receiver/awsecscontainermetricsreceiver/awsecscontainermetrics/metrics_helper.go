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
	"strconv"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/testutil/metricstestutil"
)

const (
	// BytesInMiB is the number of bytes in a MebiByte.
	BytesInMiB = 1024 * 1024
)

func getContainerMetrics(stats ContainerStats) ECSMetrics {
	memoryUtilizedInMb := (*stats.Memory.Usage - stats.Memory.Stats["cache"]) / BytesInMiB

	numOfCores := (uint64)(len(stats.CPU.CPUUsage.PerCPUUsage))
	cpuUtilized := (float64)(*stats.CPU.CPUUsage.TotalUsage / numOfCores / 1024)

	netStatArray := getNetworkStats(stats.Network)

	storageReadBytes, storageWriteBytes := extractStorageUsage(&stats.Disk)

	m := ECSMetrics{}

	m.MemoryUsage = *stats.Memory.Usage
	m.MemoryMaxUsage = *stats.Memory.MaxUsage
	m.MemoryLimit = *stats.Memory.Limit
	m.MemoryUtilized = memoryUtilizedInMb

	m.CPUTotalUsage = *stats.CPU.CPUUsage.TotalUsage
	m.CPUUsageInKernelmode = *stats.CPU.CPUUsage.UsageInKernelmode
	m.CPUUsageInUserMode = *stats.CPU.CPUUsage.UsageInUserMode
	m.NumOfCPUCores = numOfCores
	m.CPUOnlineCpus = *stats.CPU.OnlineCpus
	m.SystemCPUUsage = *stats.CPU.SystemCPUUsage
	m.CPUUtilized = cpuUtilized

	m.NetworkRateRxBytesPerSecond = *stats.NetworkRate.TxBytesPerSecond
	m.NetworkRateTxBytesPerSecond = *stats.NetworkRate.TxBytesPerSecond

	m.NetworkRxBytes = netStatArray[0]
	m.NetworkRxPackets = netStatArray[1]
	m.NetworkRxErrors = netStatArray[2]
	m.NetworkRxDropped = netStatArray[3]

	m.NetworkTxBytes = netStatArray[4]
	m.NetworkTxPackets = netStatArray[5]
	m.NetworkTxErrors = netStatArray[6]
	m.NetworkTxDropped = netStatArray[7]

	m.StorageReadBytes = storageReadBytes
	m.StorageWriteBytes = storageWriteBytes
	return m
}

func getNetworkStats(stats map[string]NetworkStats) [8]uint64 {
	var netStatArray [8]uint64
	for _, netStat := range stats {
		netStatArray[0] += *netStat.RxBytes
		netStatArray[1] += *netStat.RxPackets
		netStatArray[2] += *netStat.RxErrors
		netStatArray[3] += *netStat.RxDropped

		netStatArray[4] += *netStat.TxBytes
		netStatArray[5] += *netStat.TxPackets
		netStatArray[6] += *netStat.TxErrors
		netStatArray[7] += *netStat.TxDropped
	}
	return netStatArray
}

func extractStorageUsage(stats *DiskStats) (uint64, uint64) {
	var readBytes, writeBytes uint64
	for _, blockStat := range stats.IoServiceBytesRecursives {
		switch op := blockStat.Op; op {
		case "Read":
			readBytes = *blockStat.Value
		case "Write":
			writeBytes = *blockStat.Value
		default:
			//ignoring "Async", "Total", "Sum", etc
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
	taskMetrics.CPUUtilized += conMetrics.CPUUtilized
	taskMetrics.CPUReserved += conMetrics.CPUReserved

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

// GenerateDummyMetrics generates two dummy metrics
func GenerateDummyMetrics() consumerdata.MetricsData {
	md := consumerdata.MetricsData{}

	for i := 0; i < 2; i++ {
		md.Metrics = append(md.Metrics, createGaugeIntMetric(i))
	}
	return md
}

// createGaugeIntMetric creates a int gauge metric
func createGaugeIntMetric(i int) *metricspb.Metric {
	ts := time.Now()
	return metricstestutil.GaugeInt(
		"test_metric_"+strconv.Itoa(i),
		[]string{"label_key_0", "label_key_1"},
		metricstestutil.Timeseries(
			time.Now(),
			[]string{"label_value_0", "label_value_1"},
			&metricspb.Point{
				Timestamp: metricstestutil.Timestamp(ts),
				Value:     &metricspb.Point_Int64Value{Int64Value: int64(i)},
			},
		),
	)
}
