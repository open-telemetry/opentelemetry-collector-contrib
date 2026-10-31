// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
				Pod: observer.Pod{
					Name:      "pod-2",
					Namespace: "default",
					UID:       "pod-2-UID",
					Labels:    map[string]string{"env": "prod"},
				},
			},
		},
		{
			ID:     "namespace/pod-2-UID/container-2/https(443)",
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

	endpoints := convertPodToEndpoints("namespace", podWithNamedPorts)
	require.Equal(t, expectedEndpoints, endpoints)
}

func TestPodObjectToPortEndpointIPv6(t *testing.T) {
	pod := podWithNamedPorts.DeepCopy()
	pod.Status.PodIP = "2001:db8::10"

	endpoints := convertPodToEndpoints("namespace", pod)
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

func TestMultiContainerUnnamedPortNoCollision(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multi-container-pod",
			Namespace: "default",
			UID:       "multi-pod-uid",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "app",
					Image: "app:1.0",
					Ports: []v1.ContainerPort{
						{ContainerPort: 80, Protocol: v1.ProtocolTCP}, // Unnamed
					},
				},
				{
					Name:  "sidecar",
					Image: "sidecar:1.0",
					Ports: []v1.ContainerPort{
						{ContainerPort: 80, Protocol: v1.ProtocolTCP}, // Identical unnamed port
					},
				},
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			PodIP: "10.0.0.1",
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:        "app",
					State:       v1.ContainerState{Running: &v1.ContainerStateRunning{StartedAt: metav1.Now()}},
					ContainerID: "docker://app123",
				},
				{
					Name:        "sidecar",
					State:       v1.ContainerState{Running: &v1.ContainerStateRunning{StartedAt: metav1.Now()}},
					ContainerID: "docker://sidecar123",
				},
			},
		},
	}

	endpoints := convertPodToEndpoints("test-ns", pod)

	// Expect 5 distinct endpoints: 1 pod + 2 containers + 2 ports
	require.Len(t, endpoints, 5)

	expectedIDs := []observer.EndpointID{
		"test-ns/multi-pod-uid",
		"test-ns/multi-pod-uid/app",
		"test-ns/multi-pod-uid/app/(80)",
		"test-ns/multi-pod-uid/sidecar",
		"test-ns/multi-pod-uid/sidecar/(80)",
	}

	actualIDs := make([]observer.EndpointID, len(endpoints))
	for i, e := range endpoints {
		actualIDs[i] = e.ID
	}
	require.ElementsMatch(t, expectedIDs, actualIDs)
}

func TestMultiContainerUnnamedPortHandlerLifecycle(t *testing.T) {
	th := newTestHandler()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       "test-uid",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "c1",
					Image: "img1",
					Ports: []v1.ContainerPort{{ContainerPort: 8080, Protocol: v1.ProtocolTCP}},
				},
				{
					Name:  "c2",
					Image: "img2",
					Ports: []v1.ContainerPort{{ContainerPort: 8080, Protocol: v1.ProtocolTCP}},
				},
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			PodIP: "1.2.3.4",
			ContainerStatuses: []v1.ContainerStatus{
				{Name: "c1", State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}},
				{Name: "c2", State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}},
			},
		},
	}

	// 1. Test OnAdd: Ensure both ports are stored without overwriting
	th.OnAdd(pod, true)
	endpoints := th.ListEndpoints()
	require.Len(t, endpoints, 5, "Both container ports must exist in ListEndpoints")

	// 2. Test OnDelete: Ensure all endpoints are cleanly removed
	th.OnDelete(pod)
	require.Empty(t, th.ListEndpoints(), "All endpoints should be removed on pod delete")
}
