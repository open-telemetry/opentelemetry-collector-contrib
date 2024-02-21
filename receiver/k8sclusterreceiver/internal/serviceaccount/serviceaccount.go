package serviceaccount

import (
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/maps"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"go.opentelemetry.io/collector/pdata/pcommon"
	corev1 "k8s.io/api/core/v1"
	"strconv"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	imetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

const (
	// Keys for serviceaccount metadata.
	AttributeK8SServiceAccountUID  = "k8s.serviceaccount.uid"
	AttributeK8SServiceAccountName = "k8s.serviceaccount.name"
	ServiceAccountCreationTime     = "serviceaccount.creation_timestamp"
)

// Transform transforms the service account to remove the fields.
// IMPORTANT: Make sure to update this function before using new service account fields.
func Transform(sa *corev1.ServiceAccount) *corev1.ServiceAccount {
	newSA := &corev1.ServiceAccount{
		ObjectMeta: metadata.TransformObjectMeta(sa.ObjectMeta),
	}
	return newSA
}

func RecordMetrics(mb *imetadata.MetricsBuilder, sa *corev1.ServiceAccount, ts pcommon.Timestamp) {
	mb.RecordK8sServiceaccountSecretCountDataPoint(ts, int64(len(sa.Secrets)))

	var automountFlag string
	if sa.AutomountServiceAccountToken != nil {
		automountFlag = strconv.FormatBool(*sa.AutomountServiceAccountToken)
	}

	rb := mb.NewResourceBuilder()
	rb.SetK8sServiceaccountUID(string(sa.GetUID()))
	rb.SetK8sServiceaccountName(sa.GetName())
	rb.SetK8sClusterName("unknown")
	rb.SetK8sServiceaccountNamespace(sa.GetNamespace())
	rb.SetK8sServiceaccountLabels(mapToString(sa.GetLabels(), "&"))
	rb.SetK8sServiceaccountAnnotations(mapToString(sa.GetAnnotations(), "&"))
	rb.SetK8sServiceaccountStartTime(sa.GetCreationTimestamp().String())
	rb.SetK8sServiceaccountType("ServiceAccount")
	rb.SetK8sServiceaccountSecrets(convertSecretsToString(sa.Secrets))
	rb.SetK8sServiceaccountImagePullSecrets(sliceToString(sa.ImagePullSecrets, ","))
	rb.SetK8sServiceaccountAutomountServiceaccountToken(automountFlag)
	mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func mapToString(m map[string]string, seperator string) string {
	var res []string
	for k, v := range m {
		res = append(res, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(res, seperator)
}

func sliceToString(s []corev1.LocalObjectReference, seperator string) string {
	var res []string
	for _, secret := range s {
		res = append(res, string(secret.Name))
	}
	return strings.Join(res, seperator)
}

func convertSecretsToString(secrets []corev1.ObjectReference) string {
	var result strings.Builder

	for i, secret := range secrets {
		if i > 0 {
			result.WriteString(";")
		}

		result.WriteString("kind=")
		result.WriteString(secret.Kind)

		result.WriteString("&name=")
		result.WriteString(secret.Name)

		result.WriteString("&namespace=")
		result.WriteString(secret.Namespace)

	}

	return result.String()
}

func GetMetadata(sa *corev1.ServiceAccount) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	meta := maps.MergeStringMaps(map[string]string{}, sa.Labels)

	meta[AttributeK8SServiceAccountName] = sa.Name
	meta[ServiceAccountCreationTime] = sa.GetCreationTimestamp().Format(time.RFC3339)

	saID := experimentalmetricmetadata.ResourceID(sa.UID)
	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		saID: {
			EntityType:    "k8s.serviceaccount",
			ResourceIDKey: AttributeK8SServiceAccountUID,
			ResourceID:    saID,
			Metadata:      meta,
		},
	}
}
