// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutils // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"

import (
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func NewHPA(id string) *autoscalingv2.HorizontalPodAutoscaler {
	minReplicas := int32(2)
	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-hpa-" + id,
			Namespace: "test-namespace",
			UID:       types.UID("test-hpa-" + id + "-uid"),
		},
		Status: autoscalingv2.HorizontalPodAutoscalerStatus{
			CurrentReplicas: 5,
			DesiredReplicas: 7,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			MinReplicas: &minReplicas,
			MaxReplicas: 10,
		},
	}
}

func NewHPABeta(id string) *autoscalingv2beta2.HorizontalPodAutoscaler {
	minReplicas := int32(2)
	return &autoscalingv2beta2.HorizontalPodAutoscaler{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-hpa-" + id,
			Namespace: "test-namespace",
			UID:       types.UID("test-hpa-" + id + "-uid"),
		},
		Status: autoscalingv2beta2.HorizontalPodAutoscalerStatus{
			CurrentReplicas: 5,
			DesiredReplicas: 7,
		},
		Spec: autoscalingv2beta2.HorizontalPodAutoscalerSpec{
			MinReplicas: &minReplicas,
			MaxReplicas: 10,
		},
	}
}

func NewJob(id string) *batchv1.Job {
	p := int32(2)
	c := int32(10)
	return &batchv1.Job{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-job-" + id,
			Namespace: "test-namespace",
			UID:       types.UID("test-job-" + id + "-uid"),
			Labels: map[string]string{
				"foo":  "bar",
				"foo1": "",
			},
		},
		Spec: batchv1.JobSpec{
			Parallelism: &p,
			Completions: &c,
		},
		Status: batchv1.JobStatus{
			Active:    2,
			Succeeded: 3,
			Failed:    0,
		},
	}
}

func NewDaemonset(id string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-daemonset-" + id,
			Namespace: "test-namespace",
			UID:       types.UID("test-daemonset-" + id + "-uid"),
		},
		Status: appsv1.DaemonSetStatus{
			CurrentNumberScheduled: 3,
			NumberMisscheduled:     1,
			DesiredNumberScheduled: 5,
			NumberReady:            2,
		},
	}
}

func NewDeployment(id string) *appsv1.Deployment {
	desired := int32(10)
	return &appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-deployment-" + id,
			Namespace: "test-namespace",
			UID:       types.UID("test-deployment-" + id + "-uid"),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &desired,
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas: 3,
		},
	}
}

func NewReplicaSet(id string) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-replicaset-" + id,
			Namespace: "test-namespace",
			UID:       types.UID("test-replicaset-" + id + "-uid"),
			Labels: map[string]string{
				"foo":  "bar",
				"foo1": "",
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: func() *int32 {
				var out int32 = 3
				return &out
			}(),
		},
		Status: appsv1.ReplicaSetStatus{
			AvailableReplicas: 2,
		},
	}
}

func NewNode(id string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-node-" + id,
			UID:  types.UID("test-node-" + id + "-uid"),
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
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:              *resource.NewMilliQuantity(123, resource.DecimalSI),
				corev1.ResourceMemory:           *resource.NewQuantity(456, resource.DecimalSI),
				corev1.ResourceEphemeralStorage: *resource.NewQuantity(1234, resource.DecimalSI),
			},
		},
	}
}

func NewPodWithContainer(id string, spec *corev1.PodSpec, status *corev1.PodStatus) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-pod-" + id,
			Namespace: "test-namespace",
			UID:       types.UID("test-pod-" + id + "-uid"),
			Labels: map[string]string{
				"foo":  "bar",
				"foo1": "",
			},
		},
		Spec:   *spec,
		Status: *status,
	}
}

func NewPodSpecWithContainer(containerName string) *corev1.PodSpec {
	return &corev1.PodSpec{
		NodeName: "test-node",
		Containers: []corev1.Container{
			{
				Name:  containerName,
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
	}
}

func NewPodStatusWithContainer(containerName, containerID string) *corev1.PodStatus {
	return &corev1.PodStatus{
		Phase: corev1.PodSucceeded,
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name:         containerName,
				Ready:        true,
				RestartCount: 3,
				Image:        "container-image-name",
				ContainerID:  containerID,
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{},
				},
			},
		},
	}
}

func WithOwnerReferences(or []v1.OwnerReference, obj interface{}) interface{} {
	switch o := obj.(type) {
	case *corev1.Pod:
		o.OwnerReferences = or
		return o
	case *batchv1.Job:
		o.OwnerReferences = or
		return o
	case *appsv1.ReplicaSet:
		o.OwnerReferences = or
		return o
	}
	return obj
}
