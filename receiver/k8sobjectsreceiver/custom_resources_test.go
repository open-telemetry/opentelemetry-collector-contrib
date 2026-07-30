// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/filter"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap/zaptest"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	k8smetadata "k8s.io/client-go/metadata"
	fakemetadata "k8s.io/client-go/metadata/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sleaderelectortest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver/internal/metadata"
)

var (
	applicationGVR = schema.GroupVersionResource{
		Group: "argoproj.io", Version: "v1alpha1", Resource: "applications",
	}
	clusterWidgetGVR = schema.GroupVersionResource{
		Group: "example.com", Version: "v1", Resource: "clusterwidgets",
	}
)

type testCustomResourceDiscovery struct {
	fakediscovery.FakeDiscovery
	resources []*metav1.APIResourceList
	err       error
	calls     atomic.Int64
}

func (d *testCustomResourceDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	d.calls.Add(1)
	return d.resources, d.err
}

type testPaginatedResource struct {
	dynamic.ResourceInterface
	options []metav1.ListOptions
}

type testRetryResource struct {
	dynamic.ResourceInterface
	errs  []error
	calls int
}

func (r *testRetryResource) List(_ context.Context, _ metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	r.calls++
	if len(r.errs) != 0 {
		err := r.errs[0]
		r.errs = r.errs[1:]
		return nil, err
	}
	return &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{
			*customResource("argoproj.io/v1alpha1", "Application", "example", "default"),
		},
	}, nil
}

func (r *testPaginatedResource) List(_ context.Context, options metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	r.options = append(r.options, options)
	if len(r.options) > 2 {
		return nil, errors.New("unexpected continue token")
	}

	itemName := "first"
	continueToken := "next"
	if len(r.options) == 2 {
		itemName = "second"
		continueToken = ""
	}
	item := customResource("argoproj.io/v1alpha1", "Application", itemName, "default")
	list := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{*item}}
	list.SetContinue(continueToken)
	return list, nil
}

func TestDiscoverCustomResources(t *testing.T) {
	t.Parallel()

	crds := map[schema.GroupResource]struct{}{
		{Group: "argoproj.io", Resource: "applications"}:   {},
		{Group: "argoproj.io", Resource: "appprojects"}:    {},
		{Group: "example.com", Resource: "clusterwidgets"}: {},
	}
	resources := []*metav1.APIResourceList{
		{
			GroupVersion: "argoproj.io/v1alpha1",
			APIResources: []metav1.APIResource{
				{Name: "applications", Namespaced: true, Verbs: metav1.Verbs{"get", "list"}},
				{Name: "appprojects", Namespaced: true, Verbs: metav1.Verbs{"get"}},
			},
		},
		{
			GroupVersion: "example.com/v1",
			APIResources: []metav1.APIResource{
				{Name: "clusterwidgets", Verbs: metav1.Verbs{"list"}},
			},
		},
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Namespaced: true, Verbs: metav1.Verbs{"list"}},
			},
		},
	}

	assert.Equal(t, []discoveredCustomResource{
		{gvr: applicationGVR, namespaced: true},
		{gvr: clusterWidgetGVR},
	}, discoverCustomResources(
		crds,
		resources,
		[]CustomResourceSelector{{Group: "*", Resources: []string{"*"}}},
		nil,
		nil,
	))

	assert.Equal(t, []discoveredCustomResource{
		{gvr: applicationGVR, namespaced: true},
	}, discoverCustomResources(
		crds,
		resources,
		[]CustomResourceSelector{{Group: "argoproj.io", Resources: []string{"applications", "appprojects"}}},
		[]CustomResourceSelector{{Group: "argoproj.io", Resources: []string{"appprojects"}}},
		nil,
	))

	assert.Empty(t, discoverCustomResources(
		crds,
		resources,
		[]CustomResourceSelector{{Group: "argoproj.io", Resources: []string{"applications"}}},
		[]CustomResourceSelector{{Group: "argoproj.io", Resources: []string{"*"}}},
		nil,
	))

	assert.Equal(t, []discoveredCustomResource{
		{gvr: applicationGVR, namespaced: true},
	}, discoverCustomResources(
		crds,
		resources,
		[]CustomResourceSelector{{Group: "*", Resources: []string{"applications"}}},
		nil,
		nil,
	))

	assert.Equal(t, []discoveredCustomResource{
		{gvr: clusterWidgetGVR},
	}, discoverCustomResources(
		crds,
		resources,
		[]CustomResourceSelector{{Group: "*", Resources: []string{"*"}}},
		nil,
		map[schema.GroupResource]struct{}{
			applicationGVR.GroupResource(): {},
		},
	))
}

func TestCustomResourceCollectorCollectsFilteredResources(t *testing.T) {
	t.Parallel()

	dynamicClient := newCustomResourceDynamicClient(
		customResource("argoproj.io/v1alpha1", "Application", "included", "default"),
		customResource("argoproj.io/v1alpha1", "Application", "excluded", "kube-system"),
		customResource("example.com/v1", "ClusterWidget", "cluster", ""),
	)
	metadataClient := newCustomResourceMetadataClient(
		"applications.argoproj.io",
		"clusterwidgets.example.com",
	)
	discoveryClient := newCustomResourceDiscovery(
		apiResourceList(applicationGVR, true),
		apiResourceList(clusterWidgetGVR, false),
	)

	type consumedPage struct {
		gvr   schema.GroupVersionResource
		names []string
	}
	var consumed []consumedPage
	collector, err := newCustomResourceCollector(
		&CustomResourcesConfig{
			Interval: time.Hour,
			Include: []CustomResourceSelector{
				{Group: "*", Resources: []string{"*"}},
			},
			ExcludeNamespaces: []filter.Config{
				{Strict: "kube-system"},
			},
		},
		dynamicClient,
		metadataClient,
		discoveryClient,
		nil,
		zaptest.NewLogger(t),
		func(_ context.Context, objects *unstructured.UnstructuredList, gvr schema.GroupVersionResource) {
			page := consumedPage{gvr: gvr}
			for i := range objects.Items {
				page.names = append(page.names, objects.Items[i].GetName())
			}
			consumed = append(consumed, page)
		},
	)
	require.NoError(t, err)
	seedCustomResourceDefinitionCache(t, collector,
		"applications.argoproj.io",
		"clusterwidgets.example.com",
	)

	require.NoError(t, collector.collect(t.Context()))
	assert.Equal(t, []consumedPage{
		{gvr: applicationGVR, names: []string{"included"}},
		{gvr: clusterWidgetGVR, names: []string{"cluster"}},
	}, consumed)
}

func TestCustomResourceCollectorUsesPaginationAndSelectors(t *testing.T) {
	t.Parallel()

	var names []string
	collector := &customResourceCollector{
		config: &CustomResourcesConfig{
			LabelSelector: "team=platform",
			FieldSelector: "metadata.name!=ignored",
		},
		consume: func(_ context.Context, objects *unstructured.UnstructuredList, _ schema.GroupVersionResource) {
			for i := range objects.Items {
				names = append(names, objects.Items[i].GetName())
			}
		},
		listRetryDelay: customResourceListRetryDelay,
	}

	resource := &testPaginatedResource{}
	require.NoError(t, collector.listResource(t.Context(), resource, applicationGVR, false))
	assert.Equal(t, []string{"first", "second"}, names)
	require.Len(t, resource.options, 2)
	assert.Equal(t, customResourcePageSize, resource.options[0].Limit)
	assert.Equal(t, "team=platform", resource.options[0].LabelSelector)
	assert.Equal(t, "metadata.name!=ignored", resource.options[0].FieldSelector)
	assert.Empty(t, resource.options[0].Continue)
	assert.Equal(t, "next", resource.options[1].Continue)
}

func TestCustomResourceCollectorRetriesTransientListErrors(t *testing.T) {
	t.Parallel()

	var names []string
	collector := &customResourceCollector{
		config: &CustomResourcesConfig{},
		consume: func(_ context.Context, objects *unstructured.UnstructuredList, _ schema.GroupVersionResource) {
			for i := range objects.Items {
				names = append(names, objects.Items[i].GetName())
			}
		},
	}

	resource := &testRetryResource{
		errs: []error{
			apierrors.NewTooManyRequests("storage is (re)initializing", 0),
		},
	}
	require.NoError(t, collector.listResource(t.Context(), resource, applicationGVR, false))
	assert.Equal(t, 2, resource.calls)
	assert.Equal(t, []string{"example"}, names)
}

func TestCustomResourceCollectorDoesNotRetryPermanentListErrors(t *testing.T) {
	t.Parallel()

	collector := &customResourceCollector{
		config:  &CustomResourcesConfig{},
		consume: func(context.Context, *unstructured.UnstructuredList, schema.GroupVersionResource) {},
	}

	resource := &testRetryResource{
		errs: []error{
			apierrors.NewForbidden(applicationGVR.GroupResource(), "example", errors.New("forbidden")),
		},
	}
	require.Error(t, collector.listResource(t.Context(), resource, applicationGVR, false))
	assert.Equal(t, 1, resource.calls)
}

func TestCustomResourceCollectorRefreshesDiscoveryAfterCRDChange(t *testing.T) {
	t.Parallel()

	metadataClient := newCustomResourceMetadataClient("applications.argoproj.io")
	discoveryClient := &testCustomResourceDiscovery{
		resources: []*metav1.APIResourceList{
			apiResourceList(applicationGVR, true),
			apiResourceList(clusterWidgetGVR, false),
		},
	}
	collected := make(chan schema.GroupVersionResource, 1)
	collector, err := newCustomResourceCollector(
		&CustomResourcesConfig{
			Interval: 20 * time.Millisecond,
			Include: []CustomResourceSelector{
				{Group: "*", Resources: []string{"*"}},
			},
		},
		newCustomResourceDynamicClient(
			customResource("example.com/v1", "ClusterWidget", "newly-discovered", ""),
		),
		metadataClient,
		discoveryClient,
		nil,
		zaptest.NewLogger(t),
		func(_ context.Context, _ *unstructured.UnstructuredList, gvr schema.GroupVersionResource) {
			select {
			case collected <- gvr:
			default:
			}
		},
	)
	require.NoError(t, err)

	var wg sync.WaitGroup
	stop, err := collector.Start(t.Context(), &wg)
	require.NoError(t, err)
	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})

	require.Eventually(t, func() bool {
		return discoveryClient.calls.Load() == 1
	}, 5*time.Second, 10*time.Millisecond)
	assert.Never(t, func() bool {
		return discoveryClient.calls.Load() > 1
	}, 100*time.Millisecond, 10*time.Millisecond)

	fakeMetadataClient := metadataClient.(*fakemetadata.FakeMetadataClient)
	require.NoError(t, fakeMetadataClient.Tracker().Create(
		customResourceDefinitionGVR,
		customResourceDefinition("clusterwidgets.example.com"),
		"",
	))
	require.Eventually(t, func() bool {
		return discoveryClient.calls.Load() == 2
	}, 5*time.Second, 10*time.Millisecond)
	select {
	case gvr := <-collected:
		assert.Equal(t, clusterWidgetGVR, gvr)
	case <-time.After(5 * time.Second):
		t.Fatal("newly discovered custom resource was not collected")
	}
}

func TestCustomResourceCollectorRefreshesPreferredVersion(t *testing.T) {
	t.Parallel()

	applicationV1GVR := schema.GroupVersionResource{
		Group: "argoproj.io", Version: "v1", Resource: "applications",
	}
	discoveryClient := &testCustomResourceDiscovery{
		resources: []*metav1.APIResourceList{
			apiResourceList(applicationGVR, true),
		},
	}
	collector, err := newCustomResourceCollector(
		&CustomResourcesConfig{
			Interval: time.Hour,
			Include: []CustomResourceSelector{
				{Group: applicationGVR.Group, Resources: []string{applicationGVR.Resource}},
			},
		},
		newCustomResourceDynamicClient(),
		newCustomResourceMetadataClient(),
		discoveryClient,
		nil,
		zaptest.NewLogger(t),
		func(context.Context, *unstructured.UnstructuredList, schema.GroupVersionResource) {},
	)
	require.NoError(t, err)
	seedCustomResourceDefinitionCache(t, collector, "applications.argoproj.io")

	require.NoError(t, collector.refreshDiscovery())
	assert.Equal(t, []discoveredCustomResource{
		{gvr: applicationGVR, namespaced: true},
	}, collector.discovered)

	discoveryClient.resources = []*metav1.APIResourceList{
		apiResourceList(applicationV1GVR, true),
	}
	collector.discoveryDirty.Store(true)

	require.NoError(t, collector.refreshDiscovery())
	assert.Equal(t, []discoveredCustomResource{
		{gvr: applicationV1GVR, namespaced: true},
	}, collector.discovered)
}

func TestCustomResourceCollectorRemovesDeletedCRD(t *testing.T) {
	t.Parallel()

	discoveryClient := newCustomResourceDiscovery(apiResourceList(applicationGVR, true))
	collector, err := newCustomResourceCollector(
		&CustomResourcesConfig{
			Interval: time.Hour,
			Include: []CustomResourceSelector{
				{Group: applicationGVR.Group, Resources: []string{applicationGVR.Resource}},
			},
		},
		newCustomResourceDynamicClient(),
		newCustomResourceMetadataClient(),
		discoveryClient,
		nil,
		zaptest.NewLogger(t),
		func(context.Context, *unstructured.UnstructuredList, schema.GroupVersionResource) {},
	)
	require.NoError(t, err)
	crd := customResourceDefinition("applications.argoproj.io")
	require.NoError(t, collector.crdInformer.GetStore().Add(crd))

	require.NoError(t, collector.refreshDiscovery())
	require.Len(t, collector.discovered, 1)

	require.NoError(t, collector.crdInformer.GetStore().Delete(crd))
	collector.discoveryDirty.Store(true)

	require.NoError(t, collector.refreshDiscovery())
	assert.Empty(t, collector.discovered)
}

func TestCustomResourceCollectorUsesPartialDiscoveryAndRetries(t *testing.T) {
	t.Parallel()

	discoveryClient := &testCustomResourceDiscovery{
		resources: []*metav1.APIResourceList{
			apiResourceList(applicationGVR, true),
		},
		err: errors.New("another API group is unavailable"),
	}
	collector, err := newCustomResourceCollector(
		&CustomResourcesConfig{
			Interval: time.Hour,
			Include: []CustomResourceSelector{
				{Group: "*", Resources: []string{"*"}},
			},
		},
		newCustomResourceDynamicClient(),
		newCustomResourceMetadataClient(),
		discoveryClient,
		nil,
		zaptest.NewLogger(t),
		func(context.Context, *unstructured.UnstructuredList, schema.GroupVersionResource) {},
	)
	require.NoError(t, err)
	seedCustomResourceDefinitionCache(t, collector, "applications.argoproj.io")

	require.NoError(t, collector.refreshDiscovery())
	assert.Equal(t, []discoveredCustomResource{
		{gvr: applicationGVR, namespaced: true},
	}, collector.discovered)
	assert.True(t, collector.discoveryDirty.Load())

	discoveryClient.err = nil
	require.NoError(t, collector.refreshDiscovery())
	assert.Equal(t, int64(2), discoveryClient.calls.Load())
	assert.False(t, collector.discoveryDirty.Load())
}

func TestCustomResourceCollectorRetriesFailedDiscovery(t *testing.T) {
	t.Parallel()

	discoveryClient := &testCustomResourceDiscovery{
		err: errors.New("discovery unavailable"),
	}
	collector, err := newCustomResourceCollector(
		&CustomResourcesConfig{
			Interval: time.Hour,
			Include: []CustomResourceSelector{
				{Group: "*", Resources: []string{"*"}},
			},
		},
		newCustomResourceDynamicClient(),
		newCustomResourceMetadataClient(),
		discoveryClient,
		nil,
		zaptest.NewLogger(t),
		func(context.Context, *unstructured.UnstructuredList, schema.GroupVersionResource) {},
	)
	require.NoError(t, err)
	seedCustomResourceDefinitionCache(t, collector, "applications.argoproj.io")

	require.ErrorContains(t, collector.refreshDiscovery(), "failed to discover preferred Kubernetes resources")
	assert.True(t, collector.discoveryDirty.Load())

	discoveryClient.resources = []*metav1.APIResourceList{
		apiResourceList(applicationGVR, true),
	}
	discoveryClient.err = nil
	require.NoError(t, collector.refreshDiscovery())
	assert.Equal(t, []discoveredCustomResource{
		{gvr: applicationGVR, namespaced: true},
	}, collector.discovered)
}

func TestCustomResourceCollectorContinuesAfterNamespaceError(t *testing.T) {
	t.Parallel()

	dynamicClient := newCustomResourceDynamicClient(
		customResource("argoproj.io/v1alpha1", "Application", "example", "available"),
	)
	fakeClient := dynamicClient.(*fakedynamic.FakeDynamicClient)
	fakeClient.PrependReactor("list", "applications", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetNamespace() == "unavailable" {
			return true, nil, errors.New("forbidden")
		}
		return false, nil, nil
	})

	var names []string
	collector := &customResourceCollector{
		config: &CustomResourcesConfig{
			Namespaces: []string{"unavailable", "available"},
		},
		dynamicClient: dynamicClient,
		consume: func(_ context.Context, objects *unstructured.UnstructuredList, _ schema.GroupVersionResource) {
			for i := range objects.Items {
				names = append(names, objects.Items[i].GetName())
			}
		},
	}

	err := collector.collectResource(t.Context(), discoveredCustomResource{
		gvr:        applicationGVR,
		namespaced: true,
	})
	require.ErrorContains(t, err, `namespace "unavailable": forbidden`)
	assert.Equal(t, []string{"example"}, names)
}

func TestCustomResourcesOnlyReceiver(t *testing.T) {
	t.Parallel()

	dynamicClient := newCustomResourceDynamicClient(
		customResource("argoproj.io/v1alpha1", "Application", "example", "default"),
	)
	metadataClient := newCustomResourceMetadataClient("applications.argoproj.io")
	discoveryClient := newCustomResourceDiscovery(apiResourceList(applicationGVR, true))

	cfg := createDefaultConfig().(*Config)
	cfg.CustomResources = &CustomResourcesConfig{
		Interval: time.Hour,
		Include: []CustomResourceSelector{
			{Group: applicationGVR.Group, Resources: []string{applicationGVR.Resource}},
		},
	}
	setCustomResourceClients(cfg, dynamicClient, metadataClient, discoveryClient)
	require.NoError(t, cfg.Validate())

	consumer := newMockLogConsumer()
	receiver, err := newReceiver(receivertest.NewNopSettings(metadata.Type), cfg, consumer)
	require.NoError(t, err)
	require.NoError(t, receiver.Start(t.Context(), componenttest.NewNopHost()))
	require.Eventually(t, func() bool {
		return consumer.Count() == 1
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, receiver.Shutdown(t.Context()))

	logs := consumer.Logs()
	require.Len(t, logs, 1)
	resourceLogs := logs[0].ResourceLogs()
	require.Equal(t, 1, resourceLogs.Len())
	namespace, ok := resourceLogs.At(0).Resource().Attributes().Get("k8s.namespace.name")
	require.True(t, ok)
	assert.Equal(t, "default", namespace.Str())
	record := resourceLogs.At(0).ScopeLogs().At(0).LogRecords().At(0)
	resourceName, ok := record.Attributes().Get("k8s.resource.name")
	require.True(t, ok)
	assert.Equal(t, "applications", resourceName.Str())
	body := record.Body().AsRaw()
	assert.Equal(t, "example", body.(map[string]any)["metadata"].(map[string]any)["name"])
}

func TestCustomResourcesOnlyReceiverWithLeaderElection(t *testing.T) {
	t.Parallel()

	dynamicClient := newCustomResourceDynamicClient(
		customResource("argoproj.io/v1alpha1", "Application", "example", "default"),
	)
	metadataClient := newCustomResourceMetadataClient("applications.argoproj.io")
	discoveryClient := newCustomResourceDiscovery(apiResourceList(applicationGVR, true))
	leaderElectorID := component.MustNewID("k8s_leader_elector")
	fakeLeaderElection := &k8sleaderelectortest.FakeLeaderElection{}
	host := &k8sleaderelectortest.FakeHost{FakeLeaderElection: fakeLeaderElection}

	cfg := createDefaultConfig().(*Config)
	cfg.CustomResources = &CustomResourcesConfig{
		Interval: time.Hour,
		Include: []CustomResourceSelector{
			{Group: applicationGVR.Group, Resources: []string{applicationGVR.Resource}},
		},
	}
	cfg.K8sLeaderElector = &leaderElectorID
	setCustomResourceClients(cfg, dynamicClient, metadataClient, discoveryClient)
	require.NoError(t, cfg.Validate())

	sink := new(consumertest.LogsSink)
	receiver, err := newReceiver(receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, receiver.Start(t.Context(), host))

	assert.Never(t, func() bool {
		return sink.LogRecordCount() != 0
	}, 100*time.Millisecond, 10*time.Millisecond)

	fakeLeaderElection.InvokeOnLeading()
	require.Eventually(t, func() bool {
		return sink.LogRecordCount() == 1
	}, 5*time.Second, 10*time.Millisecond)

	fakeLeaderElection.InvokeOnStopping()
	assert.Never(t, func() bool {
		return sink.LogRecordCount() != 1
	}, 100*time.Millisecond, 10*time.Millisecond)

	fakeLeaderElection.InvokeOnLeading()
	require.Eventually(t, func() bool {
		return sink.LogRecordCount() == 2
	}, 5*time.Second, 10*time.Millisecond)

	require.NoError(t, receiver.Shutdown(t.Context()))
}

func TestExplicitCustomResourceIsNotCollectedTwice(t *testing.T) {
	t.Parallel()

	dynamicClient := newCustomResourceDynamicClient(
		customResource("argoproj.io/v1alpha1", "Application", "example", "default"),
	)
	metadataClient := newCustomResourceMetadataClient("applications.argoproj.io")
	discoveryClient := newCustomResourceDiscovery(apiResourceList(applicationGVR, true))

	cfg := createDefaultConfig().(*Config)
	cfg.Objects = []*K8sObjectsConfig{
		{
			Name:     applicationGVR.Resource,
			Group:    applicationGVR.Group,
			Mode:     "pull",
			Interval: time.Hour,
		},
	}
	cfg.CustomResources = &CustomResourcesConfig{
		Interval: time.Hour,
		Include: []CustomResourceSelector{
			{Group: applicationGVR.Group, Resources: []string{applicationGVR.Resource}},
		},
	}
	setCustomResourceClients(cfg, dynamicClient, metadataClient, discoveryClient)
	require.NoError(t, cfg.Validate())

	consumer := newMockLogConsumer()
	receiver, err := newReceiver(receivertest.NewNopSettings(metadata.Type), cfg, consumer)
	require.NoError(t, err)
	require.NoError(t, receiver.Start(t.Context(), componenttest.NewNopHost()))
	require.Eventually(t, func() bool {
		return consumer.Count() == 1
	}, 5*time.Second, 10*time.Millisecond)
	assert.Never(t, func() bool {
		return consumer.Count() > 1
	}, 100*time.Millisecond, 10*time.Millisecond)
	require.NoError(t, receiver.Shutdown(t.Context()))
}

func TestDisabledCustomResourcesCreatesNoAdditionalClients(t *testing.T) {
	t.Parallel()

	mockClient := newMockDynamicClient()
	cfg := createDefaultConfig().(*Config)
	cfg.Objects = []*K8sObjectsConfig{
		{Name: "pods", Mode: "pull", Interval: time.Hour},
	}
	cfg.makeDynamicClient = mockClient.getMockDynamicClient
	cfg.makeDiscoveryClient = getMockDiscoveryClient
	customResourceClientsCreated := false
	cfg.makeKubernetesClients = func() (kubernetesClients, error) {
		customResourceClientsCreated = true
		return kubernetesClients{}, errors.New("unexpected custom resource client creation")
	}

	receiver, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		newMockLogConsumer(),
	)
	require.NoError(t, err)
	require.NoError(t, receiver.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, receiver.Shutdown(t.Context()))
	assert.False(t, customResourceClientsCreated)
}

func TestCustomResourceCollectorStops(t *testing.T) {
	t.Parallel()

	collector, err := newCustomResourceCollector(
		&CustomResourcesConfig{
			Interval:     2 * time.Hour,
			InitialDelay: time.Hour,
			Include: []CustomResourceSelector{
				{Group: "*", Resources: []string{"*"}},
			},
		},
		newCustomResourceDynamicClient(),
		newCustomResourceMetadataClient(),
		newCustomResourceDiscovery(),
		nil,
		zaptest.NewLogger(t),
		func(context.Context, *unstructured.UnstructuredList, schema.GroupVersionResource) {},
	)
	require.NoError(t, err)

	var wg sync.WaitGroup
	stop, err := collector.Start(t.Context(), &wg)
	require.NoError(t, err)
	close(stop)
	stopped := make(chan struct{})
	go func() {
		wg.Wait()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("custom resource collector did not stop")
	}
}

func TestCustomResourceCollectorDoesNotOverlapCollectionCycles(t *testing.T) {
	t.Parallel()

	dynamicClient := newCustomResourceDynamicClient()
	fakeDynamicClient := dynamicClient.(*fakedynamic.FakeDynamicClient)
	firstListStarted := make(chan struct{})
	releaseFirstList := make(chan struct{})
	var listCalls atomic.Int64
	fakeDynamicClient.PrependReactor(
		"list",
		applicationGVR.Resource,
		func(k8stesting.Action) (bool, runtime.Object, error) {
			if listCalls.Add(1) == 1 {
				close(firstListStarted)
				<-releaseFirstList
			}
			return true, &unstructured.UnstructuredList{}, nil
		},
	)

	collector, err := newCustomResourceCollector(
		&CustomResourcesConfig{
			Interval: 10 * time.Millisecond,
			Include: []CustomResourceSelector{
				{Group: "*", Resources: []string{"*"}},
			},
		},
		dynamicClient,
		newCustomResourceMetadataClient("applications.argoproj.io"),
		newCustomResourceDiscovery(apiResourceList(applicationGVR, true)),
		nil,
		zaptest.NewLogger(t),
		func(context.Context, *unstructured.UnstructuredList, schema.GroupVersionResource) {},
	)
	require.NoError(t, err)

	var wg sync.WaitGroup
	stop, err := collector.Start(t.Context(), &wg)
	require.NoError(t, err)
	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})

	select {
	case <-firstListStarted:
	case <-time.After(time.Second):
		t.Fatal("first collection cycle did not start")
	}
	assert.Never(t, func() bool {
		return listCalls.Load() > 1
	}, 50*time.Millisecond, 5*time.Millisecond)

	close(releaseFirstList)
	require.Eventually(t, func() bool {
		return listCalls.Load() == 2
	}, time.Second, 5*time.Millisecond)
}

func TestCustomResourceCollectorRequiresCRDListAndWatchPermissions(t *testing.T) {
	tests := []struct {
		name          string
		configure     func(*fakemetadata.FakeMetadataClient)
		expectedError string
	}{
		{
			name: "list",
			configure: func(client *fakemetadata.FakeMetadataClient) {
				client.PrependReactor(
					"list",
					customResourceDefinitionGVR.Resource,
					func(k8stesting.Action) (bool, runtime.Object, error) {
						return true, nil, apierrors.NewForbidden(
							customResourceDefinitionGVR.GroupResource(),
							"",
							errors.New("missing permission"),
						)
					},
				)
			},
			expectedError: "failed to list custom resource definitions",
		},
		{
			name: "watch",
			configure: func(client *fakemetadata.FakeMetadataClient) {
				client.PrependWatchReactor(
					customResourceDefinitionGVR.Resource,
					func(k8stesting.Action) (bool, watch.Interface, error) {
						return true, nil, apierrors.NewForbidden(
							customResourceDefinitionGVR.GroupResource(),
							"",
							errors.New("missing permission"),
						)
					},
				)
			},
			expectedError: "failed to watch custom resource definitions",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metadataClient := newCustomResourceMetadataClient()
			test.configure(metadataClient.(*fakemetadata.FakeMetadataClient))

			collector, err := newCustomResourceCollector(
				&CustomResourcesConfig{
					Interval: time.Hour,
					Include: []CustomResourceSelector{
						{Group: "*", Resources: []string{"*"}},
					},
				},
				newCustomResourceDynamicClient(),
				metadataClient,
				newCustomResourceDiscovery(),
				nil,
				zaptest.NewLogger(t),
				func(context.Context, *unstructured.UnstructuredList, schema.GroupVersionResource) {},
			)
			require.NoError(t, err)
			collector.cacheSyncTimeout = 50 * time.Millisecond

			var wg sync.WaitGroup
			stop, err := collector.Start(t.Context(), &wg)
			assert.Nil(t, stop)
			require.ErrorContains(t, err, test.expectedError)

			stopped := make(chan struct{})
			go func() {
				wg.Wait()
				close(stopped)
			}()
			select {
			case <-stopped:
			case <-time.After(time.Second):
				t.Fatal("custom resource collector did not stop after informer startup failure")
			}
		})
	}
}

func newCustomResourceMetadataClient(names ...string) k8smetadata.Interface {
	scheme := fakemetadata.NewTestScheme()
	if err := metav1.AddMetaToScheme(scheme); err != nil {
		panic(err)
	}
	objects := make([]runtime.Object, 0, len(names))
	for _, name := range names {
		objects = append(objects, customResourceDefinition(name))
	}
	return fakemetadata.NewSimpleMetadataClient(scheme, objects...)
}

func setCustomResourceClients(
	cfg *Config,
	dynamicClient dynamic.Interface,
	metadataClient k8smetadata.Interface,
	discoveryClient discovery.ServerResourcesInterface,
) {
	cfg.makeKubernetesClients = func() (kubernetesClients, error) {
		return kubernetesClients{
			dynamic:   dynamicClient,
			metadata:  metadataClient,
			discovery: discoveryClient,
		}, nil
	}
}

func seedCustomResourceDefinitionCache(t *testing.T, collector *customResourceCollector, names ...string) {
	t.Helper()
	for _, name := range names {
		require.NoError(t, collector.crdInformer.GetStore().Add(customResourceDefinition(name)))
	}
}

func customResourceDefinition(name string) *metav1.PartialObjectMetadata {
	return &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
}

func newCustomResourceDynamicClient(objects ...runtime.Object) dynamic.Interface {
	return fakedynamic.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			applicationGVR:   "ApplicationList",
			clusterWidgetGVR: "ClusterWidgetList",
		},
		objects...,
	)
}

func newCustomResourceDiscovery(resources ...*metav1.APIResourceList) discovery.ServerResourcesInterface {
	return &testCustomResourceDiscovery{resources: resources}
}

func apiResourceList(gvr schema.GroupVersionResource, namespaced bool) *metav1.APIResourceList {
	return &metav1.APIResourceList{
		GroupVersion: gvr.GroupVersion().String(),
		APIResources: []metav1.APIResource{
			{
				Name:       gvr.Resource,
				Namespaced: namespaced,
				Verbs:      metav1.Verbs{"get", "list"},
			},
		},
	}
}

func customResource(apiVersion, kind, name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
}
