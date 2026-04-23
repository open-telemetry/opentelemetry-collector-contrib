// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package persistentvolume

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
	origPV := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pv-data-01",
			UID:  "pv-data-01-uid",
			Labels: map[string]string{
				"type": "ssd",
				"env":  "prod",
			},
			Annotations: map[string]string{
				"pv.kubernetes.io/provisioned-by":                  "kubernetes.io/gce-pd",
				"kubectl.kubernetes.io/last-applied-configuration": `{"apiVersion":"v1"}`,
			},
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: *resource.NewQuantity(100*1024*1024*1024, resource.BinarySI),
			},
			StorageClassName:              "standard-rwo",
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
			AccessModes:                   []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			ClaimRef: &corev1.ObjectReference{
				Name:       "data-claim",
				Namespace:  "default",
				UID:        "data-claim-uid",
				Kind:       "PersistentVolumeClaim",
				APIVersion: "v1",
			},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       "pd.csi.storage.gke.io",
					VolumeHandle: "projects/my-project/zones/us-central1-a/disks/pv-data-01",
				},
			},
			MountOptions: []string{"hard", "nfsvers=4.1"},
		},
		Status: corev1.PersistentVolumeStatus{
			Phase:   corev1.VolumeBound,
			Message: "volume is bound",
		},
	}

	wantPV := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pv-data-01",
			UID:  "pv-data-01-uid",
			Labels: map[string]string{
				"type": "ssd",
				"env":  "prod",
			},
			Annotations: map[string]string{
				"pv.kubernetes.io/provisioned-by": "kubernetes.io/gce-pd",
			},
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: *resource.NewQuantity(100*1024*1024*1024, resource.BinarySI),
			},
			StorageClassName:              "standard-rwo",
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
			ClaimRef: &corev1.ObjectReference{
				Name:      "data-claim",
				Namespace: "default",
				UID:       "data-claim-uid",
			},
		},
		Status: corev1.PersistentVolumeStatus{
			Phase: corev1.VolumeBound,
		},
	}

	assert.Equal(t, wantPV, Transform(origPV))
}

func TestTransformWithoutClaimRef(t *testing.T) {
	origPV := testutils.NewPersistentVolume("1")
	origPV.Spec.ClaimRef = nil
	origPV.Status.Phase = corev1.VolumeAvailable

	transformed := Transform(origPV)

	assert.Equal(t, corev1.VolumeAvailable, transformed.Status.Phase)
	assert.Nil(t, transformed.Spec.ClaimRef)
}

func newPVMetricsBuilder() *metadata.MetricsBuilder {
	mbc := metadata.DefaultMetricsBuilderConfig()
	mbc.Metrics.K8sPersistentvolumeStatusPhase.Enabled = true
	mbc.Metrics.K8sPersistentvolumeStorageCapacity.Enabled = true
	return metadata.NewMetricsBuilder(mbc, receivertest.NewNopSettings(metadata.Type))
}

func TestGoldenFile(t *testing.T) {
	pv := testutils.NewPersistentVolume("1")
	ts := pcommon.Timestamp(time.Now().UnixNano())
	mb := newPVMetricsBuilder()
	RecordMetrics(mb, pv, ts)
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

func TestGoldenFileOptionalAttrs(t *testing.T) {
	pv := testutils.NewPersistentVolume("1")
	ts := pcommon.Timestamp(time.Now().UnixNano())

	mbc := metadata.DefaultMetricsBuilderConfig()
	mbc.Metrics.K8sPersistentvolumeStatusPhase.Enabled = true
	mbc.Metrics.K8sPersistentvolumeStorageCapacity.Enabled = true
	mbc.ResourceAttributes.K8sPersistentvolumeReclaimPolicy.Enabled = true
	mb := metadata.NewMetricsBuilder(mbc, receivertest.NewNopSettings(metadata.Type))

	RecordMetrics(mb, pv, ts)
	m := mb.Emit()

	expectedFile := filepath.Join("testdata", "expected_optional.yaml")
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

func TestGoldenFileNoCapacity(t *testing.T) {
	pv := testutils.NewPersistentVolume("1")
	pv.Spec.Capacity = nil
	pv.Status.Phase = corev1.VolumeAvailable
	ts := pcommon.Timestamp(time.Now().UnixNano())
	mb := newPVMetricsBuilder()
	RecordMetrics(mb, pv, ts)
	m := mb.Emit()

	expectedFile := filepath.Join("testdata", "expected_no_capacity.yaml")
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
	pv := testutils.NewPersistentVolume("1")

	ts := pcommon.Timestamp(time.Now().UnixNano())
	mb := newPVMetricsBuilder()
	RecordMetrics(mb, pv, ts)
	m := mb.Emit()

	require.Equal(t, 1, m.ResourceMetrics().Len())
	rm := m.ResourceMetrics().At(0)

	assert.Equal(t,
		map[string]any{
			"k8s.persistentvolume.uid":  "test-pv-1-uid",
			"k8s.persistentvolume.name": "test-pv-1",
			"k8s.storageclass.name":     "standard",
		},
		rm.Resource().Attributes().AsRaw(),
	)

	require.Equal(t, 1, rm.ScopeMetrics().Len())
	sm := rm.ScopeMetrics().At(0)

	var foundPhase, foundCapacity bool
	for i := 0; i < sm.Metrics().Len(); i++ {
		metric := sm.Metrics().At(i)
		switch metric.Name() {
		case "k8s.persistentvolume.status.phase":
			foundPhase = true
			for j := 0; j < metric.Sum().DataPoints().Len(); j++ {
				dp := metric.Sum().DataPoints().At(j)
				phaseAttr, _ := dp.Attributes().Get("k8s.persistentvolume.status.phase")
				if phaseAttr.Str() == "Bound" {
					assert.Equal(t, int64(1), dp.IntValue())
				} else {
					assert.Equal(t, int64(0), dp.IntValue(), "non-active phase %s should be 0", phaseAttr.Str())
				}
			}
		case "k8s.persistentvolume.storage.capacity":
			foundCapacity = true
			require.Equal(t, 1, metric.Sum().DataPoints().Len())
			assert.Equal(t, int64(10*1024*1024*1024), metric.Sum().DataPoints().At(0).IntValue())
		}
	}
	assert.True(t, foundPhase, "status.phase metric should be present")
	assert.True(t, foundCapacity, "storage.capacity metric should be present")
}

func TestRecordMetricsWithoutCapacity(t *testing.T) {
	pv := testutils.NewPersistentVolume("1")
	pv.Spec.Capacity = nil

	ts := pcommon.Timestamp(time.Now().UnixNano())
	mb := newPVMetricsBuilder()
	RecordMetrics(mb, pv, ts)
	m := mb.Emit()

	require.Equal(t, 1, m.ResourceMetrics().Len())
	sm := m.ResourceMetrics().At(0).ScopeMetrics().At(0)

	for i := 0; i < sm.Metrics().Len(); i++ {
		assert.NotEqual(t, "k8s.persistentvolume.storage.capacity", sm.Metrics().At(i).Name(),
			"storage.capacity metric should not be present when Capacity is not set")
	}
}

func TestRecordMetricsAvailablePV(t *testing.T) {
	pv := testutils.NewPersistentVolume("1")
	pv.Spec.ClaimRef = nil
	pv.Status.Phase = corev1.VolumeAvailable

	ts := pcommon.Timestamp(time.Now().UnixNano())
	mb := newPVMetricsBuilder()
	RecordMetrics(mb, pv, ts)
	m := mb.Emit()

	require.Equal(t, 1, m.ResourceMetrics().Len())
	sm := m.ResourceMetrics().At(0).ScopeMetrics().At(0)

	for i := 0; i < sm.Metrics().Len(); i++ {
		metric := sm.Metrics().At(i)
		if metric.Name() == "k8s.persistentvolume.status.phase" {
			for j := 0; j < metric.Sum().DataPoints().Len(); j++ {
				dp := metric.Sum().DataPoints().At(j)
				phaseAttr, _ := dp.Attributes().Get("k8s.persistentvolume.status.phase")
				if phaseAttr.Str() == "Available" {
					assert.Equal(t, int64(1), dp.IntValue())
				} else {
					assert.Equal(t, int64(0), dp.IntValue())
				}
			}
		}
	}
}

func TestGetMetadata(t *testing.T) {
	pv := testutils.NewPersistentVolume("1")
	pv.Annotations = map[string]string{"pv.kubernetes.io/provisioned-by": "gce-pd"}

	meta := GetMetadata(pv)

	require.Len(t, meta, 1)
	require.Contains(t, meta, experimentalmetricmetadata.ResourceID("test-pv-1-uid"))

	km := meta[experimentalmetricmetadata.ResourceID("test-pv-1-uid")]
	assert.Equal(t, "k8s.persistentvolume", km.EntityType)
	assert.Equal(t, "k8s.persistentvolume.uid", km.ResourceIDKey)
	assert.Equal(t, experimentalmetricmetadata.ResourceID("test-pv-1-uid"), km.ResourceID)
	assert.Equal(t, "test-pv-1", km.Metadata["k8s.persistentvolume.name"])
	assert.Contains(t, km.Metadata, k8sPVCreationTime)
	assert.Equal(t, "bar", km.Metadata["k8s.persistentvolume.label.foo"])
	assert.Equal(t, "gce-pd", km.Metadata["k8s.persistentvolume.annotation.pv.kubernetes.io/provisioned-by"])
}

func TestGetMetadataAvailablePV(t *testing.T) {
	pv := testutils.NewPersistentVolume("1")
	pv.Spec.ClaimRef = nil
	pv.Status.Phase = corev1.VolumeAvailable

	meta := GetMetadata(pv)

	require.Len(t, meta, 1)
	require.Contains(t, meta, experimentalmetricmetadata.ResourceID("test-pv-1-uid"))
	assert.Equal(t, "k8s.persistentvolume", meta[experimentalmetricmetadata.ResourceID("test-pv-1-uid")].EntityType)
}
