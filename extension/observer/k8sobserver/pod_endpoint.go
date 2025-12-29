// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// convertPodToEndpoints converts a pod instance into a slice of endpoints. The endpoints
// include the pod itself as well as an endpoint for each container port that is mapped
// to a container that is in a running state or has recently terminated (within the TTL).
// Init containers are included when configured. The TTL applies to all terminated containers
// (both regular and init), allowing time for log collection from crashed or completed containers.
func convertPodToEndpoints(idNamespace string, pod *v1.Pod, observePodPhases map[string]bool, observeInitContainers bool, containerTerminatedTTL time.Duration) []observer.Endpoint {
	podID := observer.EndpointID(fmt.Sprintf("%s/%s", idNamespace, pod.UID))
	podIP := pod.Status.PodIP

	podDetails := observer.Pod{
		UID:         string(pod.UID),
		Annotations: pod.Annotations,
		Labels:      pod.Labels,
		Name:        pod.Name,
		Namespace:   pod.Namespace,
	}

	// Return no endpoints if the Pod is not in an observable phase
	if !observePodPhases[string(pod.Status.Phase)] {
		return nil
	}

	endpoints := []observer.Endpoint{{
		ID:      podID,
		Target:  podIP,
		Details: &podDetails,
	}}

	// Map of observable containers by name (running or recently terminated).
	observableContainers := map[string]runningContainer{}
	initContainers := map[string]runningContainer{}

	for i := range pod.Status.ContainerStatuses {
		container := &pod.Status.ContainerStatuses[i]
		if container.State.Running != nil {
			// Always include running containers
			observableContainers[container.Name] = containerIDWithRuntime(container)
		} else if container.State.Terminated != nil {
			// Include terminated containers if they terminated within the TTL
			if time.Since(container.State.Terminated.FinishedAt.Time) <= containerTerminatedTTL {
				observableContainers[container.Name] = containerIDWithRuntime(container)
			}
		}
	}

	if observeInitContainers {
		for i := range pod.Status.InitContainerStatuses {
			container := &pod.Status.InitContainerStatuses[i]
			if container.State.Running != nil {
				// Always include running init containers
				initContainers[container.Name] = containerIDWithRuntime(container)
			} else if container.State.Terminated != nil {
				// Include terminated init containers if they terminated within the TTL
				if time.Since(container.State.Terminated.FinishedAt.Time) <= containerTerminatedTTL {
					initContainers[container.Name] = containerIDWithRuntime(container)
				}
			}
		}
	}

	// Create endpoint for each named container port.
	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		var rc runningContainer
		var ok bool
		if rc, ok = observableContainers[container.Name]; !ok {
			continue
		}

		endpointID := observer.EndpointID(
			fmt.Sprintf(
				"%s/%s", podID, container.Name,
			),
		)
		endpoints = append(endpoints, observer.Endpoint{
			ID:     endpointID,
			Target: podIP,
			Details: &observer.PodContainer{
				Name:            container.Name,
				ContainerID:     rc.ID,
				Image:           container.Image,
				IsInitContainer: false,
				Pod:             podDetails,
			},
		})

		// Create endpoint for each named container port.
		for _, port := range container.Ports {
			endpointID = observer.EndpointID(
				fmt.Sprintf(
					"%s/%s(%d)", podID, port.Name, port.ContainerPort,
				),
			)
			endpoints = append(endpoints, observer.Endpoint{
				ID:     endpointID,
				Target: fmt.Sprintf("%s:%d", podIP, port.ContainerPort),
				Details: &observer.Port{
					Pod:            podDetails,
					Name:           port.Name,
					Port:           uint16(port.ContainerPort),
					Transport:      getTransport(port.Protocol),
					ContainerName:  container.Name,
					ContainerID:    rc.ID,
					ContainerImage: container.Image,
				},
			})
		}
	}

	if observeInitContainers {
		for i := range pod.Spec.InitContainers {
			container := &pod.Spec.InitContainers[i]
			rc, ok := initContainers[container.Name]
			if !ok {
				continue
			}

			endpointID := observer.EndpointID(
				fmt.Sprintf(
					"%s/%s", podID, container.Name,
				),
			)
			endpoints = append(endpoints, observer.Endpoint{
				ID:     endpointID,
				Target: podIP,
				Details: &observer.PodContainer{
					Name:            container.Name,
					ContainerID:     rc.ID,
					Image:           container.Image,
					IsInitContainer: true,
					Pod:             podDetails,
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

// containerIDWithRuntime parses the container ID to get the actual ID string
func containerIDWithRuntime(c *v1.ContainerStatus) runningContainer {
	cID := c.ContainerID
	if cID != "" {
		parts := strings.Split(cID, "://")
		if len(parts) == 2 {
			return runningContainer{parts[1], parts[0]}
		}
	}
	return runningContainer{}
}

type runningContainer struct {
	ID      string
	Runtime string
}
