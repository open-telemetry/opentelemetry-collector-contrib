// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package service // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/service"
import (
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

func shouldSkipAnnotation(key string) bool {
	return strings.HasPrefix(key, "kubectl.kubernetes.io/last-applied-configuration")
}

// Transform transforms the service to remove the fields that we don't use to reduce RAM utilization.
// IMPORTANT: Make sure to update this function before using new service fields.
func Transform(service *corev1.Service) *corev1.Service {
	om := metadata.TransformObjectMeta(service.ObjectMeta)

	// Only keep annotations that aren't excluded to save RAM.
	if len(service.Annotations) > 0 {
		om.Annotations = make(map[string]string)
		for k, v := range service.Annotations {
			if !shouldSkipAnnotation(k) {
				om.Annotations[k] = v
			}
		}
	}

	return &corev1.Service{
		ObjectMeta: om,
		Spec: corev1.ServiceSpec{
			Type:                     service.Spec.Type,
			Selector:                 service.Spec.Selector,
			PublishNotReadyAddresses: service.Spec.PublishNotReadyAddresses,
			TrafficDistribution:      service.Spec.TrafficDistribution,
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: service.Status.LoadBalancer,
		},
	}
}

func RecordMetrics(_ *zap.Logger, mb *metadata.MetricsBuilder, svc *corev1.Service, endpointCounts EndpointCountsByKey, ts pcommon.Timestamp) {
	for key, counts := range endpointCounts {
		addressTypeEnum, ok := metadata.MapAttributeK8sServiceEndpointAddressType[key.AddressType]
		if !ok {
			continue
		}

		mb.RecordK8sServiceEndpointCountDataPoint(
			ts,
			int64(counts.Ready),
			addressTypeEnum,
			metadata.AttributeK8sServiceEndpointConditionReady,
			key.Zone,
		)

		mb.RecordK8sServiceEndpointCountDataPoint(
			ts,
			int64(counts.Serving),
			addressTypeEnum,
			metadata.AttributeK8sServiceEndpointConditionServing,
			key.Zone,
		)

		mb.RecordK8sServiceEndpointCountDataPoint(
			ts,
			int64(counts.Terminating),
			addressTypeEnum,
			metadata.AttributeK8sServiceEndpointConditionTerminating,
			key.Zone,
		)
	}

	if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
		ingressCount := len(svc.Status.LoadBalancer.Ingress)
		mb.RecordK8sServiceLoadBalancerIngressCountDataPoint(ts, int64(ingressCount))
	}

	rb := mb.NewResourceBuilder()
	rb.SetK8sServiceName(svc.Name)
	rb.SetK8sServiceUID(string(svc.UID))
	rb.SetK8sNamespaceName(svc.Namespace)
	rb.SetK8sServiceType(string(svc.Spec.Type))
	if svc.Spec.TrafficDistribution != nil {
		rb.SetK8sServiceTrafficDistribution(*svc.Spec.TrafficDistribution)
	}
	rb.SetK8sServicePublishNotReadyAddresses(svc.Spec.PublishNotReadyAddresses)
	mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

// EndpointCounts tracks the counts of endpoints in different conditions
type EndpointCounts struct {
	Ready       int
	Serving     int
	Terminating int
}

// EndpointKey represents a unique combination of address type and zone for aggregation
type EndpointKey struct {
	AddressType string
	Zone        string
}

// EndpointCountsByKey maps (addressType, zone) to endpoint counts
type EndpointCountsByKey map[EndpointKey]EndpointCounts

// GetMetadata returns entity metadata for a Service.
func GetMetadata(svc *corev1.Service) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	meta := map[string]string{
		"k8s.namespace.name":             svc.Namespace,
		"k8s.service.name":               svc.Name,
		"k8s.service.creation_timestamp": svc.GetCreationTimestamp().Format(time.RFC3339),
	}

	for key, value := range svc.Spec.Selector {
		meta[fmt.Sprintf("k8s.service.selector.%s", key)] = value
	}

	for key, value := range svc.Labels {
		meta[fmt.Sprintf("k8s.service.label.%s", key)] = value
	}

	for key, value := range svc.Annotations {
		meta[fmt.Sprintf("k8s.service.annotation.%s", key)] = value
	}

	km := &metadata.KubernetesMetadata{
		EntityType:    "k8s.service",
		ResourceIDKey: "k8s.service.uid",
		ResourceID:    experimentalmetricmetadata.ResourceID(svc.UID),
		Metadata:      meta,
	}

	return map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		experimentalmetricmetadata.ResourceID(svc.UID): km,
	}
}

// GetPodServiceTags returns a set of services associated with the pod.
func GetPodServiceTags(pod *corev1.Pod, services map[string]cache.Store) map[string]string {
	properties := map[string]string{}

	for _, storeKey := range [2]string{metadata.ClusterWideInformerKey, pod.Namespace} {
		if servicesStore, ok := services[storeKey]; ok {
			for _, ser := range servicesStore.List() {
				serObj := ser.(*corev1.Service)
				if serObj.Namespace == pod.Namespace &&
					labels.Set(serObj.Spec.Selector).AsSelectorPreValidated().Matches(labels.Set(pod.Labels)) {
					properties[fmt.Sprintf("%s%s", constants.K8sServicePrefix, serObj.Name)] = ""
					return properties
				}
			}
		}
	}
	return properties
}
