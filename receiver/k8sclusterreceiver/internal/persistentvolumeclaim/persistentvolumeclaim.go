// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package persistentvolumeclaim // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/persistentvolumeclaim"

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
	k8sPVCCreationTime = "k8s.persistentvolumeclaim.creation_timestamp"
	k8sBoundPVName     = "k8s.persistentvolume.name"
)

// Transform transforms the PersistentVolumeClaim to remove the fields that we don't use to reduce RAM utilization.
// IMPORTANT: Make sure to update this function before using new PersistentVolumeClaim fields.
func Transform(pvc *corev1.PersistentVolumeClaim) *corev1.PersistentVolumeClaim {
	om := metadata.TransformObjectMeta(pvc.ObjectMeta)

	if len(pvc.Annotations) > 0 {
		om.Annotations = make(map[string]string, len(pvc.Annotations))
		for k, v := range pvc.Annotations {
			if !shouldSkipAnnotation(k) {
				om.Annotations[k] = v
			}
		}
	}

	newPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: om,
		Status: corev1.PersistentVolumeClaimStatus{
			Phase:    pvc.Status.Phase,
			Capacity: pvc.Status.Capacity,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: pvc.Spec.VolumeName,
			Resources:  pvc.Spec.Resources,
		},
	}
	if pvc.Spec.StorageClassName != nil {
		newPVC.Spec.StorageClassName = pvc.Spec.StorageClassName
	}
	return newPVC
}

func shouldSkipAnnotation(key string) bool {
	return strings.HasPrefix(key, "kubectl.kubernetes.io/last-applied-configuration")
}

// RecordMetrics records metrics for the PersistentVolumeClaim.
func RecordMetrics(mb *metadata.MetricsBuilder, pvc *corev1.PersistentVolumeClaim, ts pcommon.Timestamp) {
	e := metadata.NewK8sPersistentvolumeclaimEntity(string(pvc.UID))
	e.SetK8sPersistentvolumeclaimName(pvc.Name)
	e.SetK8sNamespaceName(pvc.Namespace)
	if pvc.Spec.StorageClassName != nil && *pvc.Spec.StorageClassName != "" {
		e.SetK8sStorageclassName(*pvc.Spec.StorageClassName)
	}
	eb := mb.ForK8sPersistentvolumeclaim(e)
	for phaseStr, phaseAttr := range metadata.MapAttributeK8sPersistentvolumeclaimStatusPhase {
		val := int64(0)
		if string(pvc.Status.Phase) == phaseStr {
			val = 1
		}
		eb.RecordK8sPersistentvolumeclaimStatusPhaseDataPoint(ts, val, phaseAttr)
	}
	if storage, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
		eb.RecordK8sPersistentvolumeclaimStorageRequestDataPoint(ts, storage.Value())
	}
	if storage, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
		eb.RecordK8sPersistentvolumeclaimStorageCapacityDataPoint(ts, storage.Value())
	}
	eb.Emit()
}

// GetMetadata returns the metadata for the PersistentVolumeClaim.
func GetMetadata(pvc *corev1.PersistentVolumeClaim) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	meta := map[string]string{}
	meta[metadata.GetOTelNameFromKind("persistentvolumeclaim")] = pvc.Name
	meta[metadata.GetOTelNameFromKind("namespace")] = pvc.Namespace
	meta[k8sPVCCreationTime] = pvc.CreationTimestamp.Format(time.RFC3339)

	if pvc.Spec.VolumeName != "" {
		meta[k8sBoundPVName] = pvc.Spec.VolumeName
	}
	for key, value := range pvc.Labels {
		meta[fmt.Sprintf("k8s.persistentvolumeclaim.label.%s", key)] = value
	}
	for key, value := range pvc.Annotations {
		meta[fmt.Sprintf("k8s.persistentvolumeclaim.annotation.%s", key)] = value
	}

	pvcID := experimentalmetricmetadata.ResourceID(pvc.UID)

	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		pvcID: {
			EntityType:    "k8s.persistentvolumeclaim",
			ResourceIDKey: "k8s.persistentvolumeclaim.uid",
			ResourceID:    pvcID,
			Metadata:      meta,
		},
	}
}
