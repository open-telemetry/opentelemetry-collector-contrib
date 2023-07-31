// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
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
	m                     []pmetric.Metrics
	metadata              Metadata
	logger                *zap.Logger
	metricGroupsToCollect map[MetricGroup]bool
	time                  time.Time
	rb                    *metadata.ResourceBuilder
	mbs                   *metadata.MetricsBuilders
}

func (a *metricDataAccumulator) nodeStats(s stats.NodeStats) {
	if !a.metricGroupsToCollect[NodeMetricGroup] {
		return
	}

	currentTime := pcommon.NewTimestampFromTime(a.time)
	addCPUMetrics(a.mbs.NodeMetricsBuilder, metadata.NodeCPUMetrics, s.CPU, currentTime)
	addMemoryMetrics(a.mbs.NodeMetricsBuilder, metadata.NodeMemoryMetrics, s.Memory, currentTime)
	addFilesystemMetrics(a.mbs.NodeMetricsBuilder, metadata.NodeFilesystemMetrics, s.Fs, currentTime)
	addNetworkMetrics(a.mbs.NodeMetricsBuilder, metadata.NodeNetworkMetrics, s.Network, currentTime)
	// todo s.Runtime.ImageFs
	a.rb.SetK8sNodeName(s.NodeName)
	a.m = append(a.m, a.mbs.NodeMetricsBuilder.Emit(
		metadata.WithStartTimeOverride(pcommon.NewTimestampFromTime(s.StartTime.Time)),
		metadata.WithResource(a.rb.Emit()),
	))
}

func (a *metricDataAccumulator) podStats(s stats.PodStats) {
	if !a.metricGroupsToCollect[PodMetricGroup] {
		return
	}

	currentTime := pcommon.NewTimestampFromTime(a.time)
	addCPUMetrics(a.mbs.PodMetricsBuilder, metadata.PodCPUMetrics, s.CPU, currentTime)
	addMemoryMetrics(a.mbs.PodMetricsBuilder, metadata.PodMemoryMetrics, s.Memory, currentTime)
	addFilesystemMetrics(a.mbs.PodMetricsBuilder, metadata.PodFilesystemMetrics, s.EphemeralStorage, currentTime)
	addNetworkMetrics(a.mbs.PodMetricsBuilder, metadata.PodNetworkMetrics, s.Network, currentTime)

	a.rb.SetK8sPodUID(s.PodRef.UID)
	a.rb.SetK8sPodName(s.PodRef.Name)
	a.rb.SetK8sNamespaceName(s.PodRef.Namespace)
	a.m = append(a.m, a.mbs.PodMetricsBuilder.Emit(
		metadata.WithStartTimeOverride(pcommon.NewTimestampFromTime(s.StartTime.Time)),
		metadata.WithResource(a.rb.Emit()),
	))
}

func (a *metricDataAccumulator) containerStats(sPod stats.PodStats, s stats.ContainerStats) {
	if !a.metricGroupsToCollect[ContainerMetricGroup] {
		return
	}

	res, err := getContainerResource(a.rb, sPod, s, a.metadata)
	if err != nil {
		a.logger.Warn(
			"failed to fetch container metrics",
			zap.String("pod", sPod.PodRef.Name),
			zap.String("container", s.Name),
			zap.Error(err))
		return
	}

	currentTime := pcommon.NewTimestampFromTime(a.time)
	addCPUMetrics(a.mbs.ContainerMetricsBuilder, metadata.ContainerCPUMetrics, s.CPU, currentTime)
	addMemoryMetrics(a.mbs.ContainerMetricsBuilder, metadata.ContainerMemoryMetrics, s.Memory, currentTime)
	addFilesystemMetrics(a.mbs.ContainerMetricsBuilder, metadata.ContainerFilesystemMetrics, s.Rootfs, currentTime)

	a.m = append(a.m, a.mbs.ContainerMetricsBuilder.Emit(
		metadata.WithStartTimeOverride(pcommon.NewTimestampFromTime(s.StartTime.Time)),
		metadata.WithResource(res),
	))
}

func (a *metricDataAccumulator) volumeStats(sPod stats.PodStats, s stats.VolumeStats) {
	if !a.metricGroupsToCollect[VolumeMetricGroup] {
		return
	}

	res, err := getVolumeResourceOptions(a.rb, sPod, s, a.metadata)
	if err != nil {
		a.logger.Warn(
			"Failed to gather additional volume metadata. Skipping metric collection.",
			zap.String("pod", sPod.PodRef.Name),
			zap.String("volume", s.Name),
			zap.Error(err))
		return
	}

	currentTime := pcommon.NewTimestampFromTime(a.time)
	addVolumeMetrics(a.mbs.OtherMetricsBuilder, metadata.K8sVolumeMetrics, s, currentTime)

	a.m = append(a.m, a.mbs.OtherMetricsBuilder.Emit(metadata.WithResource(res)))
}
