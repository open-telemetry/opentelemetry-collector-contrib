// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestNodeMetricsReportCPUMetrics(t *testing.T) {
	n := testutils.NewNode("1")

	actualResourceMetrics := GetMetrics(n, []string{"Ready", "MemoryPressure"}, []string{"cpu", "memory", "ephemeral-storage", "storage"}, zap.NewNop())

	require.Equal(t, 1, len(actualResourceMetrics))

	require.Equal(t, 5, len(actualResourceMetrics[0].Metrics))
	testutils.AssertResource(t, actualResourceMetrics[0].Resource, constants.K8sType,
		map[string]string{
			"k8s.node.uid":  "test-node-1-uid",
			"k8s.node.name": "test-node-1",
		},
	)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].Metrics[0], "k8s.node.condition_ready",
		metricspb.MetricDescriptor_GAUGE_INT64, 1)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].Metrics[1], "k8s.node.condition_memory_pressure",
		metricspb.MetricDescriptor_GAUGE_INT64, 0)

	testutils.AssertMetricsDouble(t, actualResourceMetrics[0].Metrics[2], "k8s.node.allocatable_cpu",
		metricspb.MetricDescriptor_GAUGE_DOUBLE, 0.123)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].Metrics[3], "k8s.node.allocatable_memory",
		metricspb.MetricDescriptor_GAUGE_INT64, 456)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].Metrics[4], "k8s.node.allocatable_ephemeral_storage",
		metricspb.MetricDescriptor_GAUGE_INT64, 1234)
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
