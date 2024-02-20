package persistentvolumeclaim

import (
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestTransform(t *testing.T) {
	originalPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-pvc",
			UID:  "my-pvc-uid",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: "my-pv",
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
				corev1.ReadOnlyMany,
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("10Gi"),
			},
		},
	}
	wantPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-pvc",
			UID:  "my-pvc-uid",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: "my-pv",
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
				corev1.ReadOnlyMany,
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("10Gi"),
			},
		},
	}
	assert.Equal(t, wantPVC, Transform(originalPVC))
}
