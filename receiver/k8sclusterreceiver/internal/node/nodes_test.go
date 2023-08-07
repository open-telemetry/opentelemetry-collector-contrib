// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestNodeMetricsReportCPUMetrics(t *testing.T) {
	n := testutils.NewNode("1")
	m := GetMetrics(receivertest.NewNopCreateSettings(), n,
		[]string{
			"Ready",
			"MemoryPressure",
			"DiskPressure",
			"NetworkUnavailable",
			"PIDPressure",
			"OutOfDisk",
		},
		[]string{
			"cpu",
			"memory",
			"ephemeral-storage",
			"storage",
			"pods",
			"hugepages-1Gi",
			"hugepages-2Mi",
			"not-present",
		},
	)
	expected, err := golden.ReadMetrics(filepath.Join("testdata", "expected.yaml"))
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expected, m,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
	),
	)
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

func TestTransform(t *testing.T) {
	originalNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-node",
			UID:  "my-node-uid",
			Labels: map[string]string{
				"node-role": "worker",
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("16Gi"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
			Addresses: []corev1.NodeAddress{
				{
					Type:    corev1.NodeHostName,
					Address: "my-node-hostname",
				},
				{
					Type:    corev1.NodeInternalIP,
					Address: "192.168.1.100",
				},
			},
		},
	}
	wantNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-node",
			UID:  "my-node-uid",
			Labels: map[string]string{
				"node-role": "worker",
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
		},
	}
	assert.Equal(t, wantNode, Transform(originalNode))
}
