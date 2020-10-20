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
	"errors"
	"testing"

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

// TestMetadataErrorCases walks through the error cases of collecting
// metadata. Happy paths are covered in metadata_test.go and volume_test.go.
func TestMetadataErrorCases(t *testing.T) {
	tests := []struct {
		name                            string
		metricGroupsToCollect           map[MetricGroup]bool
		testScenario                    func(acc metricDataAccumulator)
		metadata                        Metadata
		numMDs                          int
		numLogs                         int
		logMessages                     []string
		detailedPVCLabelsSetterOverride func(volCacheID, volumeClaim, namespace string, labels map[string]string) error
	}{
		{
			name: "Fails to get container metadata",
			metricGroupsToCollect: map[MetricGroup]bool{
				ContainerMetricGroup: true,
			},
			metadata: NewMetadata([]MetadataLabel{MetadataLabelContainerID}, &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							UID: types.UID("pod-uid-123"),
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									// different container name
									Name:        "container2",
									ContainerID: "test-container",
								},
							},
						},
					},
				},
			}, nil),
			testScenario: func(acc metricDataAccumulator) {
				now := metav1.Now()
				podResource := &resourcepb.Resource{
					Labels: map[string]string{
						"k8s.pod.uid":        "pod-uid-123",
						"k8s.container.name": "container1",
					},
				}
				containerStats := stats.ContainerStats{
					Name:      "container1",
					StartTime: now,
				}

				acc.containerStats(podResource, containerStats)
			},
			numMDs:  0,
			numLogs: 1,
			logMessages: []string{
				"failed to fetch container metrics",
			},
		},
		{
			name: "Fails to get volume metadata - no pods data",
			metricGroupsToCollect: map[MetricGroup]bool{
				VolumeMetricGroup: true,
			},
			metadata: NewMetadata([]MetadataLabel{MetadataLabelVolumeType}, nil, nil),
			testScenario: func(acc metricDataAccumulator) {
				podResource := &resourcepb.Resource{
					Labels: map[string]string{
						"k8s.pod.uid": "pod-uid-123",
					},
				}
				volumeStats := stats.VolumeStats{
					Name: "volume-1",
				}

				acc.volumeStats(podResource, volumeStats)
			},
			numMDs:  0,
			numLogs: 1,
			logMessages: []string{
				"Failed to gather additional volume metadata. Skipping metric collection.",
			},
		},
		{
			name: "Fails to get volume metadata - volume not found",
			metricGroupsToCollect: map[MetricGroup]bool{
				VolumeMetricGroup: true,
			},
			metadata: NewMetadata([]MetadataLabel{MetadataLabelVolumeType}, &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							UID: types.UID("pod-uid-123"),
						},
						Spec: v1.PodSpec{
							Volumes: []v1.Volume{
								{
									Name: "volume-0",
									VolumeSource: v1.VolumeSource{
										HostPath: &v1.HostPathVolumeSource{},
									},
								},
							},
						},
					},
				},
			}, nil),
			testScenario: func(acc metricDataAccumulator) {
				podResource := &resourcepb.Resource{
					Labels: map[string]string{
						"k8s.pod.uid": "pod-uid-123",
					},
				}
				volumeStats := stats.VolumeStats{
					Name: "volume-1",
				}

				acc.volumeStats(podResource, volumeStats)
			},
			numMDs:  0,
			numLogs: 1,
			logMessages: []string{
				"Failed to gather additional volume metadata. Skipping metric collection.",
			},
		},
		{
			name: "Fails to get volume metadata - metadata from PVC",
			metricGroupsToCollect: map[MetricGroup]bool{
				VolumeMetricGroup: true,
			},
			metadata: NewMetadata([]MetadataLabel{MetadataLabelVolumeType}, &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							UID: types.UID("pod-uid-123"),
						},
						Spec: v1.PodSpec{
							Volumes: []v1.Volume{
								{
									Name: "volume-0",
									VolumeSource: v1.VolumeSource{
										PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
											ClaimName: "claim",
										},
									},
								},
							},
						},
					},
				},
			}, nil),
			detailedPVCLabelsSetterOverride: func(volCacheID, volumeClaim, namespace string, labels map[string]string) error {
				// Mock failure cases.
				return errors.New("")
			},
			testScenario: func(acc metricDataAccumulator) {
				podResource := &resourcepb.Resource{
					Labels: map[string]string{
						"k8s.pod.uid": "pod-uid-123",
					},
				}
				volumeStats := stats.VolumeStats{
					Name: "volume-0",
				}

				acc.volumeStats(podResource, volumeStats)
			},
			numMDs:  0,
			numLogs: 1,
			logMessages: []string{
				"Failed to gather additional volume metadata. Skipping metric collection.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observedLogger, logs := observer.New(zapcore.WarnLevel)
			logger := zap.New(observedLogger)

			var mds []consumerdata.MetricsData
			tt.metadata.DetailedPVCLabelsSetter = tt.detailedPVCLabelsSetterOverride
			acc := metricDataAccumulator{
				m:                     mds,
				metadata:              tt.metadata,
				logger:                logger,
				metricGroupsToCollect: tt.metricGroupsToCollect,
			}

			tt.testScenario(acc)

			assert.Equal(t, tt.numMDs, len(mds))
			require.Equal(t, tt.numLogs, logs.Len())
			for i := 0; i < tt.numLogs; i++ {
				assert.Equal(t, tt.logMessages[i], logs.All()[i].Message)
			}
		})
	}
}

func TestNilHandling(t *testing.T) {
	acc := metricDataAccumulator{
		metricGroupsToCollect: map[MetricGroup]bool{
			PodMetricGroup:       true,
			NodeMetricGroup:      true,
			ContainerMetricGroup: true,
			VolumeMetricGroup:    true,
		},
	}
	resource := &resourcepb.Resource{}
	assert.NotPanics(t, func() {
		acc.nodeStats(stats.NodeStats{})
	})
	assert.NotPanics(t, func() {
		acc.podStats(resource, stats.PodStats{})
	})
	assert.NotPanics(t, func() {
		acc.containerStats(resource, stats.ContainerStats{})
	})
	assert.NotPanics(t, func() {
		acc.volumeStats(resource, stats.VolumeStats{})
	})
}
