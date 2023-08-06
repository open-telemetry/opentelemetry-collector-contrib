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
	mbs                   *metadata.MetricsBuilders
}

func (a *metricDataAccumulator) nodeStats(s stats.NodeStats) {
	if !a.metricGroupsToCollect[NodeMetricGroup] {
		return
	}

	currentTime := pcommon.NewTimestampFromTime(a.time)
	rb := a.mbs.NodeMetricsBuilder.NewResourceBuilder()
	rb.SetK8sNodeName(s.NodeName)
	rmb := a.mbs.NodeMetricsBuilder.ResourceMetricsBuilder(rb.Emit(),
		metadata.WithStartTimeOverride(pcommon.NewTimestampFromTime(s.StartTime.Time)))
	addCPUMetrics(rmb, metadata.NodeCPUMetrics, s.CPU, currentTime)
	addMemoryMetrics(rmb, metadata.NodeMemoryMetrics, s.Memory, currentTime)
	addFilesystemMetrics(rmb, metadata.NodeFilesystemMetrics, s.Fs, currentTime)
	addNetworkMetrics(rmb, metadata.NodeNetworkMetrics, s.Network, currentTime)
	// todo s.Runtime.ImageFs

	a.m = append(a.m, a.mbs.NodeMetricsBuilder.Emit())
}

func (a *metricDataAccumulator) podStats(s stats.PodStats) {
	if !a.metricGroupsToCollect[PodMetricGroup] {
		return
	}

	rb := a.mbs.PodMetricsBuilder.NewResourceBuilder()
	rb.SetK8sPodUID(s.PodRef.UID)
	rb.SetK8sPodName(s.PodRef.Name)
	rb.SetK8sNamespaceName(s.PodRef.Namespace)
	rmb := a.mbs.PodMetricsBuilder.ResourceMetricsBuilder(rb.Emit(),
		metadata.WithStartTimeOverride(pcommon.NewTimestampFromTime(s.StartTime.Time)))
	currentTime := pcommon.NewTimestampFromTime(a.time)
	addCPUMetrics(rmb, metadata.PodCPUMetrics, s.CPU, currentTime)
	addMemoryMetrics(rmb, metadata.PodMemoryMetrics, s.Memory, currentTime)
	addFilesystemMetrics(rmb, metadata.PodFilesystemMetrics, s.EphemeralStorage, currentTime)
	addNetworkMetrics(rmb, metadata.PodNetworkMetrics, s.Network, currentTime)

	a.m = append(a.m, a.mbs.PodMetricsBuilder.Emit())
}

func (a *metricDataAccumulator) containerStats(sPod stats.PodStats, s stats.ContainerStats) {
	if !a.metricGroupsToCollect[ContainerMetricGroup] {
		return
	}

	rb := a.mbs.ContainerMetricsBuilder.NewResourceBuilder()
	res, err := getContainerResource(rb, sPod, s, a.metadata)
	if err != nil {
		a.logger.Warn(
			"failed to fetch container metrics",
			zap.String("pod", sPod.PodRef.Name),
			zap.String("container", s.Name),
			zap.Error(err))
		return
	}

	currentTime := pcommon.NewTimestampFromTime(a.time)
	rmb := a.mbs.ContainerMetricsBuilder.ResourceMetricsBuilder(res,
		metadata.WithStartTimeOverride(pcommon.NewTimestampFromTime(s.StartTime.Time)))
	addCPUMetrics(rmb, metadata.ContainerCPUMetrics, s.CPU, currentTime)
	addMemoryMetrics(rmb, metadata.ContainerMemoryMetrics, s.Memory, currentTime)
	addFilesystemMetrics(rmb, metadata.ContainerFilesystemMetrics, s.Rootfs, currentTime)

	a.m = append(a.m, a.mbs.ContainerMetricsBuilder.Emit())
}

func (a *metricDataAccumulator) volumeStats(sPod stats.PodStats, s stats.VolumeStats) {
	if !a.metricGroupsToCollect[VolumeMetricGroup] {
		return
	}

	rb := a.mbs.OtherMetricsBuilder.NewResourceBuilder()
	res, err := getVolumeResourceOptions(rb, sPod, s, a.metadata)
	if err != nil {
		a.logger.Warn(
			"Failed to gather additional volume metadata. Skipping metric collection.",
			zap.String("pod", sPod.PodRef.Name),
			zap.String("volume", s.Name),
			zap.Error(err))
		return
	}

	rmb := a.mbs.OtherMetricsBuilder.ResourceMetricsBuilder(res)
	currentTime := pcommon.NewTimestampFromTime(a.time)
	addVolumeMetrics(rmb, metadata.K8sVolumeMetrics, s, currentTime)

	a.m = append(a.m, a.mbs.OtherMetricsBuilder.Emit())
}
