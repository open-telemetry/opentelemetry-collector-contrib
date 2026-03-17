// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package container

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestGetMetadata(t *testing.T) {
	refTime := v1.Now()
	pod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			UID:       types.UID("test-pod-uid"),
		},
		Spec: corev1.PodSpec{
			NodeName: "test-node",
		},
	}

	tests := []struct {
		name               string
		containerState     corev1.ContainerState
		expectedStatus     string
		expectedReason     string
		expectedStartedAt  string
		containerName      string
		containerID        string
		containerImage     string
		containerImageName string
		containerImageTag  string
		podName            string
		podUID             string
		nodeName           string
		namespaceName      string
	}{
		{
			name: "Running container",
			containerState: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{
					StartedAt: refTime,
				},
			},
			expectedStatus:     containerStatusRunning,
			expectedStartedAt:  refTime.Format(time.RFC3339),
			containerName:      "my-test-container1",
			containerID:        "f37ee861-f093-4cea-aa26-f39fff8b0998",
			containerImage:     "docker/someimage1:v1.0",
			containerImageName: "docker/someimage1",
			containerImageTag:  "v1.0",
			podName:            pod.Name,
			podUID:             string(pod.UID),
			namespaceName:      "test-namespace",
			nodeName:           "test-node",
		},
		{
			name: "Terminated container",
			containerState: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					ContainerID: "container-id",
					Reason:      "Completed",
					StartedAt:   refTime,
					FinishedAt:  refTime,
					ExitCode:    0,
				},
			},
			expectedStatus:     containerStatusTerminated,
			expectedReason:     "Completed",
			expectedStartedAt:  refTime.Format(time.RFC3339),
			containerName:      "my-test-container2",
			containerID:        "f37ee861-f093-4cea-aa26-f39fff8b0997",
			containerImage:     "docker/someimage2:v1.1",
			containerImageName: "docker/someimage2",
			containerImageTag:  "v1.1",
			podName:            pod.Name,
			podUID:             string(pod.UID),
			namespaceName:      "test-namespace",
			nodeName:           "test-node",
		},
		{
			name: "Waiting container",
			containerState: corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{
					Reason: "CrashLoopBackOff",
				},
			},
			expectedStatus:     containerStatusWaiting,
			expectedReason:     "CrashLoopBackOff",
			containerName:      "my-test-container3",
			containerID:        "f37ee861-f093-4cea-aa26-f39fff8b0996",
			containerImage:     "docker/someimage3:latest",
			containerImageName: "docker/someimage3",
			containerImageTag:  "latest",
			podName:            pod.Name,
			podUID:             string(pod.UID),
			namespaceName:      "test-namespace",
			nodeName:           "test-node",
		},
	}
	logger := zap.NewNop()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := corev1.ContainerStatus{
				State:       tt.containerState,
				Name:        tt.containerName,
				ContainerID: tt.containerID,
				Image:       tt.containerImage,
			}
			md := GetMetadata(pod, cs, logger)

			require.NotNil(t, md)
			assert.Equal(t, tt.expectedStatus, md.Metadata[containerKeyStatus])
			if tt.expectedReason != "" {
				assert.Equal(t, tt.expectedReason, md.Metadata[containerKeyStatusReason])
			}
			if tt.containerState.Running != nil || tt.containerState.Terminated != nil {
				assert.Contains(t, md.Metadata, containerCreationTimestamp)
				assert.Equal(t, tt.expectedStartedAt, md.Metadata[containerCreationTimestamp])
			}
			assert.Equal(t, tt.containerName, md.Metadata[containerName])
			assert.Equal(t, tt.containerImageName, md.Metadata[containerImageName])
			assert.Equal(t, tt.containerImageTag, md.Metadata[containerImageTag])
			assert.Equal(t, tt.podName, md.Metadata["k8s.pod.name"])
			assert.Equal(t, tt.podUID, md.Metadata["k8s.pod.uid"])
			assert.Equal(t, tt.namespaceName, md.Metadata["k8s.namespace.name"])
			assert.Equal(t, tt.nodeName, md.Metadata["k8s.node.name"])
		})
	}
}

// TestGetMetadataWithDigestOnlyStatus tests the scenario where the container runtime
// reports only the digest in ContainerStatus.Image (e.g., on EKS), but the full
// image reference with tag is available in the pod spec.
// See: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/46541
func TestGetMetadataWithDigestOnlyStatus(t *testing.T) {
	refTime := v1.Now()

	// Pod with container spec that has full image reference
	pod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			UID:       types.UID("test-pod-uid"),
		},
		Spec: corev1.PodSpec{
			NodeName: "test-node",
			Containers: []corev1.Container{
				{
					Name:  "my-container",
					Image: "registry.example.com/myapp:v1.2.3@sha256:abc123def456",
				},
			},
		},
	}

	// ContainerStatus with digest-only image (as reported by some runtimes like EKS)
	cs := corev1.ContainerStatus{
		State: corev1.ContainerState{
			Running: &corev1.ContainerStateRunning{
				StartedAt: refTime,
			},
		},
		Name:        "my-container",
		ContainerID: "containerd://abc123",
		Image:       "sha256:abc123def456", // Digest-only, as reported by EKS
	}

	logger := zap.NewNop()
	md := GetMetadata(pod, cs, logger)

	require.NotNil(t, md)
	// Should use the image from spec, not the digest-only status
	assert.Equal(t, "registry.example.com/myapp", md.Metadata[containerImageName])
	assert.Equal(t, "v1.2.3", md.Metadata[containerImageTag])
}

// TestGetMetadataWithInitContainer tests that init containers are also handled correctly.
func TestGetMetadataWithInitContainer(t *testing.T) {
	refTime := v1.Now()

	pod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			UID:       types.UID("test-pod-uid"),
		},
		Spec: corev1.PodSpec{
			NodeName: "test-node",
			InitContainers: []corev1.Container{
				{
					Name:  "init-container",
					Image: "busybox:1.35",
				},
			},
		},
	}

	cs := corev1.ContainerStatus{
		State: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				Reason:     "Completed",
				StartedAt:  refTime,
				FinishedAt: refTime,
				ExitCode:   0,
			},
		},
		Name:        "init-container",
		ContainerID: "containerd://init123",
		Image:       "sha256:busyboxdigest",
	}

	logger := zap.NewNop()
	md := GetMetadata(pod, cs, logger)

	require.NotNil(t, md)
	assert.Equal(t, "busybox", md.Metadata[containerImageName])
	assert.Equal(t, "1.35", md.Metadata[containerImageTag])
}
