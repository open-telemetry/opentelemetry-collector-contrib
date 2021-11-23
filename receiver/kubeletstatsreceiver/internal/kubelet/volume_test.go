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

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
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
		want                            map[string]pdata.AttributeValue
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
			want: map[string]pdata.AttributeValue{
				"k8s.volume.name":                pdata.NewAttributeValueString("volume0"),
				"k8s.volume.type":                pdata.NewAttributeValueString("awsElasticBlockStore"),
				"aws.volume.id":                  pdata.NewAttributeValueString("volume_id"),
				"fs.type":                        pdata.NewAttributeValueString("fs_type"),
				"partition":                      pdata.NewAttributeValueString("10"),
				"k8s.persistentvolumeclaim.name": pdata.NewAttributeValueString("claim-name"),
				"k8s.pod.uid":                    pdata.NewAttributeValueString("uid-1234"),
				"k8s.pod.name":                   pdata.NewAttributeValueString("pod-name"),
				"k8s.namespace.name":             pdata.NewAttributeValueString("pod-namespace"),
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
			want: map[string]pdata.AttributeValue{
				"k8s.volume.name":                pdata.NewAttributeValueString("volume0"),
				"k8s.volume.type":                pdata.NewAttributeValueString("gcePersistentDisk"),
				"gce.pd.name":                    pdata.NewAttributeValueString("pd_name"),
				"fs.type":                        pdata.NewAttributeValueString("fs_type"),
				"partition":                      pdata.NewAttributeValueString("10"),
				"k8s.persistentvolumeclaim.name": pdata.NewAttributeValueString("claim-name"),
				"k8s.pod.uid":                    pdata.NewAttributeValueString("uid-1234"),
				"k8s.pod.name":                   pdata.NewAttributeValueString("pod-name"),
				"k8s.namespace.name":             pdata.NewAttributeValueString("pod-namespace"),
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
			want: map[string]pdata.AttributeValue{
				"k8s.volume.name":                pdata.NewAttributeValueString("volume0"),
				"k8s.volume.type":                pdata.NewAttributeValueString("glusterfs"),
				"glusterfs.endpoints.name":       pdata.NewAttributeValueString("endpoints_name"),
				"glusterfs.path":                 pdata.NewAttributeValueString("path"),
				"k8s.persistentvolumeclaim.name": pdata.NewAttributeValueString("claim-name"),
				"k8s.pod.uid":                    pdata.NewAttributeValueString("uid-1234"),
				"k8s.pod.name":                   pdata.NewAttributeValueString("pod-name"),
				"k8s.namespace.name":             pdata.NewAttributeValueString("pod-namespace"),
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
			want: map[string]pdata.AttributeValue{
				"k8s.volume.name":                pdata.NewAttributeValueString("volume0"),
				"k8s.volume.type":                pdata.NewAttributeValueString("local"),
				"k8s.persistentvolumeclaim.name": pdata.NewAttributeValueString("claim-name"),
				"k8s.pod.uid":                    pdata.NewAttributeValueString("uid-1234"),
				"k8s.pod.name":                   pdata.NewAttributeValueString("pod-name"),
				"k8s.namespace.name":             pdata.NewAttributeValueString("pod-namespace"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podStats := stats.PodStats{
				PodRef: stats.PodReference{
					UID:       tt.pod.uid,
					Name:      tt.pod.name,
					Namespace: tt.pod.namespace,
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

			volumeResource := pdata.NewResource()
			err := fillVolumeResource(volumeResource, podStats, stats.VolumeStats{Name: tt.volumeName}, metadata)
			require.NoError(t, err)
			require.Equal(t, pdata.NewAttributeMapFromMap(tt.want).Sort(), volumeResource.Attributes().Sort())
		})
	}
}
