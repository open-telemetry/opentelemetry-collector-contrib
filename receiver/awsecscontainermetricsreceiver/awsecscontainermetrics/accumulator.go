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
	task_prefix      = "task."
	container_prefix = "container."
)

func (acc *metricDataAccumulator) getStats(containerStatsMap map[string]ContainerStats, metadata TaskMetadata) {

	var taskMemLimit uint64
	var taskCpuLimit float64
	for _, containerMetadata := range metadata.Containers {
		stats := containerStatsMap[containerMetadata.DockerId]

		taskMemLimit += *containerMetadata.Limits.Memory
		taskCpuLimit += *containerMetadata.Limits.CPU

		containerResource := containerResource(containerMetadata)
		labelKeys, labelValues := containerLabelKeysAndValues(containerMetadata)
		acc.accumulate(
			timestampProto(time.Now()),
			containerResource,

			memMetrics(container_prefix, &stats.Memory, containerMetadata, labelKeys, labelValues),
			diskMetrics(container_prefix, &stats.Disk, labelKeys, labelValues),
			networkMetrics(container_prefix, stats.Network, labelKeys, labelValues),
			networkRateMetrics(container_prefix, &stats.NetworkRate, labelKeys, labelValues),
			cpuMetrics(container_prefix, &stats.CPU, containerMetadata, labelKeys, labelValues),
		)
	}

	taskCpuLimitInVCpu := taskCpuLimit / 1024
	taskLimit := Limit{Memory: &taskMemLimit, CPU: &taskCpuLimitInVCpu}
	if metadata.Limits.CPU != nil {
		taskLimit.CPU = metadata.Limits.CPU
	}
	if metadata.Limits.Memory != nil {
		taskLimit.Memory = metadata.Limits.Memory
	}

	// Task metrics- aggregation of containers
	taskStat := aggregateTaskStats(containerStatsMap)
	taskResource := taskResource(metadata)
	taskLabelKeys, taskLabelValues := taskLabelKeysAndValues(metadata)
	acc.accumulate(
		timestampProto(time.Now()),
		taskResource,
		taskMetrics(task_prefix, &taskStat, taskLimit, taskLabelKeys, taskLabelValues),
	)
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

func aggregateTaskStats(containerStatsMap map[string]ContainerStats) TaskStats {
	var memUsage, memMaxUsage, memLimit, memUtilized uint64
	var netRateRx, netRateTx float64
	var rBytes, rPackets, rErrors, rDropped uint64
	var tBytes, tPackets, tErrors, tDropped uint64
	var cpuTotal, cpuKernel, cpuUser, cpuSystem, cpuCores, cpuOnline uint64
	var storageRead, storageWrite uint64

	for _, value := range containerStatsMap {
		memUsage += *value.Memory.Usage
		memMaxUsage += *value.Memory.MaxUsage
		memLimit += *value.Memory.Limit
		memoryUtilizedInMb := (*value.Memory.Usage - value.Memory.Stats["cache"]) / BytesInMiB
		memUtilized += memoryUtilizedInMb

		netRateRx += *value.NetworkRate.RxBytesPerSecond
		netRateTx += *value.NetworkRate.TxBytesPerSecond

		for _, netStat := range value.Network {
			rBytes += *netStat.RxBytes
			rPackets += *netStat.RxPackets
			rErrors += *netStat.RxErrors
			rDropped += *netStat.RxDropped

			tBytes += *netStat.TxBytes
			tPackets += *netStat.TxPackets
			tErrors += *netStat.TxErrors
			tDropped += *netStat.TxDropped
		}

		cpuTotal += *value.CPU.CpuUsage.TotalUsage
		cpuKernel += *value.CPU.CpuUsage.UsageInKernelmode
		cpuUser += *value.CPU.CpuUsage.UsageInUserMode
		cpuSystem += *value.CPU.SystemCpuUsage
		cpuOnline += *value.CPU.OnlineCpus
		numOfCores := (uint64)(len(value.CPU.CpuUsage.PerCpuUsage))
		cpuCores += numOfCores

		read, write := extractStorageUsage(&value.Disk)
		if read != nil {
			storageRead += *read
		}
		if write != nil {
			storageWrite += *write
		}
	}
	taskStat := TaskStats{
		MemoryUsage:                 &memUsage,
		MemoryMaxUsage:              &memMaxUsage,
		MemoryLimit:                 &memLimit,
		MemoryUtilized:              &memUtilized,
		NetworkRateRxBytesPerSecond: &netRateRx,
		NetworkRateTxBytesPerSecond: &netRateTx,
		NetworkRxBytes:              &rBytes,
		NetworkRxPackets:            &rPackets,
		NetworkRxErrors:             &rErrors,
		NetworkRxDropped:            &rDropped,
		NetworkTxBytes:              &tBytes,
		NetworkTxPackets:            &tPackets,
		NetworkTxErrors:             &tErrors,
		NetworkTxDropped:            &tDropped,
		CPUTotalUsage:               &cpuTotal,
		CPUUsageInKernelmode:        &cpuKernel,
		CPUUsageInUserMode:          &cpuUser,
		SystemCPUUsage:              &cpuSystem,
		CPUOnlineCpus:               &cpuOnline,
		NumOfCPUCores:               &cpuCores,
		StorageReadBytes:            &storageRead,
		StorageWriteBytes:           &storageWrite,
	}
	return taskStat
}
