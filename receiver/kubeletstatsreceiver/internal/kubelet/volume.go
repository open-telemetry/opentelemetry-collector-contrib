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
	"strconv"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	v1 "k8s.io/api/core/v1"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
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
		labels[labelVolumeType] = labelValueConfigMapVolume
	case volume.DownwardAPI != nil:
		labels[labelVolumeType] = labelValueDownwardAPIVolume
	case volume.EmptyDir != nil:
		labels[labelVolumeType] = labelValueEmptyDirVolume
	case volume.Secret != nil:
		labels[labelVolumeType] = labelValueSecretVolume
	case volume.PersistentVolumeClaim != nil:
		labels[labelVolumeType] = labelValuePersistentVolumeClaim
		labels[labelPersistentVolumeClaimName] = volume.PersistentVolumeClaim.ClaimName
	case volume.HostPath != nil:
		labels[labelVolumeType] = labelValueHostPathVolume
	case volume.AWSElasticBlockStore != nil:
		awsElasticBlockStoreDims(*volume.AWSElasticBlockStore, labels)
	case volume.GCEPersistentDisk != nil:
		gcePersistentDiskDims(*volume.GCEPersistentDisk, labels)
	case volume.Glusterfs != nil:
		glusterfsDims(*volume.Glusterfs, labels)
	}
}

func GetPersistentVolumeLabels(pv v1.PersistentVolumeSource, labels map[string]string) {
	// TODO: Support more types
	switch {
	case pv.Local != nil:
		labels[labelVolumeType] = labelValueLocalVolume
	case pv.AWSElasticBlockStore != nil:
		awsElasticBlockStoreDims(*pv.AWSElasticBlockStore, labels)
	case pv.GCEPersistentDisk != nil:
		gcePersistentDiskDims(*pv.GCEPersistentDisk, labels)
	case pv.Glusterfs != nil:
		// pv.Glusterfs is a GlusterfsPersistentVolumeSource instead of GlusterfsVolumeSource,
		// convert to GlusterfsVolumeSource so a single method can handle both structs. This
		// can be broken out into separate methods if one is interested in different sets
		// of labels from the two structs in the future.
		glusterfsDims(v1.GlusterfsVolumeSource{
			EndpointsName: pv.Glusterfs.EndpointsName,
			Path:          pv.Glusterfs.Path,
			ReadOnly:      pv.Glusterfs.ReadOnly,
		}, labels)
	}
}

func awsElasticBlockStoreDims(vs v1.AWSElasticBlockStoreVolumeSource, labels map[string]string) {
	labels[labelVolumeType] = labelValueAWSEBSVolume
	// AWS specific labels.
	labels["aws.volume.id"] = vs.VolumeID
	labels["fs.type"] = vs.FSType
	labels["partition"] = strconv.Itoa(int(vs.Partition))
}

func gcePersistentDiskDims(vs v1.GCEPersistentDiskVolumeSource, labels map[string]string) {
	labels[labelVolumeType] = labelValueGCEPDVolume
	// GCP specific labels.
	labels["gce.pd.name"] = vs.PDName
	labels["fs.type"] = vs.FSType
	labels["partition"] = strconv.Itoa(int(vs.Partition))
}

func glusterfsDims(vs v1.GlusterfsVolumeSource, labels map[string]string) {
	labels[labelVolumeType] = labelValueGlusterFSVolume
	// GlusterFS specific labels.
	labels["glusterfs.endpoints.name"] = vs.EndpointsName
	labels["glusterfs.path"] = vs.Path
}
