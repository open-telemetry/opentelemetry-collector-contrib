// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	v1 "k8s.io/api/core/v1"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

const (
	csiDriverGCP = "pd.csi.storage.gke.io"
)

func addVolumeMetrics(mb *metadata.MetricsBuilder, volumeMetrics metadata.VolumeMetrics, s stats.VolumeStats, currentTime pcommon.Timestamp) {
	recordIntDataPoint(mb, volumeMetrics.Available, s.AvailableBytes, currentTime)
	recordIntDataPoint(mb, volumeMetrics.Capacity, s.CapacityBytes, currentTime)
	recordIntDataPoint(mb, volumeMetrics.Inodes, s.Inodes, currentTime)
	recordIntDataPoint(mb, volumeMetrics.InodesFree, s.InodesFree, currentTime)
	recordIntDataPoint(mb, volumeMetrics.InodesUsed, s.InodesUsed, currentTime)
}

func setResourcesFromVolume(rb *metadata.ResourceBuilder, volume v1.Volume) {
	switch {
	// TODO: Support more types
	case volume.ConfigMap != nil:
		rb.SetK8sVolumeType(labelValueConfigMapVolume)
	case volume.DownwardAPI != nil:
		rb.SetK8sVolumeType(labelValueDownwardAPIVolume)
	case volume.EmptyDir != nil:
		rb.SetK8sVolumeType(labelValueEmptyDirVolume)
	case volume.Secret != nil:
		rb.SetK8sVolumeType(labelValueSecretVolume)
	case volume.PersistentVolumeClaim != nil:
		rb.SetK8sVolumeType(labelValuePersistentVolumeClaim)
		rb.SetK8sPersistentvolumeclaimName(volume.PersistentVolumeClaim.ClaimName)
	case volume.HostPath != nil:
		rb.SetK8sVolumeType(labelValueHostPathVolume)
	case volume.AWSElasticBlockStore != nil:
		awsElasticBlockStoreDims(rb, *volume.AWSElasticBlockStore)
	case volume.GCEPersistentDisk != nil:
		gcePersistentDiskDims(rb, *volume.GCEPersistentDisk)
	case volume.Glusterfs != nil:
		glusterfsDims(rb, *volume.Glusterfs)
	}
}

func SetPersistentVolumeLabels(rb *metadata.ResourceBuilder, pv v1.PersistentVolumeSource) {
	// TODO: Support more types
	switch {
	case pv.Local != nil:
		rb.SetK8sVolumeType(labelValueLocalVolume)
	case pv.AWSElasticBlockStore != nil:
		awsElasticBlockStoreDims(rb, *pv.AWSElasticBlockStore)
	case pv.GCEPersistentDisk != nil:
		gcePersistentDiskDims(rb, *pv.GCEPersistentDisk)
	case pv.Glusterfs != nil:
		// pv.Glusterfs is a GlusterfsPersistentVolumeSource instead of GlusterfsVolumeSource,
		// convert to GlusterfsVolumeSource so a single method can handle both structs. This
		// can be broken out into separate methods if one is interested in different sets
		// of labels from the two structs in the future.
		glusterfsDims(rb, v1.GlusterfsVolumeSource{
			EndpointsName: pv.Glusterfs.EndpointsName,
			Path:          pv.Glusterfs.Path,
			ReadOnly:      pv.Glusterfs.ReadOnly,
		})
	case pv.CSI != nil:
		csiPersistentVolumeDims(rb, *pv.CSI)
	}
}

func awsElasticBlockStoreDims(rb *metadata.ResourceBuilder, vs v1.AWSElasticBlockStoreVolumeSource) {
	rb.SetK8sVolumeType(labelValueAWSEBSVolume)
	// AWS specific labels.
	rb.SetAwsVolumeID(vs.VolumeID)
	rb.SetFsType(vs.FSType)
	rb.SetPartition(strconv.Itoa(int(vs.Partition)))
}

func gcePersistentDiskDims(rb *metadata.ResourceBuilder, vs v1.GCEPersistentDiskVolumeSource) {
	rb.SetK8sVolumeType(labelValueGCEPDVolume)
	// GCP specific labels.
	rb.SetGcePdName(vs.PDName)
	rb.SetFsType(vs.FSType)
	rb.SetPartition(strconv.Itoa(int(vs.Partition)))
}

func glusterfsDims(rb *metadata.ResourceBuilder, vs v1.GlusterfsVolumeSource) {
	rb.SetK8sVolumeType(labelValueGlusterFSVolume)
	// GlusterFS specific labels.
	rb.SetGlusterfsEndpointsName(vs.EndpointsName)
	rb.SetGlusterfsPath(vs.Path)
}

func csiPersistentVolumeDims(rb *metadata.ResourceBuilder, vs v1.CSIPersistentVolumeSource) {
	rb.SetK8sVolumeType(labelValueCSIPersistentVolume)

	// CSI specific labels.
	rb.SetCsiVolumeHandle(vs.VolumeHandle)
	rb.SetCsiDriver(vs.Driver)
	rb.SetFsType(vs.FSType)

	// CSI driver specific labels.
	switch vs.Driver {
	case csiDriverGCP:
		// This is in one of two formats:
		// projects/<project>/regions/<region>/disks/<disk>
		// projects/<project>/zones/<zone>/disks/<disk>
		parts := strings.SplitN(vs.VolumeHandle, "/", 6)
		rb.SetGcePdName(parts[len(parts)-1])
	}
}
