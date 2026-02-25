// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestNodeObjectToK8sNodeEndpoint(t *testing.T) {
	expectedNode := observer.Endpoint{
		ID:     "namespace/name-uid",
		Target: "internalIP",
		Details: &observer.K8sNode{
			UID:                 "uid",
			Annotations:         map[string]string{"annotation-key": "annotation-value"},
			Labels:              map[string]string{"label-key": "label-value"},
			Name:                "name",
			InternalIP:          "internalIP",
			InternalDNS:         "internalDNS",
			Hostname:            "hostname",
			ExternalIP:          "externalIP",
			ExternalDNS:         "externalDNS",
			KubeletEndpointPort: 1234,
		},
	}

	endpoint := convertNodeToEndpoint("namespace", newNode("name", "hostname"))
	require.Equal(t, expectedNode, endpoint)
}
