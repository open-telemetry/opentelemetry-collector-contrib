// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package persistentvolumeclaim

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestTransform(t *testing.T) {
	scName := "standard-rwo"
	origPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data-claim",
			Namespace: "default",
			UID:       "data-claim-uid",
			Labels: map[string]string{
				"app": "postgres",
			},
			Annotations: map[string]string{
				"volume.beta.kubernetes.io/storage-provisioner":    "pd.csi.storage.gke.io",
				"kubectl.kubernetes.io/last-applied-configuration": `{"apiVersion":"v1"}`,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
			VolumeName:       "pv-data-01",
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: *resource.NewQuantity(50*1024*1024*1024, resource.BinarySI),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: *resource.NewQuantity(50*1024*1024*1024, resource.BinarySI),
			},
		},
	}

	wantPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data-claim",
			Namespace: "default",
			UID:       "data-claim-uid",
			Labels: map[string]string{
				"app": "postgres",
			},
			Annotations: map[string]string{
				"volume.beta.kubernetes.io/storage-provisioner": "pd.csi.storage.gke.io",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
			VolumeName:       "pv-data-01",
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: *resource.NewQuantity(50*1024*1024*1024, resource.BinarySI),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: *resource.NewQuantity(50*1024*1024*1024, resource.BinarySI),
			},
		},
	}

	assert.Equal(t, wantPVC, Transform(origPVC))
}

func TestTransformWithoutStorageClass(t *testing.T) {
	origPVC := testutils.NewPersistentVolumeClaim("1")
	origPVC.Spec.StorageClassName = nil

	transformed := Transform(origPVC)

	assert.Equal(t, "test-pvc-1", transformed.Name)
	assert.Equal(t, corev1.ClaimBound, transformed.Status.Phase)
	assert.Nil(t, transformed.Spec.StorageClassName)
}

func newPVCMetricsBuilder() *metadata.MetricsBuilder {
	mbc := metadata.NewDefaultMetricsBuilderConfig()
	mbc.Metrics.K8sPersistentvolumeclaimStatusPhase.Enabled = true
	mbc.Metrics.K8sPersistentvolumeclaimStorageRequest.Enabled = true
	mbc.Metrics.K8sPersistentvolumeclaimStorageCapacity.Enabled = true
	return metadata.NewMetricsBuilder(mbc, receivertest.NewNopSettings(metadata.Type))
}

func TestGoldenFile(t *testing.T) {
	pvc := testutils.NewPersistentVolumeClaim("1")
	ts := pcommon.Timestamp(time.Now().UnixNano())
	mb := newPVCMetricsBuilder()
	RecordMetrics(mb, pvc, ts)
	m := mb.Emit()

	expectedFile := filepath.Join("testdata", "expected.yaml")
	// golden.WriteMetrics(t, expectedFile, m)
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expected, m,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
}

func TestGoldenFilePending(t *testing.T) {
	pvc := testutils.NewPersistentVolumeClaim("1")
	pvc.Status.Phase = corev1.ClaimPending
	pvc.Status.Capacity = nil
	pvc.Spec.VolumeName = ""

	ts := pcommon.Timestamp(time.Now().UnixNano())
	mb := newPVCMetricsBuilder()
	RecordMetrics(mb, pvc, ts)
	m := mb.Emit()

	expectedFile := filepath.Join("testdata", "expected_pending.yaml")
	// golden.WriteMetrics(t, expectedFile, m)
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expected, m,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
}

func TestRecordMetrics(t *testing.T) {
	pvc := testutils.NewPersistentVolumeClaim("1")

	ts := pcommon.Timestamp(time.Now().UnixNano())
	mb := newPVCMetricsBuilder()
	RecordMetrics(mb, pvc, ts)
	m := mb.Emit()

	require.Equal(t, 1, m.ResourceMetrics().Len())
	rm := m.ResourceMetrics().At(0)

	assert.Equal(t,
		map[string]any{
			"k8s.persistentvolumeclaim.uid":  "test-pvc-1-uid",
			"k8s.persistentvolumeclaim.name": "test-pvc-1",
			"k8s.namespace.name":             "test-namespace",
			"k8s.storageclass.name":          "standard",
		},
		rm.Resource().Attributes().AsRaw(),
	)

	require.Equal(t, 1, rm.ScopeMetrics().Len())
	sm := rm.ScopeMetrics().At(0)

	var foundPhase, foundRequest, foundCapacity bool
	for i := 0; i < sm.Metrics().Len(); i++ {
		metric := sm.Metrics().At(i)
		switch metric.Name() {
		case "k8s.persistentvolumeclaim.status.phase":
			foundPhase = true
			for j := 0; j < metric.Sum().DataPoints().Len(); j++ {
				dp := metric.Sum().DataPoints().At(j)
				phaseAttr, _ := dp.Attributes().Get("k8s.persistentvolumeclaim.status.phase")
				if phaseAttr.Str() == "Bound" {
					assert.Equal(t, int64(1), dp.IntValue())
				} else {
					assert.Equal(t, int64(0), dp.IntValue(), "non-active phase %s should be 0", phaseAttr.Str())
				}
			}
		case "k8s.persistentvolumeclaim.storage.request":
			foundRequest = true
			require.Equal(t, 1, metric.Sum().DataPoints().Len())
			assert.Equal(t, int64(5*1024*1024*1024), metric.Sum().DataPoints().At(0).IntValue())
		case "k8s.persistentvolumeclaim.storage.capacity":
			foundCapacity = true
			require.Equal(t, 1, metric.Sum().DataPoints().Len())
			assert.Equal(t, int64(10*1024*1024*1024), metric.Sum().DataPoints().At(0).IntValue())
		}
	}
	assert.True(t, foundPhase, "status.phase metric should be present")
	assert.True(t, foundRequest, "storage.request metric should be present")
	assert.True(t, foundCapacity, "storage.capacity metric should be present")
}

func TestRecordMetricsPendingPVC(t *testing.T) {
	pvc := testutils.NewPersistentVolumeClaim("1")
	pvc.Status.Phase = corev1.ClaimPending
	pvc.Status.Capacity = nil
	pvc.Spec.VolumeName = ""

	ts := pcommon.Timestamp(time.Now().UnixNano())
	mb := newPVCMetricsBuilder()
	RecordMetrics(mb, pvc, ts)
	m := mb.Emit()

	require.Equal(t, 1, m.ResourceMetrics().Len())
	sm := m.ResourceMetrics().At(0).ScopeMetrics().At(0)

	var foundRequest, foundCapacity, foundPhase bool
	for i := 0; i < sm.Metrics().Len(); i++ {
		metric := sm.Metrics().At(i)
		switch metric.Name() {
		case "k8s.persistentvolumeclaim.storage.request":
			foundRequest = true
		case "k8s.persistentvolumeclaim.storage.capacity":
			foundCapacity = true
		case "k8s.persistentvolumeclaim.status.phase":
			foundPhase = true
			for j := 0; j < metric.Sum().DataPoints().Len(); j++ {
				dp := metric.Sum().DataPoints().At(j)
				phaseAttr, _ := dp.Attributes().Get("k8s.persistentvolumeclaim.status.phase")
				if phaseAttr.Str() == "Pending" {
					assert.Equal(t, int64(1), dp.IntValue())
				} else {
					assert.Equal(t, int64(0), dp.IntValue())
				}
			}
		}
	}
	assert.True(t, foundRequest, "storage.request metric should be present for pending PVC")
	assert.False(t, foundCapacity, "storage.capacity metric should NOT be present for pending PVC")
	assert.True(t, foundPhase, "status.phase metric should be present for pending PVC")
}

func TestGetMetadata(t *testing.T) {
	pvc := testutils.NewPersistentVolumeClaim("1")
	pvc.Annotations = map[string]string{"volume.beta.kubernetes.io/storage-provisioner": "gce-pd"}

	meta := GetMetadata(pvc)

	require.Len(t, meta, 1)
	require.Contains(t, meta, experimentalmetricmetadata.ResourceID("test-pvc-1-uid"))

	km := meta[experimentalmetricmetadata.ResourceID("test-pvc-1-uid")]
	assert.Equal(t, "k8s.persistentvolumeclaim", km.EntityType)
	assert.Equal(t, "k8s.persistentvolumeclaim.uid", km.ResourceIDKey)
	assert.Equal(t, experimentalmetricmetadata.ResourceID("test-pvc-1-uid"), km.ResourceID)
	assert.Equal(t, "test-pvc-1", km.Metadata["k8s.persistentvolumeclaim.name"])
	assert.Equal(t, "test-namespace", km.Metadata["k8s.namespace.name"])
	assert.Equal(t, "test-pv-1", km.Metadata["k8s.persistentvolume.name"])
	assert.Contains(t, km.Metadata, k8sPVCCreationTime)
	assert.Equal(t, "bar", km.Metadata["k8s.persistentvolumeclaim.label.foo"])
	assert.Equal(t, "gce-pd", km.Metadata["k8s.persistentvolumeclaim.annotation.volume.beta.kubernetes.io/storage-provisioner"])
}

func TestGetMetadataPendingPVC(t *testing.T) {
	pvc := testutils.NewPersistentVolumeClaim("1")
	pvc.Status.Phase = corev1.ClaimPending
	pvc.Spec.VolumeName = ""

	meta := GetMetadata(pvc)

	require.Len(t, meta, 1)
	require.Contains(t, meta, experimentalmetricmetadata.ResourceID("test-pvc-1-uid"))

	km := meta[experimentalmetricmetadata.ResourceID("test-pvc-1-uid")]
	assert.Equal(t, "k8s.persistentvolumeclaim", km.EntityType)
	_, hasPVName := km.Metadata["k8s.persistentvolume.name"]
	assert.False(t, hasPVName, "bound PV name should not be present for pending PVC")
}
