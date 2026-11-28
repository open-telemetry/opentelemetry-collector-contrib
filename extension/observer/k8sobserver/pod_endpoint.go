// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

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
	// terminatedContainers also reports containers that ran and are not running now, in a pod of
	// any phase.
	terminatedContainers bool
}

// convertPodToEndpoints converts a pod instance into a slice of endpoints. The endpoints
// include the pod itself, when it is running, as well as an endpoint for each reported
// container and for each container port of a container that is in a running state.
func convertPodToEndpoints(idNamespace string, pod *v1.Pod, opts podEndpointOptions) []observer.Endpoint {
	podID := podEndpointID(idNamespace, pod)
	podIP := pod.Status.PodIP
	podRunning := pod.Status.Phase == v1.PodRunning
	podPending := pod.Status.Phase == v1.PodPending
	podFinished := pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed

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
		statusByName := make(map[string]*v1.ContainerStatus, len(statuses))
		for i := range statuses {
			statusByName[statuses[i].Name] = &statuses[i]
		}

		for i := range containers {
			container := &containers[i]
			status, ok := statusByName[container.Name]
			if !ok {
				continue
			}
			// The status of a container can still say running after the pod finished.
			running := status.State.Running != nil && !podFinished
			// A container that is not running is reported only when it ran.
			if !running && (!opts.terminatedContainers || status.ContainerID == "") {
				continue
			}
			rc := containerIDWithRuntime(status)

			endpoints = append(endpoints, observer.Endpoint{
				ID:     containerEndpointID(podID, container.Name),
				Target: podIP,
				Details: &observer.PodContainer{
					Name:        container.Name,
					ContainerID: rc.ID,
					Image:       container.Image,
					Kind:        kind,
					Running:     running,
					Pod:         podDetails,
				},
			})

			// A port can be reached only while its container runs. The pod has to run too, or for
			// an init container be pending and have an address.
			if !running || (!podRunning && (!podPending || kind != containerKindInit || podIP == "")) {
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
	if opts.initContainers && (podRunning || podPending || opts.terminatedContainers) {
		appendContainers(pod.Spec.InitContainers, pod.Status.InitContainerStatuses, containerKindInit)
	}
	if !podRunning && !opts.terminatedContainers {
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

func podEndpointID(idNamespace string, pod *v1.Pod) observer.EndpointID {
	return observer.EndpointID(fmt.Sprintf("%s/%s", idNamespace, pod.UID))
}

func containerEndpointID(podID observer.EndpointID, containerName string) observer.EndpointID {
	return observer.EndpointID(fmt.Sprintf("%s/%s", podID, containerName))
}

// containerFinishTimes returns when the containers of a pod that are not running last finished,
// by the ID of the container endpoint.
func containerFinishTimes(idNamespace string, pod *v1.Pod) map[observer.EndpointID]time.Time {
	podID := podEndpointID(idNamespace, pod)
	finished := map[observer.EndpointID]time.Time{}
	for _, statuses := range [][]v1.ContainerStatus{
		pod.Status.InitContainerStatuses,
		pod.Status.ContainerStatuses,
		pod.Status.EphemeralContainerStatuses,
	} {
		for i := range statuses {
			status := &statuses[i]
			var terminated *v1.ContainerStateTerminated
			switch {
			case status.State.Running != nil:
				continue
			case status.State.Terminated != nil:
				terminated = status.State.Terminated
			default:
				// A container that waits for its next start, as in a crash loop.
				terminated = status.LastTerminationState.Terminated
			}
			if terminated != nil && !terminated.FinishedAt.IsZero() {
				finished[containerEndpointID(podID, status.Name)] = terminated.FinishedAt.Time
			}
		}
	}
	return finished
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
