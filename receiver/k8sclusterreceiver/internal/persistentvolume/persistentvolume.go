// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package persistentvolume // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/persistentvolume"

import (
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

const (
	k8sPVCreationTime = "k8s.persistentvolume.creation_timestamp"
)

// Transform transforms the PersistentVolume to remove the fields that we don't use to reduce RAM utilization.
// IMPORTANT: Make sure to update this function before using new PersistentVolume fields.
func Transform(pv *corev1.PersistentVolume) *corev1.PersistentVolume {
	om := metadata.TransformObjectMeta(pv.ObjectMeta)

	if len(pv.Annotations) > 0 {
		om.Annotations = make(map[string]string, len(pv.Annotations))
		for k, v := range pv.Annotations {
			if !shouldSkipAnnotation(k) {
				om.Annotations[k] = v
			}
		}
	}

	newPV := &corev1.PersistentVolume{
		ObjectMeta: om,
		Status: corev1.PersistentVolumeStatus{
			Phase: pv.Status.Phase,
		},
		Spec: corev1.PersistentVolumeSpec{
			StorageClassName:              pv.Spec.StorageClassName,
			PersistentVolumeReclaimPolicy: pv.Spec.PersistentVolumeReclaimPolicy,
			Capacity:                      pv.Spec.Capacity,
		},
	}
	if pv.Spec.ClaimRef != nil {
		newPV.Spec.ClaimRef = &corev1.ObjectReference{
			Name:      pv.Spec.ClaimRef.Name,
			Namespace: pv.Spec.ClaimRef.Namespace,
			UID:       pv.Spec.ClaimRef.UID,
		}
	}
	return newPV
}

func shouldSkipAnnotation(key string) bool {
	return strings.HasPrefix(key, "kubectl.kubernetes.io/last-applied-configuration")
}

// RecordMetrics records metrics for the PersistentVolume.
func RecordMetrics(mb *metadata.MetricsBuilder, pv *corev1.PersistentVolume, ts pcommon.Timestamp) {
	e := metadata.NewK8sPersistentvolumeEntity(string(pv.UID))
	e.SetK8sPersistentvolumeName(pv.Name)
	if pv.Spec.StorageClassName != "" {
		e.SetK8sStorageclassName(pv.Spec.StorageClassName)
	}
	if pv.Spec.PersistentVolumeReclaimPolicy != "" {
		e.SetK8sPersistentvolumeReclaimPolicy(string(pv.Spec.PersistentVolumeReclaimPolicy))
	}
	eb := mb.ForK8sPersistentvolume(e)
	for phaseStr, phaseAttr := range metadata.MapAttributeK8sPersistentvolumeStatusPhase {
		val := int64(0)
		if string(pv.Status.Phase) == phaseStr {
			val = 1
		}
		eb.RecordK8sPersistentvolumeStatusPhaseDataPoint(ts, val, phaseAttr)
	}
	if storage, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
		eb.RecordK8sPersistentvolumeStorageCapacityDataPoint(ts, storage.Value())
	}
	eb.Emit()
}

// GetMetadata returns the metadata for the PersistentVolume.
func GetMetadata(pv *corev1.PersistentVolume) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	meta := map[string]string{}
	meta[metadata.GetOTelNameFromKind("persistentvolume")] = pv.Name
	meta[k8sPVCreationTime] = pv.CreationTimestamp.Format(time.RFC3339)

	for key, value := range pv.Labels {
		meta[fmt.Sprintf("k8s.persistentvolume.label.%s", key)] = value
	}
	for key, value := range pv.Annotations {
		meta[fmt.Sprintf("k8s.persistentvolume.annotation.%s", key)] = value
	}

	pvID := experimentalmetricmetadata.ResourceID(pv.UID)

	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		pvID: {
			EntityType:    "k8s.persistentvolume",
			ResourceIDKey: "k8s.persistentvolume.uid",
			ResourceID:    pvID,
			Metadata:      meta,
		},
	}
}
