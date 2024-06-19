package clusterrole

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
	// Keys for clusterrole metadata.
	AttributeK8SClusterRoleUID  = "k8s.clusterrole.uid"
	AttributeK8SClusterRoleName = "k8s.clusterrole.name"
	ClusterRoleCreationTime     = "clusterrole.creation_timestamp"
)

// Transform transforms the clusterrole to remove the fields.
// IMPORTANT: Make sure to update this function before using new clusterrole fields.
func Transform(r *rbacv1.ClusterRole) *rbacv1.ClusterRole {
	newCR := &rbacv1.ClusterRole{
		ObjectMeta: metadata.TransformObjectMeta(r.ObjectMeta),
	}
	return newCR
}

func RecordMetrics(mb *imetadata.MetricsBuilder, cr *rbacv1.ClusterRole, ts pcommon.Timestamp) {
	mb.RecordK8sClusterroleRuleCountDataPoint(ts, int64(len(cr.Rules)))

	rb := mb.NewResourceBuilder()
	rb.SetK8sClusterroleUID(string(cr.GetUID()))
	rb.SetK8sClusterroleName(cr.GetName())
	rb.SetK8sClusterName("unknown")
	rb.SetK8sClusterroleType("ClusterRole")
	rb.SetK8sClusterroleStartTime(cr.GetCreationTimestamp().String())
	rb.SetK8sClusterroleLabels(mapToString(cr.GetLabels(), "&"))
	rb.SetK8sClusterroleAnnotations(mapToString(cr.GetAnnotations(), "&"))
	rb.SetK8sClusterroleRules(convertRulesToString(cr.Rules))
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

func GetMetadata(r *rbacv1.ClusterRole) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	meta := maps.MergeStringMaps(map[string]string{}, r.Labels)

	meta[AttributeK8SClusterRoleName] = r.Name
	meta[ClusterRoleCreationTime] = r.GetCreationTimestamp().Format(time.RFC3339)

	rID := experimentalmetricmetadata.ResourceID(r.UID)
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		rID: {
			EntityType:    "k8s.clusterrole",
			ResourceIDKey: AttributeK8SClusterRoleUID,
			ResourceID:    rID,
			Metadata:      meta,
		},
	}
}
