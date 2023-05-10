// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"go.opentelemetry.io/collector/obsreport/obsreporttest"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/gvk"
)

func TestReceiver(t *testing.T) {
	tt, err := obsreporttest.SetupTelemetry(component.NewID(typeStr))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, tt.Shutdown(context.Background()))
	}()

	client := newFakeClientWithAllResources()
	osQuotaClient := fakeQuota.NewSimpleClientset()
	sink := new(consumertest.MetricsSink)

	r := setupReceiver(client, osQuotaClient, sink, 10*time.Second, tt)

	// Setup k8s resources.
	numPods := 2
	numNodes := 1
	numQuotas := 2
	numClusterQuotaMetrics := numQuotas * 4
	createPods(t, client, numPods)
	createNodes(t, client, numNodes)
	createClusterQuota(t, osQuotaClient, 2)

	ctx := context.Background()
	require.NoError(t, r.Start(ctx, componenttest.NewNopHost()))

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

func TestReceiverTimesOutAfterStartup(t *testing.T) {
	tt, err := obsreporttest.SetupTelemetry(component.NewID(typeStr))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, tt.Shutdown(context.Background()))
	}()
	client := newFakeClientWithAllResources()

	// Mock initial cache sync timing out, using a small timeout.
	r := setupReceiver(client, nil, consumertest.NewNop(), 1*time.Millisecond, tt)

	createPods(t, client, 1)

	ctx := context.Background()
	require.NoError(t, r.Start(ctx, componenttest.NewNopHost()))
	require.Eventually(t, func() bool {
		return r.resourceWatcher.initialSyncTimedOut.Load()
	}, 10*time.Second, 100*time.Millisecond)
	require.NoError(t, r.Shutdown(ctx))
}

func TestReceiverWithManyResources(t *testing.T) {
	tt, err := obsreporttest.SetupTelemetry(component.NewID(typeStr))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, tt.Shutdown(context.Background()))
	}()

	client := newFakeClientWithAllResources()
	osQuotaClient := fakeQuota.NewSimpleClientset()
	sink := new(consumertest.MetricsSink)

	r := setupReceiver(client, osQuotaClient, sink, 10*time.Second, tt)

	numPods := 1000
	numQuotas := 2
	numExpectedMetrics := numPods + numQuotas*4
	createPods(t, client, numPods)
	createClusterQuota(t, osQuotaClient, 2)

	ctx := context.Background()
	require.NoError(t, r.Start(ctx, componenttest.NewNopHost()))

	require.Eventually(t, func() bool {
		// 4 points from the cluster quota.
		return sink.DataPointCount() == numExpectedMetrics
	}, 10*time.Second, 100*time.Millisecond,
		"metrics not collected")

	require.NoError(t, r.Shutdown(ctx))
}

var numCalls *atomic.Int32
var consumeMetadataInvocation = func() {
	if numCalls != nil {
		numCalls.Add(1)
	}
}

func TestReceiverWithMetadata(t *testing.T) {
	tt, err := obsreporttest.SetupTelemetry(component.NewID(typeStr))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, tt.Shutdown(context.Background()))
	}()

	client := newFakeClientWithAllResources()
	next := &mockExporterWithK8sMetadata{MetricsSink: new(consumertest.MetricsSink)}
	numCalls = &atomic.Int32{}

	r := setupReceiver(client, nil, next, 10*time.Second, tt)
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

	// Should not result in ConsumerKubernetesMetadata invocation.
	r.resourceWatcher.onUpdate(pods[0], pods[0])

	deletePods(t, client, 1)

	// Ensure ConsumeKubernetesMetadata is called twice, once for the add and
	// then for the update.
	require.Eventually(t, func() bool {
		return int(numCalls.Load()) == 2
	}, 10*time.Second, 100*time.Millisecond,
		"metadata not collected")

	require.NoError(t, r.Shutdown(ctx))
}

func getUpdatedPod(pod *corev1.Pod) interface{} {
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
	consumer consumer.Metrics,
	initialSyncTimeout time.Duration,
	tt obsreporttest.TestTelemetry) *kubernetesReceiver {

	distribution := distributionKubernetes
	if osQuotaClient != nil {
		distribution = distributionOpenShift
	}

	config := &Config{
		CollectionInterval:         1 * time.Second,
		NodeConditionTypesToReport: []string{"Ready"},
		AllocatableTypesToReport:   []string{"cpu", "memory"},
		Distribution:               distribution,
	}

	r, _ := newReceiver(context.Background(), tt.ToReceiverCreateSettings(), config, consumer)
	kr := r.(*kubernetesReceiver)
	kr.resourceWatcher.makeClient = func(_ k8sconfig.APIConfig) (kubernetes.Interface, error) {
		return client, nil
	}
	kr.resourceWatcher.makeOpenShiftQuotaClient = func(_ k8sconfig.APIConfig) (quotaclientset.Interface, error) {
		return osQuotaClient, nil
	}
	kr.resourceWatcher.initialTimeout = initialSyncTimeout
	return kr
}

func newFakeClientWithAllResources() *fake.Clientset {
	client := fake.NewSimpleClientset()
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
		{
			GroupVersion: "autoscaling/v2beta2",
			APIResources: []v1.APIResource{
				gvkToAPIResource(gvk.HorizontalPodAutoscalerBeta),
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
