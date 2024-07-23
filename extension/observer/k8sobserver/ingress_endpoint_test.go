// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestIngressObjectToPortEndpoint(t *testing.T) {
	expectedEndpoints := []observer.Endpoint{
		{
			ID:     "namespace/ingress-1-UID/host-1/",
			Target: "https://host-1/",
			Details: &observer.K8sIngress{
				Name:      "application-ingress",
				Namespace: "default",
				UID:       "namespace/ingress-1-UID/host-1/",
				Labels:    map[string]string{"env": "prod"},
				Scheme:    "https",
				Host:      "host-1",
				Path:      "/",
			},
		},
		{
			ID:     "namespace/ingress-1-UID/host.2.host/",
			Target: "https://host.2.host/",
			Details: &observer.K8sIngress{
				Name:      "application-ingress",
				Namespace: "default",
				UID:       "namespace/ingress-1-UID/host.2.host/",
				Labels:    map[string]string{"env": "prod"},
				Scheme:    "https",
				Host:      "host.2.host",
				Path:      "/",
			},
		},
		{
			ID:     "namespace/ingress-1-UID/host.3.host/test",
			Target: "http://host.3.host/test",
			Details: &observer.K8sIngress{
				Name:      "application-ingress",
				Namespace: "default",
				UID:       "namespace/ingress-1-UID/host.3.host/test",
				Labels:    map[string]string{"env": "prod"},
				Scheme:    "http",
				Host:      "host.3.host",
				Path:      "/test",
			},
		},
	}

	endpoints := convertIngressToEndpoints("namespace", ingressMultipleHost)
	require.Equal(t, expectedEndpoints, endpoints)
}
