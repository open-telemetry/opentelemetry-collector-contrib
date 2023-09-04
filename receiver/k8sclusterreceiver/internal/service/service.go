// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package service // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/service"
import (
	"fmt"

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	imetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

// Transform transforms the pod to remove the fields that we don't use to reduce RAM utilization.
// IMPORTANT: Make sure to update this function before using new service fields.
func Transform(service *corev1.Service) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metadata.TransformObjectMeta(service.ObjectMeta),
		Spec: corev1.ServiceSpec{
			Selector: service.Spec.Selector,
		},
	}
}

// GetPodServiceTags returns a set of services associated with the pod.
func GetPodServiceTags(pod *corev1.Pod, services cache.Store) map[string]string {
	properties := map[string]string{}

	for _, ser := range services.List() {
		serObj := ser.(*corev1.Service)
		if serObj.Namespace == pod.Namespace &&
			labels.Set(serObj.Spec.Selector).AsSelectorPreValidated().Matches(labels.Set(pod.Labels)) {
			properties[fmt.Sprintf("%s%s", constants.K8sServicePrefix, serObj.Name)] = ""
		}
	}

	return properties
}

func RecordMetrics(logger *zap.Logger, mb *imetadata.MetricsBuilder, svc *corev1.Service, ts pcommon.Timestamp) {

	mb.RecordK8sServicePortCountDataPoint(ts, int64(len(svc.Spec.Ports)))
	rb := mb.NewResourceBuilder()

	rb.SetK8sServiceClusterIP(svc.Spec.ClusterIP)
	rb.SetK8sServiceName(svc.ObjectMeta.Name)
	rb.SetK8sServiceNamespace(svc.ObjectMeta.Namespace)
	rb.SetK8sServiceUID(string(svc.UID))
	rb.SetK8sServiceType(string(svc.Spec.Type))
	rb.SetK8sClusterName("unknown")
	mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func getResourceForService(svc *corev1.Service) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: "k8s",
		Labels: map[string]string{
			"k8s.service.uid":                     string(svc.UID),
			conventions.AttributeServiceNamespace: svc.ObjectMeta.Namespace,
			conventions.AttributeServiceName:      svc.ObjectMeta.Name,
			"k8s.service.cluster_ip":              svc.Spec.ClusterIP,
			"k8s.service.type":                    string(svc.Spec.Type),
			"k8s.cluster.name":                    "unknown",
		},
	}
}
