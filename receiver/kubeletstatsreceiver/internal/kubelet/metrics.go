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

package kubelet

import (
	"time"

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
)

func MetricsData(
	logger *zap.Logger, summary *stats.Summary,
	metadata Metadata, typeStr string,
	metricGroupsToCollect map[MetricGroup]bool) []pdata.Metrics {
	acc := &metricDataAccumulator{
		metadata:              metadata,
		logger:                logger,
		metricGroupsToCollect: metricGroupsToCollect,
		time:                  time.Now(),
		typeStr:               typeStr,
	}

	acc.nodeStats(summary.Node)
	for _, podStats := range summary.Pods {
		acc.podStats(podStats)
		for _, containerStats := range podStats.Containers {
			// propagate the pod resource down to the container
			acc.containerStats(podStats, containerStats)
		}

		for _, volumeStats := range podStats.VolumeStats {
			// propagate the pod resource down to the container
			acc.volumeStats(podStats, volumeStats)
		}
	}
	return acc.m
}
