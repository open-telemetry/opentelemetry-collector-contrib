// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
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
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestNodeMetrics(t *testing.T) {
	n := newNode("1")

	actualResourceMetrics := getMetricsForNode(n, []string{"Ready", "MemoryPressure"})

	require.Equal(t, 1, len(actualResourceMetrics))

	require.Equal(t, 2, len(actualResourceMetrics[0].metrics))
	testutils.AssertResource(t, actualResourceMetrics[0].resource, k8sType,
		map[string]string{
			"k8s.node.uid":     "test-node-1-uid",
			"k8s.node.name":    "test-node-1",
			"k8s.cluster.name": "test-cluster",
		},
	)

	testutils.AssertMetrics(t, actualResourceMetrics[0].metrics[0], "k8s.node.condition_ready",
		metricspb.MetricDescriptor_GAUGE_INT64, 1)

	testutils.AssertMetrics(t, actualResourceMetrics[0].metrics[1], "k8s.node.condition_memory_pressure",
		metricspb.MetricDescriptor_GAUGE_INT64, 0)
}

func newNode(id string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: v1.ObjectMeta{
			Name:        "test-node-" + id,
			UID:         types.UID("test-node-" + id + "-uid"),
			ClusterName: "test-cluster",
			Labels: map[string]string{
				"foo":  "bar",
				"foo1": "",
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
				{
					Status: corev1.ConditionFalse,
					Type:   corev1.NodeMemoryPressure,
				},
			},
		},
	}
}

func TestGetNodeConditionMetric(t *testing.T) {
	tests := []struct {
		name                   string
		nodeConditionTypeValue string
		want                   string
	}{
		{"Metric for Node condition Ready",
			"Ready",
			"k8s.node.condition_ready",
		},
		{"Metric for Node condition MemoryPressure",
			"MemoryPressure",
			"k8s.node.condition_memory_pressure",
		},
		{"Metric for Node condition DiskPressure",
			"DiskPressure",
			"k8s.node.condition_disk_pressure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getNodeConditionMetric(tt.nodeConditionTypeValue); got != tt.want {
				t.Errorf("getNodeConditionMetric() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeConditionValue(t *testing.T) {
	type args struct {
		node     *corev1.Node
		condType corev1.NodeConditionType
	}
	tests := []struct {
		name string
		args args
		want int64
	}{
		{
			name: "Node with Ready condition true",
			args: args{
				node: &corev1.Node{Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionTrue,
						},
					},
				}},
				condType: corev1.NodeReady,
			},
			want: 1,
		},
		{
			name: "Node with Ready condition false",
			args: args{
				node: &corev1.Node{Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionFalse,
						},
					},
				}},
				condType: corev1.NodeReady,
			},
			want: 0,
		},
		{
			name: "Node with Ready condition unknown",
			args: args{
				node: &corev1.Node{Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionUnknown,
						},
					},
				}},
				condType: corev1.NodeReady,
			},
			want: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nodeConditionValue(tt.args.node, tt.args.condType); got != tt.want {
				t.Errorf("nodeConditionValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
