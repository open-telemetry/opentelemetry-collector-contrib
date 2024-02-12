package persistentvolume

import (
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestTransform(t *testing.T) {
	originalPV := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-pv",
			UID:  "my-pv-uid",
		},
		Spec: corev1.PersistentVolumeSpec{
			StorageClassName: "standard",
			MountOptions: []string{
				"azureFile",
				"nfs",
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("10Gi"),
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
		},
		Status: corev1.PersistentVolumeStatus{
			Phase: corev1.VolumeBound,
		},
	}
	wantPV := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-pv",
			UID:  "my-pv-uid",
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("10Gi"),
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
		},
		Status: corev1.PersistentVolumeStatus{
			Phase: corev1.VolumeBound,
		},
	}
	assert.Equal(t, wantPV, Transform(originalPV))
}
