// Copyright The OpenTelemetry Authors
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
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	v1 "k8s.io/api/core/v1"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func addVolumeMetrics(mb *metadata.MetricsBuilder, volumeMetrics metadata.VolumeMetrics, s stats.VolumeStats, currentTime pcommon.Timestamp) {
	recordIntDataPoint(mb, volumeMetrics.Available, s.AvailableBytes, currentTime)
	recordIntDataPoint(mb, volumeMetrics.Capacity, s.CapacityBytes, currentTime)
	recordIntDataPoint(mb, volumeMetrics.Inodes, s.Inodes, currentTime)
	recordIntDataPoint(mb, volumeMetrics.InodesFree, s.InodesFree, currentTime)
	recordIntDataPoint(mb, volumeMetrics.InodesUsed, s.InodesUsed, currentTime)
}

func getResourcesFromVolume(volume v1.Volume) []metadata.ResourceMetricsOption {
	switch {
	// TODO: Support more types
	case volume.ConfigMap != nil:
		return []metadata.ResourceMetricsOption{metadata.WithK8sVolumeType(labelValueConfigMapVolume)}
	case volume.DownwardAPI != nil:
		return []metadata.ResourceMetricsOption{metadata.WithK8sVolumeType(labelValueDownwardAPIVolume)}
	case volume.EmptyDir != nil:
		return []metadata.ResourceMetricsOption{metadata.WithK8sVolumeType(labelValueEmptyDirVolume)}
	case volume.Secret != nil:
		return []metadata.ResourceMetricsOption{metadata.WithK8sVolumeType(labelValueSecretVolume)}
	case volume.PersistentVolumeClaim != nil:
		return []metadata.ResourceMetricsOption{metadata.WithK8sVolumeType(labelValuePersistentVolumeClaim),
			metadata.WithK8sPersistentvolumeclaimName(volume.PersistentVolumeClaim.ClaimName)}
	case volume.HostPath != nil:
		return []metadata.ResourceMetricsOption{metadata.WithK8sVolumeType(labelValueHostPathVolume)}
	case volume.AWSElasticBlockStore != nil:
		return awsElasticBlockStoreDims(*volume.AWSElasticBlockStore)
	case volume.GCEPersistentDisk != nil:
		return gcePersistentDiskDims(*volume.GCEPersistentDisk)
	case volume.Glusterfs != nil:
		return glusterfsDims(*volume.Glusterfs)
	}
	return nil
}

func GetPersistentVolumeLabels(pv v1.PersistentVolumeSource) []metadata.ResourceMetricsOption {
	// TODO: Support more types
	switch {
	case pv.Local != nil:
		return []metadata.ResourceMetricsOption{metadata.WithK8sVolumeType(labelValueLocalVolume)}
	case pv.AWSElasticBlockStore != nil:
		return awsElasticBlockStoreDims(*pv.AWSElasticBlockStore)
	case pv.GCEPersistentDisk != nil:
		return gcePersistentDiskDims(*pv.GCEPersistentDisk)
	case pv.Glusterfs != nil:
		// pv.Glusterfs is a GlusterfsPersistentVolumeSource instead of GlusterfsVolumeSource,
		// convert to GlusterfsVolumeSource so a single method can handle both structs. This
		// can be broken out into separate methods if one is interested in different sets
		// of labels from the two structs in the future.
		return glusterfsDims(v1.GlusterfsVolumeSource{
			EndpointsName: pv.Glusterfs.EndpointsName,
			Path:          pv.Glusterfs.Path,
			ReadOnly:      pv.Glusterfs.ReadOnly,
		})
	}
	return nil
}

func awsElasticBlockStoreDims(vs v1.AWSElasticBlockStoreVolumeSource) []metadata.ResourceMetricsOption {
	return []metadata.ResourceMetricsOption{
		metadata.WithK8sVolumeType(labelValueAWSEBSVolume),
		// AWS specific labels.
		metadata.WithAwsVolumeID(vs.VolumeID),
		metadata.WithFsType(vs.FSType),
		metadata.WithPartition(strconv.Itoa(int(vs.Partition))),
	}
}

func gcePersistentDiskDims(vs v1.GCEPersistentDiskVolumeSource) []metadata.ResourceMetricsOption {
	return []metadata.ResourceMetricsOption{
		metadata.WithK8sVolumeType(labelValueGCEPDVolume),
		// GCP specific labels.
		metadata.WithGcePdName(vs.PDName),
		metadata.WithFsType(vs.FSType),
		metadata.WithPartition(strconv.Itoa(int(vs.Partition))),
	}
}

func glusterfsDims(vs v1.GlusterfsVolumeSource) []metadata.ResourceMetricsOption {
	return []metadata.ResourceMetricsOption{
		metadata.WithK8sVolumeType(labelValueGlusterFSVolume),
		// GlusterFS specific labels.
		metadata.WithGlusterfsEndpointsName(vs.EndpointsName),
		metadata.WithGlusterfsPath(vs.Path),
	}
}
