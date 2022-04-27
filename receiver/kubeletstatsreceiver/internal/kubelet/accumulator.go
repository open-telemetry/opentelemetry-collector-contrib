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
}

const (
	scopeName = "otelcol/kubeletstatsreceiver"
)

func (a *metricDataAccumulator) nodeStats(s stats.NodeStats) {
	if !a.metricGroupsToCollect[NodeMetricGroup] {
		return
	}

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	fillNodeResource(rm.Resource(), s)

	ilm := rm.ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName(scopeName)

	startTime := pcommon.NewTimestampFromTime(s.StartTime.Time)
	currentTime := pcommon.NewTimestampFromTime(a.time)
	addCPUMetrics(ilm.Metrics(), metadata.NodeCpuMetrics, s.CPU, startTime, currentTime)
	addMemoryMetrics(ilm.Metrics(), metadata.NodeMemoryMetrics, s.Memory, currentTime)
	addFilesystemMetrics(ilm.Metrics(), metadata.NodeFilesystemMetrics, s.Fs, currentTime)
	addNetworkMetrics(ilm.Metrics(), metadata.NodeNetworkMetrics, s.Network, startTime, currentTime)
	// todo s.Runtime.ImageFs

	a.m = append(a.m, md)
}

func (a *metricDataAccumulator) podStats(s stats.PodStats) {
	if !a.metricGroupsToCollect[PodMetricGroup] {
		return
	}

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	fillPodResource(rm.Resource(), s)

	ilm := rm.ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName(scopeName)

	startTime := pcommon.NewTimestampFromTime(s.StartTime.Time)
	currentTime := pcommon.NewTimestampFromTime(a.time)
	addCPUMetrics(ilm.Metrics(), metadata.PodCpuMetrics, s.CPU, startTime, currentTime)
	addMemoryMetrics(ilm.Metrics(), metadata.PodMemoryMetrics, s.Memory, currentTime)
	addFilesystemMetrics(ilm.Metrics(), metadata.PodFilesystemMetrics, s.EphemeralStorage, currentTime)
	addNetworkMetrics(ilm.Metrics(), metadata.PodNetworkMetrics, s.Network, startTime, currentTime)

	a.m = append(a.m, md)
}

func (a *metricDataAccumulator) containerStats(sPod stats.PodStats, s stats.ContainerStats) {
	if !a.metricGroupsToCollect[ContainerMetricGroup] {
		return
	}

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()

	if err := fillContainerResource(rm.Resource(), sPod, s, a.metadata); err != nil {
		a.logger.Warn(
			"failed to fetch container metrics",
			zap.String("pod", sPod.PodRef.Name),
			zap.String("container", s.Name),
			zap.Error(err))
		return
	}

	ilm := rm.ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName(scopeName)

	startTime := pcommon.NewTimestampFromTime(s.StartTime.Time)
	currentTime := pcommon.NewTimestampFromTime(a.time)
	addCPUMetrics(ilm.Metrics(), metadata.ContainerCpuMetrics, s.CPU, startTime, currentTime)
	addMemoryMetrics(ilm.Metrics(), metadata.ContainerMemoryMetrics, s.Memory, currentTime)
	addFilesystemMetrics(ilm.Metrics(), metadata.ContainerFilesystemMetrics, s.Rootfs, currentTime)
	a.m = append(a.m, md)
}

func (a *metricDataAccumulator) volumeStats(sPod stats.PodStats, s stats.VolumeStats) {
	if !a.metricGroupsToCollect[VolumeMetricGroup] {
		return
	}

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()

	if err := fillVolumeResource(rm.Resource(), sPod, s, a.metadata); err != nil {
		a.logger.Warn(
			"Failed to gather additional volume metadata. Skipping metric collection.",
			zap.String("pod", sPod.PodRef.Name),
			zap.String("volume", s.Name),
			zap.Error(err))
		return
	}

	ilm := rm.ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName(scopeName)

	currentTime := pcommon.NewTimestampFromTime(a.time)
	addVolumeMetrics(ilm.Metrics(), metadata.K8sVolumeMetrics, s, currentTime)
	a.m = append(a.m, md)
}
