// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeletstatsreceiver

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// getValidMockedObjects returns a list of volume claims and persistent
// volume objects based on values present in testdata/pods.json. These
// values will be used to mock objects returned by the Kuberentes API.
func getValidMockedObjects() []runtime.Object {
	return []runtime.Object{
		volumeClaim1,
		awsPersistentVolume,
		volumeClaim2,
		gcePersistentVolume,
		volumeClaim3,
		glusterFSPersistentVolume,
	}
}

var volumeClaim1 = getPVC("volume_claim_1", "kube-system", "storage-provisioner-token-qzlx6")
var volumeClaim2 = getPVC("volume_claim_2", "kube-system", "kube-proxy")
var volumeClaim3 = getPVC("volume_claim_3", "kube-system", "coredns-token-dzc5t")

func getPVC(claimName, namespace, volumeName string) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      claimName,
			Namespace: namespace,
			UID:       types.UID(claimName),
		},
		Spec: v1.PersistentVolumeClaimSpec{
			VolumeName: volumeName,
		},
	}
}

var awsPersistentVolume = func() *v1.PersistentVolume {
	return &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "storage-provisioner-token-qzlx6",
			UID:  "volume_name_1",
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeSource: v1.PersistentVolumeSource{
				AWSElasticBlockStore: &v1.AWSElasticBlockStoreVolumeSource{
					VolumeID:  "volume_id",
					FSType:    "fs_type",
					Partition: 10,
				},
			},
		},
	}
}()

var gcePersistentVolume = func() *v1.PersistentVolume {
	return &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-proxy",
			UID:  "volume_name_2",
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeSource: v1.PersistentVolumeSource{
				GCEPersistentDisk: &v1.GCEPersistentDiskVolumeSource{
					PDName:    "pd_name",
					FSType:    "fs_type",
					Partition: 10,
				},
			},
		},
	}
}()

var glusterFSPersistentVolume = func() *v1.PersistentVolume {
	return &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "coredns-token-dzc5t",
			UID:  "volume_name_3",
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeSource: v1.PersistentVolumeSource{
				Glusterfs: &v1.GlusterfsPersistentVolumeSource{
					EndpointsName: "endpoints_name",
					Path:          "path",
				},
			},
		},
	}
}()

func getMockedObjectsWithEmptyVolumeName() []runtime.Object {
	return []runtime.Object{
		volumeClaim1,
		awsPersistentVolume,
		volumeClaim2,
		gcePersistentVolume,
		volumeClaimWithEmptyVolumeName,
	}
}

var volumeClaimWithEmptyVolumeName = func() *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:         "volume_claim_3",
			GenerateName: "volume_claim_3",
			Namespace:    "kube-system",
			UID:          "volume_claim_3",
		},
		Spec: v1.PersistentVolumeClaimSpec{
			VolumeName: "",
		},
	}
}()

func getMockedObjectsWithNonExistentVolumeName() []runtime.Object {
	return []runtime.Object{
		volumeClaim1,
		awsPersistentVolume,
		volumeClaim2,
		gcePersistentVolume,
		volumeClaim3,
	}
}
