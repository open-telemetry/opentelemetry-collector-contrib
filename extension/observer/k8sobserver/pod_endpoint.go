// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// The kinds of a container, by where the pod spec declares it.
const (
	containerKindApp       = "app"
	containerKindEphemeral = "ephemeral"
)

// podEndpointOptions selects the containers of a pod that get endpoints, next to its app
// containers.
type podEndpointOptions struct {
	// ephemeralContainers also reports the ephemeral containers of a pod.
	ephemeralContainers bool
}

// convertPodToEndpoints converts a pod instance into a slice of endpoints. The endpoints
// include the pod itself as well as an endpoint for each container port that is mapped
// to a container that is in a running state.
func convertPodToEndpoints(idNamespace string, pod *v1.Pod, opts podEndpointOptions) []observer.Endpoint {
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
	runningContainers := map[string]runningContainer{}

	for i := range pod.Status.ContainerStatuses {
		container := &pod.Status.ContainerStatuses[i]
		if container.State.Running != nil {
			runningContainers[container.Name] = containerIDWithRuntime(container)
		}
	}

	containers := pod.Spec.Containers
	containerKinds := map[string]string{}
	if opts.ephemeralContainers {
		for i := range pod.Status.EphemeralContainerStatuses {
			container := &pod.Status.EphemeralContainerStatuses[i]
			if container.State.Running != nil {
				runningContainers[container.Name] = containerIDWithRuntime(container)
			}
		}
		containers = slices.Clone(containers)
		for i := range pod.Spec.EphemeralContainers {
			container := v1.Container(pod.Spec.EphemeralContainers[i].EphemeralContainerCommon)
			containerKinds[container.Name] = containerKindEphemeral
			containers = append(containers, container)
		}
	}

	// Create endpoint for each named container port.
	for i := range containers {
		container := &containers[i]
		kind, ok := containerKinds[container.Name]
		if !ok {
			kind = containerKindApp
		}
		var rc runningContainer
		if rc, ok = runningContainers[container.Name]; !ok {
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
				Name:        container.Name,
				ContainerID: rc.ID,
				Image:       container.Image,
				Kind:        kind,
				Pod:         podDetails,
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
				Target: net.JoinHostPort(podIP, strconv.Itoa(int(port.ContainerPort))),
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
