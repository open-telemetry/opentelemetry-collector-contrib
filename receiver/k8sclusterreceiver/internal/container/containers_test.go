// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package container

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

var testPod = &corev1.Pod{
	ObjectMeta: v1.ObjectMeta{
		Name:      "test-pod",
		Namespace: "test-namespace",
		UID:       types.UID("test-pod-uid"),
	},
	Spec: corev1.PodSpec{
		NodeName: "test-node",
		Containers: []corev1.Container{
			{
				Name:  "test-container",
				Image: "docker/test-image:v1.0",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
			},
		},
	},
}

func TestRecordSpecMetrics(t *testing.T) {
	tests := []struct {
		name            string
		containerStatus *corev1.ContainerStatus
		metricsConfig   func(metadata.MetricsBuilderConfig) metadata.MetricsBuilderConfig
		expectedFile    string
	}{
		{
			name: "running container",
			containerStatus: &corev1.ContainerStatus{
				Name:         "test-container",
				Image:        "docker/test-image:v1.0",
				ContainerID:  "docker://abc123",
				Ready:        true,
				RestartCount: 2,
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{},
				},
			},
			expectedFile: "expected_running.yaml",
		},
		{
			name: "terminated container",
			containerStatus: &corev1.ContainerStatus{
				Name:         "test-container",
				Image:        "docker/test-image:v1.0",
				ContainerID:  "docker://def456",
				Ready:        false,
				RestartCount: 5,
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Reason:   "OOMKilled",
						ExitCode: 137,
					},
				},
				LastTerminationState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Reason: "OOMKilled",
					},
				},
			},
			metricsConfig: func(mbc metadata.MetricsBuilderConfig) metadata.MetricsBuilderConfig {
				mbc.Metrics.K8sContainerStatusState.Enabled = true
				mbc.Metrics.K8sContainerStatusReason.Enabled = true
				return mbc
			},
			expectedFile: "expected_terminated.yaml",
		},
		{
			name: "waiting container",
			containerStatus: &corev1.ContainerStatus{
				Name:         "test-container",
				Image:        "docker/test-image:v1.0",
				Ready:        false,
				RestartCount: 3,
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason: "CrashLoopBackOff",
					},
				},
			},
			metricsConfig: func(mbc metadata.MetricsBuilderConfig) metadata.MetricsBuilderConfig {
				mbc.Metrics.K8sContainerStatusState.Enabled = true
				mbc.Metrics.K8sContainerStatusReason.Enabled = true
				return mbc
			},
			expectedFile: "expected_waiting.yaml",
		},
		{
			name:            "no matching container status",
			containerStatus: nil,
			expectedFile:    "expected_no_status.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := testPod.DeepCopy()
			if tt.containerStatus != nil {
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{*tt.containerStatus}
			}

			mbc := metadata.NewDefaultMetricsBuilderConfig()
			if tt.metricsConfig != nil {
				mbc = tt.metricsConfig(mbc)
			}
			mb := metadata.NewMetricsBuilder(mbc, receivertest.NewNopSettings(metadata.Type))
			ts := pcommon.Timestamp(time.Now().UnixNano())
			assert.NotPanics(t, func() {
				RecordSpecMetrics(zap.NewNop(), mb, pod.Spec.Containers[0], pod, ts)
			})
			m := mb.Emit()

			expectedFile := filepath.Join("testdata", tt.expectedFile)
			// golden.WriteMetrics(t, expectedFile, m)
			expected, err := golden.ReadMetrics(expectedFile)
			require.NoError(t, err)
			require.NoError(t, pmetrictest.CompareMetrics(expected, m,
				pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreStartTimestamp(),
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricsOrder(),
				pmetrictest.IgnoreScopeMetricsOrder(),
				pmetrictest.IgnoreMetricDataPointsOrder(),
			))
		})
	}
}

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
			Containers: []corev1.Container{
				{Name: "my-test-container1", Image: "docker/someimage1:v1.0"},
				{Name: "my-test-container2", Image: "docker/someimage2:v1.1"},
				{Name: "my-test-container3", Image: "docker/someimage3:latest"},
			},
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

func TestGetMetadataDigestFallback(t *testing.T) {
	refTime := v1.Now()
	pod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			UID:       types.UID("test-pod-uid"),
		},
		Spec: corev1.PodSpec{
			NodeName: "test-node",
			Containers: []corev1.Container{
				{Name: "http", Image: "registry.example.com/myapp:v1.2.3@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8"},
			},
		},
	}
	logger := zap.NewNop()

	cs := corev1.ContainerStatus{
		State: corev1.ContainerState{
			Running: &corev1.ContainerStateRunning{
				StartedAt: refTime,
			},
		},
		Name:        "http",
		ContainerID: "containerd://abc123",
		// Some container runtimes (e.g. containerd on EKS) set Image
		// to the digest rather than the full image reference.
		Image: "sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
	}
	md := GetMetadata(pod, cs, logger)

	require.NotNil(t, md)
	assert.Equal(t, "registry.example.com/myapp", md.Metadata[containerImageName])
	assert.Equal(t, "v1.2.3", md.Metadata[containerImageTag])
}

func TestIsBareDigest(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  bool
	}{
		{
			name:  "sha256 digest",
			image: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			want:  true,
		},
		{
			name:  "sha512 digest",
			image: "sha512:3d425c5a102d441da33030949ba5ec22e388ed0529c298a1984d62486d4924806949708b834229206ee5a36ba30f6de6d09989019e5790a8b665539f9489efd5",
			want:  true,
		},
		{
			name:  "normal image with tag",
			image: "docker/someimage:v1.0",
			want:  false,
		},
		{
			name:  "normal image with registry",
			image: "registry.example.com/myapp:latest",
			want:  false,
		},
		{
			name:  "image with digest",
			image: "registry.example.com/myapp@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			want:  false,
		},
		{
			name:  "simple image short tag",
			image: "nginx:latest",
			want:  false,
		},
		{
			name:  "empty string",
			image: "",
			want:  false,
		},
		{
			name:  "no colon",
			image: "justanimage",
			want:  false,
		},
		{
			name:  "hex too short",
			image: "sha256:abc123",
			want:  false,
		},
		{
			name:  "non-hex characters",
			image: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852bZZZ",
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isBareDigest(tt.image))
		})
	}
}

func TestResolveImageString(t *testing.T) {
	tests := []struct {
		name        string
		statusImage string
		specImage   string
		want        string
	}{
		{
			name:        "normal status image",
			statusImage: "docker/someimage:v1.0",
			specImage:   "docker/someimage:v1.0",
			want:        "docker/someimage:v1.0",
		},
		{
			name:        "bare digest falls back to spec",
			statusImage: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			specImage:   "registry.example.com/myapp:v1.0",
			want:        "registry.example.com/myapp:v1.0",
		},
		{
			name:        "empty status falls back to spec",
			statusImage: "",
			specImage:   "registry.example.com/myapp:v1.0",
			want:        "registry.example.com/myapp:v1.0",
		},
		{
			name:        "bare digest without spec returns status",
			statusImage: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			specImage:   "",
			want:        "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:        "both empty",
			statusImage: "",
			specImage:   "",
			want:        "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resolveImageString(tt.statusImage, tt.specImage))
		})
	}
}

func TestSpecImageForContainer(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "myregistry.com/app:v1"},
				{Name: "sidecar", Image: "myregistry.com/sidecar:v2"},
			},
			InitContainers: []corev1.Container{
				{Name: "init", Image: "myregistry.com/init:latest"},
			},
		},
	}

	assert.Equal(t, "myregistry.com/app:v1", specImageForContainer(pod, "app"))
	assert.Equal(t, "myregistry.com/sidecar:v2", specImageForContainer(pod, "sidecar"))
	assert.Equal(t, "myregistry.com/init:latest", specImageForContainer(pod, "init"))
	assert.Empty(t, specImageForContainer(pod, "nonexistent"))
}
