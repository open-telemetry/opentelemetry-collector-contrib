// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestPodObjectToPortEndpoint(t *testing.T) {
	expectedEndpoints := []observer.Endpoint{
		{
			ID:     "namespace/pod-2-UID",
			Target: "1.2.3.4",
			Details: &observer.Pod{
				Name:      "pod-2",
				Namespace: "default",
				UID:       "pod-2-UID",
				Labels:    map[string]string{"env": "prod"},
			},
		},
		{
			ID:     "namespace/pod-2-UID/container-2",
			Target: "1.2.3.4",
			Details: &observer.PodContainer{
				Name:        "container-2",
				Image:       "container-image-2",
				ContainerID: "a808232bb4a57d421bb16f20dc9ab2a441343cb0aae8c369dc375838c7a49fd7",
				Kind:        "app",
				Pod: observer.Pod{
					Name:      "pod-2",
					Namespace: "default",
					UID:       "pod-2-UID",
					Labels:    map[string]string{"env": "prod"},
				},
			},
		},
		{
			ID:     "namespace/pod-2-UID/https(443)",
			Target: "1.2.3.4:443",
			Details: &observer.Port{
				Name: "https", Pod: observer.Pod{
					Name:      "pod-2",
					Namespace: "default",
					UID:       "pod-2-UID",
					Labels:    map[string]string{"env": "prod"},
				},
				Port:           443,
				Transport:      observer.ProtocolTCP,
				ContainerName:  "container-2",
				ContainerID:    "a808232bb4a57d421bb16f20dc9ab2a441343cb0aae8c369dc375838c7a49fd7",
				ContainerImage: "container-image-2",
			},
		},
	}

	endpoints := convertPodToEndpoints("namespace", podWithNamedPorts, podEndpointOptions{})
	require.Equal(t, expectedEndpoints, endpoints)
}

func TestPodObjectToPortEndpointIPv6(t *testing.T) {
	pod := podWithNamedPorts.DeepCopy()
	pod.Status.PodIP = "2001:db8::10"

	endpoints := convertPodToEndpoints("namespace", pod, podEndpointOptions{})
	require.Len(t, endpoints, 3)

	endpoint := endpoints[2]
	require.Equal(t, "[2001:db8::10]:443", endpoint.Target)

	env, err := endpoint.Env()
	require.NoError(t, err)
	require.Equal(t, "[2001:db8::10]:443", env["endpoint"])
	require.Equal(t, "2001:db8::10", env["host"])
	require.Equal(t, string(observer.PortType), env["type"])
	require.Equal(t, uint16(443), env["port"])
}

func TestPodEndpointsEphemeralContainers(t *testing.T) {
	pod := podWithNamedPorts.DeepCopy()
	pod.Spec.EphemeralContainers = []v1.EphemeralContainer{
		{EphemeralContainerCommon: v1.EphemeralContainerCommon{Name: "debugger", Image: "debug-image"}},
		{EphemeralContainerCommon: v1.EphemeralContainerCommon{Name: "old-debugger", Image: "debug-image"}},
	}
	pod.Status.EphemeralContainerStatuses = []v1.ContainerStatus{
		{Name: "debugger", ContainerID: "containerd://debugger-id", State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}},
		{Name: "old-debugger", ContainerID: "containerd://old-debugger-id", State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{}}},
	}

	// Without the option the ephemeral containers get no endpoint.
	require.Len(t, convertPodToEndpoints("namespace", pod, podEndpointOptions{}), 3)

	// With it the one that runs gets a container endpoint.
	endpoints := convertPodToEndpoints("namespace", pod, podEndpointOptions{ephemeralContainers: true})
	require.Len(t, endpoints, 4)
	require.Equal(t, observer.Endpoint{
		ID:     "namespace/pod-2-UID/debugger",
		Target: "1.2.3.4",
		Details: &observer.PodContainer{
			Name:        "debugger",
			Image:       "debug-image",
			ContainerID: "debugger-id",
			Kind:        "ephemeral",
			Pod: observer.Pod{
				Name:      "pod-2",
				Namespace: "default",
				UID:       "pod-2-UID",
				Labels:    map[string]string{"env": "prod"},
			},
		},
	}, endpoints[3])
	require.Equal(t, "app", endpoints[1].Details.(*observer.PodContainer).Kind)

	env, err := endpoints[3].Env()
	require.NoError(t, err)
	require.Equal(t, "ephemeral", env["container_kind"])

	// The spec of the pod is not changed.
	require.Len(t, pod.Spec.Containers, len(podWithNamedPorts.Spec.Containers))
}
