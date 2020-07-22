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

package observer

import (
	"reflect"
	"testing"
)

func TestEndpointToEnv(t *testing.T) {
	tests := []struct {
		name     string
		endpoint Endpoint
		want     EndpointEnv
		wantErr  bool
	}{
		{
			name: "Pod",
			endpoint: Endpoint{
				ID:     EndpointID("pod_id"),
				Target: "192.68.73.2",
				Details: Pod{
					Name: "pod_name",
					Labels: map[string]string{
						"label_key": "label_val",
					},
					Annotations: map[string]string{
						"annotation_1": "value_1",
					},
				},
			},
			want: EndpointEnv{
				"type": map[string]interface{}{
					"port": false,
					"pod":  true,
				},
				"endpoint": "192.68.73.2",
				"name":     "pod_name",
				"labels": map[string]string{
					"label_key": "label_val",
				},
				"annotations": map[string]string{
					"annotation_1": "value_1",
				},
			},
			wantErr: false,
		},
		{
			name: "K8s port",
			endpoint: Endpoint{
				ID:     EndpointID("port_id"),
				Target: "192.68.73.2",
				Details: Port{
					Name: "port_name",
					Pod: Pod{
						Name: "pod_name",
						Labels: map[string]string{
							"label_key": "label_val",
						},
						Annotations: map[string]string{
							"annotation_1": "value_1",
						},
					},
					Port:      2379,
					Transport: ProtocolTCP,
				},
			},
			want: EndpointEnv{
				"type": map[string]interface{}{
					"port": true,
					"pod":  false,
				},
				"endpoint": "192.68.73.2",
				"name":     "port_name",
				"port":     uint16(2379),
				"pod": map[string]interface{}{
					"name": "pod_name",
					"labels": map[string]string{
						"label_key": "label_val",
					},
					"annotations": map[string]string{
						"annotation_1": "value_1",
					},
				},
				"transport": ProtocolTCP,
			},
			wantErr: false,
		},
		{
			name: "Unsupported endpoint",
			endpoint: Endpoint{
				ID:      EndpointID("port_id"),
				Target:  "127.0.0.1:2379",
				Details: map[string]interface{}{},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EndpointToEnv(tt.endpoint)
			if (err != nil) != tt.wantErr {
				t.Errorf("EndpointToEnv() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EndpointToEnv() got = %v, want %v", got, tt.want)
			}
		})
	}
}
