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

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
)

// metricDataAccumulator defines the accumulator
type metricDataAccumulator struct {
	mds []pmetric.Metrics
}

// getMetricsData generates OT Metrics data from task metadata and docker stats
func (acc *metricDataAccumulator) getMetricsData(containerStatsMap map[string]*ContainerStats, metadata ecsutil.TaskMetadata, logger *zap.Logger) {

	taskMetrics := ECSMetrics{}
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	taskResource := taskResource(metadata)

	for _, containerMetadata := range metadata.Containers {

		containerResource := containerResource(containerMetadata, logger)
		taskResource.Attributes().Range(func(k string, av pcommon.Value) bool {
			av.CopyTo(containerResource.Attributes().PutEmpty(k))
			return true
		})

		stats, ok := containerStatsMap[containerMetadata.DockerID]

		if ok && !isEmptyStats(stats) {

			containerMetrics := convertContainerMetrics(stats, logger, containerMetadata)
			acc.accumulate(convertToOTLPMetrics(containerPrefix, containerMetrics, containerResource, timestamp))
			aggregateTaskMetrics(&taskMetrics, containerMetrics)

		} else if containerMetadata.FinishedAt != "" && containerMetadata.StartedAt != "" {

			duration, err := calculateDuration(containerMetadata.StartedAt, containerMetadata.FinishedAt)

			if err != nil {
				logger.Warn("Error time format error found for this container:" + containerMetadata.ContainerName)
			}

			acc.accumulate(convertStoppedContainerDataToOTMetrics(containerPrefix, containerResource, timestamp, duration))

		}
	}
	overrideWithTaskLevelLimit(&taskMetrics, metadata)
	acc.accumulate(convertToOTLPMetrics(taskPrefix, taskMetrics, taskResource, timestamp))
}

func (acc *metricDataAccumulator) accumulate(md pmetric.Metrics) {
	acc.mds = append(acc.mds, md)
}

func isEmptyStats(stats *ContainerStats) bool {
	return stats == nil || stats.ID == ""
}

func convertContainerMetrics(stats *ContainerStats, logger *zap.Logger, containerMetadata ecsutil.ContainerMetadata) ECSMetrics {
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

func overrideWithTaskLevelLimit(taskMetrics *ECSMetrics, metadata ecsutil.TaskMetadata) {
	// Overwrite Memory limit with task level limit
	if metadata.Limits.Memory != nil {
		taskMetrics.MemoryReserved = *metadata.Limits.Memory
	}

	// Overwrite CPU limit with task level limit
	if metadata.Limits.CPU != nil {
		taskMetrics.CPUReserved = *metadata.Limits.CPU * cpusInVCpu
	}

	// taskMetrics.CPUReserved cannot be zero. In ECS, user needs to set CPU limit
	// at least in one place (either in task level or in container level). If the
	// task level CPULimit is not present, we calculate it from the summation of
	// all container CPU limits.
	if taskMetrics.CPUReserved > 0 {
		taskMetrics.CPUUtilized = taskMetrics.CPUUsageInVCPU * cpusInVCpu
	}
}

func calculateDuration(startTime, endTime string) (float64, error) {
	start, err := time.Parse(time.RFC3339Nano, startTime)
	if err != nil {
		return 0, err
	}
	end, err := time.Parse(time.RFC3339Nano, endTime)
	if err != nil {
		return 0, err
	}
	duration := end.Sub(start)
	return duration.Seconds(), nil
}
