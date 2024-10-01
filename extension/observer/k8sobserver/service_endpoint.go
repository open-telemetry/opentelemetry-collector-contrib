// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"fmt"

	v1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// convertServiceToEndpoints converts a service instance into a slice of endpoints. The endpoints
// include the service itself only.
func convertServiceToEndpoints(idNamespace string, service *v1.Service) []observer.Endpoint {
	serviceID := observer.EndpointID(fmt.Sprintf("%s/%s", idNamespace, service.UID))

	serviceDetails := observer.K8sService{
		UID:         string(service.UID),
		Annotations: service.Annotations,
		Labels:      service.Labels,
		Name:        service.Name,
		Namespace:   service.Namespace,
		ClusterIP:   service.Spec.ClusterIP,
		ServiceType: string(service.Spec.Type),
	}

	endpoints := []observer.Endpoint{{
		ID:      serviceID,
		Target:  generateServiceTarget(&serviceDetails),
		Details: &serviceDetails,
	}}

	return endpoints
}

func generateServiceTarget(service *observer.K8sService) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", service.Name, service.Namespace)
}
