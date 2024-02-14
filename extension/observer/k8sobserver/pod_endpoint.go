// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"fmt"

	v1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// convertPodToEndpoints converts a pod instance into a slice of endpoints. The endpoints
// include the pod itself as well as an endpoint for each container port that is mapped
// to a container that is in a running state.
func convertPodToEndpoints(idNamespace string, pod *v1.Pod) []observer.Endpoint {
	podID := observer.EndpointID(fmt.Sprintf("%s/%s", idNamespace, pod.UID))
	podIP := pod.Status.PodIP

	podDetails := observer.Pod{
		UID:         string(pod.UID),
		Annotations: pod.Annotations,
		Labels:      pod.Labels,
		Name:        pod.Name,
		Namespace:   pod.Namespace,
	}

	// Return no endpoints if the Pod is not running
	if pod.Status.Phase != v1.PodRunning {
		return nil
	}

	endpoints := []observer.Endpoint{{
		ID:      podID,
		Target:  podIP,
		Details: &podDetails,
	}}

	// Map of running containers by name.
	containerRunning := map[string]bool{}

	for _, container := range pod.Status.ContainerStatuses {
		if container.State.Running != nil {
			containerRunning[container.Name] = true
		}
	}

	// Create endpoint for each named container port.
	for _, container := range pod.Spec.Containers {
		if !containerRunning[container.Name] {
			continue
		}

		for _, port := range container.Ports {
			endpointID := observer.EndpointID(
				fmt.Sprintf(
					"%s/%s(%d)", podID, port.Name, port.ContainerPort,
				),
			)
			endpoints = append(endpoints, observer.Endpoint{
				ID:     endpointID,
				Target: fmt.Sprintf("%s:%d", podIP, port.ContainerPort),
				Details: &observer.Port{
					Pod:       podDetails,
					Name:      port.Name,
					Port:      uint16(port.ContainerPort),
					Transport: getTransport(port.Protocol),
				},
			})
		}
	}

	return endpoints
}

func getTransport(protocol v1.Protocol) observer.Transport {
	switch protocol {
	case v1.ProtocolTCP:
		return observer.ProtocolTCP
	case v1.ProtocolUDP:
		return observer.ProtocolUDP
	}
	return observer.ProtocolUnknown
}
