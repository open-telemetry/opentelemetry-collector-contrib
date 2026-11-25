// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	v1 "k8s.io/api/core/v1"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func addVolumeMetrics(mb *metadata.MetricsBuilder, volumeMetrics metadata.VolumeMetrics, s *stats.VolumeStats, currentTime pcommon.Timestamp) {
	recordIntDataPoint(mb, volumeMetrics.Available, s.AvailableBytes, currentTime)
	recordIntDataPoint(mb, volumeMetrics.Capacity, s.CapacityBytes, currentTime)
	recordIntDataPoint(mb, volumeMetrics.Used, s.UsedBytes, currentTime)
	recordIntDataPoint(mb, volumeMetrics.Inodes, s.Inodes, currentTime)
	recordIntDataPoint(mb, volumeMetrics.InodesFree, s.InodesFree, currentTime)
	recordIntDataPoint(mb, volumeMetrics.InodesUsed, s.InodesUsed, currentTime)
}

func setResourcesFromVolume(rb *metadata.ResourceBuilder, volume *v1.Volume) {
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
		rb.SetK8sVolumeType(labelValueAWSEBSVolume)
	case volume.GCEPersistentDisk != nil:
		rb.SetK8sVolumeType(labelValueGCEPDVolume)
	case volume.Glusterfs != nil:
		rb.SetK8sVolumeType(labelValueGlusterFSVolume)
	}
}

func SetPersistentVolumeLabels(rb *metadata.ResourceBuilder, pv v1.PersistentVolumeSource) {
	// TODO: Support more types
	switch {
	case pv.Local != nil:
		rb.SetK8sVolumeType(labelValueLocalVolume)
	case pv.AWSElasticBlockStore != nil:
		rb.SetK8sVolumeType(labelValueAWSEBSVolume)
	case pv.GCEPersistentDisk != nil:
		rb.SetK8sVolumeType(labelValueGCEPDVolume)
	case pv.Glusterfs != nil:
		rb.SetK8sVolumeType(labelValueGlusterFSVolume)
	}
}
