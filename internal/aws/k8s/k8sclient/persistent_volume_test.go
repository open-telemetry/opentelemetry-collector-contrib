// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

var pvObjects = []runtime.Object{
	&corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pv-1",
			UID:  "pv-1-uid",
		},
		Spec: corev1.PersistentVolumeSpec{
			ClaimRef: &corev1.ObjectReference{
				Name:      "pvc-1",
				Namespace: "test-namespace",
			},
		},
	},
	&corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pv-2",
			UID:  "pv2-uid",
		},
		Spec: corev1.PersistentVolumeSpec{
			ClaimRef: &corev1.ObjectReference{
				Name:      "pvc-2",
				Namespace: "another-namespace",
			},
		},
	},
	&corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pv-3",
			UID:  "pv3-uid",
		},
		Spec: corev1.PersistentVolumeSpec{
			ClaimRef: &corev1.ObjectReference{
				Name:      "pvc-3",
				Namespace: "test-namespace",
			},
		},
	},
}

func TestPersistentVolumeClient_GetPersistentVolumeMetrics(t *testing.T) {
	setOption := PersistentVolumeSyncCheckerOption(&mockReflectorSyncChecker{})

	fakeClientSet := fake.NewSimpleClientset(pvObjects...)
	client, err := newPersistentVolumeClient(fakeClientSet, zap.NewNop(), setOption)
	assert.NoError(t, err)

	for _, obj := range pvObjects {
		assert.NoError(t, client.store.Add(obj))
	}

	metrics := client.GetPersistentVolumeMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, 3, metrics.ClusterCount)

	client.shutdown()
	assert.True(t, client.stopped)
}

func TestPersistentVolumeClient_EmptyStore(t *testing.T) {
	setOption := PersistentVolumeSyncCheckerOption(&mockReflectorSyncChecker{})

	fakeClientSet := fake.NewSimpleClientset()
	client, err := newPersistentVolumeClient(fakeClientSet, zap.NewNop(), setOption)
	assert.NoError(t, err)

	// Test with empty store
	metrics := client.GetPersistentVolumeMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, 0, metrics.ClusterCount)

	client.shutdown()
}

func TestTransformFuncPersistentVolume(t *testing.T) {
	info, err := transformFuncPersistentVolume(nil)
	assert.Nil(t, info)
	assert.Error(t, err)

	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pv",
			UID:  "test-pv",
		},
		Spec: corev1.PersistentVolumeSpec{
			ClaimRef: &corev1.ObjectReference{
				Name:      "test-pv",
				Namespace: "test-namespace",
			},
		},
		Status: corev1.PersistentVolumeStatus{
			Phase: corev1.VolumeAvailable,
		},
	}
	result, err := transformFuncPersistentVolume(pv)
	assert.NoError(t, err)

	expectedInfo := &PersistentVolumeInfo{
		Name: "test-pv",
	}
	assert.Equal(t, expectedInfo, result)
}

func TestNoOpPersistentVolumeClient(t *testing.T) {
	client := &noOpPersistentVolumeClient{}

	metrics := client.GetPersistentVolumeMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, 0, metrics.ClusterCount)

	client.shutdown()
}
