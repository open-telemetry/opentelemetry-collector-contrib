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

package kubelet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
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
			name:      "volume_type_valid",
			labels:    []MetadataLabel{MetadataLabelVolumeType},
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
		name      string
		metadata  Metadata
		args      []string
		wantError string
		want      map[string]interface{}
	}{
		{
			name:     "no_labels",
			metadata: NewMetadata([]MetadataLabel{}, nil, nil),
			args:     []string{"uid", "container.id", "container"},
			want:     map[string]interface{}{},
		},
		{
			name: "set_container_id_valid",
			metadata: NewMetadata([]MetadataLabel{MetadataLabelContainerID}, &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							UID: "uid-1234",
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									Name:        "container1",
									ContainerID: "test-container",
								},
							},
							InitContainerStatuses: []v1.ContainerStatus{
								{
									Name:        "init-container1",
									ContainerID: "test-init-container",
								},
							},
						},
					},
				},
			}, nil),
			args: []string{"uid-1234", "container.id", "container1"},
			want: map[string]interface{}{
				string(MetadataLabelContainerID): "test-container",
			},
		},
		{
			name: "set_init_container_id_valid",
			metadata: NewMetadata([]MetadataLabel{MetadataLabelContainerID}, &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							UID: "uid-1234",
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									Name:        "container1",
									ContainerID: "test-container",
								},
							},
							InitContainerStatuses: []v1.ContainerStatus{
								{
									Name:        "init-container1",
									ContainerID: "test-init-container",
								},
							},
						},
					},
				},
			}, nil),
			args: []string{"uid-1234", "container.id", "init-container1"},
			want: map[string]interface{}{
				string(MetadataLabelContainerID): "test-init-container",
			},
		},
		{
			name:      "set_container_id_no_metadata",
			metadata:  NewMetadata([]MetadataLabel{MetadataLabelContainerID}, nil, nil),
			args:      []string{"uid-1234", "container.id", "container1"},
			wantError: "pods metadata were not fetched",
		},
		{
			name: "set_container_id_not_found",
			metadata: NewMetadata([]MetadataLabel{MetadataLabelContainerID}, &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							UID: "uid-1234",
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
			}, nil),
			args:      []string{"uid-1234", "container.id", "container1"},
			wantError: "pod \"uid-1234\" with container \"container1\" not found in the fetched metadata",
		},
		{
			name: "set_container_id_is_empty",
			metadata: NewMetadata([]MetadataLabel{MetadataLabelContainerID}, &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							UID: "uid-1234",
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									Name:        "container1",
									ContainerID: "",
								},
							},
						},
					},
				},
			}, nil),
			args:      []string{"uid-1234", "container.id", "container1"},
			wantError: "pod \"uid-1234\" with container \"container1\" has an empty containerID",
		},
		{
			name:      "set_volume_type_no_metadata",
			metadata:  NewMetadata([]MetadataLabel{MetadataLabelVolumeType}, nil, nil),
			args:      []string{"uid-1234", "k8s.volume.type", "volume0"},
			wantError: "pods metadata were not fetched",
		},
		{
			name: "set_volume_type_not_found",
			metadata: NewMetadata([]MetadataLabel{MetadataLabelVolumeType}, &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							UID: "uid-1234",
						},
						Spec: v1.PodSpec{
							Volumes: []v1.Volume{
								{
									Name:         "volume0",
									VolumeSource: v1.VolumeSource{},
								},
							},
						},
					},
				},
			}, nil),
			args:      []string{"uid-1234", "k8s.volume.type", "volume1"},
			wantError: "pod \"uid-1234\" with volume \"volume1\" not found in the fetched metadata",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ro, err := tt.metadata.getExtraResources(stats.PodReference{UID: tt.args[0]}, MetadataLabel(tt.args[1]), tt.args[2])

			r := pmetric.NewResourceMetrics()
			rac := metadata.DefaultResourceAttributesConfig()
			for _, op := range ro {
				op(rac, r)
			}

			if tt.wantError == "" {
				require.NoError(t, err)
				temp := r.Resource().Attributes().AsRaw()
				assert.EqualValues(t, tt.want, temp)
			} else {
				assert.Equal(t, tt.wantError, err.Error())
			}
		})
	}
}

// Test happy paths for volume type metadata.
func TestSetExtraLabelsForVolumeTypes(t *testing.T) {
	tests := []struct {
		name string
		vs   v1.VolumeSource
		args []string
		want map[string]interface{}
	}{
		{
			name: "hostPath",
			vs: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{},
			},
			args: []string{"uid-1234", "k8s.volume.type"},
			want: map[string]interface{}{
				"k8s.volume.type": "hostPath",
			},
		},
		{
			name: "configMap",
			vs: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{},
			},
			args: []string{"uid-1234", "k8s.volume.type"},
			want: map[string]interface{}{
				"k8s.volume.type": "configMap",
			},
		},
		{
			name: "emptyDir",
			vs: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
			args: []string{"uid-1234", "k8s.volume.type"},
			want: map[string]interface{}{
				"k8s.volume.type": "emptyDir",
			},
		},
		{
			name: "secret",
			vs: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{},
			},
			args: []string{"uid-1234", "k8s.volume.type"},
			want: map[string]interface{}{
				"k8s.volume.type": "secret",
			},
		},
		{
			name: "downwardAPI",
			vs: v1.VolumeSource{
				DownwardAPI: &v1.DownwardAPIVolumeSource{},
			},
			args: []string{"uid-1234", "k8s.volume.type"},
			want: map[string]interface{}{
				"k8s.volume.type": "downwardAPI",
			},
		},
		{
			name: "persistentVolumeClaim",
			vs: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					ClaimName: "claim-name",
				},
			},
			args: []string{"uid-1234", "k8s.volume.type"},
			want: map[string]interface{}{
				"k8s.volume.type":                "persistentVolumeClaim",
				"k8s.persistentvolumeclaim.name": "claim-name",
			},
		},
		{
			name: "awsElasticBlockStore",
			vs: v1.VolumeSource{
				AWSElasticBlockStore: &v1.AWSElasticBlockStoreVolumeSource{
					VolumeID:  "volume_id",
					FSType:    "fs_type",
					Partition: 10,
				},
			},
			args: []string{"uid-1234", "k8s.volume.type"},
			want: map[string]interface{}{
				"k8s.volume.type": "awsElasticBlockStore",
				"aws.volume.id":   "volume_id",
				"fs.type":         "fs_type",
				"partition":       "10",
			},
		},
		{
			name: "gcePersistentDisk",
			vs: v1.VolumeSource{
				GCEPersistentDisk: &v1.GCEPersistentDiskVolumeSource{
					PDName:    "pd_name",
					FSType:    "fs_type",
					Partition: 10,
				},
			},
			args: []string{"uid-1234", "k8s.volume.type"},
			want: map[string]interface{}{
				"k8s.volume.type": "gcePersistentDisk",
				"gce.pd.name":     "pd_name",
				"fs.type":         "fs_type",
				"partition":       "10",
			},
		},
		{
			name: "glusterfs",
			vs: v1.VolumeSource{
				Glusterfs: &v1.GlusterfsVolumeSource{
					EndpointsName: "endspoints_name",
					Path:          "path",
				},
			},
			args: []string{"uid-1234", "k8s.volume.type"},
			want: map[string]interface{}{
				"k8s.volume.type":          "glusterfs",
				"glusterfs.endpoints.name": "endspoints_name",
				"glusterfs.path":           "path",
			},
		},
		{
			name: "unsupported type",
			vs:   v1.VolumeSource{},
			args: []string{"uid-1234", "k8s.volume.type"},
			want: map[string]interface{}{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			volName := "volume0"
			rac := metadata.DefaultResourceAttributesConfig()
			metadata := NewMetadata([]MetadataLabel{MetadataLabelVolumeType}, &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							UID: "uid-1234",
						},
						Spec: v1.PodSpec{
							Volumes: []v1.Volume{
								{
									Name:         volName,
									VolumeSource: tt.vs,
								},
							},
						},
					},
				},
			}, func(volCacheID, volumeClaim, namespace string) ([]metadata.ResourceMetricsOption, error) {
				return nil, nil
			})
			ro, _ := metadata.getExtraResources(stats.PodReference{UID: tt.args[0]}, MetadataLabel(tt.args[1]), volName)

			rm := pmetric.NewResourceMetrics()
			for _, op := range ro {
				op(rac, rm)
			}

			assert.Equal(t, tt.want, rm.Resource().Attributes().AsRaw())
		})
	}
}
