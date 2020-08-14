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

package kubelet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestValidateMetadataLabelsConfig(t *testing.T) {

	tests := []struct {
		name      string
		labels    []MetadataLabel
		wantError string
	}{
		{
			name:      "no_labels",
			labels:    []MetadataLabel{},
			wantError: "",
		},
		{
			name:      "container_id_valid",
			labels:    []MetadataLabel{MetadataLabelContainerID},
			wantError: "",
		},
		{
			name:      "container_id_duplicate",
			labels:    []MetadataLabel{MetadataLabelContainerID, MetadataLabelContainerID},
			wantError: "duplicate metadata label: \"container.id\"",
		},
		{
			name:      "unknown_label",
			labels:    []MetadataLabel{MetadataLabel("wrong-label")},
			wantError: "label \"wrong-label\" is not supported",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMetadataLabelsConfig(tt.labels)
			if tt.wantError == "" {
				require.NoError(t, err)
			} else {
				assert.Equal(t, tt.wantError, err.Error())
			}
		})
	}
}

func TestSetExtraLabels(t *testing.T) {

	tests := []struct {
		name          string
		metadata      Metadata
		podUID        string
		containerName string
		wantError     string
		want          map[string]string
	}{
		{
			name:          "no_labels",
			metadata:      NewMetadata([]MetadataLabel{}, nil),
			podUID:        "uid",
			containerName: "container",
			want:          map[string]string{},
		},
		{
			name: "set_container_id_valid",
			metadata: NewMetadata(
				[]MetadataLabel{MetadataLabelContainerID},
				&v1.PodList{
					Items: []v1.Pod{
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: types.UID("uid-1234"),
							},
							Status: v1.PodStatus{
								ContainerStatuses: []v1.ContainerStatus{
									{
										Name:        "container1",
										ContainerID: "test-container",
									},
								},
							},
						},
					},
				},
			),
			podUID:        "uid-1234",
			containerName: "container1",
			want: map[string]string{
				string(MetadataLabelContainerID): "test-container",
			},
		},
		{
			name:          "set_container_id_no_metadata",
			metadata:      NewMetadata([]MetadataLabel{MetadataLabelContainerID}, nil),
			podUID:        "uid-1234",
			containerName: "container1",
			wantError:     "pods metadata were not fetched",
		},
		{
			name: "set_container_id_not_found",
			metadata: NewMetadata(
				[]MetadataLabel{MetadataLabelContainerID},
				&v1.PodList{
					Items: []v1.Pod{
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: types.UID("uid-1234"),
							},
							Status: v1.PodStatus{
								ContainerStatuses: []v1.ContainerStatus{
									{
										Name:        "container2",
										ContainerID: "another-container",
									},
								},
							},
						},
					},
				},
			),
			podUID:        "uid-1234",
			containerName: "container1",
			wantError:     "pod \"uid-1234\" with container \"container1\" not found in the fetched metadata",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := map[string]string{}
			err := tt.metadata.setExtraLabels(fields, tt.podUID, tt.containerName)
			if tt.wantError == "" {
				require.NoError(t, err)
				assert.EqualValues(t, tt.want, fields)
			} else {
				assert.Equal(t, tt.wantError, err.Error())
			}
		})
	}
}
