// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

	// We need a separate loop here because /stats/summary does not return pods that are not running
	if metadata.PodsMetadata != nil {
		for _, pod := range metadata.PodsMetadata.Items {
			acc.podState(pod)
			for _, containerStatus := range pod.Status.ContainerStatuses {
				acc.containerState(pod, containerStatus)
			}
		}
	}

	return acc.m
}
