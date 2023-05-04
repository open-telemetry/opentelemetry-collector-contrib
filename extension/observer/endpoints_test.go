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
			},
		},
		{
			name: "K8s port",
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

func TestEndpointEquals(t *testing.T) {
	tests := []struct {
		name     string
		first    Endpoint
		second   Endpoint
		areEqual bool
	}{
		{
			name:  "equal empty endpoints",
			first: Endpoint{}, second: Endpoint{},
			areEqual: true,
		},
		{
			name:     "equal ID",
			first:    Endpoint{ID: "id"},
			second:   Endpoint{ID: "id"},
			areEqual: true,
		},
		{
			name:     "unequal ID",
			first:    Endpoint{ID: "first"},
			second:   Endpoint{ID: "second"},
			areEqual: false,
		},
		{
			name:     "equal Target",
			first:    Endpoint{Target: "target"},
			second:   Endpoint{Target: "target"},
			areEqual: true,
		},
		{
			name:     "unequal Target",
			first:    Endpoint{Target: "first"},
			second:   Endpoint{Target: "second"},
			areEqual: false,
		},
		{
			name:     "equal empty Port",
			first:    Endpoint{Details: &Port{}},
			second:   Endpoint{Details: &Port{}},
			areEqual: true,
		},
		{
			name:     "equal Port Name",
			first:    Endpoint{Details: &Port{Name: "port_name"}},
			second:   Endpoint{Details: &Port{Name: "port_name"}},
			areEqual: true,
		},
		{
			name:     "unequal Port Name",
			first:    Endpoint{Details: &Port{Name: "first"}},
			second:   Endpoint{Details: &Port{Name: "second"}},
			areEqual: false,
		},
		{
			name:     "equal Port Port",
			first:    Endpoint{Details: &Port{Port: 2379}},
			second:   Endpoint{Details: &Port{Port: 2379}},
			areEqual: true,
		},
		{
			name:     "unequal Port Port",
			first:    Endpoint{Details: &Port{Port: 0}},
			second:   Endpoint{Details: &Port{Port: 1}},
			areEqual: false,
		},
		{
			name:     "equal Port Transport",
			first:    Endpoint{Details: &Port{Transport: "transport"}},
			second:   Endpoint{Details: &Port{Transport: "transport"}},
			areEqual: true,
		},
		{
			name:     "unequal Port Transport",
			first:    Endpoint{Details: &Port{Transport: "first"}},
			second:   Endpoint{Details: &Port{Transport: "second"}},
			areEqual: false,
		},
		{
			name: "equal Port",
			first: Endpoint{
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
			second: Endpoint{
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
			areEqual: true,
		},
		{
			name: "unequal Port Pod Label",
			first: Endpoint{
				ID:     EndpointID("port_id"),
				Target: "192.68.73.2",
				Details: &Port{
					Name: "port_name",
					Pod: Pod{
						Name: "pod_name",
						Labels: map[string]string{
							"key_one": "val_one",
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
			second: Endpoint{
				ID:     EndpointID("port_id"),
				Target: "192.68.73.2",
				Details: &Port{
					Name: "port_name",
					Pod: Pod{
						Name: "pod_name",
						Labels: map[string]string{
							"key_two": "val_two",
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
			areEqual: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.first.equals(tt.second), tt.areEqual)
			require.Equal(t, tt.second.equals(tt.first), tt.areEqual)
		})
	}
}
