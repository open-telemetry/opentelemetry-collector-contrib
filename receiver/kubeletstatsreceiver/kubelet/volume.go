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
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	v1 "k8s.io/api/core/v1"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

func volumeMetrics(metricPrefix string, volumeStats stats.VolumeStats) []*metricspb.Metric {
	return []*metricspb.Metric{
		volumeAvailableMetric(metricPrefix, volumeStats),
		volumeCapacityMetric(metricPrefix, volumeStats),
		volumeInodesMetric(metricPrefix, volumeStats),
		volumeInodesFreeMetric(metricPrefix, volumeStats),
		volumeInodesUsedMetric(metricPrefix, volumeStats),
	}
}

func volumeAvailableMetric(prefix string, s stats.VolumeStats) *metricspb.Metric {
	if s.AvailableBytes == nil {
		return nil
	}
	return intGaugeWithDescription(
		prefix+"available", "By",
		"The number of available bytes in the volume.",
		s.AvailableBytes,
	)
}

func volumeCapacityMetric(prefix string, s stats.VolumeStats) *metricspb.Metric {
	if s.CapacityBytes == nil {
		return nil
	}
	return intGaugeWithDescription(
		prefix+"capacity", "By",
		"The total capacity in bytes of the volume.",
		s.CapacityBytes,
	)
}

func volumeInodesMetric(prefix string, s stats.VolumeStats) *metricspb.Metric {
	if s.Inodes == nil {
		return nil
	}
	return intGaugeWithDescription(
		prefix+"inodes", "1",
		"The total inodes in the filesystem.",
		s.Inodes,
	)
}

func volumeInodesFreeMetric(prefix string, s stats.VolumeStats) *metricspb.Metric {
	if s.InodesFree == nil {
		return nil
	}
	return intGaugeWithDescription(
		prefix+"inodes.free", "1",
		"The free inodes in the filesystem.",
		s.InodesFree,
	)
}

func volumeInodesUsedMetric(prefix string, s stats.VolumeStats) *metricspb.Metric {
	if s.InodesUsed == nil {
		return nil
	}
	return intGaugeWithDescription(
		prefix+"inodes.used", "1",
		"The inodes used by the filesystem. This may not equal inodes -"+
			" free because filesystem may share inodes with other filesystems.",
		s.InodesUsed,
	)
}

func getLabelsFromVolume(volume v1.Volume, labels map[string]string) {
	switch {
	// TODO: Support more types
	case volume.ConfigMap != nil:
		labels[labelVolumeType] = "configMap"
	case volume.DownwardAPI != nil:
		labels[labelVolumeType] = "downwardAPI"
	case volume.EmptyDir != nil:
		labels[labelVolumeType] = "emptyDir"
	case volume.Secret != nil:
		labels[labelVolumeType] = "secret"
	case volume.PersistentVolumeClaim != nil:
		labels[labelVolumeType] = "persistentVolumeClaim"
	case volume.HostPath != nil:
		labels[labelVolumeType] = "hostPath"
	}
}
