// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// Helper phase maps for tests
var (
	runningOnly       = map[string]bool{"Running": true}
	runningAndPending = map[string]bool{"Running": true, "Pending": true}
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
				Name:            "container-2",
				Image:           "container-image-2",
				ContainerID:     "a808232bb4a57d421bb16f20dc9ab2a441343cb0aae8c369dc375838c7a49fd7",
				IsInitContainer: false,
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

	// Running pod with default phases (Running only)
	endpoints := convertPodToEndpoints("namespace", podWithNamedPorts, runningOnly, true, DefaultContainerTerminatedTTL)
	require.Equal(t, expectedEndpoints, endpoints)
}

func TestPodObjectWithRunningInitContainerInPendingPod(t *testing.T) {
	expectedEndpoints := []observer.Endpoint{
		{
			ID:      "namespace/pod-init-pending-UID",
			Target:  "",
			Details: &observer.Pod{Name: "pod-init-pending", Namespace: "default", UID: "pod-init-pending-UID", Labels: map[string]string{"env": "prod"}},
		},
		{
			ID:     "namespace/pod-init-pending-UID/init-1",
			Target: "",
			Details: &observer.PodContainer{
				Name:            "init-1",
				Image:           "init-image-1",
				ContainerID:     "init-running-id",
				IsInitContainer: true,
				Pod: observer.Pod{
					Name:      "pod-init-pending",
					Namespace: "default",
					UID:       "pod-init-pending-UID",
					Labels:    map[string]string{"env": "prod"},
				},
			},
		},
	}

	// Pending pod requires Pending in observe_pod_phases
	endpoints := convertPodToEndpoints("namespace", podPendingWithRunningInit, runningAndPending, true, DefaultContainerTerminatedTTL)
	require.Equal(t, expectedEndpoints, endpoints)
}

func TestPodObjectPendingPodNotObservedByDefault(t *testing.T) {
	// Pending pods should not be observed when only Running is in observe_pod_phases (default)
	endpoints := convertPodToEndpoints("namespace", podPendingWithRunningInit, runningOnly, true, DefaultContainerTerminatedTTL)
	require.Nil(t, endpoints)
}

func TestPodObjectWithTerminatedInitContainerInRunningPod(t *testing.T) {
	expectedEndpoints := []observer.Endpoint{
		{
			ID:     "namespace/pod-init-running-UID",
			Target: "1.2.3.4",
			Details: &observer.Pod{
				Name:      "pod-init-running",
				Namespace: "default",
				UID:       "pod-init-running-UID",
				Labels:    map[string]string{"env": "prod"},
			},
		},
		{
			ID:     "namespace/pod-init-running-UID/init-1",
			Target: "1.2.3.4",
			Details: &observer.PodContainer{
				Name:            "init-1",
				Image:           "init-image-1",
				ContainerID:     "init-terminated-id",
				IsInitContainer: true,
				Pod: observer.Pod{
					Name:      "pod-init-running",
					Namespace: "default",
					UID:       "pod-init-running-UID",
					Labels:    map[string]string{"env": "prod"},
				},
			},
		},
	}

	// Running pod - observe_pod_phases just needs Running
	endpoints := convertPodToEndpoints("namespace", podRunningWithTerminatedInit, runningOnly, true, DefaultContainerTerminatedTTL)
	require.Equal(t, expectedEndpoints, endpoints)
}

func TestPodObjectInitContainersDisabled(t *testing.T) {
	endpoints := convertPodToEndpoints("namespace", podRunningWithTerminatedInit, runningOnly, false, DefaultContainerTerminatedTTL)
	require.Len(t, endpoints, 1)
	require.Equal(t, observer.EndpointID("namespace/pod-init-running-UID"), endpoints[0].ID)
}

func TestPodObjectTerminatedInitContainerExpiredTTL(t *testing.T) {
	// Test that terminated init containers are excluded after TTL expires
	endpoints := convertPodToEndpoints("namespace", podRunningWithTerminatedInit, runningOnly, true, 0)
	require.Len(t, endpoints, 1)
	require.Equal(t, observer.EndpointID("namespace/pod-init-running-UID"), endpoints[0].ID)
}

func TestPodObjectTerminatedInitContainerWithinTTL(t *testing.T) {
	// Test that terminated init containers are included within TTL
	endpoints := convertPodToEndpoints("namespace", podRunningWithTerminatedInit, runningOnly, true, time.Hour)
	require.Len(t, endpoints, 2)
}

func TestPodObjectTerminatedContainerWithinTTL(t *testing.T) {
	// Test that terminated regular containers are included within TTL
	expectedEndpoints := []observer.Endpoint{
		{
			ID:     "namespace/pod-terminated-container-UID",
			Target: "1.2.3.4",
			Details: &observer.Pod{
				Name:      "pod-terminated-container",
				Namespace: "default",
				UID:       "pod-terminated-container-UID",
				Labels:    map[string]string{"env": "prod"},
			},
		},
		{
			ID:     "namespace/pod-terminated-container-UID/container-2",
			Target: "1.2.3.4",
			Details: &observer.PodContainer{
				Name:            "container-2",
				Image:           "container-image-2",
				ContainerID:     "container-2-terminated-id",
				IsInitContainer: false,
				Pod: observer.Pod{
					Name:      "pod-terminated-container",
					Namespace: "default",
					UID:       "pod-terminated-container-UID",
					Labels:    map[string]string{"env": "prod"},
				},
			},
		},
		{
			ID:     "namespace/pod-terminated-container-UID/https(443)",
			Target: "1.2.3.4:443",
			Details: &observer.Port{
				Name: "https",
				Pod: observer.Pod{
					Name:      "pod-terminated-container",
					Namespace: "default",
					UID:       "pod-terminated-container-UID",
					Labels:    map[string]string{"env": "prod"},
				},
				Port:           443,
				Transport:      observer.ProtocolTCP,
				ContainerName:  "container-2",
				ContainerID:    "container-2-terminated-id",
				ContainerImage: "container-image-2",
			},
		},
	}
	endpoints := convertPodToEndpoints("namespace", podRunningWithTerminatedContainer, runningOnly, false, time.Hour)
	require.Equal(t, expectedEndpoints, endpoints)
}

func TestPodObjectTerminatedContainerExpiredTTL(t *testing.T) {
	// Test that terminated regular containers are excluded after TTL expires
	endpoints := convertPodToEndpoints("namespace", podRunningWithTerminatedContainer, runningOnly, false, 0)
	require.Len(t, endpoints, 1)
	require.Equal(t, observer.EndpointID("namespace/pod-terminated-container-UID"), endpoints[0].ID)
}

func TestPodObjectInitContainerWithPort(t *testing.T) {
	// Test that init containers with ports get port endpoints just like regular containers
	expectedEndpoints := []observer.Endpoint{
		{
			ID:     "namespace/pod-init-with-port-UID",
			Target: "1.2.3.4",
			Details: &observer.Pod{
				Name:      "pod-init-with-port",
				Namespace: "default",
				UID:       "pod-init-with-port-UID",
				Labels:    map[string]string{"env": "prod"},
			},
		},
		{
			ID:     "namespace/pod-init-with-port-UID/init-with-port",
			Target: "1.2.3.4",
			Details: &observer.PodContainer{
				Name:            "init-with-port",
				Image:           "init-image-with-port",
				ContainerID:     "init-with-port-id",
				IsInitContainer: true,
				Pod: observer.Pod{
					Name:      "pod-init-with-port",
					Namespace: "default",
					UID:       "pod-init-with-port-UID",
					Labels:    map[string]string{"env": "prod"},
				},
			},
		},
		{
			ID:     "namespace/pod-init-with-port-UID/init-http(8080)",
			Target: "1.2.3.4:8080",
			Details: &observer.Port{
				Name: "init-http",
				Pod: observer.Pod{
					Name:      "pod-init-with-port",
					Namespace: "default",
					UID:       "pod-init-with-port-UID",
					Labels:    map[string]string{"env": "prod"},
				},
				Port:           8080,
				Transport:      observer.ProtocolTCP,
				ContainerName:  "init-with-port",
				ContainerID:    "init-with-port-id",
				ContainerImage: "init-image-with-port",
			},
		},
	}
	endpoints := convertPodToEndpoints("namespace", podRunningWithInitContainerWithPort, runningOnly, true, time.Hour)
	require.Equal(t, expectedEndpoints, endpoints)
}

func TestPodObjectCrashLoopBackOffWithinTTL(t *testing.T) {
	// Test that containers in CrashLoopBackOff are observable within TTL
	expectedEndpoints := []observer.Endpoint{
		{
			ID:     "namespace/pod-crashloop-UID",
			Target: "1.2.3.4",
			Details: &observer.Pod{
				Name:      "pod-crashloop",
				Namespace: "default",
				UID:       "pod-crashloop-UID",
				Labels:    map[string]string{"env": "prod"},
			},
		},
		{
			ID:     "namespace/pod-crashloop-UID/crashloop-container",
			Target: "1.2.3.4",
			Details: &observer.PodContainer{
				Name:            "crashloop-container",
				Image:           "crashloop-image",
				ContainerID:     "crashloop-container-id",
				IsInitContainer: false,
				Pod: observer.Pod{
					Name:      "pod-crashloop",
					Namespace: "default",
					UID:       "pod-crashloop-UID",
					Labels:    map[string]string{"env": "prod"},
				},
			},
		},
		{
			ID:     "namespace/pod-crashloop-UID/http(8080)",
			Target: "1.2.3.4:8080",
			Details: &observer.Port{
				Name: "http",
				Pod: observer.Pod{
					Name:      "pod-crashloop",
					Namespace: "default",
					UID:       "pod-crashloop-UID",
					Labels:    map[string]string{"env": "prod"},
				},
				Port:           8080,
				Transport:      observer.ProtocolTCP,
				ContainerName:  "crashloop-container",
				ContainerID:    "crashloop-container-id",
				ContainerImage: "crashloop-image",
			},
		},
	}
	endpoints := convertPodToEndpoints("namespace", podWithCrashLoopBackOff, runningOnly, false, time.Hour)
	require.Equal(t, expectedEndpoints, endpoints)
}

func TestPodObjectCrashLoopBackOffExpiredTTL(t *testing.T) {
	// Test that containers in CrashLoopBackOff are excluded after TTL expires
	endpoints := convertPodToEndpoints("namespace", podWithCrashLoopBackOff, runningOnly, false, 0)
	require.Len(t, endpoints, 1)
	require.Equal(t, observer.EndpointID("namespace/pod-crashloop-UID"), endpoints[0].ID)
}
