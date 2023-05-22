// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pod

import (
	"fmt"
	"strings"
	"testing"

	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
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
		testutils.NewPodStatusWithContainer("container-name", containerIDWithPreifx("container-id")),
	)

	actualResourceMetrics := GetMetrics(pod, zap.NewNop())

	require.Len(t, actualResourceMetrics, 2)
	testutils.AssertResource(t, actualResourceMetrics[0].Resource, constants.K8sType,
		map[string]string{
			"k8s.pod.uid":        "test-pod-1-uid",
			"k8s.pod.name":       "test-pod-1",
			"k8s.node.name":      "test-node",
			"k8s.namespace.name": "test-namespace",
		},
	)

	require.Len(t, actualResourceMetrics[0].Metrics, 1)
	testutils.AssertMetricsInt(t, actualResourceMetrics[0].Metrics[0], "k8s.pod.phase",
		metricspb.MetricDescriptor_GAUGE_INT64, 3)

	require.Len(t, actualResourceMetrics[1].Metrics, 4)
	testutils.AssertResource(t, actualResourceMetrics[1].Resource, "container",
		map[string]string{
			"container.id":         "container-id",
			"k8s.container.name":   "container-name",
			"container.image.name": "container-image-name",
			"container.image.tag":  "latest",
			"k8s.pod.uid":          "test-pod-1-uid",
			"k8s.pod.name":         "test-pod-1",
			"k8s.node.name":        "test-node",
			"k8s.namespace.name":   "test-namespace",
		},
	)

	testutils.AssertMetricsInt(t, actualResourceMetrics[1].Metrics[0], "k8s.container.restarts",
		metricspb.MetricDescriptor_GAUGE_INT64, 3)

	testutils.AssertMetricsInt(t, actualResourceMetrics[1].Metrics[1], "k8s.container.ready",
		metricspb.MetricDescriptor_GAUGE_INT64, 1)

	testutils.AssertMetricsDouble(t, actualResourceMetrics[1].Metrics[2], "k8s.container.cpu_request",
		metricspb.MetricDescriptor_GAUGE_DOUBLE, 10.0)

	testutils.AssertMetricsDouble(t, actualResourceMetrics[1].Metrics[3], "k8s.container.cpu_limit",
		metricspb.MetricDescriptor_GAUGE_DOUBLE, 20.0)
}

var containerIDWithPreifx = func(containerID string) string {
	return "docker://" + containerID
}

func TestListResourceMetrics(t *testing.T) {
	rms := map[string]*agentmetricspb.ExportMetricsServiceRequest{
		"resource-1": {Resource: &resourcepb.Resource{Type: "type-1"}},
		"resource-2": {Resource: &resourcepb.Resource{Type: "type-2"}},
		"resource-3": {Resource: &resourcepb.Resource{Type: "type-1"}},
	}

	actual := listResourceMetrics(rms)
	expected := []*agentmetricspb.ExportMetricsServiceRequest{
		{Resource: &resourcepb.Resource{Type: "type-1"}},
		{Resource: &resourcepb.Resource{Type: "type-2"}},
		{Resource: &resourcepb.Resource{Type: "type-1"}},
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

			observedLogger, logs := observer.New(zapcore.WarnLevel)
			logger := zap.New(observedLogger)

			name := fmt.Sprintf("(%s) - %s", kind, tt.name)
			t.Run(name, func(t *testing.T) {
				actual := GetMetadata(testCase.resource, testCase.metadataStore, logger)
				require.Equal(t, len(testCase.want), len(actual))

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
	kindLower := strings.ToLower(to.kind)
	kindObjName := fmt.Sprintf("test-%s-0", kindLower)
	kindObjUID := fmt.Sprintf("test-%s-0-uid", kindLower)
	kindNameLabel := fmt.Sprintf("k8s.%s.name", kindLower)
	kindUIDLabel := fmt.Sprintf("k8s.%s.uid", kindLower)

	out := map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		experimentalmetricmetadata.ResourceID(podUIDLabel): {
			ResourceIDKey: "k8s.pod.uid",
			ResourceID:    experimentalmetricmetadata.ResourceID(podUIDLabel),
			Metadata: map[string]string{
				kindNameLabel: kindObjName,
				kindUIDLabel:  kindObjUID,
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
	ms := &metadata.Store{}

	if to.wantNilCache {
		return ms
	}

	store := &testutils.MockStore{
		Cache:   map[string]interface{}{},
		WantErr: to.wantErrFromCache,
	}

	switch to.kind {
	case "Job":
		ms.Jobs = store
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
		ms.ReplicaSets = store
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
		}, testutils.NewPodWithContainer("0", &corev1.PodSpec{}, &corev1.PodStatus{}),
	).(*corev1.Pod)
}
