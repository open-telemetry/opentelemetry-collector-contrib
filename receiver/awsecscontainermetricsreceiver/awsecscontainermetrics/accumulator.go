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
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"go.opentelemetry.io/collector/consumer/consumerdata"
)

type metricDataAccumulator struct {
	md []*consumerdata.MetricsData
}

const (
	taskPrefix      = "ecs.task."
	containerPrefix = "ecs.container."
	cpusInVCpu      = 1024
)

func (acc *metricDataAccumulator) getMetricsData(containerStatsMap map[string]ContainerStats, metadata TaskMetadata) {

	var taskMemLimit uint64
	var taskCPULimit float64
	taskMetrics := ECSMetrics{}

	for _, containerMetadata := range metadata.Containers {
		stats := containerStatsMap[containerMetadata.DockerId]
		containerMetrics := getContainerMetrics(stats)
		containerMetrics.MemoryReserved = *containerMetadata.Limits.Memory
		containerMetrics.CPUReserved = *containerMetadata.Limits.CPU

		taskMemLimit += *containerMetadata.Limits.Memory
		taskCPULimit += *containerMetadata.Limits.CPU

		containerResource := containerResource(containerMetadata)
		labelKeys, labelValues := containerLabelKeysAndValues(containerMetadata)

		acc.accumulate(
			timestampProto(time.Now()),
			containerResource,

			convertToOTMetrics(containerPrefix, containerMetrics, labelKeys, labelValues),
		)

		aggregateTaskMetrics(&taskMetrics, containerMetrics)
		// Container level metrics accumulation
		// acc.accumulate(
		// 	timestampProto(time.Now()),
		// 	containerResource,

		// 	memMetrics(containerPrefix, &stats.Memory, containerMetadata, labelKeys, labelValues),
		// 	diskMetrics(containerPrefix, &stats.Disk, labelKeys, labelValues),
		// 	networkMetrics(containerPrefix, stats.Network, labelKeys, labelValues),
		// 	networkRateMetrics(containerPrefix, &stats.NetworkRate, labelKeys, labelValues),
		// 	cpuMetrics(containerPrefix, &stats.CPU, containerMetadata, labelKeys, labelValues),
		// )
	}

	taskCpuLimitInVCpu := taskCPULimit / cpusInVCpu
	taskLimit := Limit{Memory: &taskMemLimit, CPU: &taskCpuLimitInVCpu}

	// Overwrite CPU limit with task level limit
	if metadata.Limits.CPU != nil {
		taskLimit.CPU = metadata.Limits.CPU
	}

	// Overwrite Memory limit with task level limit
	if metadata.Limits.Memory != nil {
		taskLimit.Memory = metadata.Limits.Memory
	}

	// Overwrite Memory limit with task level limit
	if metadata.Limits.Memory != nil {
		taskMetrics.MemoryReserved = *metadata.Limits.Memory
	}

	taskMetrics.CPUReserved = taskMetrics.CPUReserved / cpusInVCpu

	// Overwrite CPU limit with task level limit
	if metadata.Limits.CPU != nil {
		taskMetrics.CPUReserved = *metadata.Limits.CPU
	}

	taskResource := taskResource(metadata)
	taskLabelKeys, taskLabelValues := taskLabelKeysAndValues(metadata)
	acc.accumulate(
		timestampProto(time.Now()),
		taskResource,
		convertToOTMetrics(taskPrefix, taskMetrics, taskLabelKeys, taskLabelValues),
	)

	// Task metrics- summation from all containers
	// taskStat := aggregateTaskStats(containerStatsMap)
	// taskResource := taskResource(metadata)
	// taskLabelKeys, taskLabelValues := taskLabelKeysAndValues(metadata)
	// acc.accumulate(
	// 	timestampProto(time.Now()),
	// 	taskResource,
	// 	taskMetrics(taskPrefix, &taskStat, taskLimit, taskLabelKeys, taskLabelValues),
	// )
}

func (acc *metricDataAccumulator) accumulate(
	startTime *timestamp.Timestamp,
	r *resourcepb.Resource,
	m ...[]*metricspb.Metric,
) {
	var resourceMetrics []*metricspb.Metric
	for _, metrics := range m {
		for _, metric := range metrics {
			if metric != nil {
				metric.Timeseries[0].StartTimestamp = startTime
				resourceMetrics = append(resourceMetrics, metric)
			}
		}
	}

	acc.md = append(acc.md, &consumerdata.MetricsData{
		Metrics:  resourceMetrics,
		Resource: r,
	})
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

func aggregateTaskStats(containerStatsMap map[string]ContainerStats) TaskStats {
	var memUsage, memMaxUsage, memLimit, memUtilized uint64
	var netStatArray [8]uint64
	var netRateRx, netRateTx float64
	var cpuTotal, cpuKernel, cpuUser, cpuSystem, cpuCores, cpuOnline uint64
	//var storageRead, storageWrite uint64

	for _, value := range containerStatsMap {
		memUsage += *value.Memory.Usage
		memMaxUsage += *value.Memory.MaxUsage
		memLimit += *value.Memory.Limit
		memoryUtilizedInMb := (*value.Memory.Usage - value.Memory.Stats["cache"]) / BytesInMiB
		memUtilized += memoryUtilizedInMb

		netRateRx += *value.NetworkRate.RxBytesPerSecond
		netRateTx += *value.NetworkRate.TxBytesPerSecond

		net := getNetworkStats(value.Network)

		for i, val := range net {
			netStatArray[i] += val
		}

		cpuTotal += *value.CPU.CpuUsage.TotalUsage
		cpuKernel += *value.CPU.CpuUsage.UsageInKernelmode
		cpuUser += *value.CPU.CpuUsage.UsageInUserMode
		cpuSystem += *value.CPU.SystemCpuUsage
		cpuOnline += *value.CPU.OnlineCpus
		numOfCores := (uint64)(len(value.CPU.CpuUsage.PerCpuUsage))
		cpuCores += numOfCores

		// read, write := extractStorageUsage(&value.Disk)
		// if read != nil {
		// 	storageRead += *read
		// }
		// if write != nil {
		// 	storageWrite += *write
		// }
	}
	taskStat := TaskStats{
		Memory:      MemoryStats{Usage: &memUsage, MaxUsage: &memMaxUsage, Limit: &memLimit, MemoryUtilized: &memUtilized},
		NetworkRate: NetworkRateStats{RxBytesPerSecond: &netRateRx, TxBytesPerSecond: &netRateTx},
		Network: NetworkStats{RxBytes: &netStatArray[0], RxPackets: &netStatArray[1], RxErrors: &netStatArray[2], RxDropped: &netStatArray[3],
			TxBytes: &netStatArray[4], TxPackets: &netStatArray[5], TxErrors: &netStatArray[6], TxDropped: &netStatArray[7]},
		// NetworkRxBytes:              &rBytes,
		// NetworkRxPackets:            &rPackets,
		// NetworkRxErrors:             &rErrors,
		// NetworkRxDropped:            &rDropped,
		// NetworkTxBytes:              &tBytes,
		// NetworkTxPackets:            &tPackets,
		// NetworkTxErrors:             &tErrors,
		// NetworkTxDropped:            &tDropped,
		// CPUTotalUsage:               &cpuTotal,
		// CPUUsageInKernelmode:        &cpuKernel,
		// CPUUsageInUserMode:          &cpuUser,
		// SystemCPUUsage:              &cpuSystem,
		// CPUOnlineCpus:               &cpuOnline,
		// NumOfCPUCores:               &cpuCores,
		// StorageReadBytes:            &storageRead,
		// StorageWriteBytes:           &storageWrite,
	}
	return taskStat
}
