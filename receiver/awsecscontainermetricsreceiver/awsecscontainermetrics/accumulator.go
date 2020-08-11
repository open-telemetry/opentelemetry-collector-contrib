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

func (acc *metricDataAccumulator) getStats(containerStatsMap map[string]ContainerStats) {
	for _, value := range containerStatsMap {
		acc.accumulate(
			timestampProto(time.Now()),

			memMetrics(container_prefix, &value.Memory),
			diskMetrics(container_prefix, &value.Disk),
			networkMetrics(container_prefix, value.Network),
			networkRateMetrics(container_prefix, &value.NetworkRate),
			cpuMetrics(container_prefix, &value.CPU),
		)
	}

	taskStat := aggregateTaskStats(containerStatsMap)
	acc.accumulate(
		timestampProto(time.Now()),
		taskMetrics(task_prefix, taskStat),
	)
}

func (acc *metricDataAccumulator) accumulate(
	startTime *timestamp.Timestamp,
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
		Metrics: resourceMetrics,
		Resource: &resourcepb.Resource{
			Type:   "awsecscontainermetrics", // k8s/pod/container
			Labels: map[string]string{"receiver": "awsecscontainermetrics"},
		},
	})
}

func aggregateTaskStats(containerStatsMap map[string]ContainerStats) TaskStats {
	var memUsage, memMaxUsage, memLimit uint64
	for _, value := range containerStatsMap {
		memUsage += *value.Memory.Usage
		memMaxUsage += *value.Memory.MaxUsage
		memLimit += *value.Memory.Limit
	}
	taskStat := TaskStats{
		MemoryUsage
	}
	return taskStat
}
