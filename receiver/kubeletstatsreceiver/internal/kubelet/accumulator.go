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
	m                     []pdata.Metrics
	metadata              Metadata
	logger                *zap.Logger
	metricGroupsToCollect map[MetricGroup]bool
	time                  time.Time
	typeStr               string
}

const (
	k8sPrefix       = "k8s."
	nodePrefix      = k8sPrefix + "node."
	podPrefix       = k8sPrefix + "pod."
	containerPrefix = "container."
)

func (a *metricDataAccumulator) nodeStats(s stats.NodeStats) {
	if !a.metricGroupsToCollect[NodeMetricGroup] {
		return
	}

	md := pdata.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	fillNodeResource(rm.Resource(), s)

	ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName(a.typeStr)

	startTime := pdata.NewTimestampFromTime(s.StartTime.Time)
	currentTime := pdata.NewTimestampFromTime(a.time)
	addCPUMetrics(ilm.Metrics(), nodePrefix, s.CPU, startTime, currentTime)
	addMemoryMetrics(ilm.Metrics(), nodePrefix, s.Memory, currentTime)
	addFilesystemMetrics(ilm.Metrics(), nodePrefix, s.Fs, currentTime)
	addNetworkMetrics(ilm.Metrics(), nodePrefix, s.Network, startTime, currentTime)
	// todo s.Runtime.ImageFs

	a.m = append(a.m, md)
}

func (a *metricDataAccumulator) podStats(s stats.PodStats) {
	if !a.metricGroupsToCollect[PodMetricGroup] {
		return
	}

	md := pdata.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	fillPodResource(rm.Resource(), s)

	ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName(a.typeStr)

	startTime := pdata.NewTimestampFromTime(s.StartTime.Time)
	currentTime := pdata.NewTimestampFromTime(a.time)
	addCPUMetrics(ilm.Metrics(), podPrefix, s.CPU, startTime, currentTime)
	addMemoryMetrics(ilm.Metrics(), podPrefix, s.Memory, currentTime)
	addFilesystemMetrics(ilm.Metrics(), podPrefix, s.EphemeralStorage, currentTime)
	addNetworkMetrics(ilm.Metrics(), podPrefix, s.Network, startTime, currentTime)

	a.m = append(a.m, md)
}

func (a *metricDataAccumulator) containerStats(sPod stats.PodStats, s stats.ContainerStats) {
	if !a.metricGroupsToCollect[ContainerMetricGroup] {
		return
	}

	md := pdata.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()

	if err := fillContainerResource(rm.Resource(), sPod, s, a.metadata); err != nil {
		a.logger.Warn(
			"failed to fetch container metrics",
			zap.String("pod", sPod.PodRef.Name),
			zap.String("container", s.Name),
			zap.Error(err))
		return
	}

	ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName(a.typeStr)

	startTime := pdata.NewTimestampFromTime(s.StartTime.Time)
	currentTime := pdata.NewTimestampFromTime(a.time)
	addCPUMetrics(ilm.Metrics(), containerPrefix, s.CPU, startTime, currentTime)
	addMemoryMetrics(ilm.Metrics(), containerPrefix, s.Memory, currentTime)
	addFilesystemMetrics(ilm.Metrics(), containerPrefix, s.Rootfs, currentTime)
	a.m = append(a.m, md)
}

func (a *metricDataAccumulator) volumeStats(sPod stats.PodStats, s stats.VolumeStats) {
	if !a.metricGroupsToCollect[VolumeMetricGroup] {
		return
	}

	md := pdata.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()

	if err := fillVolumeResource(rm.Resource(), sPod, s, a.metadata); err != nil {
		a.logger.Warn(
			"Failed to gather additional volume metadata. Skipping metric collection.",
			zap.String("pod", sPod.PodRef.Name),
			zap.String("volume", s.Name),
			zap.Error(err))
		return
	}

	ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName(a.typeStr)

	currentTime := pdata.NewTimestampFromTime(a.time)
	addVolumeMetrics(ilm.Metrics(), k8sPrefix, s, currentTime)
	a.m = append(a.m, md)
}
