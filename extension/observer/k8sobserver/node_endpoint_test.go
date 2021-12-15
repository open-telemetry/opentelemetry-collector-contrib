// Copyright 2020, OpenTelemetry Authors
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
			Metadata: map[string]interface{}{
				"annotations": map[string]interface{}{
					"annotation-key": "annotation-value",
				},
				"creationTimestamp": nil,
				"labels": map[string]interface{}{
					"label-key": "label-value",
				},
				"name":      "name",
				"namespace": "namespace",
				"uid":       "uid",
			},
			Spec: map[string]interface{}{},
			Status: map[string]interface{}{
				"addresses": []interface{}{
					map[string]interface{}{
						"address": "hostname",
						"type":    "Hostname",
					},
					map[string]interface{}{
						"address": "externalDNS",
						"type":    "ExternalDNS",
					},
					map[string]interface{}{
						"address": "externalIP",
						"type":    "ExternalIP",
					},
					map[string]interface{}{
						"address": "internalDNS",
						"type":    "InternalDNS",
					},
					map[string]interface{}{
						"address": "internalIP",
						"type":    "InternalIP",
					},
				},
				"daemonEndpoints": map[string]interface{}{
					"kubeletEndpoint": map[string]interface{}{
						// float64 is a product of json unmarshalling
						"Port": float64(1234),
					},
				},
				"nodeInfo": map[string]interface{}{
					"architecture":            "architecture",
					"bootID":                  "boot-id",
					"containerRuntimeVersion": "runtime-version",
					"kernelVersion":           "kernel-version",
					"kubeProxyVersion":        "kube-proxy-version",
					"kubeletVersion":          "kubelet-version",
					"machineID":               "machine-id",
					"operatingSystem":         "operating-system",
					"osImage":                 "os-image",
					"systemUUID":              "system-uuid",
				},
				"phase": "Running",
			},
		},
	}

	endpoint := convertNodeToEndpoint("namespace", NewNode("name", "hostname"))
	require.Equal(t, expectedNode, endpoint)
}
