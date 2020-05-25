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

package collection

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/testutils"
)

func TestPodAndContainerMetrics(t *testing.T) {
	pod := newPodWithContainer("1")

	actualResourceMetrics := getMetricsForPod(pod)

	require.Equal(t, 2, len(actualResourceMetrics))

	rm := actualResourceMetrics[0]

	require.Equal(t, 1, len(actualResourceMetrics[0].metrics))
	testutils.AssertResource(t, *rm.resource, k8sType,
		map[string]string{
			"k8s.pod.uid":        "test-pod-1-uid",
			"k8s.pod.name":       "test-pod-1",
			"k8s.node.name":      "test-node",
			"k8s.namespace.name": "test-namespace",
			"k8s.cluster.name":   "test-cluster",
		},
	)

	testutils.AssertMetrics(t, *rm.metrics[0], "k8s/pod/phase",
		metricspb.MetricDescriptor_GAUGE_INT64, 3)

	rm = actualResourceMetrics[1]

	require.Equal(t, 4, len(actualResourceMetrics[1].metrics))
	testutils.AssertResource(t, *rm.resource, "container",
		map[string]string{
			"container.id":         "container-id",
			"container.spec.name":  "container-name",
			"container.image.name": "container-image-name",
			"k8s.pod.uid":          "test-pod-1-uid",
			"k8s.pod.name":         "test-pod-1",
			"k8s.node.name":        "test-node",
			"k8s.namespace.name":   "test-namespace",
			"k8s.cluster.name":     "test-cluster",
		},
	)

	testutils.AssertMetrics(t, *rm.metrics[0], "k8s/container/restarts",
		metricspb.MetricDescriptor_GAUGE_INT64, 3)

	testutils.AssertMetrics(t, *rm.metrics[1], "k8s/container/ready",
		metricspb.MetricDescriptor_GAUGE_INT64, 1)

	testutils.AssertMetricsWithLabels(t, *rm.metrics[2], "k8s/container/request",
		metricspb.MetricDescriptor_GAUGE_INT64, map[string]string{"resource": "cpu"}, 10000)

	testutils.AssertMetricsWithLabels(t, *rm.metrics[3], "k8s/container/limit",
		metricspb.MetricDescriptor_GAUGE_INT64, map[string]string{"resource": "cpu"}, 20000)
}

func TestPodAndContainerMetadata(t *testing.T) {
	pod := newPodWithContainer("1")

	actualMetadata := getMetadataForPod(pod,
		&metadataStore{},
	)

	require.Equal(t, 2, len(actualMetadata))

	// Assert metadata from Pod.
	require.Equal(t,
		KubernetesMetadata{
			resourceIDKey: "k8s.pod.uid",
			resourceID:    "test-pod-1-uid",
			metadata: map[string]string{
				"pod.creation_timestamp": "0001-01-01T00:00:00Z",
				"foo":                    "bar",
				"foo1":                   "",
			},
		},
		*actualMetadata["test-pod-1-uid"],
	)

	// Assert metadata from Container.
	require.Equal(t,
		KubernetesMetadata{
			resourceIDKey: "container.id",
			resourceID:    "container-id",
			metadata: map[string]string{
				"container.status": "running",
			},
		},
		*actualMetadata["container-id"],
	)
}

func newPodWithContainer(id string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:        "test-pod-" + id,
			Namespace:   "test-namespace",
			UID:         types.UID("test-pod-" + id + "-uid"),
			ClusterName: "test-cluster",
			Labels: map[string]string{
				"foo":  "bar",
				"foo1": "",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "test-node",
			Containers: []corev1.Container{
				{
					Name:  "container-name",
					Image: "container-image-name",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: *resource.NewQuantity(20, resource.DecimalSI),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: *resource.NewQuantity(10, resource.DecimalSI),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "container-name",
					Ready:        true,
					RestartCount: 3,
					Image:        "container-image-name",
					ContainerID:  "docker://container-id",
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
		},
	}
}

func TestListResourceMetrics(t *testing.T) {
	rms := map[string]*resourceMetrics{
		"resource-1": {resource: &resourcepb.Resource{Type: "type-1"}},
		"resource-2": {resource: &resourcepb.Resource{Type: "type-2"}},
		"resource-3": {resource: &resourcepb.Resource{Type: "type-1"}},
	}

	actual := listResourceMetrics(rms)
	expected := []*resourceMetrics{
		{resource: &resourcepb.Resource{Type: "type-1"}},
		{resource: &resourcepb.Resource{Type: "type-2"}},
		{resource: &resourcepb.Resource{Type: "type-1"}},
	}

	require.ElementsMatch(t, expected, actual)
}

func TestPhaseToInt(t *testing.T) {
	tests := []struct {
		name  string
		phase corev1.PodPhase
		want  int32
	}{
		{
			name:  "Pod phase pending",
			phase: corev1.PodPending,
			want:  1,
		},
		{
			name:  "Pod phase running",
			phase: corev1.PodRunning,
			want:  2,
		},
		{
			name:  "Pod phase succeeded",
			phase: corev1.PodSucceeded,
			want:  3,
		},
		{
			name:  "Pod phase failed",
			phase: corev1.PodFailed,
			want:  4,
		},
		{
			name:  "Pod phase unknown",
			phase: corev1.PodUnknown,
			want:  5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := phaseToInt(tt.phase); got != tt.want {
				t.Errorf("phaseToInt() = %v, want %v", got, tt.want)
			}
		})
	}
}
