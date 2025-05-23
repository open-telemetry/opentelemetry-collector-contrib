// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	conventions "go.opentelemetry.io/otel/semconv/v1.18.0"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestNodeMetricsReportCPUMetrics(t *testing.T) {
	n := testutils.NewNode("1")
	rb := metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig())
	rm := CustomMetrics(receivertest.NewNopSettings(metadata.Type), rb, n,
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
		pcommon.Timestamp(time.Now().UnixNano()),
	)
	m := pmetric.NewMetrics()
	rm.MoveTo(m.ResourceMetrics().AppendEmpty())

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

func TestNodeOptionalMetrics(t *testing.T) {
	n := testutils.NewNode("2")
	rac := metadata.DefaultResourceAttributesConfig()
	rac.K8sKubeletVersion.Enabled = true
	rac.ContainerRuntime.Enabled = true
	rac.ContainerRuntimeVersion.Enabled = true
	rac.OsType.Enabled = true
	rac.OsDescription.Enabled = true

	rb := metadata.NewResourceBuilder(rac)
	rm := CustomMetrics(receivertest.NewNopSettings(metadata.Type), rb, n,
		[]string{},
		[]string{
			"cpu",
			"memory",
		},

		pcommon.Timestamp(time.Now().UnixNano()),
	)
	m := pmetric.NewMetrics()
	rm.MoveTo(m.ResourceMetrics().AppendEmpty())

	expected, err := golden.ReadMetrics(filepath.Join("testdata", "expected_optional.yaml"))
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

func TestNodeMetrics(t *testing.T) {
	n := testutils.NewNode("1")

	ts := pcommon.Timestamp(time.Now().UnixNano())
	mbc := metadata.DefaultMetricsBuilderConfig()
	mbc.Metrics.K8sNodeCondition.Enabled = true
	mb := metadata.NewMetricsBuilder(mbc, receivertest.NewNopSettings(metadata.Type))
	RecordMetrics(mb, n, ts)
	m := mb.Emit()

	expectedFile := filepath.Join("testdata", "expected_mdatagen.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expected, m,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	),
	)
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
			NodeInfo: corev1.NodeSystemInfo{
				MachineID:               "24736a453e8f47a1ad2f9d95d31085f5",
				SystemUUID:              "444005f7-e2e8-42fd-ab87-9f8496790a29",
				BootID:                  "d7ee9a98-ff89-4eed-b723-cffd38ea6c0f",
				KernelVersion:           "6.4.12-arch1-1",
				OSImage:                 "Ubuntu 22.04.1 LTS",
				ContainerRuntimeVersion: "containerd://1.6.9",
				KubeletVersion:          "v1.25.3",
				KubeProxyVersion:        "v1.25.3",
				OperatingSystem:         "linux",
				Architecture:            "amd64",
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
			NodeInfo: corev1.NodeSystemInfo{
				KubeletVersion:          "v1.25.3",
				OSImage:                 "Ubuntu 22.04.1 LTS",
				ContainerRuntimeVersion: "containerd://1.6.9",
				OperatingSystem:         "linux",
			},
		},
	}
	assert.Equal(t, wantNode, Transform(originalNode))
}

func TestNodeMetadata(t *testing.T) {
	creationTimestamp := time.Now()
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-node",
			UID:               "test-node-uid",
			Labels:            map[string]string{"env": "production"},
			CreationTimestamp: metav1.Time{Time: creationTimestamp},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   "FrequentUnregisterNetDevice",
					Status: "False",
					LastHeartbeatTime: metav1.Time{
						Time: time.Now(),
					},
					LastTransitionTime: metav1.Time{
						Time: time.Now(),
					},
					Message: "node is functioning properly",
					Reason:  "NoFrequentUnregisterNetDevice",
				},
				{
					Type:   "MemoryPressure",
					Status: "False",
					LastHeartbeatTime: metav1.Time{
						Time: time.Now(),
					},
					LastTransitionTime: metav1.Time{
						Time: time.Now(),
					},
					Reason:  "KubeletHasSufficientMemory",
					Message: "kubelet has sufficient memory available",
				},
				{
					Type:   "DiskPressure",
					Status: "False",
					LastHeartbeatTime: metav1.Time{
						Time: time.Now(),
					},
					LastTransitionTime: metav1.Time{
						Time: time.Now(),
					},
					Reason:  "KubeletHasNoDiskPressure",
					Message: "kubelet has no disk pressure",
				},
				{
					Type:   "PIDPressure",
					Status: "False",
					LastHeartbeatTime: metav1.Time{
						Time: time.Now(),
					},
					LastTransitionTime: metav1.Time{
						Time: time.Now(),
					},
					Reason:  "KubeletHasSufficientPID",
					Message: "kubelet has sufficient PID available",
				},
				{
					Type:   "Ready",
					Status: "True",
					LastHeartbeatTime: metav1.Time{
						Time: time.Now(),
					},
					LastTransitionTime: metav1.Time{
						Time: time.Now(),
					},
					Reason:  "KubeletReady",
					Message: "kubelet is posting ready status",
				},
			},
			NodeInfo: corev1.NodeSystemInfo{
				MachineID:               "70ebe86154de42bda73a4ffe181afa3d",
				SystemUUID:              "70ebe86154de42bda73a4ffe181afa3d",
				BootID:                  "541c31d4-d1e2-4660-a3b2-484abbb1cbce",
				KernelVersion:           "6.10.4-linuxkit",
				OSImage:                 "Debian GNU/Linux 12 (bookworm)",
				ContainerRuntimeVersion: "containerd://1.7.15",
				KubeletVersion:          "v1.30.0",
				OperatingSystem:         "linux",
				Architecture:            "amd64",
			},
		},
	}

	expectedMeta := map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		experimentalmetricmetadata.ResourceID("test-node-uid"): {
			EntityType:    "k8s.node",
			ResourceIDKey: "k8s.node.uid",
			ResourceID:    experimentalmetricmetadata.ResourceID("test-node-uid"),
			Metadata: map[string]string{
				"env":                                "production",
				string(conventions.K8SNodeNameKey):   "test-node",
				"k8s.node.condition_ready":           "true",
				"k8s.node.condition_memory_pressure": "false",
				"k8s.node.condition_disk_pressure":   "false",
				"k8s.node.condition_pid_pressure":    "false",
				"node.creation_timestamp":            creationTimestamp.Format(time.RFC3339),
			},
		},
	}

	actualMeta := GetMetadata(node)
	require.NotNil(t, actualMeta)
	require.Equal(t, expectedMeta, actualMeta)
}
