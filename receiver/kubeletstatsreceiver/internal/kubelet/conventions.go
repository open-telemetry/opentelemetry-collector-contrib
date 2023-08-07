// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

const (
	labelVolumeType = "k8s.volume.type"

	// Volume types.
	labelValuePersistentVolumeClaim = "persistentVolumeClaim"
	labelValueConfigMapVolume       = "configMap"
	labelValueDownwardAPIVolume     = "downwardAPI"
	labelValueEmptyDirVolume        = "emptyDir"
	labelValueSecretVolume          = "secret"
	labelValueHostPathVolume        = "hostPath"
	labelValueLocalVolume           = "local"
	labelValueAWSEBSVolume          = "awsElasticBlockStore"
	labelValueGCEPDVolume           = "gcePersistentDisk"
	labelValueGlusterFSVolume       = "glusterfs"
)
