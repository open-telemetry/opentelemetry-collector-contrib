// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exportercreator

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
	ID:      observer.EndpointID("pod-1"),
	Target:  "localhost",
	Details: &pod,
}

var service = observer.K8sService{
	UID:       "uid-1",
	Namespace: "default",
	Name:      "service-1",
	Labels: map[string]string{
		"app":    "redis2",
		"region": "west-1",
	},
	Annotations: map[string]string{
		"scrape": "true",
	},
}

var serviceEndpoint = observer.Endpoint{
	ID:      observer.EndpointID("service-1"),
	Target:  "localhost",
	Details: &service,
}

var portEndpoint = observer.Endpoint{
	ID:     observer.EndpointID("port-1"),
	Target: "localhost:1234",
	Details: &observer.Port{
		Name:           "http",
		Pod:            pod,
		Port:           1234,
		Transport:      observer.ProtocolTCP,
		ContainerName:  "container-1",
		ContainerID:    "container-id-1",
		ContainerImage: "redis:latest",
	},
}

var hostportEndpoint = observer.Endpoint{
	ID:     observer.EndpointID("hostport-1"),
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
	ID:      observer.EndpointID("container-1"),
	Target:  "localhost:1234",
	Details: &container,
}

var k8sNodeEndpoint = observer.Endpoint{
	ID:     observer.EndpointID("k8s.node-1"),
	Target: "2.3.4.5",
	Details: &observer.K8sNode{
		Annotations: map[string]string{
			"node.alpha.kubernetes.io/ttl": "0",
		},
		ExternalDNS:         "an.external.dns",
		ExternalIP:          "1.2.3.4",
		Hostname:            "a.hostname",
		InternalDNS:         "an.internal.dns",
		InternalIP:          "2.3.4.5",
		KubeletEndpointPort: 10250,
		Labels: map[string]string{
			"kubernetes.io/arch": "amd64",
			"kubernetes.io/os":   "linux",
		},
		Name: "a.name",
		UID:  "b344f2a7-1ec1-40f0-8557-8a9bfd8b6f99",
	},
}

var podContainerEndpoint = observer.Endpoint{
	ID:     observer.EndpointID("pod-container-1"),
	Target: "localhost:6379",
	Details: &observer.PodContainer{
		Image: "redis",
		Name:  "redis",
		Pod:   pod,
	},
}

var kafkaTopicsEndpoint = observer.Endpoint{
	ID:      observer.EndpointID("topic1"),
	Target:  "topic1",
	Details: &observer.KafkaTopic{},
}

var crdEndpoint = observer.Endpoint{
	ID:     observer.EndpointID("default/crd-1"),
	Target: "crd-1",
	Details: &observer.K8sCRD{
		Name:      "crd-1",
		UID:       "crd-uid-1",
		Namespace: "default",
		Group:     "telemetry.opentelemetry.io",
		Version:   "v1alpha1",
		Kind:      "Exporter",
		Labels: map[string]string{
			"app": "myapp",
		},
		Spec: map[string]any{
			"exporterType": "prometheusremotewrite",
			"endpoint":     "https://example.com/write",
		},
	},
}

var jsonFileEndpoint = observer.Endpoint{
	ID:     observer.EndpointID("jsonfile/endpoint-1"),
	Target: "localhost:8080",
	Details: &mockJSONFileEndpoint{
		ID:   "endpoint-1",
		Name: "Test Service",
		Labels: map[string]string{
			"service": "test-service",
			"env":     "production",
		},
	},
}

var unsupportedEndpoint = observer.Endpoint{
	ID:      observer.EndpointID("endpoint-1"),
	Target:  "localhost:1234",
	Details: nil,
}

var portRule = func() *rule {
	r, _ := newRule(`type == "port"`)
	return &r
}()
