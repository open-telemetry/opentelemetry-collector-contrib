// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestServiceObjectToEndpoint(t *testing.T) {
	expectedEndpoints := []observer.Endpoint{
		{
			ID:     "namespace/service-1-UID",
			Target: "service-1.default.svc.cluster.local",
			Details: &observer.K8sService{
				Name:        "service-1",
				Namespace:   "default",
				UID:         "service-1-UID",
				Labels:      map[string]string{"env": "prod"},
				ServiceType: "ClusterIP",
				ClusterIP:   "1.2.3.4",
			}},
	}

	endpoints := convertServiceToEndpoints("namespace", serviceWithClusterIP)
	require.Equal(t, expectedEndpoints, endpoints)
}
