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

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

// metricDataAccumulator defines the accumulator
type metricDataAccumulator struct {
	mds []pdata.Metrics
}

// getMetricsData generates OT Metrics data from task metadata and docker stats
func (acc *metricDataAccumulator) getMetricsData(containerStatsMap map[string]*ContainerStats, metadata TaskMetadata, logger *zap.Logger) {

	taskMetrics := ECSMetrics{}
	timestamp := pdata.TimeToUnixNano(time.Now())
	taskResource := taskResource(metadata)

	for _, containerMetadata := range metadata.Containers {

		containerResource := containerResource(containerMetadata)
		taskResource.Attributes().ForEach(func(k string, av pdata.AttributeValue) {
			containerResource.Attributes().Upsert(k, av)
		})

		stats, ok := containerStatsMap[containerMetadata.DockerID]

		if ok && !isEmptyStats(stats) {
			containerMetrics := convertContainerMetrics(stats, logger, containerMetadata)
			acc.accumulate(convertToOTLPMetrics(ContainerPrefix, containerMetrics, containerResource, timestamp))
			aggregateTaskMetrics(&taskMetrics, containerMetrics)
			overrideWithTaskLevelLimit(&taskMetrics, metadata)
			acc.accumulate(convertToOTLPMetrics(TaskPrefix, taskMetrics, taskResource, timestamp))
		} else {
			acc.accumulate(convertResourceToRMS(containerResource))
		}
	}
}

func (acc *metricDataAccumulator) accumulate(rms pdata.ResourceMetricsSlice) {
	md := pdata.Metrics(rms)

	acc.mds = append(acc.mds, md)
}

func isEmptyStats(stats *ContainerStats) bool {
	return stats == nil || stats.ID == ""
}

func convertResourceToRMS(containerResource pdata.Resource) pdata.ResourceMetricsSlice {
	rms := pdata.NewResourceMetricsSlice()
	rms.Resize(1)
	rm := rms.At(0)
	containerResource.CopyTo(rm.Resource())
	return rms
}

func convertContainerMetrics(stats *ContainerStats, logger *zap.Logger, containerMetadata ContainerMetadata) ECSMetrics {
	containerMetrics := getContainerMetrics(stats, logger)
	if containerMetadata.Limits.Memory != nil {
		containerMetrics.MemoryReserved = *containerMetadata.Limits.Memory
	}

	if containerMetadata.Limits.CPU != nil {
		containerMetrics.CPUReserved = *containerMetadata.Limits.CPU
	}

	if containerMetrics.CPUReserved > 0 {
		containerMetrics.CPUUtilized = (containerMetrics.CPUUtilized / containerMetrics.CPUReserved)
	}
	return containerMetrics
}

func overrideWithTaskLevelLimit(taskMetrics *ECSMetrics, metadata TaskMetadata) {
	// Overwrite Memory limit with task level limit
	if metadata.Limits.Memory != nil {
		taskMetrics.MemoryReserved = *metadata.Limits.Memory
	}

	taskMetrics.CPUReserved = taskMetrics.CPUReserved / CPUsInVCpu

	// Overwrite CPU limit with task level limit
	if metadata.Limits.CPU != nil {
		taskMetrics.CPUReserved = *metadata.Limits.CPU
	}

	// taskMetrics.CPUReserved cannot be zero. In ECS, user needs to set CPU limit
	// at least in one place (either in task level or in container level). If the
	// task level CPULimit is not present, we calculate it from the summation of
	// all container CPU limits.
	if taskMetrics.CPUReserved > 0 {
		taskMetrics.CPUUtilized = ((taskMetrics.CPUUsageInVCPU / taskMetrics.CPUReserved) * 100)
	}
}
