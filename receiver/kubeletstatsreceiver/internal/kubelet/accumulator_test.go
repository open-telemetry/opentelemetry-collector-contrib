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
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name: "container2",
								},
							},
						},
					},
				},
			}, NodeInfo{}, nil),
			testScenario: func(acc metricDataAccumulator) {
				now := metav1.Now()
				podStats := &stats.PodStats{
					PodRef: stats.PodReference{
						UID: "pod-uid-123",
					},
				}
				containerStats := &stats.ContainerStats{
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
			metadata: NewMetadata([]MetadataLabel{MetadataLabelVolumeType}, nil, NodeInfo{}, nil),
			testScenario: func(acc metricDataAccumulator) {
				podStats := &stats.PodStats{
					PodRef: stats.PodReference{
						UID: "pod-uid-123",
					},
				}
				volumeStats := &stats.VolumeStats{
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
			}, NodeInfo{}, nil),
			testScenario: func(acc metricDataAccumulator) {
				podStats := &stats.PodStats{
					PodRef: stats.PodReference{
						UID: "pod-uid-123",
					},
				}
				volumeStats := &stats.VolumeStats{
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
			}, NodeInfo{}, nil),
			detailedPVCLabelsSetterOverride: func(*metadata.ResourceBuilder, string, string, string) error {
				// Mock failure cases.
				return errors.New("")
			},
			testScenario: func(acc metricDataAccumulator) {
				podStats := &stats.PodStats{
					PodRef: stats.PodReference{
						UID: "pod-uid-123",
					},
				}
				volumeStats := &stats.VolumeStats{
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
				mbs: &metadata.MetricsBuilders{
					NodeMetricsBuilder:      metadata.NewMetricsBuilder(metadata.NewDefaultMetricsBuilderConfig(), receivertest.NewNopSettings(metadata.Type)),
					PodMetricsBuilder:       metadata.NewMetricsBuilder(metadata.NewDefaultMetricsBuilderConfig(), receivertest.NewNopSettings(metadata.Type)),
					ContainerMetricsBuilder: metadata.NewMetricsBuilder(metadata.NewDefaultMetricsBuilderConfig(), receivertest.NewNopSettings(metadata.Type)),
					OtherMetricsBuilder:     metadata.NewMetricsBuilder(metadata.NewDefaultMetricsBuilderConfig(), receivertest.NewNopSettings(metadata.Type)),
				},
			}

			tt.testScenario(acc)

			assert.Len(t, acc.m, tt.numMDs)
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
		mbs: &metadata.MetricsBuilders{
			NodeMetricsBuilder:      metadata.NewMetricsBuilder(metadata.NewDefaultMetricsBuilderConfig(), receivertest.NewNopSettings(metadata.Type)),
			PodMetricsBuilder:       metadata.NewMetricsBuilder(metadata.NewDefaultMetricsBuilderConfig(), receivertest.NewNopSettings(metadata.Type)),
			ContainerMetricsBuilder: metadata.NewMetricsBuilder(metadata.NewDefaultMetricsBuilderConfig(), receivertest.NewNopSettings(metadata.Type)),
			OtherMetricsBuilder:     metadata.NewMetricsBuilder(metadata.NewDefaultMetricsBuilderConfig(), receivertest.NewNopSettings(metadata.Type)),
		},
	}
	assert.NotPanics(t, func() {
		acc.nodeStats(stats.NodeStats{})
	})
	assert.NotPanics(t, func() {
		acc.podStats(&stats.PodStats{})
	})
	assert.NotPanics(t, func() {
		acc.containerStats(&stats.PodStats{}, &stats.ContainerStats{})
	})
	assert.NotPanics(t, func() {
		acc.volumeStats(&stats.PodStats{}, &stats.VolumeStats{})
	})
	assert.NotPanics(t, func() {
		acc.pvcStats(&stats.VolumeStats{PVCRef: &stats.PVCReference{}})
	})
}

func TestPVCStatsUIDLookupFails(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	mbc := metadata.NewDefaultMetricsBuilderConfig()
	mbc.Metrics.K8sPersistentvolumeclaimVolumeUsage.Enabled = true
	mbc.ResourceAttributes.K8sPersistentvolumeclaimUID.Enabled = true
	mbs := &metadata.MetricsBuilders{
		OtherMetricsBuilder: metadata.NewMetricsBuilder(mbc, receivertest.NewNopSettings(metadata.Type)),
	}

	lookups := 0
	md := Metadata{
		PVCUIDGetter: func(string, string) (string, error) {
			lookups++
			return "", errors.New(`persistentvolumeclaims "data" is forbidden`)
		},
	}

	usedBytes := uint64(42)
	vs := &stats.VolumeStats{
		Name:    "data",
		PVCRef:  &stats.PVCReference{Name: "data", Namespace: "default"},
		FsStats: stats.FsStats{UsedBytes: &usedBytes},
	}

	// Each scrape uses a fresh accumulator, as MetricsData does.
	for scrape := 1; scrape <= 2; scrape++ {
		acc := metricDataAccumulator{
			metadata:              md,
			logger:                zap.New(core),
			metricGroupsToCollect: map[MetricGroup]bool{VolumeMetricGroup: true},
			nodeName:              "worker-1",
			mbs:                   mbs,
		}

		// Same PVC seen from two pods, plus a volume without a PVC.
		acc.pvcStats(vs)
		acc.pvcStats(vs)
		acc.pvcStats(&stats.VolumeStats{Name: "cache", FsStats: stats.FsStats{UsedBytes: &usedBytes}})

		require.Len(t, acc.m, 1)
		// One lookup per PVC per scrape: deduplicated within a scrape, not cached across scrapes.
		assert.Equal(t, scrape, lookups)
		attrs := acc.m[0].ResourceMetrics().At(0).Resource().Attributes()
		_, hasUID := attrs.Get("k8s.persistentvolumeclaim.uid")
		assert.False(t, hasUID)
		pvcName, _ := attrs.Get("k8s.persistentvolumeclaim.name")
		assert.Equal(t, "data", pvcName.Str())
		assert.Equal(t, 1, acc.m[0].DataPointCount())
	}

	require.Equal(t, 2, logs.Len())
	for _, l := range logs.All() {
		assert.Equal(t, zapcore.DebugLevel, l.Level)
		assert.Equal(t, "Failed to look up PersistentVolumeClaim UID.", l.Message)
	}
}
