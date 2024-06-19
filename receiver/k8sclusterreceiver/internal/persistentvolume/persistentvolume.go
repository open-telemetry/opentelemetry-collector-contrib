package persistentvolume

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
	// Keys for persistentvolume metadata.
	AttributeK8SPersistentvolumeUID  = "k8s.persistentvolume.uid"
	AttributeK8SPersistentvolumeName = "k8s.persistentvolume.name"
	persistentvolumeCreationTime     = "persistentvolume.creation_timestamp"
)

// Transform transforms the persistent-volume to remove the fields that we don't use to reduce RAM utilization.
// IMPORTANT: Make sure to update this function before using new persistent-volume fields.
func Transform(pv *corev1.PersistentVolume) *corev1.PersistentVolume {
	newPv := &corev1.PersistentVolume{
		ObjectMeta: metadata.TransformObjectMeta(pv.ObjectMeta),
		Status: corev1.PersistentVolumeStatus{
			Phase: pv.Status.Phase,
		},
	}
	newPv.Spec.Capacity = pv.Spec.Capacity
	for _, c := range pv.Spec.AccessModes {
		newPv.Spec.AccessModes = append(newPv.Spec.AccessModes, c)
	}
	return newPv
}

func RecordMetrics(mb *imetadata.MetricsBuilder, pv *corev1.PersistentVolume, ts pcommon.Timestamp) {
	var capacity int64
	for _, quantity := range pv.Spec.Capacity {
		capacity += quantity.Value()
	}

	mb.RecordK8sPersistentvolumeCapacityDataPoint(ts, capacity)
	rb := mb.NewResourceBuilder()
	rb.SetK8sPersistentvolumeUID(string(pv.GetUID()))
	rb.SetK8sPersistentvolumeName(pv.GetName())
	rb.SetK8sPersistentvolumeNamespace(pv.GetNamespace())
	rb.SetK8sPersistentvolumeLabels(mapToString(pv.GetLabels(), "&"))
	rb.SetK8sPersistentvolumeAnnotations(mapToString(pv.GetAnnotations(), "&"))
	rb.SetK8sPersistentvolumePhase(string(pv.Status.Phase))
	rb.SetK8sPersistentvolumeAccessModes(sliceToString(pv.Spec.AccessModes, ","))
	rb.SetK8sPersistentvolumeFinalizers(strings.Join(pv.Finalizers, ","))
	rb.SetK8sPersistentvolumeReclaimPolicy(string(pv.Spec.PersistentVolumeReclaimPolicy))
	rb.SetK8sPersistentvolumeStartTime(pv.GetCreationTimestamp().String())
	rb.SetK8sPersistentvolumeStorageClass(pv.Spec.StorageClassName)
	rb.SetK8sPersistentvolumeType("PersistentVolume")
	rb.SetK8sPersistentvolumeVolumeMode(string(*pv.Spec.VolumeMode))
	rb.SetK8sPersistentvolumeclaimUID(string((*pv.Spec.ClaimRef).UID))
	rb.SetK8sPersistentvolumeclaimName((pv.Spec.ClaimRef).Name)
	rb.SetK8sClusterName("unknown")
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

func GetMetadata(pv *corev1.PersistentVolume) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	meta := maps.MergeStringMaps(map[string]string{}, pv.Labels)

	meta[AttributeK8SPersistentvolumeName] = pv.Name
	meta[persistentvolumeCreationTime] = pv.GetCreationTimestamp().Format(time.RFC3339)

	pvID := experimentalmetricmetadata.ResourceID(pv.UID)
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		pvID: {
			EntityType:    "k8s.persistentvolume",
			ResourceIDKey: AttributeK8SPersistentvolumeUID,
			ResourceID:    pvID,
			Metadata:      meta,
		},
	}
}
