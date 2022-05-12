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

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
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
	a.mbs.WithNodeStartTime.Reset(metadata.WithStartTime(pcommon.NewTimestampFromTime(s.StartTime.Time)))
	currentTime := pcommon.NewTimestampFromTime(a.time)

	addCPUMetrics(a.mbs.WithNodeStartTime, metadata.NodeCPUMetrics, s.CPU, currentTime)
	addMemoryMetrics(a.mbs.WithNodeStartTime, metadata.NodeMemoryMetrics, s.Memory, currentTime)
	addFilesystemMetrics(a.mbs.WithNodeStartTime, metadata.NodeFilesystemMetrics, s.Fs, currentTime)
	addNetworkMetrics(a.mbs.WithNodeStartTime, metadata.NodeNetworkMetrics, s.Network, currentTime)
	// todo s.Runtime.ImageFs

	a.m = append(a.m, a.mbs.WithNodeStartTime.Emit(metadata.WithK8sNodeName(s.NodeName)))
}

func (a *metricDataAccumulator) podStats(s stats.PodStats) {
	if !a.metricGroupsToCollect[PodMetricGroup] {
		return
	}

	a.mbs.WithPodStartTime.Reset(metadata.WithStartTime(pcommon.NewTimestampFromTime(s.StartTime.Time)))
	currentTime := pcommon.NewTimestampFromTime(a.time)

	addCPUMetrics(a.mbs.WithPodStartTime, metadata.PodCPUMetrics, s.CPU, currentTime)
	addMemoryMetrics(a.mbs.WithPodStartTime, metadata.PodMemoryMetrics, s.Memory, currentTime)
	addFilesystemMetrics(a.mbs.WithPodStartTime, metadata.PodFilesystemMetrics, s.EphemeralStorage, currentTime)
	addNetworkMetrics(a.mbs.WithPodStartTime, metadata.PodNetworkMetrics, s.Network, currentTime)

	a.m = append(a.m, a.mbs.WithPodStartTime.Emit(metadata.WithK8sPodUID(s.PodRef.UID),
		metadata.WithK8sPodName(s.PodRef.Name), metadata.WithK8sNamespaceName(s.PodRef.Namespace)))
}

func (a *metricDataAccumulator) containerStats(sPod stats.PodStats, s stats.ContainerStats) {
	if !a.metricGroupsToCollect[ContainerMetricGroup] {
		return
	}

	ro, err := getContainerResourceOptions(sPod, s, a.metadata)
	if err != nil {
		a.logger.Warn(
			"failed to fetch container metrics",
			zap.String("pod", sPod.PodRef.Name),
			zap.String("container", s.Name),
			zap.Error(err))
		return
	}

	a.mbs.WithPodStartTime.Reset(metadata.WithStartTime(pcommon.NewTimestampFromTime(s.StartTime.Time)))
	currentTime := pcommon.NewTimestampFromTime(a.time)

	addCPUMetrics(a.mbs.WithPodStartTime, metadata.ContainerCPUMetrics, s.CPU, currentTime)
	addMemoryMetrics(a.mbs.WithPodStartTime, metadata.ContainerMemoryMetrics, s.Memory, currentTime)
	addFilesystemMetrics(a.mbs.WithPodStartTime, metadata.ContainerFilesystemMetrics, s.Rootfs, currentTime)

	a.m = append(a.m, a.mbs.WithPodStartTime.Emit(ro...))
}

func (a *metricDataAccumulator) volumeStats(sPod stats.PodStats, s stats.VolumeStats) {
	if !a.metricGroupsToCollect[VolumeMetricGroup] {
		return
	}

	ro, err := getVolumeResourceOptions(sPod, s, a.metadata)
	if err != nil {
		a.logger.Warn(
			"Failed to gather additional volume metadata. Skipping metric collection.",
			zap.String("pod", sPod.PodRef.Name),
			zap.String("volume", s.Name),
			zap.Error(err))
		return
	}

	currentTime := pcommon.NewTimestampFromTime(a.time)
	addVolumeMetrics(a.mbs.WithDefaultStartTime, metadata.K8sVolumeMetrics, s, currentTime)

	a.m = append(a.m, a.mbs.WithDefaultStartTime.Emit(ro...))
}
