// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pod

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/gvk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

var commonPodMetadata = map[string]string{
	"foo":                    "bar",
	"foo1":                   "",
	"pod.creation_timestamp": "0001-01-01T00:00:00Z",
}

func TestPodAndContainerMetricsReportCPUMetrics(t *testing.T) {
	pod := testutils.NewPodWithContainer(
		"1",
		testutils.NewPodSpecWithContainer("container-name"),
		testutils.NewPodStatusWithContainer("container-name", containerIDWithPrefix("container-id")),
	)

	ts := pcommon.Timestamp(time.Now().UnixNano())
	mb := metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopSettings(metadata.Type))
	RecordMetrics(zap.NewNop(), mb, pod, ts)
	m := mb.Emit()
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

func TestPodStatusReasonAndContainerMetricsReportCPUMetrics(t *testing.T) {
	pod := testutils.NewPodWithContainer(
		"1",
		testutils.NewPodSpecWithContainer("container-name"),
		testutils.NewEvictedTerminatedPodStatusWithContainer("container-name", containerIDWithPrefix("container-id")),
	)

	mbc := metadata.DefaultMetricsBuilderConfig()
	mbc.Metrics.K8sPodStatusReason.Enabled = true
	mbc.ResourceAttributes.K8sPodQosClass.Enabled = true
	mbc.ResourceAttributes.K8sContainerStatusLastTerminatedReason.Enabled = true
	ts := pcommon.Timestamp(time.Now().UnixNano())
	mb := metadata.NewMetricsBuilder(mbc, receivertest.NewNopSettings(metadata.Type))
	RecordMetrics(zap.NewNop(), mb, pod, ts)
	m := mb.Emit()

	expected, err := golden.ReadMetrics(filepath.Join("testdata", "expected_evicted.yaml"))
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

var containerIDWithPrefix = func(containerID string) string {
	return "docker://" + containerID
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

func TestDataCollectorSyncMetadataForPodWorkloads(t *testing.T) {
	tests := []struct {
		name             string
		withParentOR     bool
		emptyCache       bool
		wantNilCache     bool
		wantErrFromCache bool
		logMessage       string
	}{
		{
			name: "Pod metadata with Owner reference",
		},
		{
			name:         "Pod metadata with parent Owner reference",
			withParentOR: true,
		},
		{
			name:         "Owner reference - cache nil",
			wantNilCache: true,
		},
		{
			name:       "Owner reference - does not exist in cache",
			emptyCache: true,
			logMessage: "Resource does not exist in store, properties from it will not be synced.",
		},
		{
			name:             "Owner reference - cache error",
			wantErrFromCache: true,
			logMessage:       "Failed to get resource from store, properties from it will not be synced.",
		},
	}

	for _, kind := range []string{"Job", "ReplicaSet"} {
		for _, tt := range tests {
			testCase := testCaseForPodWorkload(testCaseOptions{
				kind:             kind,
				withParentOR:     tt.withParentOR,
				emptyCache:       tt.emptyCache,
				wantNilCache:     tt.wantNilCache,
				wantErrFromCache: tt.wantErrFromCache,
				logMessage:       tt.logMessage,
			})

			// Ensure required mockups are available.
			require.NotNil(t, testCase.metadataStore)
			require.NotNil(t, testCase.resource)

			observedLogger, logs := observer.New(zapcore.DebugLevel)
			logger := zap.New(observedLogger)

			name := fmt.Sprintf("(%s) - %s", kind, tt.name)
			t.Run(name, func(t *testing.T) {
				actual := GetMetadata(testCase.resource, testCase.metadataStore, logger)
				require.Len(t, actual, len(testCase.want))

				for key, item := range testCase.want {
					got, exists := actual[key]
					require.True(t, exists)

					for k, v := range commonPodMetadata {
						item.Metadata[k] = v
					}
					require.Equal(t, *item, *got)

					if testCase.logMessage != "" {
						require.GreaterOrEqual(t, 1, logs.Len())
						require.Equal(t, testCase.logMessage, logs.All()[0].Message)
					}
				}
			})
		}
	}
}

type testCase struct {
	metadataStore *metadata.Store
	resource      *corev1.Pod
	want          map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata
	logMessage    string
}

type testCaseOptions struct {
	kind             string
	withParentOR     bool
	emptyCache       bool
	wantNilCache     bool
	wantErrFromCache bool
	logMessage       string
}

func testCaseForPodWorkload(to testCaseOptions) testCase {
	out := testCase{
		metadataStore: mockMetadataStore(to),
		resource:      podWithOwnerReference(to.kind),
		want:          expectedKubernetesMetadata(to),
		logMessage:    to.logMessage,
	}
	return out
}

func expectedKubernetesMetadata(to testCaseOptions) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	podUIDLabel := "test-pod-0-uid"
	podNameLabel := "test-pod-0"
	kindLower := strings.ToLower(to.kind)
	kindObjName := fmt.Sprintf("test-%s-0", kindLower)
	namespaceLabel := "test-namespace"
	kindObjUID := fmt.Sprintf("test-%s-0-uid", kindLower)
	kindNameLabel := fmt.Sprintf("k8s.%s.name", kindLower)
	kindUIDLabel := fmt.Sprintf("k8s.%s.uid", kindLower)

	out := map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		experimentalmetricmetadata.ResourceID(podUIDLabel): {
			EntityType:    "k8s.pod",
			ResourceIDKey: "k8s.pod.uid",
			ResourceID:    experimentalmetricmetadata.ResourceID(podUIDLabel),
			Metadata: map[string]string{
				kindNameLabel:                    kindObjName,
				kindUIDLabel:                     kindObjUID,
				conventions.AttributeK8SNodeName: "test-node",
				"k8s.pod.name":                   podNameLabel,
				"k8s.pod.phase":                  "Unknown", // Default value when phase is not set.
				"k8s.namespace.name":             namespaceLabel,
			},
		},
	}

	withoutInfoFromCache := to.emptyCache || to.wantNilCache || to.wantErrFromCache

	// Add metadata gathered from informer caches to expected metadata.
	if !withoutInfoFromCache {
		out[experimentalmetricmetadata.ResourceID(podUIDLabel)].Metadata["k8s.workload.kind"] = to.kind
		out[experimentalmetricmetadata.ResourceID(podUIDLabel)].Metadata["k8s.workload.name"] = kindObjName
	}

	// If the Pod's Owner Kind is not the actual owner (CronJobs -> Jobs and Deployments -> ReplicaSets),
	// add metadata additional metadata to expected values.
	if to.withParentOR {
		switch to.kind {
		case "Job":
			out[experimentalmetricmetadata.ResourceID(podUIDLabel)].Metadata["k8s.cronjob.uid"] = "test-cronjob-0-uid"
			out[experimentalmetricmetadata.ResourceID(podUIDLabel)].Metadata["k8s.cronjob.name"] = "test-cronjob-0"
			out[experimentalmetricmetadata.ResourceID(podUIDLabel)].Metadata["k8s.workload.name"] = "test-cronjob-0"
			out[experimentalmetricmetadata.ResourceID(podUIDLabel)].Metadata["k8s.workload.kind"] = "CronJob"
		case "ReplicaSet":
			out[experimentalmetricmetadata.ResourceID(podUIDLabel)].Metadata["k8s.deployment.uid"] = "test-deployment-0-uid"
			out[experimentalmetricmetadata.ResourceID(podUIDLabel)].Metadata["k8s.deployment.name"] = "test-deployment-0"
			out[experimentalmetricmetadata.ResourceID(podUIDLabel)].Metadata["k8s.workload.name"] = "test-deployment-0"
			out[experimentalmetricmetadata.ResourceID(podUIDLabel)].Metadata["k8s.workload.kind"] = "Deployment"
		}
	}
	return out
}

func mockMetadataStore(to testCaseOptions) *metadata.Store {
	ms := metadata.NewStore()

	if to.wantNilCache {
		return ms
	}

	store := &testutils.MockStore{
		Cache:   map[string]any{},
		WantErr: to.wantErrFromCache,
	}

	switch to.kind {
	case "Job":
		ms.Setup(gvk.Job, store)
		if !to.emptyCache {
			if to.withParentOR {
				store.Cache["test-namespace/test-job-0"] = testutils.WithOwnerReferences(
					[]v1.OwnerReference{
						{
							Kind: "CronJob",
							Name: "test-cronjob-0",
							UID:  "test-cronjob-0-uid",
						},
					}, testutils.NewJob("0"),
				)
			} else {
				store.Cache["test-namespace/test-job-0"] = testutils.NewJob("0")
			}
		}
		return ms
	case "ReplicaSet":
		ms.Setup(gvk.ReplicaSet, store)
		if !to.emptyCache {
			if to.withParentOR {
				store.Cache["test-namespace/test-replicaset-0"] = testutils.WithOwnerReferences(
					[]v1.OwnerReference{
						{
							Kind: "Deployment",
							Name: "test-deployment-0",
							UID:  "test-deployment-0-uid",
						},
					}, testutils.NewReplicaSet("0"),
				)
			} else {
				store.Cache["test-namespace/test-replicaset-0"] = testutils.NewReplicaSet("0")
			}
		}
		return ms
	}

	return ms
}

func podWithOwnerReference(kind string) *corev1.Pod {
	kindLower := strings.ToLower(kind)
	return testutils.WithOwnerReferences(
		[]v1.OwnerReference{
			{
				Kind: kind,
				Name: fmt.Sprintf("test-%s-0", kindLower),
				UID:  types.UID(fmt.Sprintf("test-%s-0-uid", kindLower)),
			},
		}, testutils.NewPodWithContainer("0", &corev1.PodSpec{NodeName: "test-node"}, &corev1.PodStatus{}),
	).(*corev1.Pod)
}

func TestTransform(t *testing.T) {
	containerState := corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: v1.Now()}}
	originalPod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      "my-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app":     "my-app",
				"version": "v1",
			},
			Annotations: map[string]string{
				"example.com/annotation": "some-value",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			NodeName:      "node-1",
			HostNetwork:   true,
			HostIPC:       true,
			HostPID:       true,
			DNSPolicy:     corev1.DNSClusterFirst,
			TerminationGracePeriodSeconds: func() *int64 {
				gracePeriodSeconds := int64(30)
				return &gracePeriodSeconds
			}(),
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:  func() *int64 { uid := int64(1000); return &uid }(),
				RunAsGroup: func() *int64 { gid := int64(2000); return &gid }(),
				FSGroup:    func() *int64 { gid := int64(3000); return &gid }(),
			},
			Containers: []corev1.Container{
				{
					Name:            "my-container",
					Image:           "nginx:latest",
					ImagePullPolicy: corev1.PullAlways,
					Ports: []corev1.ContainerPort{
						{
							Name:          "http",
							ContainerPort: 80,
							Protocol:      corev1.ProtocolTCP,
						},
						{
							Name:          "https",
							ContainerPort: 443,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "MY_ENV",
							Value: "my-value",
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase:     corev1.PodRunning,
			Reason:    "Evicted",
			HostIP:    "192.168.1.100",
			PodIP:     "10.244.0.5",
			StartTime: &v1.Time{Time: v1.Now().Add(-5 * time.Minute)},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "invalid-container",
					Image:        "redis:latest",
					RestartCount: 1,
				},
				{
					Name:         "my-container",
					Image:        "nginx:latest",
					ContainerID:  "abc12345",
					RestartCount: 2,
					Ready:        true,
					State:        containerState,
				},
			},
		},
	}
	wantPod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      "my-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app":     "my-app",
				"version": "v1",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "node-1",
			Containers: []corev1.Container{
				{
					Name: "my-container",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase:  corev1.PodRunning,
			Reason: "Evicted",
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "my-container",
					Image:        "nginx:latest",
					ContainerID:  "abc12345",
					RestartCount: 2,
					Ready:        true,
					State:        containerState,
				},
			},
		},
	}
	assert.Equal(t, wantPod, Transform(originalPod))
}

func TestPodMetadata(t *testing.T) {
	tests := []struct {
		name             string
		podName          string
		namespace        string
		statusPhase      corev1.PodPhase
		statusReason     string
		expectedMetadata map[string]string
	}{
		{
			name:         "Pod with status reason",
			statusPhase:  corev1.PodFailed,
			statusReason: "Evicted",
			expectedMetadata: map[string]string{
				"k8s.pod.name":                   "test-pod-0",
				"k8s.namespace.name":             "test-namespace",
				"k8s.pod.phase":                  "Failed",
				"k8s.pod.status_reason":          "Evicted",
				"k8s.workload.kind":              "Deployment",
				"k8s.workload.name":              "test-deployment-0",
				"k8s.replicaset.name":            "test-replicaset-0",
				"k8s.replicaset.uid":             "test-replicaset-0-uid",
				"k8s.deployment.name":            "test-deployment-0",
				"k8s.deployment.uid":             "test-deployment-0-uid",
				conventions.AttributeK8SNodeName: "test-node",
			},
		},
		{
			name:         "Pod without status reason",
			statusPhase:  corev1.PodRunning,
			statusReason: "",
			expectedMetadata: map[string]string{
				"k8s.pod.name":                   "test-pod-0",
				"k8s.namespace.name":             "test-namespace",
				"k8s.pod.phase":                  "Running",
				"k8s.workload.kind":              "Deployment",
				"k8s.workload.name":              "test-deployment-0",
				"k8s.replicaset.name":            "test-replicaset-0",
				"k8s.replicaset.uid":             "test-replicaset-0-uid",
				"k8s.deployment.name":            "test-deployment-0",
				"k8s.deployment.uid":             "test-deployment-0-uid",
				conventions.AttributeK8SNodeName: "test-node",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := podWithOwnerReference("ReplicaSet")
			pod.Status.Phase = tt.statusPhase
			pod.Status.Reason = tt.statusReason

			metadataStore := mockMetadataStore(testCaseOptions{
				kind:         "ReplicaSet",
				withParentOR: true,
			})
			logger := zap.NewNop()
			meta := GetMetadata(pod, metadataStore, logger)

			require.NotNil(t, meta)
			require.Contains(t, meta, experimentalmetricmetadata.ResourceID("test-pod-0-uid"))
			podMeta := meta["test-pod-0-uid"].Metadata

			allExpectedMetadata := make(map[string]string)
			for key, value := range commonPodMetadata {
				allExpectedMetadata[key] = value
			}
			for key, value := range tt.expectedMetadata {
				allExpectedMetadata[key] = value
			}
			assert.Equal(t, allExpectedMetadata, podMeta)
		})
	}
}
