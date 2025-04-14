// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclusterreceiver

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	quotaclientset "github.com/openshift/client-go/quota/clientset/versioned"
	fakeQuota "github.com/openshift/client-go/quota/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector/k8sleaderelectortest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/gvk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

type nopHost struct {
	component.Host
}

func (nh *nopHost) GetExporters() map[pipeline.Signal]map[component.ID]component.Component {
	return nil
}

func newNopHost() component.Host {
	return &nopHost{
		componenttest.NewNopHost(),
	}
}

func TestReceiver(t *testing.T) {
	tt := componenttest.NewTelemetry()
	defer func() {
		require.NoError(t, tt.Shutdown(context.Background()))
	}()

	client := newFakeClientWithAllResources()
	osQuotaClient := fakeQuota.NewSimpleClientset()
	sink := new(consumertest.MetricsSink)

	r := setupReceiver(client, osQuotaClient, sink, nil, 10*time.Second, tt, "", component.MustNewID("foo"))

	// Setup k8s resources.
	numPods := 2
	numNodes := 1
	numQuotas := 2
	numClusterQuotaMetrics := numQuotas * 4
	createPods(t, client, numPods)
	createNodes(t, client, numNodes)
	createClusterQuota(t, osQuotaClient, 2)

	ctx := context.Background()
	require.NoError(t, r.Start(ctx, newNopHost()))

	// Expects metric data from nodes and pods where each metric data
	// struct corresponds to one resource.
	expectedNumMetrics := numPods + numNodes + numClusterQuotaMetrics
	var initialDataPointCount int
	require.Eventually(t, func() bool {
		initialDataPointCount = sink.DataPointCount()
		return initialDataPointCount == expectedNumMetrics
	}, 10*time.Second, 100*time.Millisecond,
		"metrics not collected")

	numPodsToDelete := 1
	deletePods(t, client, numPodsToDelete)

	// Expects metric data from a node, since other resources were deleted.
	expectedNumMetrics = (numPods - numPodsToDelete) + numNodes + numClusterQuotaMetrics
	var metricsCountDelta int
	require.Eventually(t, func() bool {
		metricsCountDelta = sink.DataPointCount() - initialDataPointCount
		return metricsCountDelta == expectedNumMetrics
	}, 10*time.Second, 100*time.Millisecond,
		"updated metrics not collected")

	require.NoError(t, r.Shutdown(ctx))
}

func TestReceiverWithLeaderElection(t *testing.T) {
	fakeLeaderElection := &k8sleaderelectortest.FakeLeaderElection{}
	fakeHost := &k8sleaderelectortest.FakeHost{
		FakeLeaderElection: fakeLeaderElection,
	}

	client := newFakeClientWithAllResources()
	osQuotaClient := fakeQuota.NewSimpleClientset()
	sink := new(consumertest.MetricsSink)
	tt := componenttest.NewTelemetry()
	kr := setupReceiver(client, osQuotaClient, sink, nil, 10*time.Second, tt, "", component.MustNewID("k8s_leader_elector"))

	// Setup k8s resources.
	numPods := 2

	createPods(t, client, numPods)

	err := kr.Start(context.Background(), fakeHost)
	require.NoError(t, err)

	// elected leader
	fakeLeaderElection.InvokeOnLeading()

	expectedNumMetrics := numPods
	var initialDataPointCount int
	require.Eventually(t, func() bool {
		initialDataPointCount = sink.DataPointCount()
		return initialDataPointCount == expectedNumMetrics
	}, 20*time.Second, 100*time.Millisecond,
		"metrics not collected")

	// lost election
	fakeLeaderElection.InvokeOnStopping()
	require.NoError(t, kr.Shutdown(context.Background()))
}

func TestNamespacedReceiver(t *testing.T) {
	tt := componenttest.NewTelemetry()
	defer func() {
		require.NoError(t, tt.Shutdown(context.Background()))
	}()

	client := newFakeClientWithAllResources()
	osQuotaClient := fakeQuota.NewSimpleClientset()
	sink := new(consumertest.MetricsSink)

	r := setupReceiver(client, osQuotaClient, sink, nil, 10*time.Second, tt, "test", component.MustNewID("foo"))

	// Setup k8s resources.
	numPods := 2
	numNodes := 1
	numQuotas := 2

	createPods(t, client, numPods)
	createNodes(t, client, numNodes)
	createClusterQuota(t, osQuotaClient, numQuotas)

	ctx := context.Background()
	require.NoError(t, r.Start(ctx, newNopHost()))

	// Expects metric data from pods  only, where each metric data
	// struct corresponds to one resource.
	// Nodes and ClusterResourceQuotas should not be observed as these are non-namespaced resources
	expectedNumMetrics := numPods
	var initialDataPointCount int
	require.Eventually(t, func() bool {
		initialDataPointCount = sink.DataPointCount()
		return initialDataPointCount == expectedNumMetrics
	}, 10*time.Second, 100*time.Millisecond,
		"metrics not collected")

	numPodsToDelete := 1
	deletePods(t, client, numPodsToDelete)

	// Expects metric data from a node, since other resources were deleted.
	expectedNumMetrics = numPods - numPodsToDelete
	var metricsCountDelta int
	require.Eventually(t, func() bool {
		metricsCountDelta = sink.DataPointCount() - initialDataPointCount
		return metricsCountDelta == expectedNumMetrics
	}, 10*time.Second, 100*time.Millisecond,
		"updated metrics not collected")

	require.NoError(t, r.Shutdown(ctx))
}

func TestReceiverTimesOutAfterStartup(t *testing.T) {
	tt := componenttest.NewTelemetry()
	defer func() {
		require.NoError(t, tt.Shutdown(context.Background()))
	}()
	client := newFakeClientWithAllResources()

	// Mock initial cache sync timing out, using a small timeout.
	r := setupReceiver(client, nil, consumertest.NewNop(), nil, 1*time.Millisecond, tt, "", component.MustNewID("foo"))

	createPods(t, client, 1)

	ctx := context.Background()
	require.NoError(t, r.Start(ctx, newNopHost()))
	require.Eventually(t, func() bool {
		return r.resourceWatcher.initialSyncTimedOut.Load()
	}, 10*time.Second, 100*time.Millisecond)
	require.NoError(t, r.Shutdown(ctx))
}

func TestReceiverWithManyResources(t *testing.T) {
	tt := componenttest.NewTelemetry()
	defer func() {
		require.NoError(t, tt.Shutdown(context.Background()))
	}()

	client := newFakeClientWithAllResources()
	osQuotaClient := fakeQuota.NewSimpleClientset()
	sink := new(consumertest.MetricsSink)

	r := setupReceiver(client, osQuotaClient, sink, nil, 10*time.Second, tt, "", component.MustNewID("foo"))

	numPods := 1000
	numQuotas := 2
	numExpectedMetrics := numPods + numQuotas*4
	createPods(t, client, numPods)
	createClusterQuota(t, osQuotaClient, 2)

	ctx := context.Background()
	require.NoError(t, r.Start(ctx, newNopHost()))

	require.Eventually(t, func() bool {
		// 4 points from the cluster quota.
		return sink.DataPointCount() == numExpectedMetrics
	}, 10*time.Second, 100*time.Millisecond,
		"metrics not collected")

	require.NoError(t, r.Shutdown(ctx))
}

var (
	numCalls                  *atomic.Int32
	consumeMetadataInvocation = func() {
		if numCalls != nil {
			numCalls.Add(1)
		}
	}
)

func TestReceiverWithMetadata(t *testing.T) {
	tt := componenttest.NewTelemetry()
	defer func() {
		require.NoError(t, tt.Shutdown(context.Background()))
	}()

	client := newFakeClientWithAllResources()
	metricsConsumer := &mockExporterWithK8sMetadata{MetricsSink: new(consumertest.MetricsSink)}
	numCalls = &atomic.Int32{}

	logsConsumer := new(consumertest.LogsSink)

	r := setupReceiver(client, nil, metricsConsumer, logsConsumer, 10*time.Second, tt, "", component.MustNewID("foo"))
	r.config.MetadataExporters = []string{"nop/withmetadata"}

	// Setup k8s resources.
	pods := createPods(t, client, 1)

	ctx := context.Background()
	require.NoError(t, r.Start(ctx, newNopHostWithExporters()))

	// Mock an update on the Pod object. It appears that the fake clientset
	// does not pass on events for updates to resources.
	require.Len(t, pods, 1)
	updatedPod := getUpdatedPod(pods[0])
	r.resourceWatcher.onUpdate(pods[0], updatedPod)

	// Should not result in ConsumerKubernetesMetadata invocation since the pod
	// is not changed. Should result in entity event because they are emitted even
	// if the entity is not changed.
	r.resourceWatcher.onUpdate(updatedPod, updatedPod)

	deletePods(t, client, 1)

	// Ensure ConsumeKubernetesMetadata is called twice, once for the add and
	// then for the update. Note the second update does not result in metadata call
	// since the pod is not changed.
	require.Eventually(t, func() bool {
		return int(numCalls.Load()) == 2
	}, 10*time.Second, 100*time.Millisecond,
		"metadata not collected")

	// Must have 3 entity events: once for the add, followed by an update and
	// then another update, which unlike metadata calls actually happens since
	// even unchanged entities trigger an event.
	require.Eventually(t, func() bool {
		return logsConsumer.LogRecordCount() == 3
	}, 10*time.Second, 100*time.Millisecond,
		"entity events not collected")

	require.NoError(t, r.Shutdown(ctx))
}

func getUpdatedPod(pod *corev1.Pod) any {
	return &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			UID:       pod.UID,
			Labels: map[string]string{
				"key": "value",
			},
		},
	}
}

func setupReceiver(
	client *fake.Clientset,
	osQuotaClient quotaclientset.Interface,
	metricsConsumer consumer.Metrics,
	logsConsumer consumer.Logs,
	initialSyncTimeout time.Duration,
	tt *componenttest.Telemetry,
	namespace string,
	leaderElector component.ID,
) *kubernetesReceiver {
	distribution := distributionKubernetes
	if osQuotaClient != nil {
		distribution = distributionOpenShift
	}
	config := &Config{
		CollectionInterval:         1 * time.Second,
		NodeConditionTypesToReport: []string{"Ready"},
		AllocatableTypesToReport:   []string{"cpu", "memory"},
		Distribution:               distribution,
		MetricsBuilderConfig:       metadata.DefaultMetricsBuilderConfig(),
		Namespace:                  namespace,
	}

	if leaderElector.Type().String() == "k8s_leader_elector" {
		config.K8sLeaderElector = &leaderElector
	}

	r, _ := newReceiver(context.Background(), receiver.Settings{ID: component.NewID(metadata.Type), TelemetrySettings: tt.NewTelemetrySettings(), BuildInfo: component.NewDefaultBuildInfo()}, config)
	kr := r.(*kubernetesReceiver)
	kr.metricsConsumer = metricsConsumer
	kr.resourceWatcher.makeClient = func(_ k8sconfig.APIConfig) (kubernetes.Interface, error) {
		return client, nil
	}
	kr.resourceWatcher.makeOpenShiftQuotaClient = func(_ k8sconfig.APIConfig) (quotaclientset.Interface, error) {
		return osQuotaClient, nil
	}
	kr.resourceWatcher.initialTimeout = initialSyncTimeout
	kr.resourceWatcher.entityLogConsumer = logsConsumer
	return kr
}

func newFakeClientWithAllResources() *fake.Clientset {
	client := fake.NewClientset()
	client.Resources = []*v1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []v1.APIResource{
				gvkToAPIResource(gvk.Pod),
				gvkToAPIResource(gvk.Node),
				gvkToAPIResource(gvk.Namespace),
				gvkToAPIResource(gvk.ReplicationController),
				gvkToAPIResource(gvk.ResourceQuota),
				gvkToAPIResource(gvk.Service),
			},
		},
		{
			GroupVersion: "apps/v1",
			APIResources: []v1.APIResource{
				gvkToAPIResource(gvk.DaemonSet),
				gvkToAPIResource(gvk.Deployment),
				gvkToAPIResource(gvk.ReplicaSet),
				gvkToAPIResource(gvk.StatefulSet),
			},
		},
		{
			GroupVersion: "batch/v1",
			APIResources: []v1.APIResource{
				gvkToAPIResource(gvk.Job),
				gvkToAPIResource(gvk.CronJob),
			},
		},
		{
			GroupVersion: "autoscaling/v2",
			APIResources: []v1.APIResource{
				gvkToAPIResource(gvk.HorizontalPodAutoscaler),
			},
		},
	}
	return client
}

func gvkToAPIResource(gvk schema.GroupVersionKind) v1.APIResource {
	return v1.APIResource{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind,
	}
}
