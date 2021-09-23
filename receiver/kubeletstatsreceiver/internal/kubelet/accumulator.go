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

	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
)

type MetricGroup string

// Values for MetricGroup enum.
const (
	ContainerMetricGroup = MetricGroup("container")
	PodMetricGroup       = MetricGroup("pod")
	NodeMetricGroup      = MetricGroup("node")
	VolumeMetricGroup    = MetricGroup("volume")
)

// ValidMetricGroups map of valid metrics.
var ValidMetricGroups = map[MetricGroup]bool{
	ContainerMetricGroup: true,
	PodMetricGroup:       true,
	NodeMetricGroup:      true,
	VolumeMetricGroup:    true,
}

type metricDataAccumulator struct {
	m                     []*agentmetricspb.ExportMetricsServiceRequest
	metadata              Metadata
	logger                *zap.Logger
	metricGroupsToCollect map[MetricGroup]bool
	time                  time.Time
}

const (
	k8sPrefix       = "k8s."
	nodePrefix      = k8sPrefix + "node."
	podPrefix       = k8sPrefix + "pod."
	containerPrefix = "container."
	volumePrefix    = k8sPrefix + "volume."
)

func (a *metricDataAccumulator) nodeStats(s stats.NodeStats) {
	if !a.metricGroupsToCollect[NodeMetricGroup] {
		return
	}

	// todo s.Runtime.ImageFs
	a.accumulate(
		timestamppb.New(s.StartTime.Time),
		nodeResource(s),

		cpuMetrics(nodePrefix, s.CPU),
		fsMetrics(nodePrefix, s.Fs),
		memMetrics(nodePrefix, s.Memory),
		networkMetrics(nodePrefix, s.Network),
	)
}

func (a *metricDataAccumulator) podStats(podResource *resourcepb.Resource, s stats.PodStats) {
	if !a.metricGroupsToCollect[PodMetricGroup] {
		return
	}

	a.accumulate(
		timestamppb.New(s.StartTime.Time),
		podResource,

		cpuMetrics(podPrefix, s.CPU),
		fsMetrics(podPrefix, s.EphemeralStorage),
		memMetrics(podPrefix, s.Memory),
		networkMetrics(podPrefix, s.Network),
	)
}

func (a *metricDataAccumulator) containerStats(podResource *resourcepb.Resource, s stats.ContainerStats) {
	if !a.metricGroupsToCollect[ContainerMetricGroup] {
		return
	}

	resource, err := containerResource(podResource, s, a.metadata)
	if err != nil {
		a.logger.Warn("failed to fetch container metrics", zap.String("pod", podResource.Labels[conventions.AttributeK8SPodName]),
			zap.String("container", podResource.Labels[conventions.AttributeK8SContainerName]), zap.Error(err))
		return
	}

	// todo s.Logs
	a.accumulate(
		timestamppb.New(s.StartTime.Time),
		resource,

		cpuMetrics(containerPrefix, s.CPU),
		memMetrics(containerPrefix, s.Memory),
		fsMetrics(containerPrefix, s.Rootfs),
	)
}

func (a *metricDataAccumulator) volumeStats(podResource *resourcepb.Resource, s stats.VolumeStats) {
	if !a.metricGroupsToCollect[VolumeMetricGroup] {
		return
	}

	volume, err := volumeResource(podResource, s, a.metadata)
	if err != nil {
		a.logger.Warn(
			"Failed to gather additional volume metadata. Skipping metric collection.",
			zap.String("pod", podResource.Labels[conventions.AttributeK8SPodName]),
			zap.String("volume", podResource.Labels[labelVolumeName]),
			zap.Error(err),
		)
		return
	}

	a.accumulate(
		nil,
		volume,
		volumeMetrics(volumePrefix, s),
	)
}

func (a *metricDataAccumulator) accumulate(
	startTime *timestamppb.Timestamp,
	r *resourcepb.Resource,
	m ...[]*metricspb.Metric,
) {
	var resourceMetrics []*metricspb.Metric
	for _, metrics := range m {
		for _, metric := range applyCurrentTime(metrics, a.time) {
			if metric != nil {
				metric.Timeseries[0].StartTimestamp = startTime
				resourceMetrics = append(resourceMetrics, metric)
			}
		}
	}
	a.m = append(a.m, &agentmetricspb.ExportMetricsServiceRequest{
		Resource: r,
		Metrics:  resourceMetrics,
	})
}
