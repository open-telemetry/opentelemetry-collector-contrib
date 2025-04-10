// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
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
		want      map[string]any
	}{
		{
			name:     "no_labels",
			metadata: NewMetadata([]MetadataLabel{}, nil, NodeInfo{}, nil),
			args:     []string{"uid", "container.id", "container"},
			want:     map[string]any{},
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
			}, NodeInfo{}, nil),
			args: []string{"uid-1234", "container.id", "container1"},
			want: map[string]any{
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
			}, NodeInfo{}, nil),
			args: []string{"uid-1234", "container.id", "init-container1"},
			want: map[string]any{
				string(MetadataLabelContainerID): "test-init-container",
			},
		},
		{
			name:      "set_container_id_no_metadata",
			metadata:  NewMetadata([]MetadataLabel{MetadataLabelContainerID}, nil, NodeInfo{}, nil),
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
			}, NodeInfo{}, nil),
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
			}, NodeInfo{}, nil),
			args:      []string{"uid-1234", "container.id", "container1"},
			wantError: "pod \"uid-1234\" with container \"container1\" has an empty containerID",
		},
		{
			name:      "set_volume_type_no_metadata",
			metadata:  NewMetadata([]MetadataLabel{MetadataLabelVolumeType}, nil, NodeInfo{}, nil),
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
			}, NodeInfo{}, nil),
			args:      []string{"uid-1234", "k8s.volume.type", "volume1"},
			wantError: "pod \"uid-1234\" with volume \"volume1\" not found in the fetched metadata",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig())
			err := tt.metadata.setExtraResources(rb, stats.PodReference{UID: tt.args[0]}, MetadataLabel(tt.args[1]), tt.args[2])
			res := rb.Emit()

			if tt.wantError == "" {
				require.NoError(t, err)
				temp := res.Attributes().AsRaw()
				assert.Equal(t, tt.want, temp)
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
		want map[string]any
	}{
		{
			name: "hostPath",
			vs: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{},
			},
			args: []string{"uid-1234", "k8s.volume.type"},
			want: map[string]any{
				"k8s.volume.type": "hostPath",
			},
		},
		{
			name: "configMap",
			vs: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{},
			},
			args: []string{"uid-1234", "k8s.volume.type"},
			want: map[string]any{
				"k8s.volume.type": "configMap",
			},
		},
		{
			name: "emptyDir",
			vs: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
			args: []string{"uid-1234", "k8s.volume.type"},
			want: map[string]any{
				"k8s.volume.type": "emptyDir",
			},
		},
		{
			name: "secret",
			vs: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{},
			},
			args: []string{"uid-1234", "k8s.volume.type"},
			want: map[string]any{
				"k8s.volume.type": "secret",
			},
		},
		{
			name: "downwardAPI",
			vs: v1.VolumeSource{
				DownwardAPI: &v1.DownwardAPIVolumeSource{},
			},
			args: []string{"uid-1234", "k8s.volume.type"},
			want: map[string]any{
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
			want: map[string]any{
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
			want: map[string]any{
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
			want: map[string]any{
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
					EndpointsName: "endpoints_name",
					Path:          "path",
				},
			},
			args: []string{"uid-1234", "k8s.volume.type"},
			want: map[string]any{
				"k8s.volume.type":          "glusterfs",
				"glusterfs.endpoints.name": "endpoints_name",
				"glusterfs.path":           "path",
			},
		},
		{
			name: "unsupported type",
			vs:   v1.VolumeSource{},
			args: []string{"uid-1234", "k8s.volume.type"},
			want: map[string]any{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			volName := "volume0"
			md := NewMetadata([]MetadataLabel{MetadataLabelVolumeType}, &v1.PodList{
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
			}, NodeInfo{}, func(*metadata.ResourceBuilder, string, string, string) error {
				return nil
			})
			rb := metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig())
			err := md.setExtraResources(rb, stats.PodReference{UID: tt.args[0]}, MetadataLabel(tt.args[1]), volName)
			require.NoError(t, err)

			assert.Equal(t, tt.want, rb.Emit().Attributes().AsRaw())
		})
	}
}

// Test happy paths for volume type metadata.
func TestCpuAndMemoryGetters(t *testing.T) {
	tests := []struct {
		name                       string
		metadata                   Metadata
		podUID                     string
		containerName              string
		wantPodCPULimit            float64
		wantPodCPURequest          float64
		wantContainerCPULimit      float64
		wantContainerCPURequest    float64
		wantPodMemoryLimit         int64
		wantPodMemoryRequest       int64
		wantContainerMemoryLimit   int64
		wantContainerMemoryRequest int64
	}{
		{
			name:     "no metadata",
			metadata: NewMetadata([]MetadataLabel{}, nil, NodeInfo{}, nil),
		},
		{
			name: "pod happy path",
			metadata: NewMetadata([]MetadataLabel{}, &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							UID: "uid-1234",
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name: "container-1",
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											v1.ResourceCPU:    k8sresource.MustParse("100m"),
											v1.ResourceMemory: k8sresource.MustParse("1G"),
										},
										Limits: v1.ResourceList{
											v1.ResourceCPU:    k8sresource.MustParse("100m"),
											v1.ResourceMemory: k8sresource.MustParse("1G"),
										},
									},
								},
								{
									Name: "container-2",
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											v1.ResourceCPU:    k8sresource.MustParse("2"),
											v1.ResourceMemory: k8sresource.MustParse("3G"),
										},
										Limits: v1.ResourceList{
											v1.ResourceCPU:    k8sresource.MustParse("2"),
											v1.ResourceMemory: k8sresource.MustParse("3G"),
										},
									},
								},
							},
						},
					},
				},
			}, NodeInfo{}, nil),
			podUID:                     "uid-1234",
			containerName:              "container-2",
			wantPodCPULimit:            2.1,
			wantPodCPURequest:          2.1,
			wantContainerCPULimit:      2,
			wantContainerCPURequest:    2,
			wantPodMemoryLimit:         4000000000,
			wantPodMemoryRequest:       4000000000,
			wantContainerMemoryLimit:   3000000000,
			wantContainerMemoryRequest: 3000000000,
		},
		{
			name: "unknown pod",
			metadata: NewMetadata([]MetadataLabel{}, &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							UID: "uid-1234",
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name: "container-1",
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											v1.ResourceCPU:    k8sresource.MustParse("1"),
											v1.ResourceMemory: k8sresource.MustParse("1G"),
										},
										Limits: v1.ResourceList{
											v1.ResourceCPU:    k8sresource.MustParse("1"),
											v1.ResourceMemory: k8sresource.MustParse("1G"),
										},
									},
								},
								{
									Name: "container-2",
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											v1.ResourceCPU:    k8sresource.MustParse("2"),
											v1.ResourceMemory: k8sresource.MustParse("3G"),
										},
										Limits: v1.ResourceList{
											v1.ResourceCPU:    k8sresource.MustParse("2"),
											v1.ResourceMemory: k8sresource.MustParse("3G"),
										},
									},
								},
							},
						},
					},
				},
			}, NodeInfo{}, nil),
			podUID: "uid-12345",
		},
		{
			name: "unknown container",
			metadata: NewMetadata([]MetadataLabel{}, &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							UID: "uid-1234",
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name: "container-1",
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											v1.ResourceCPU:    k8sresource.MustParse("300m"),
											v1.ResourceMemory: k8sresource.MustParse("1G"),
										},
										Limits: v1.ResourceList{
											v1.ResourceCPU:    k8sresource.MustParse("300m"),
											v1.ResourceMemory: k8sresource.MustParse("1G"),
										},
									},
								},
								{
									Name: "container-2",
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											v1.ResourceCPU:    k8sresource.MustParse("400m"),
											v1.ResourceMemory: k8sresource.MustParse("3G"),
										},
										Limits: v1.ResourceList{
											v1.ResourceCPU:    k8sresource.MustParse("400m"),
											v1.ResourceMemory: k8sresource.MustParse("3G"),
										},
									},
								},
							},
						},
					},
				},
			}, NodeInfo{}, nil),
			podUID:               "uid-1234",
			containerName:        "container-3",
			wantPodCPULimit:      0.7,
			wantPodCPURequest:    0.7,
			wantPodMemoryLimit:   4000000000,
			wantPodMemoryRequest: 4000000000,
		},
		{
			name: "container limit not set",
			metadata: NewMetadata([]MetadataLabel{}, &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							UID: "uid-1234",
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name: "container-1",
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											v1.ResourceCPU:    k8sresource.MustParse("1"),
											v1.ResourceMemory: k8sresource.MustParse("1G"),
										},
									},
								},
								{
									Name: "container-2",
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											v1.ResourceCPU:    k8sresource.MustParse("1"),
											v1.ResourceMemory: k8sresource.MustParse("1G"),
										},
									},
								},
							},
						},
					},
				},
			}, NodeInfo{}, nil),
			podUID:                     "uid-1234",
			containerName:              "container-2",
			wantPodCPURequest:          2,
			wantContainerCPURequest:    1,
			wantPodMemoryRequest:       2000000000,
			wantContainerMemoryRequest: 1000000000,
		},
		{
			name: "container request not set",
			metadata: NewMetadata([]MetadataLabel{}, &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							UID: "uid-1234",
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name: "container-1",
									Resources: v1.ResourceRequirements{
										Limits: v1.ResourceList{
											v1.ResourceCPU:    k8sresource.MustParse("1"),
											v1.ResourceMemory: k8sresource.MustParse("1G"),
										},
									},
								},
								{
									Name: "container-2",
									Resources: v1.ResourceRequirements{
										Limits: v1.ResourceList{
											v1.ResourceCPU:    k8sresource.MustParse("1"),
											v1.ResourceMemory: k8sresource.MustParse("1G"),
										},
									},
								},
							},
						},
					},
				},
			}, NodeInfo{}, nil),
			podUID:                   "uid-1234",
			containerName:            "container-2",
			wantPodCPULimit:          2,
			wantContainerCPULimit:    1,
			wantPodMemoryLimit:       2000000000,
			wantContainerMemoryLimit: 1000000000,
		},
		{
			name: "container limit not set but other is",
			metadata: NewMetadata([]MetadataLabel{}, &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							UID: "uid-1234",
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name: "container-1",
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											v1.ResourceCPU:    k8sresource.MustParse("1"),
											v1.ResourceMemory: k8sresource.MustParse("1G"),
										},
										Limits: v1.ResourceList{
											v1.ResourceCPU:    k8sresource.MustParse("1"),
											v1.ResourceMemory: k8sresource.MustParse("1G"),
										},
									},
								},
								{
									Name: "container-2",
								},
							},
						},
					},
				},
			}, NodeInfo{}, nil),
			podUID:                     "uid-1234",
			containerName:              "container-1",
			wantContainerCPULimit:      1,
			wantContainerCPURequest:    1,
			wantContainerMemoryLimit:   1000000000,
			wantContainerMemoryRequest: 1000000000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantPodCPULimit, tt.metadata.podResources[tt.podUID].cpuLimit)
			require.Equal(t, tt.wantPodCPURequest, tt.metadata.podResources[tt.podUID].cpuRequest)
			require.Equal(t, tt.wantContainerCPULimit, tt.metadata.containerResources[tt.podUID+tt.containerName].cpuLimit)
			require.Equal(t, tt.wantContainerCPURequest, tt.metadata.containerResources[tt.podUID+tt.containerName].cpuRequest)
			require.Equal(t, tt.wantPodMemoryLimit, tt.metadata.podResources[tt.podUID].memoryLimit)
			require.Equal(t, tt.wantPodMemoryRequest, tt.metadata.podResources[tt.podUID].memoryRequest)
			require.Equal(t, tt.wantContainerMemoryLimit, tt.metadata.containerResources[tt.podUID+tt.containerName].memoryLimit)
			require.Equal(t, tt.wantContainerMemoryRequest, tt.metadata.containerResources[tt.podUID+tt.containerName].memoryRequest)
		})
	}
}
