// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"testing"

	"github.com/stretchr/testify/require"

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
