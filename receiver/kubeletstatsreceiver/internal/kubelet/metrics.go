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

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func MetricsData(
	logger *zap.Logger, summary *stats.Summary,
	metadata Metadata,
	metricGroupsToCollect map[MetricGroup]bool,
	mbs *metadata.MetricsBuilders) []pmetric.Metrics {
	acc := &metricDataAccumulator{
		metadata:              metadata,
		logger:                logger,
		metricGroupsToCollect: metricGroupsToCollect,
		time:                  time.Now(),
		mbs:                   mbs,
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
