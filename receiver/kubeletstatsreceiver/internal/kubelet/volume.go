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

	"go.opentelemetry.io/collector/model/pdata"
	v1 "k8s.io/api/core/v1"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func addVolumeMetrics(dest pdata.MetricSlice, prefix string, s stats.VolumeStats, currentTime pdata.Timestamp) {
	addIntGauge(dest, prefix, metadata.M.VolumeAvailable, s.AvailableBytes, currentTime)
	addIntGauge(dest, prefix, metadata.M.VolumeCapacity, s.CapacityBytes, currentTime)
	addIntGauge(dest, prefix, metadata.M.VolumeInodes, s.Inodes, currentTime)
	addIntGauge(dest, prefix, metadata.M.VolumeInodesFree, s.InodesFree, currentTime)
	addIntGauge(dest, prefix, metadata.M.VolumeInodesUsed, s.InodesUsed, currentTime)
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
