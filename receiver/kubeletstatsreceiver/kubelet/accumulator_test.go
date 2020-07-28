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
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

// TestContainerStatsMetadataNotFound walks through the error cases of containerStats.
// Happy paths are covered in metadata_test.go
func TestContainerStatsMetadataNotFound(t *testing.T) {
	now := metav1.Now()
	podResource := &resourcepb.Resource{
		Labels: map[string]string{
			labelPodUID:        "pod-uid-123",
			labelContainerName: "container1",
		},
	}
	containerStats := stats.ContainerStats{
		Name:      "container1",
		StartTime: now,
	}
	metadata := NewMetadata(
		[]MetadataLabel{MetadataLabelContainerID},
		&v1.PodList{
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
		},
	)

	observedLogger, logs := observer.New(zapcore.WarnLevel)
	logger := zap.New(observedLogger)

	mds := []*consumerdata.MetricsData{}
	acc := metricDataAccumulator{
		m:        mds,
		metadata: metadata,
		logger:   logger,
	}

	acc.containerStats(podResource, containerStats)

	assert.Equal(t, 0, len(mds))
	assert.Equal(t, 1, logs.Len())
	assert.Equal(t, "failed to fetch container metrics", logs.All()[0].Message)
}
