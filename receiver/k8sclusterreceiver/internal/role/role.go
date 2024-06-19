package role

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
	// Keys for role metadata.
	AttributeK8SRoleUID  = "k8s.role.uid"
	AttributeK8SRoleName = "k8s.role.name"
	RoleCreationTime     = "role.creation_timestamp"
)

// Transform transforms the role to remove the fields.
// IMPORTANT: Make sure to update this function before using new role fields.
func Transform(r *rbacv1.Role) *rbacv1.Role {
	newR := &rbacv1.Role{
		ObjectMeta: metadata.TransformObjectMeta(r.ObjectMeta),
	}
	return newR
}

func RecordMetrics(mb *imetadata.MetricsBuilder, r *rbacv1.Role, ts pcommon.Timestamp) {
	mb.RecordK8sRoleRuleCountDataPoint(ts, int64(len(r.Rules)))

	rb := mb.NewResourceBuilder()
	rb.SetK8sRoleUID(string(r.GetUID()))
	rb.SetK8sRoleName(r.GetName())
	rb.SetK8sClusterName("unknown")
	rb.SetK8sRoleNamespace(r.GetNamespace())
	rb.SetK8sRoleLabels(mapToString(r.GetLabels(), "&"))
	rb.SetK8sRoleAnnotations(mapToString(r.GetAnnotations(), "&"))
	rb.SetK8sRoleStartTime(r.GetCreationTimestamp().String())
	rb.SetK8sRoleType("Role")
	rb.SetK8sRoleRules(convertRulesToString(r.Rules))
	mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func mapToString(m map[string]string, seperator string) string {
	var res []string
	for k, v := range m {
		res = append(res, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(res, seperator)
}

func convertRulesToString(rules []rbacv1.PolicyRule) string {
	var result strings.Builder

	for i, rule := range rules {
		if i > 0 {
			result.WriteString(";")
		}

		result.WriteString("verbs=")
		result.WriteString(strings.Join(rule.Verbs, ","))

		result.WriteString("&apiGroups=")
		result.WriteString(strings.Join(rule.APIGroups, ","))

		result.WriteString("&resources=")
		result.WriteString(strings.Join(rule.Resources, ","))

		result.WriteString("&resourceNames=")
		result.WriteString(strings.Join(rule.ResourceNames, ","))

		result.WriteString("&nonResourceURLs=")
		result.WriteString(strings.Join(rule.NonResourceURLs, ","))

	}

	return result.String()
}

func GetMetadata(r *rbacv1.Role) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	meta := maps.MergeStringMaps(map[string]string{}, r.Labels)

	meta[AttributeK8SRoleName] = r.Name
	meta[RoleCreationTime] = r.GetCreationTimestamp().Format(time.RFC3339)

	rID := experimentalmetricmetadata.ResourceID(r.UID)
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		rID: {
			EntityType:    "k8s.role",
			ResourceIDKey: AttributeK8SRoleUID,
			ResourceID:    rID,
			Metadata:      meta,
		},
	}
}
