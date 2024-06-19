package persistentvolumeclaim

import (
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/maps"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	imetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

const (
	// Keys for persistentvolumeclaim metadata.
	AttributeK8SPersistentvolumeclaimUID  = "k8s.persistentvolumeclaim.uid"
	AttributeK8SPersistentvolumeclaimName = "k8s.persistentvolumeclaim.name"
	persistentvolumeclaimCreationTime     = "persistentvolumeclaim.creation_timestamp"
)

// Transform transforms the persistent-volume-claim to remove the fields that we don't use to reduce RAM utilization.
// IMPORTANT: Make sure to update this function before using new persistent-volume-claim fields.
func Transform(pvc *corev1.PersistentVolumeClaim) *corev1.PersistentVolumeClaim {
	newPvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metadata.TransformObjectMeta(pvc.ObjectMeta),
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: pvc.Status.Phase,
		},
	}
	newPvc.Status.Capacity = pvc.Status.Capacity
	newPvc.Spec.VolumeName = pvc.Spec.VolumeName
	for _, c := range pvc.Spec.AccessModes {
		newPvc.Spec.AccessModes = append(newPvc.Spec.AccessModes, c)
	}
	return newPvc
}

func RecordMetrics(mb *imetadata.MetricsBuilder, pvc *corev1.PersistentVolumeClaim, ts pcommon.Timestamp) {
	var capacity int64
	var allocated int64

	for _, quantity := range pvc.Status.Capacity {
		capacity += quantity.Value()
	}
	for _, quantity := range pvc.Status.AllocatedResources {
		allocated += quantity.Value()
	}

	mb.RecordK8sPersistentvolumeclaimCapacityDataPoint(ts, capacity)
	mb.RecordK8sPersistentvolumeclaimAllocatedDataPoint(ts, allocated)

	rb := mb.NewResourceBuilder()
	rb.SetK8sPersistentvolumeclaimUID(string(pvc.GetUID()))
	rb.SetK8sPersistentvolumeclaimName(pvc.GetName())
	rb.SetK8sClusterName("unknown")
	rb.SetK8sPersistentvolumeclaimNamespace(pvc.GetNamespace())
	rb.SetK8sPersistentvolumeclaimLabels(mapToString(pvc.GetLabels(), "&"))
	rb.SetK8sPersistentvolumeclaimPhase(string(pvc.Status.Phase))
	rb.SetK8sPersistentvolumeclaimSelector("")
	rb.SetK8sPersistentvolumeclaimStorageClass(*pvc.Spec.StorageClassName)
	rb.SetK8sPersistentvolumeclaimVolumeMode(string(*pvc.Spec.VolumeMode))
	rb.SetK8sPersistentvolumeclaimAccessModes(sliceToString(pvc.Spec.AccessModes, ","))
	rb.SetK8sPersistentvolumeclaimFinalizers(strings.Join(pvc.Finalizers, ","))
	rb.SetK8sPersistentvolumeclaimStartTime(pvc.GetCreationTimestamp().String())
	rb.SetK8sPersistentvolumeclaimAnnotations(mapToString(pvc.GetAnnotations(), "&"))
	rb.SetK8sPersistentvolumeclaimVolumeName(pvc.Spec.VolumeName)
	rb.SetK8sPersistentvolumeclaimType("PersistentVolumeClaim")
	mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func mapToString(m map[string]string, seperator string) string {
	var res []string
	for k, v := range m {
		res = append(res, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(res, seperator)
}

func sliceToString(s []corev1.PersistentVolumeAccessMode, seperator string) string {
	var res []string
	for _, mode := range s {
		res = append(res, string(mode))
	}
	return strings.Join(res, seperator)
}

func GetMetadata(pvc *corev1.PersistentVolumeClaim) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	meta := maps.MergeStringMaps(map[string]string{}, pvc.Labels)

	meta[AttributeK8SPersistentvolumeclaimName] = pvc.Name
	meta[persistentvolumeclaimCreationTime] = pvc.GetCreationTimestamp().Format(time.RFC3339)

	pvcID := experimentalmetricmetadata.ResourceID(pvc.UID)
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		pvcID: {
			EntityType:    "k8s.persistentvolumeclaim",
			ResourceIDKey: AttributeK8SPersistentvolumeclaimUID,
			ResourceID:    pvcID,
			Metadata:      meta,
		},
	}
}
