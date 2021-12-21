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

package receivercreator

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var pod = observer.Pod{
	UID:       "uid-1",
	Namespace: "default",
	Name:      "pod-1",
	Labels: map[string]string{
		"app":    "redis",
		"region": "west-1",
	},
	Annotations: map[string]string{
		"scrape": "true",
	},
}

var podEndpoint = observer.Endpoint{
	ID:      "pod-1",
	Target:  "localhost",
	Details: &pod,
}

var portEndpoint = observer.Endpoint{
	ID:     "port-1",
	Target: "localhost:1234",
	Details: &observer.Port{
		Name:      "http",
		Pod:       pod,
		Port:      1234,
		Transport: observer.ProtocolTCP,
	},
}

var hostportEndpoint = observer.Endpoint{
	ID:     "port-1",
	Target: "localhost:1234",
	Details: &observer.HostPort{
		ProcessName: "splunk",
		Command:     "./splunk",
		Port:        1234,
		Transport:   observer.ProtocolTCP,
	},
}

var container = observer.Container{
	Name:          "otel-agent",
	Image:         "otelcol",
	Port:          8080,
	AlternatePort: 80,
	Transport:     observer.ProtocolTCP,
	Command:       "otelcol -c",
	Host:          "localhost",
	ContainerID:   "abc123",
	Labels: map[string]string{
		"env":    "prod",
		"region": "east-1",
	},
}

var containerEndpoint = observer.Endpoint{
	ID:      "container-1",
	Target:  "localhost:1234",
	Details: &container,
}

var k8sNodeEndpoint = observer.Endpoint{
	ID:     "k8s.node-1",
	Target: "2.3.4.5",
	Details: &observer.K8sNode{
		Annotations: map[string]string{
			"node.alpha.kubernetes.io/ttl":                           "0",
			"volumes.kubernetes.io/controller-managed-attach-detach": "true",
		},
		ExternalDNS:         "an.external.dns",
		ExternalIP:          "1.2.3.4",
		Hostname:            "a.hostname",
		InternalDNS:         "an.internal.dns",
		InternalIP:          "2.3.4.5",
		KubeletEndpointPort: 10250,
		Labels: map[string]string{
			"beta.kubernetes.io/arch": "amd64",
			"beta.kubernetes.io/os":   "linux",
		},
		Metadata: map[string]interface{}{
			"annotations": map[string]string{
				"node.alpha.kubernetes.io/ttl":                           "0",
				"volumes.kubernetes.io/controller-managed-attach-detach": "true",
			},
			"labels": map[string]string{
				"beta.kubernetes.io/arch": "amd64",
				"beta.kubernetes.io/os":   "linux",
			},
			"name":            "a.name",
			"resourceVersion": "1",
			"uid":             "b344f2a7-1ec1-40f0-8557-8a9bfd8b6f99",
		},
		Name: "a.name",
		Spec: map[string]interface{}{
			"taints": []string{},
		},
		Status: map[string]interface{}{
			"addresses": []map[string]interface{}{
				{"address": "2.3.4.5", "type": "InternalIP"},
			},
			"allocatable": map[string]interface{}{},
			"capacity":    map[string]interface{}{"cpu": "2", "pods": "1"},
			"daemonEndpoints": map[string]interface{}{
				"kubeletEndpoint": map[string]interface{}{"Port": "10250"},
			},
			"images": []map[string]interface{}{
				{
					"names":     []string{"an.image:latest", "another.image:latest"},
					"sizeBytes": 15747069,
				},
				{
					"names":     []string{"an.image.name"},
					"sizeBytes": 685708,
				},
			},
			"nodeInfo": map[string]interface{}{
				"architecture":            "amd64",
				"containerRuntimeVersion": "containerd://0.0.1",
				"kubeletVersion":          "v1.2.3",
			},
		},
		UID: "b344f2a7-1ec1-40f0-8557-8a9bfd8b6f99",
	},
}

var unsupportedEndpoint = observer.Endpoint{
	ID:      "endpoint-1",
	Target:  "localhost:1234",
	Details: nil,
}
