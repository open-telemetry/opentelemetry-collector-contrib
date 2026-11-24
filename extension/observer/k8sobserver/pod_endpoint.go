// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// The kinds of a container, by where the pod spec declares it.
const (
	containerKindApp       = "app"
	containerKindInit      = "init"
	containerKindEphemeral = "ephemeral"
)

// podEndpointOptions selects the containers of a pod that get endpoints, next to its app
// containers.
type podEndpointOptions struct {
	// initContainers also reports the init containers of a pod, native sidecars included.
	initContainers bool
	// ephemeralContainers also reports the ephemeral containers of a pod.
	ephemeralContainers bool
}

// convertPodToEndpoints converts a pod instance into a slice of endpoints. The endpoints
// include the pod itself, when it is running, as well as an endpoint for each reported
// container that is in a running state and for each of its container ports.
func convertPodToEndpoints(idNamespace string, pod *v1.Pod, opts podEndpointOptions) []observer.Endpoint {
	podID := observer.EndpointID(fmt.Sprintf("%s/%s", idNamespace, pod.UID))
	podIP := pod.Status.PodIP
	podRunning := pod.Status.Phase == v1.PodRunning

	podDetails := observer.Pod{
		UID:         string(pod.UID),
		Annotations: pod.Annotations,
		Labels:      pod.Labels,
		Name:        pod.Name,
		Namespace:   pod.Namespace,
	}

	var endpoints []observer.Endpoint
	if podRunning {
		endpoints = append(endpoints, observer.Endpoint{
			ID:      podID,
			Target:  podIP,
			Details: &podDetails,
		})
	}

	appendContainers := func(containers []v1.Container, statuses []v1.ContainerStatus, kind string) {
		// Map of running containers by name.
		runningContainers := map[string]runningContainer{}
		for i := range statuses {
			status := &statuses[i]
			if status.State.Running != nil {
				runningContainers[status.Name] = containerIDWithRuntime(status)
			}
		}

		for i := range containers {
			container := &containers[i]
			rc, ok := runningContainers[container.Name]
			if !ok {
				continue
			}

			endpoints = append(endpoints, observer.Endpoint{
				ID:     observer.EndpointID(fmt.Sprintf("%s/%s", podID, container.Name)),
				Target: podIP,
				Details: &observer.PodContainer{
					Name:        container.Name,
					ContainerID: rc.ID,
					Image:       container.Image,
					Kind:        kind,
					Pod:         podDetails,
				},
			})

			// A port can be reached only once the pod has an address.
			if podIP == "" {
				continue
			}
			// Create endpoint for each named container port.
			for _, port := range container.Ports {
				endpoints = append(endpoints, observer.Endpoint{
					ID:     observer.EndpointID(fmt.Sprintf("%s/%s(%d)", podID, port.Name, port.ContainerPort)),
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
	}

	// Init containers run while the pod is still pending. They come before the app containers,
	// so that the port of an app container is the one kept when both declare a port with the
	// same name and number.
	if opts.initContainers && (podRunning || pod.Status.Phase == v1.PodPending) {
		appendContainers(pod.Spec.InitContainers, pod.Status.InitContainerStatuses, containerKindInit)
	}
	if !podRunning {
		return endpoints
	}
	appendContainers(pod.Spec.Containers, pod.Status.ContainerStatuses, containerKindApp)
	if opts.ephemeralContainers {
		ephemeral := make([]v1.Container, 0, len(pod.Spec.EphemeralContainers))
		for i := range pod.Spec.EphemeralContainers {
			ephemeral = append(ephemeral, v1.Container(pod.Spec.EphemeralContainers[i].EphemeralContainerCommon))
		}
		appendContainers(ephemeral, pod.Status.EphemeralContainerStatuses, containerKindEphemeral)
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
