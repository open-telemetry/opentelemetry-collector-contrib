package ingress

import (
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	netv1 "k8s.io/api/networking/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/maps"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	imetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

const (
	// Keys for ingress metadata.
	AttributeK8SIngressUID  = "k8s.ingress.uid"
	AttributeK8SIngressName = "k8s.ingress.name"
	IngressCreationTime     = "ingress.creation_timestamp"
)

// Transform transforms the ingress to remove the fields.
// IMPORTANT: Make sure to update this function before using new ingress fields.
func Transform(r *netv1.Ingress) *netv1.Ingress {
	newI := &netv1.Ingress{
		ObjectMeta: metadata.TransformObjectMeta(r.ObjectMeta),
	}
	return newI
}

func RecordMetrics(mb *imetadata.MetricsBuilder, i *netv1.Ingress, ts pcommon.Timestamp) {
	mb.RecordK8sIngressRuleCountDataPoint(ts, int64(len(i.Spec.Rules)))

	rb := mb.NewResourceBuilder()
	rb.SetK8sIngressUID(string(i.GetUID()))
	rb.SetK8sIngressName(i.GetName())
	rb.SetK8sClusterName("unknown")
	rb.SetK8sIngressNamespace(i.GetNamespace())
	rb.SetK8sIngressLabels(mapToString(i.GetLabels(), "&"))
	rb.SetK8sIngressAnnotations(mapToString(i.GetAnnotations(), "&"))
	rb.SetK8sIngressStartTime(i.GetCreationTimestamp().String())
	rb.SetK8sIngressType("Ingress")
	rb.SetK8sIngressRules(convertIngressRulesToString(i.Spec.Rules))
	mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func mapToString(m map[string]string, seperator string) string {
	var res []string
	for k, v := range m {
		res = append(res, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(res, seperator)
}

func convertIngressRulesToString(rules []netv1.IngressRule) string {
	var result strings.Builder

	for i, rule := range rules {
		if i > 0 {
			result.WriteString(";")
		}

		result.WriteString("host=")
		result.WriteString(rule.Host)

		result.WriteString("&http=(paths=")
		for j, path := range rule.HTTP.Paths {
			if j > 0 {
				result.WriteString("&")
			}

			result.WriteString("(path=")
			result.WriteString(path.Path)
			result.WriteString("&pathType=")
			result.WriteString(string(*path.PathType))
			result.WriteString("&backend=(service=(name=")
			result.WriteString(path.Backend.Service.Name)
			result.WriteString("&port=(number=")
			result.WriteString(fmt.Sprintf("%d", path.Backend.Service.Port.Number))
			result.WriteString(")))")
		}
		result.WriteString(")")
	}

	return result.String()
}

func GetMetadata(i *netv1.Ingress) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	meta := maps.MergeStringMaps(map[string]string{}, i.Labels)

	meta[AttributeK8SIngressName] = i.Name
	meta[IngressCreationTime] = i.GetCreationTimestamp().Format(time.RFC3339)

	iID := experimentalmetricmetadata.ResourceID(i.UID)
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		iID: {
			EntityType:    "k8s.ingress",
			ResourceIDKey: AttributeK8SIngressUID,
			ResourceID:    iID,
			Metadata:      meta,
		},
	}
}
