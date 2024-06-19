package clusterrolebinding

import (
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/maps"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	imetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

const (
	// Keys for clusterrolebinding metadata.
	AttributeK8SClusterRoleBindingUID  = "k8s.clusterrolebinding.uid"
	AttributeK8SClusterRoleBindingName = "k8s.clusterrolebinding.name"
	ClusterRoleBindingCreationTime     = "clusterrolebinding.creation_timestamp"
)

// Transform transforms the clusterrolebinding to remove the fields.
// IMPORTANT: Make sure to update this function before using new clusterrolebinding fields.
func Transform(rb *rbacv1.ClusterRoleBinding) *rbacv1.ClusterRoleBinding {
	newCRB := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metadata.TransformObjectMeta(rb.ObjectMeta),
	}
	return newCRB
}

func RecordMetrics(mb *imetadata.MetricsBuilder, crbind *rbacv1.ClusterRoleBinding, ts pcommon.Timestamp) {
	mb.RecordK8sClusterrolebindingSubjectCountDataPoint(ts, int64(len(crbind.Subjects)))

	rb := mb.NewResourceBuilder()
	rb.SetK8sClusterrolebindingUID(string(crbind.GetUID()))
	rb.SetK8sClusterrolebindingName(crbind.GetName())
	rb.SetK8sClusterName("unknown")
	rb.SetK8sClusterrolebindingLabels(mapToString(crbind.GetLabels(), "&"))
	rb.SetK8sClusterrolebindingAnnotations(mapToString(crbind.GetAnnotations(), "&"))
	rb.SetK8sClusterrolebindingStartTime(crbind.GetCreationTimestamp().String())
	rb.SetK8sClusterrolebindingType("ClusterRoleBinding")
	rb.SetK8sClusterrolebindingSubjects(convertSubjectsToString(crbind.Subjects))
	rb.SetK8sClusterrolebindingRoleRef(fmt.Sprintf("apiGroup=%s&kind=%s&name=%s",
		crbind.RoleRef.APIGroup,
		crbind.RoleRef.Kind,
		crbind.RoleRef.Name))
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

func GetMetadata(crb *rbacv1.ClusterRoleBinding) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	meta := maps.MergeStringMaps(map[string]string{}, crb.Labels)

	meta[AttributeK8SClusterRoleBindingName] = crb.Name
	meta[ClusterRoleBindingCreationTime] = crb.GetCreationTimestamp().Format(time.RFC3339)

	crbID := experimentalmetricmetadata.ResourceID(crb.UID)
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		crbID: {
			EntityType:    "k8s.clusterrolebinding",
			ResourceIDKey: AttributeK8SClusterRoleBindingUID,
			ResourceID:    crbID,
			Metadata:      meta,
		},
	}
}
