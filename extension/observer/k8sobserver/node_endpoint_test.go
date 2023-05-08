// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	endpoint := convertNodeToEndpoint("namespace", NewNode("name", "hostname"))
	require.Equal(t, expectedNode, endpoint)
}
