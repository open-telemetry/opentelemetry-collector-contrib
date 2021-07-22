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

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
)

type pod struct {
	uid       string
	name      string
	namespace string
}

// Tests for correctness of additional labels collected from PVCs.
func TestDetailedPVCLabels(t *testing.T) {
	tests := []struct {
		name                            string
		volumeName                      string
		volumeSource                    v1.VolumeSource
		pod                             pod
		detailedPVCLabelsSetterOverride func(volCacheID, volumeClaim, namespace string, labels map[string]string) error
		want                            map[string]string
	}{
		{
			name:       "persistentVolumeClaim - with detailed PVC labels (AWS)",
			volumeName: "volume0",
			volumeSource: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					ClaimName: "claim-name",
				},
			},
			pod: pod{uid: "uid-1234", name: "pod-name", namespace: "pod-namespace"},
			detailedPVCLabelsSetterOverride: func(volCacheID, volumeClaim, namespace string, labels map[string]string) error {
				GetPersistentVolumeLabels(v1.PersistentVolumeSource{
					AWSElasticBlockStore: &v1.AWSElasticBlockStoreVolumeSource{
						VolumeID:  "volume_id",
						FSType:    "fs_type",
						Partition: 10,
					},
				}, labels)
				return nil
			},
			want: map[string]string{
				"k8s.volume.name":                "volume0",
				"k8s.volume.type":                "awsElasticBlockStore",
				"aws.volume.id":                  "volume_id",
				"fs.type":                        "fs_type",
				"partition":                      "10",
				"k8s.persistentvolumeclaim.name": "claim-name",
				"k8s.pod.uid":                    "uid-1234",
				"k8s.pod.name":                   "pod-name",
				"k8s.namespace.name":             "pod-namespace",
			},
		},
		{
			name:       "persistentVolumeClaim - with detailed PVC labels (GCP)",
			volumeName: "volume0",
			volumeSource: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					ClaimName: "claim-name",
				},
			},
			pod: pod{uid: "uid-1234", name: "pod-name", namespace: "pod-namespace"},
			detailedPVCLabelsSetterOverride: func(volCacheID, volumeClaim, namespace string, labels map[string]string) error {
				GetPersistentVolumeLabels(v1.PersistentVolumeSource{
					GCEPersistentDisk: &v1.GCEPersistentDiskVolumeSource{
						PDName:    "pd_name",
						FSType:    "fs_type",
						Partition: 10,
					},
				}, labels)
				return nil
			},
			want: map[string]string{
				"k8s.volume.name":                "volume0",
				"k8s.volume.type":                "gcePersistentDisk",
				"gce.pd.name":                    "pd_name",
				"fs.type":                        "fs_type",
				"partition":                      "10",
				"k8s.persistentvolumeclaim.name": "claim-name",
				"k8s.pod.uid":                    "uid-1234",
				"k8s.pod.name":                   "pod-name",
				"k8s.namespace.name":             "pod-namespace",
			},
		},
		{
			name:       "persistentVolumeClaim - with detailed PVC labels (GlusterFS)",
			volumeName: "volume0",
			volumeSource: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					ClaimName: "claim-name",
				},
			},
			pod: pod{uid: "uid-1234", name: "pod-name", namespace: "pod-namespace"},
			detailedPVCLabelsSetterOverride: func(volCacheID, volumeClaim, namespace string, labels map[string]string) error {
				GetPersistentVolumeLabels(v1.PersistentVolumeSource{
					Glusterfs: &v1.GlusterfsPersistentVolumeSource{
						EndpointsName: "endpoints_name",
						Path:          "path",
					},
				}, labels)
				return nil
			},
			want: map[string]string{
				"k8s.volume.name":                "volume0",
				"k8s.volume.type":                "glusterfs",
				"glusterfs.endpoints.name":       "endpoints_name",
				"glusterfs.path":                 "path",
				"k8s.persistentvolumeclaim.name": "claim-name",
				"k8s.pod.uid":                    "uid-1234",
				"k8s.pod.name":                   "pod-name",
				"k8s.namespace.name":             "pod-namespace",
			},
		},
		{
			name:       "persistentVolumeClaim - with detailed PVC labels (local)",
			volumeName: "volume0",
			volumeSource: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					ClaimName: "claim-name",
				},
			},
			pod: pod{uid: "uid-1234", name: "pod-name", namespace: "pod-namespace"},
			detailedPVCLabelsSetterOverride: func(volCacheID, volumeClaim, namespace string, labels map[string]string) error {
				GetPersistentVolumeLabels(v1.PersistentVolumeSource{
					Local: &v1.LocalVolumeSource{
						Path: "path",
					},
				}, labels)
				return nil
			},
			want: map[string]string{
				"k8s.volume.name":                "volume0",
				"k8s.volume.type":                "local",
				"k8s.persistentvolumeclaim.name": "claim-name",
				"k8s.pod.uid":                    "uid-1234",
				"k8s.pod.name":                   "pod-name",
				"k8s.namespace.name":             "pod-namespace",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podResource := &resourcepb.Resource{
				Labels: map[string]string{
					"k8s.pod.uid":        tt.pod.uid,
					"k8s.pod.name":       tt.pod.name,
					"k8s.namespace.name": tt.pod.namespace,
				},
			}
			metadata := NewMetadata([]MetadataLabel{MetadataLabelVolumeType}, &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							UID:       types.UID(tt.pod.uid),
							Name:      tt.pod.name,
							Namespace: tt.pod.namespace,
						},
						Spec: v1.PodSpec{
							Volumes: []v1.Volume{
								{
									Name:         tt.volumeName,
									VolumeSource: tt.volumeSource,
								},
							},
						},
					},
				},
			}, nil)
			metadata.DetailedPVCLabelsSetter = tt.detailedPVCLabelsSetterOverride

			volume, err := volumeResource(podResource, stats.VolumeStats{Name: tt.volumeName}, metadata)
			require.NoError(t, err)
			require.NotNil(t, volume)
			require.Equal(t, tt.want, volume.Labels)
		})
	}
}
