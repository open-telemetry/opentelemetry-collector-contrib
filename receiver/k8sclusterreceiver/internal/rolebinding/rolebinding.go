package rolebinding

import (
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/maps"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"go.opentelemetry.io/collector/pdata/pcommon"
	rbacv1 "k8s.io/api/rbac/v1"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	imetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

const (
	// Keys for rolebinding metadata.
	AttributeK8SRoleBindingUID  = "k8s.rolebinding.uid"
	AttributeK8SRoleBindingName = "k8s.rolebinding.name"
	RoleBindingCreationTime     = "rolebinding.creation_timestamp"
)

// Transform transforms the rolebinding to remove the fields.
// IMPORTANT: Make sure to update this function before using new rolebinding fields.
func Transform(rb *rbacv1.RoleBinding) *rbacv1.RoleBinding {
	newRB := &rbacv1.RoleBinding{
		ObjectMeta: metadata.TransformObjectMeta(rb.ObjectMeta),
	}
	return newRB
}

func RecordMetrics(mb *imetadata.MetricsBuilder, rbind *rbacv1.RoleBinding, ts pcommon.Timestamp) {
	mb.RecordK8sRolebindingSubjectCountDataPoint(ts, int64(len(rbind.Subjects)))

	rb := mb.NewResourceBuilder()
	rb.SetK8sRolebindingUID(string(rbind.GetUID()))
	rb.SetK8sRolebindingName(rbind.GetName())
	rb.SetK8sClusterName("unknown")
	rb.SetK8sRolebindingNamespace(rbind.GetNamespace())
	rb.SetK8sRolebindingLabels(mapToString(rbind.GetLabels(), "&"))
	rb.SetK8sRolebindingAnnotations(mapToString(rbind.GetAnnotations(), "&"))
	rb.SetK8sRolebindingStartTime(rbind.GetCreationTimestamp().String())
	rb.SetK8sRolebindingType("RoleBinding")
	rb.SetK8sRolebindingSubjects(convertSubjectsToString(rbind.Subjects))
	rb.SetK8sRolebindingRoleRef(fmt.Sprintf("apiGroup=%s&kind=%s&name=%s",
		rbind.RoleRef.APIGroup,
		rbind.RoleRef.Kind,
		rbind.RoleRef.Name))
	mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func mapToString(m map[string]string, seperator string) string {
	var res []string
	for k, v := range m {
		res = append(res, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(res, seperator)
}

func convertSubjectsToString(subjects []rbacv1.Subject) string {
	var result strings.Builder

	for i, subject := range subjects {
		if i > 0 {
			result.WriteString(";")
		}

		result.WriteString("kind=")
		result.WriteString(subject.Kind)

		result.WriteString("&name=")
		result.WriteString(subject.Name)

		result.WriteString("&namespace=")
		result.WriteString(subject.Namespace)
	}

	return result.String()
}

func GetMetadata(rb *rbacv1.RoleBinding) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	meta := maps.MergeStringMaps(map[string]string{}, rb.Labels)

	meta[AttributeK8SRoleBindingName] = rb.Name
	meta[RoleBindingCreationTime] = rb.GetCreationTimestamp().Format(time.RFC3339)

	rbID := experimentalmetricmetadata.ResourceID(rb.UID)
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		rbID: {
			EntityType:    "k8s.rolebinding",
			ResourceIDKey: AttributeK8SRoleBindingUID,
			ResourceID:    rbID,
			Metadata:      meta,
		},
	}
}
