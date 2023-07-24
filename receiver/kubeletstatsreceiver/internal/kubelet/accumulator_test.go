// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
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
		detailedPVCLabelsSetterOverride func(rb *metadata.ResourceBuilder, volCacheID, volumeClaim, namespace string) error
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
							UID: "pod-uid-123",
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
				podStats := stats.PodStats{
					PodRef: stats.PodReference{
						UID: "pod-uid-123",
					},
				}
				containerStats := stats.ContainerStats{
					Name:      "container1",
					StartTime: now,
				}

				acc.containerStats(podStats, containerStats)
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
				podStats := stats.PodStats{
					PodRef: stats.PodReference{
						UID: "pod-uid-123",
					},
				}
				volumeStats := stats.VolumeStats{
					Name: "volume-1",
				}

				acc.volumeStats(podStats, volumeStats)
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
							UID: "pod-uid-123",
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
				podStats := stats.PodStats{
					PodRef: stats.PodReference{
						UID: "pod-uid-123",
					},
				}
				volumeStats := stats.VolumeStats{
					Name: "volume-1",
				}

				acc.volumeStats(podStats, volumeStats)
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
							UID: "pod-uid-123",
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
			detailedPVCLabelsSetterOverride: func(rb *metadata.ResourceBuilder, volCacheID, volumeClaim, namespace string) error {
				// Mock failure cases.
				return errors.New("")
			},
			testScenario: func(acc metricDataAccumulator) {
				podStats := stats.PodStats{
					PodRef: stats.PodReference{
						UID: "pod-uid-123",
					},
				}
				volumeStats := stats.VolumeStats{
					Name: "volume-0",
				}

				acc.volumeStats(podStats, volumeStats)
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

			tt.metadata.DetailedPVCResourceSetter = tt.detailedPVCLabelsSetterOverride
			acc := metricDataAccumulator{
				metadata:              tt.metadata,
				logger:                logger,
				metricGroupsToCollect: tt.metricGroupsToCollect,
				rb:                    metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig()),
				mbs: &metadata.MetricsBuilders{
					NodeMetricsBuilder:  metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
					PodMetricsBuilder:   metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
					OtherMetricsBuilder: metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
				},
			}

			tt.testScenario(acc)

			assert.Equal(t, tt.numMDs, len(acc.m))
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
		rb: metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig()),
		mbs: &metadata.MetricsBuilders{
			NodeMetricsBuilder:      metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
			PodMetricsBuilder:       metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
			ContainerMetricsBuilder: metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
			OtherMetricsBuilder:     metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
		},
	}
	assert.NotPanics(t, func() {
		acc.nodeStats(stats.NodeStats{})
	})
	assert.NotPanics(t, func() {
		acc.podStats(stats.PodStats{})
	})
	assert.NotPanics(t, func() {
		acc.containerStats(stats.PodStats{}, stats.ContainerStats{})
	})
	assert.NotPanics(t, func() {
		acc.volumeStats(stats.PodStats{}, stats.VolumeStats{})
	})
}
