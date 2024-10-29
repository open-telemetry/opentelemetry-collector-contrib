// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient

import (
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var nodeArray = []any{
	&v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "ip-192-168-200-63.eu-west-1.compute.internal",
			GenerateName:    "",
			Namespace:       "",
			SelfLink:        "/api/v1/nodes/ip-192-168-200-63.eu-west-1.compute.internal",
			UID:             "9e31e901-4c14-11e9-9bd4-02cf86190d00",
			ResourceVersion: "6505830",
			Generation:      0,
			CreationTimestamp: metav1.Time{
				Time: time.Now(),
			},
			Labels: map[string]string{
				"kubernetes.io/arch":                       "amd64",
				"beta.kubernetes.io/instance-type":         "t3.medium",
				"kubernetes.io/os":                         "linux",
				"failure-domain.beta.kubernetes.io/region": "eu-west-1",
				"failure-domain.beta.kubernetes.io/zone":   "eu-west-1c",
				"kubernetes.io/hostname":                   "ip-192-168-200-63.eu-west-1.compute.internal",
			},
			Annotations: map[string]string{
				"node.alpha.kubernetes.io/ttl":                           "0",
				"volumes.kubernetes.io/controller-managed-attach-detach": "true",
			},
		},
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
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
			NodeInfo: v1.NodeSystemInfo{
				MachineID:               "ec2bb261412a689dd19139d9a526407f",
				SystemUUID:              "EC2BB261-412A-689D-D191-39D9A526407F",
				BootID:                  "1d5db5f1-03e8-48f3-9c49-21781a9ba1ae",
				KernelVersion:           "4.14.97-90.72.amzn2.x86_64",
				OSImage:                 "Amazon Linux 2",
				ContainerRuntimeVersion: "docker://18.6.1",
				KubeletVersion:          "v1.11.5",
				KubeProxyVersion:        "v1.11.5",
				OperatingSystem:         "linux",
				Architecture:            "amd64",
			},
		},
	},
	&v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "ip-192-168-76-61.eu-west-1.compute.internal",
			GenerateName:    "",
			Namespace:       "",
			SelfLink:        "/api/v1/nodes/ip-192-168-76-61.eu-west-1.compute.internal",
			UID:             "9f9e79a7-4c14-11e9-b47e-066a7a20bac8",
			ResourceVersion: "6505829",
			Generation:      0,
			CreationTimestamp: metav1.Time{
				Time: time.Now(),
			},
			Labels: map[string]string{
				"kubernetes.io/os":                         "linux",
				"failure-domain.beta.kubernetes.io/region": "eu-west-1",
				"failure-domain.beta.kubernetes.io/zone":   "eu-west-1a",
				"kubernetes.io/hostname":                   "ip-192-168-76-61.eu-west-1.compute.internal",
				"kubernetes.io/arch":                       "amd64",
				"beta.kubernetes.io/instance-type":         "t3.medium",
			},
			Annotations: map[string]string{
				"node.alpha.kubernetes.io/ttl":                           "0",
				"volumes.kubernetes.io/controller-managed-attach-detach": "true",
			},
		},
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
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
			NodeInfo: v1.NodeSystemInfo{
				MachineID:               "ec275328a762912e9c1777bc59328231",
				SystemUUID:              "EC275328-A762-912E-9C17-77BC59328231",
				BootID:                  "02a66fbd-7030-4f7d-85c4-935a32b5d3e9",
				KernelVersion:           "4.14.97-90.72.amzn2.x86_64",
				OSImage:                 "Amazon Linux 2",
				ContainerRuntimeVersion: "docker://18.6.1",
				KubeletVersion:          "v1.11.5",
				KubeProxyVersion:        "v1.11.5",
				OperatingSystem:         "linux",
				Architecture:            "amd64",
			},
		},
	},
	&v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "ip-192-168-153-1.eu-west-1.compute.internal",
			GenerateName:    "",
			Namespace:       "",
			SelfLink:        "/api/v1/nodes/ip-192-168-153-1.eu-west-1.compute.internal",
			UID:             "9eb3a09d-4c14-11e9-b47e-066a7a20bac8",
			ResourceVersion: "6505831",
			Generation:      0,
			CreationTimestamp: metav1.Time{
				Time: time.Now(),
			},
			Labels: map[string]string{
				"kubernetes.io/arch":                       "amd64",
				"beta.kubernetes.io/instance-type":         "t3.medium",
				"kubernetes.io/os":                         "linux",
				"failure-domain.beta.kubernetes.io/region": "eu-west-1",
				"failure-domain.beta.kubernetes.io/zone":   "eu-west-1b",
				"kubernetes.io/hostname":                   "ip-192-168-153-1.eu-west-1.compute.internal",
			},
			Annotations: map[string]string{
				"node.alpha.kubernetes.io/ttl":                           "0",
				"volumes.kubernetes.io/controller-managed-attach-detach": "true",
			},
		},
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
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
				{ // This entry shows failed node
					Type:   "DiskPressure",
					Status: "True",
					LastHeartbeatTime: metav1.Time{
						Time: time.Now(),
					},
					LastTransitionTime: metav1.Time{
						Time: time.Now(),
					},
					Reason:  "KubeletHasDiskPressure",
					Message: "kubelet has disk pressure",
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
			NodeInfo: v1.NodeSystemInfo{
				MachineID:               "ec2eb21af60b929ba89f44fb5d86508f",
				SystemUUID:              "EC2EB21A-F60B-929B-A89F-44FB5D86508F",
				BootID:                  "3b67d19f-cfa6-4925-a728-ce3f3e28991b",
				KernelVersion:           "4.14.97-90.72.amzn2.x86_64",
				OSImage:                 "Amazon Linux 2",
				ContainerRuntimeVersion: "docker://18.6.1",
				KubeletVersion:          "v1.11.5",
				KubeProxyVersion:        "v1.11.5",
				OperatingSystem:         "linux",
				Architecture:            "amd64",
			},
		},
	},
}

func TestNodeClient(t *testing.T) {
	t.Skip("Flaky test, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9001")
	setOption := nodeSyncCheckerOption(&mockReflectorSyncChecker{})

	fakeClientSet := fake.NewSimpleClientset()
	client := newNodeClient(fakeClientSet, zap.NewNop(), setOption)
	assert.NoError(t, client.store.Replace(nodeArray, ""))

	expectedClusterNodeCount := 3
	expectedClusterFailedNodeCount := 1
	clusterNodeCount := client.ClusterNodeCount()
	clusterFailedNodeCount := client.ClusterFailedNodeCount()
	log.Printf("clusterNodeCount: %v, clusterFailedNodeCount: %v", clusterNodeCount, clusterFailedNodeCount)

	assert.Equal(t, expectedClusterNodeCount, clusterNodeCount)
	assert.Equal(t, expectedClusterFailedNodeCount, clusterFailedNodeCount)
	client.shutdown()
	assert.True(t, client.stopped)
}

func TestTransformFuncNode(t *testing.T) {
	info, err := transformFuncNode(nil)
	assert.Nil(t, info)
	assert.Error(t, err)
}
