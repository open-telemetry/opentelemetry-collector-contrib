// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package observer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndpointEnv(t *testing.T) {
	tests := []struct {
		name     string
		endpoint Endpoint
		want     EndpointEnv
	}{
		{
			name: "Pod",
			endpoint: Endpoint{
				ID:     EndpointID("pod_id"),
				Target: "192.68.73.2",
				Details: &Pod{
					Name: "pod_name",
					UID:  "pod-uid",
					Labels: map[string]string{
						"label_key": "label_val",
					},
					Annotations: map[string]string{
						"annotation_1": "value_1",
					},
					Namespace: "pod-namespace",
				},
			},
			want: EndpointEnv{
				"type":     "pod",
				"endpoint": "192.68.73.2",
				"id":       "pod_id",
				"name":     "pod_name",
				"labels": map[string]string{
					"label_key": "label_val",
				},
				"annotations": map[string]string{
					"annotation_1": "value_1",
				},
				"uid":       "pod-uid",
				"namespace": "pod-namespace",
				"host":      "192.68.73.2",
			},
		},
		{
			name: "K8s pod port",
			endpoint: Endpoint{
				ID:     EndpointID("port_id"),
				Target: "192.68.73.2",
				Details: &Port{
					Name: "port_name",
					Pod: Pod{
						Name: "pod_name",
						Labels: map[string]string{
							"label_key": "label_val",
						},
						Annotations: map[string]string{
							"annotation_1": "value_1",
						},
						Namespace: "pod-namespace",
						UID:       "pod-uid",
					},
					Port:      2379,
					Transport: ProtocolTCP,
				},
			},
			want: EndpointEnv{
				"type":     "port",
				"endpoint": "192.68.73.2",
				"id":       "port_id",
				"name":     "port_name",
				"port":     uint16(2379),
				"pod": EndpointEnv{
					"name": "pod_name",
					"labels": map[string]string{
						"label_key": "label_val",
					},
					"annotations": map[string]string{
						"annotation_1": "value_1",
					},
					"uid":       "pod-uid",
					"namespace": "pod-namespace",
				},
				"transport": ProtocolTCP,
				"host":      "192.68.73.2",
			},
		},
		{
			name: "Service",
			endpoint: Endpoint{
				ID:     EndpointID("service_id"),
				Target: "service.namespace",
				Details: &K8sService{
					Name: "service_name",
					UID:  "service-uid",
					Labels: map[string]string{
						"label_key": "label_val",
					},
					Annotations: map[string]string{
						"annotation_1": "value_1",
					},
					Namespace:   "service-namespace",
					ServiceType: "LoadBalancer",
					ClusterIP:   "192.68.73.2",
				},
			},
			want: EndpointEnv{
				"type":     "k8s.service",
				"endpoint": "service.namespace",
				"id":       "service_id",
				"name":     "service_name",
				"labels": map[string]string{
					"label_key": "label_val",
				},
				"annotations": map[string]string{
					"annotation_1": "value_1",
				},
				"uid":          "service-uid",
				"namespace":    "service-namespace",
				"cluster_ip":   "192.68.73.2",
				"service_type": "LoadBalancer",
				"host":         "service.namespace",
			},
		},
		{
			name: "Host port",
			endpoint: Endpoint{
				ID:     EndpointID("port_id"),
				Target: "127.0.0.1",
				Details: &HostPort{
					ProcessName: "process_name",
					Command:     "./cmd --config config.yaml",
					Port:        2379,
					Transport:   ProtocolUDP,
					IsIPv6:      true,
				},
			},
			want: EndpointEnv{
				"type":         "hostport",
				"endpoint":     "127.0.0.1",
				"id":           "port_id",
				"process_name": "process_name",
				"command":      "./cmd --config config.yaml",
				"is_ipv6":      true,
				"port":         uint16(2379),
				"transport":    ProtocolUDP,
				"host":         "127.0.0.1",
			},
		},
		{
			name: "Container",
			endpoint: Endpoint{
				ID:     EndpointID("container_endpoint_id"),
				Target: "127.0.0.1",
				Details: &Container{
					Name:          "otel-collector",
					Image:         "otel-collector-image",
					Tag:           "1.0.0",
					Port:          2379,
					AlternatePort: 2380,
					Command:       "./cmd --config config.yaml",
					ContainerID:   "abcdefg123456",
					Host:          "127.0.0.1",
					Transport:     ProtocolTCP,
					Labels: map[string]string{
						"label_key": "label_val",
					},
				},
			},
			want: EndpointEnv{
				"type":           "container",
				"id":             "container_endpoint_id",
				"name":           "otel-collector",
				"image":          "otel-collector-image",
				"tag":            "1.0.0",
				"port":           uint16(2379),
				"alternate_port": uint16(2380),
				"command":        "./cmd --config config.yaml",
				"container_id":   "abcdefg123456",
				"host":           "127.0.0.1",
				"transport":      ProtocolTCP,
				"labels": map[string]string{
					"label_key": "label_val",
				},
				"endpoint": "127.0.0.1",
			},
		},
		{
			name: "Kubernetes Node",
			endpoint: Endpoint{
				ID:     EndpointID("k8s_node_endpoint_id"),
				Target: "127.0.0.1:1234",
				Details: &K8sNode{
					Name:        "a-k8s-node",
					UID:         "a-k8s-node-uid",
					Hostname:    "a-k8s-node-hostname",
					ExternalIP:  "1.2.3.4",
					InternalIP:  "127.0.0.1",
					ExternalDNS: "an-external-dns",
					InternalDNS: "an-internal-dns",
					Annotations: map[string]string{
						"annotation_key": "annotation_val",
					},
					Labels: map[string]string{
						"label_key": "label_val",
					},
					KubeletEndpointPort: 1234,
				},
			},
			want: EndpointEnv{
				"type":                  "k8s.node",
				"id":                    "k8s_node_endpoint_id",
				"name":                  "a-k8s-node",
				"uid":                   "a-k8s-node-uid",
				"hostname":              "a-k8s-node-hostname",
				"endpoint":              "127.0.0.1:1234",
				"external_dns":          "an-external-dns",
				"external_ip":           "1.2.3.4",
				"internal_dns":          "an-internal-dns",
				"internal_ip":           "127.0.0.1",
				"kubelet_endpoint_port": uint16(1234),
				"annotations": map[string]string{
					"annotation_key": "annotation_val",
				},
				"labels": map[string]string{
					"label_key": "label_val",
				},
				"host": "127.0.0.1",
				"port": "1234",
			},
		},
		{
			// This is an invalid test case, to ensure "port" keeps the original value and
			// isn't overwritten by a port parsed from the "Target". The two ports shouldn't mismatch
			// if they're exposed in both places.
			name: "K8s pod port - conflicting ports",
			endpoint: Endpoint{
				ID:     EndpointID("port_id"),
				Target: "192.68.73.2:4321",
				Details: &Port{
					Name: "port_name",
					Pod: Pod{
						Name: "pod_name",
						Labels: map[string]string{
							"label_key": "label_val",
						},
						Annotations: map[string]string{
							"annotation_1": "value_1",
						},
						Namespace: "pod-namespace",
						UID:       "pod-uid",
					},
					Port:      2379,
					Transport: ProtocolTCP,
				},
			},
			want: EndpointEnv{
				"type":     "port",
				"endpoint": "192.68.73.2:4321",
				"id":       "port_id",
				"name":     "port_name",
				"port":     uint16(2379),
				"pod": EndpointEnv{
					"name": "pod_name",
					"labels": map[string]string{
						"label_key": "label_val",
					},
					"annotations": map[string]string{
						"annotation_1": "value_1",
					},
					"uid":       "pod-uid",
					"namespace": "pod-namespace",
				},
				"transport": ProtocolTCP,
				"host":      "192.68.73.2",
			},
		},
		{
			name: "Kafka topic",
			endpoint: Endpoint{
				ID:      EndpointID("topic1"),
				Target:  "topic1",
				Details: &KafkaTopic{},
			},
			want: EndpointEnv{
				"id":       "topic1",
				"type":     "kafka.topics",
				"host":     "topic1",
				"endpoint": "topic1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.endpoint.Env()
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
