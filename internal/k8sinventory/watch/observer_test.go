// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiWatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	k8s_testing "k8s.io/client-go/testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory"
)

func TestObserver(t *testing.T) {
	mockClient := newMockDynamicClient()
	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{
			"environment": "production",
		}, "1"),
	)

	cfg := Config{
		Config: k8sinventory.Config{
			Gvr: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespaces: []string{"default"},
		},
	}

	receivedEventsChan := make(chan *apiWatch.Event)

	obs, err := New(mockClient, cfg, zap.NewNop(), nil, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

	time.Sleep(time.Millisecond * 100)

	mockClient.createPods(
		generatePod("pod2", "default", map[string]any{
			"environment": "test",
		}, "2"),
		generatePod("pod3", "default_ignore", map[string]any{
			"environment": "production",
		}, "3"),
		generatePod("pod4", "default", map[string]any{
			"environment": "production",
		}, "4"),
	)

	verifyReceivedEvents(t, 2, receivedEventsChan, stopChan)

	wg.Wait()
}

func TestObserverWithInitialState(t *testing.T) {
	mockClient := newMockDynamicClient()
	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{
			"environment": "production",
		}, "1"),
	)

	cfg := Config{
		Config: k8sinventory.Config{
			Gvr: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespaces: []string{"default"},
		},
		IncludeInitialState: true,
	}

	receivedEventsChan := make(chan *apiWatch.Event)

	obs, err := New(mockClient, cfg, zap.NewNop(), nil, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

	verifyReceivedEvents(t, 1, receivedEventsChan, stopChan)

	wg.Wait()
}

func TestObserverExcludeDelete(t *testing.T) {
	mockClient := newMockDynamicClient()

	cfg := Config{
		Config: k8sinventory.Config{
			Gvr: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespaces: []string{"default"},
		},
		IncludeInitialState: true,
		Exclude: map[apiWatch.EventType]bool{
			apiWatch.Deleted: true,
		},
	}

	receivedEventsChan := make(chan *apiWatch.Event)

	obs, err := New(mockClient, cfg, zap.NewNop(), nil, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

	<-time.After(time.Millisecond * 100)

	pod := generatePod("pod1", "default", map[string]any{
		"environment": "production",
	}, "1")

	// create and delete the pod - only the creation event should be received
	mockClient.createPods(pod)
	mockClient.deletePods(pod)

	verifyReceivedEvents(t, 1, receivedEventsChan, stopChan)

	wg.Wait()
}

func TestObserverEmptyNamespaces(t *testing.T) {
	mockClient := newMockDynamicClient()

	cfg := Config{
		Config: k8sinventory.Config{
			Gvr: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespaces: []string{}, // empty to watch all namespaces
		},
	}

	receivedEventsChan := make(chan *apiWatch.Event)

	obs, err := New(mockClient, cfg, zap.NewNop(), nil, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

	time.Sleep(time.Millisecond * 100)

	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{"env": "test"}, "1"),
		generatePod("pod2", "other", map[string]any{"env": "prod"}, "2"),
	)

	verifyReceivedEvents(t, 2, receivedEventsChan, stopChan)

	wg.Wait()
}

func TestObserverMultipleNamespaces(t *testing.T) {
	mockClient := newMockDynamicClient()

	cfg := Config{
		Config: k8sinventory.Config{
			Gvr: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespaces: []string{"default", "other"},
		},
	}

	receivedEventsChan := make(chan *apiWatch.Event)

	obs, err := New(mockClient, cfg, zap.NewNop(), nil, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

	time.Sleep(time.Millisecond * 100)

	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{"env": "test"}, "1"),
		generatePod("pod2", "other", map[string]any{"env": "prod"}, "2"),
		generatePod("pod3", "ignored", map[string]any{"env": "dev"}, "3"),
	)

	verifyReceivedEvents(t, 2, receivedEventsChan, stopChan)

	wg.Wait()
}

func TestObserverWithSelectors(t *testing.T) {
	mockClient := newMockDynamicClient()

	cfg := Config{
		Config: k8sinventory.Config{
			Gvr: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespaces:      []string{"default"},
			LabelSelector:   "environment=test",
			FieldSelector:   "",
			ResourceVersion: "",
		},
	}

	receivedEventsChan := make(chan *apiWatch.Event)

	obs, err := New(mockClient, cfg, zap.NewNop(), nil, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

	time.Sleep(time.Millisecond * 100)

	// Since fake client doesn't filter, it will return all, but the code path is covered
	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{"environment": "test"}, "1"),
		generatePod("pod2", "default", map[string]any{"environment": "prod"}, "2"),
	)

	verifyReceivedEvents(t, 2, receivedEventsChan, stopChan)

	wg.Wait()
}

func TestObserverInitialStateError(t *testing.T) {
	mockClient := newMockDynamicClient()

	// Make list return error for initial state
	mockClient.client.(*fake.FakeDynamicClient).PrependReactor("list", "pods", func(_ k8s_testing.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("mock list error")
	})

	cfg := Config{
		Config: k8sinventory.Config{
			Gvr: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespaces: []string{"default"},
		},
		IncludeInitialState: true,
	}

	receivedEventsChan := make(chan *apiWatch.Event)

	obs, err := New(mockClient, cfg, zap.NewNop(), nil, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

	time.Sleep(time.Millisecond * 100)

	// No events should be received due to error
	select {
	case <-receivedEventsChan:
		t.Fatal("unexpected event received")
	case <-time.After(100 * time.Millisecond):
		// ok
	}

	close(stopChan)

	wg.Wait()
}

func TestObserverInitialStateNoObjects(t *testing.T) {
	mockClient := newMockDynamicClient()

	cfg := Config{
		Config: k8sinventory.Config{
			Gvr: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespaces: []string{"default"},
		},
		IncludeInitialState: true,
	}

	receivedEventsChan := make(chan *apiWatch.Event)

	obs, err := New(mockClient, cfg, zap.NewNop(), nil, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

	time.Sleep(time.Millisecond * 100)

	// No events since no objects
	select {
	case <-receivedEventsChan:
		t.Fatal("unexpected event received")
	case <-time.After(100 * time.Millisecond):
		// ok
	}

	close(stopChan)

	wg.Wait()
}

// TestSendInitialStateReturnsListRV verifies that sendInitialState returns the
// list's own ResourceVersion, not just the highest individual object RV.
// The list RV is always >= any individual object RV and is the correct starting
// point for the subsequent watch to avoid a race window between two List calls.
func TestSendInitialStateReturnsListRV(t *testing.T) {
	mockClient := newMockDynamicClient()
	// Set list RV to "999", higher than any individual object RV below.
	mockClient.setListResourceVersion("999")
	mockClient.createPods(
		generatePod("pod1", "default", nil, "100"),
		generatePod("pod2", "default", nil, "200"),
	)

	cfg := Config{
		Config: k8sinventory.Config{
			Gvr:        schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Namespaces: []string{"default"},
		},
		IncludeInitialState: true,
	}

	obs, err := New(mockClient, cfg, zap.NewNop(), nil, nil)
	require.NoError(t, err)

	resource := mockClient.Resource(cfg.Gvr)
	listRV := obs.sendInitialState(t.Context(), resource.Namespace("default"), "default", func(string) {})
	assert.Equal(t, "999", listRV, "sendInitialState should return the list's own ResourceVersion")
}

// TestInitialStateListRVPersistedAsCheckpoint verifies that after sendInitialState
// the checkpoint is updated with the list's RV (via setLatestRV in startWatch),
// which is more accurate than the highest individual object RV.
func TestInitialStateListRVPersistedAsCheckpoint(t *testing.T) {
	mockClient := newMockDynamicClient()
	storageClient := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")

	// List RV "999" is higher than any individual pod RV — after startup the
	// checkpoint should hold "999", not "200".
	mockClient.setListResourceVersion("999")
	mockClient.createPods(
		generatePod("pod1", "default", nil, "100"),
		generatePod("pod2", "default", nil, "200"),
	)

	cfg := Config{
		Config: k8sinventory.Config{
			Gvr:        schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Namespaces: []string{"default"},
		},
		IncludeInitialState: true,
	}

	obs, err := New(mockClient, cfg, zap.NewNop(), storageClient, nil)
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	stopChan := obs.Start(t.Context(), &wg)

	time.Sleep(200 * time.Millisecond)

	close(stopChan)
	wg.Wait()

	cp := newCheckpointer(storageClient, zap.NewNop())
	rv, err := cp.GetCheckpoint(t.Context(), "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "999", rv, "checkpoint should hold the list RV, not a lower individual object RV")
}

// TestSendInitialStateUnparsableRVEmitsEvent verifies that when an object has a
// non-integer resourceVersion (which cannot be compared against the persisted RV),
// the event is still emitted rather than silently dropped. Emitting a potential
// duplicate is safer than missing an event.
func TestSendInitialStateUnparsableRVEmitsEvent(t *testing.T) {
	mockClient := newMockDynamicClient()
	storageClient := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")

	// Persist a checkpoint so the deduplication path is active.
	cp := newCheckpointer(storageClient, zap.NewNop())
	require.NoError(t, cp.SetCheckpoint(t.Context(), "default", "pods", "100"))
	require.NoError(t, cp.Flush(t.Context()))

	// pod1 has a valid RV <= persisted (should be skipped).
	// pod2 has an unparsable RV (should be emitted despite parse failure).
	// pod3 has a valid RV > persisted (should be emitted normally).
	mockClient.createPods(
		generatePod("pod1", "default", nil, "50"),
		generatePod("pod2", "default", nil, "not-a-number"),
		generatePod("pod3", "default", nil, "200"),
	)

	cfg := Config{
		Config: k8sinventory.Config{
			Gvr:        schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Namespaces: []string{"default"},
		},
	}

	obs, err := New(mockClient, cfg, zap.NewNop(), storageClient, nil)
	require.NoError(t, err)

	var emitted []string
	obs.handleWatchEventFunc = func(event *apiWatch.Event) {
		obj, ok := event.Object.(*unstructured.Unstructured)
		require.True(t, ok)
		emitted = append(emitted, obj.GetName())
	}

	resource := mockClient.Resource(cfg.Gvr)
	obs.sendInitialState(t.Context(), resource.Namespace("default"), "default", func(string) {})

	assert.ElementsMatch(t, []string{"pod2", "pod3"}, emitted,
		"pod1 (rv<=persisted) must be skipped; pod2 (unparsable rv) and pod3 (rv>persisted) must be emitted")
}

func verifyReceivedEvents(t *testing.T, numEvents int, receivedEventsChan chan *apiWatch.Event, stopChan chan struct{}) {
	receivedEvents := 0

	exit := false
	for {
		select {
		case <-receivedEventsChan:
			receivedEvents++
			if receivedEvents == numEvents {
				exit = true
			}
		case <-time.After(10 * time.Second):
			t.Log("timed out waiting for expected events")
			t.Fail()
			exit = true
		}
		if exit {
			break
		}
	}

	close(stopChan)
}

type mockDynamicClient struct {
	client dynamic.Interface
}

func (c mockDynamicClient) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return c.client.Resource(resource)
}

func newMockDynamicClient() mockDynamicClient {
	scheme := runtime.NewScheme()
	objs := []runtime.Object{}

	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}: "PodList",
	}

	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objs...)
	return mockDynamicClient{
		client: fakeClient,
	}
}

func (c mockDynamicClient) createPods(objects ...*unstructured.Unstructured) {
	pods := c.client.Resource(schema.GroupVersionResource{
		Version:  "v1",
		Resource: "pods",
	})
	for _, pod := range objects {
		_, _ = pods.Namespace(pod.GetNamespace()).Create(context.Background(), pod, v1.CreateOptions{})
	}
}

func (c mockDynamicClient) deletePods(objects ...*unstructured.Unstructured) {
	pods := c.client.Resource(schema.GroupVersionResource{
		Version:  "v1",
		Resource: "pods",
	})
	for _, pod := range objects {
		_ = pods.Namespace(pod.GetNamespace()).Delete(context.Background(), pod.GetName(), v1.DeleteOptions{})
	}
}

// setListResourceVersion creates a new mock client with a custom List reactor
func (c *mockDynamicClient) setListResourceVersion(resourceVersion string) {
	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}: "PodList",
	}

	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)

	// Add reactor to set resourceVersion on list operations
	fakeClient.PrependReactor("list", "*", func(_ k8s_testing.Action) (handled bool, ret runtime.Object, err error) {
		// Don't handle, let default action occur
		return false, nil, nil
	})

	fakeClient.PrependWatchReactor("*", func(_ k8s_testing.Action) (handled bool, ret apiWatch.Interface, err error) {
		// Don't handle, let default action occur
		return false, nil, nil
	})

	// Wrap to intercept List calls
	c.client = &listResourceVersionInterceptor{
		Interface:       fakeClient,
		resourceVersion: resourceVersion,
	}
}

// listResourceVersionInterceptor wraps a dynamic client to set resourceVersion on List results
type listResourceVersionInterceptor struct {
	dynamic.Interface
	resourceVersion string
}

func (l *listResourceVersionInterceptor) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &namespacedResourceInterceptor{
		NamespaceableResourceInterface: l.Interface.Resource(resource),
		resourceVersion:                l.resourceVersion,
	}
}

type namespacedResourceInterceptor struct {
	dynamic.NamespaceableResourceInterface
	resourceVersion string
}

func (n *namespacedResourceInterceptor) Namespace(ns string) dynamic.ResourceInterface {
	return &resourceInterceptor{
		ResourceInterface: n.NamespaceableResourceInterface.Namespace(ns),
		resourceVersion:   n.resourceVersion,
	}
}

func (n *namespacedResourceInterceptor) List(ctx context.Context, opts v1.ListOptions) (*unstructured.UnstructuredList, error) {
	list, err := n.NamespaceableResourceInterface.List(ctx, opts)
	if err == nil && list != nil {
		list.SetResourceVersion(n.resourceVersion)
	}
	return list, err
}

type resourceInterceptor struct {
	dynamic.ResourceInterface
	resourceVersion string
}

func (r *resourceInterceptor) List(ctx context.Context, opts v1.ListOptions) (*unstructured.UnstructuredList, error) {
	list, err := r.ResourceInterface.List(ctx, opts)
	if err == nil && list != nil {
		list.SetResourceVersion(r.resourceVersion)
	}
	return list, err
}

func generatePod(name, namespace string, labels map[string]any, resourceVersion string) *unstructured.Unstructured {
	pod := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pods",
			"metadata": map[string]any{
				"namespace": namespace,
				"name":      name,
				"labels":    labels,
			},
		},
	}

	pod.SetResourceVersion(resourceVersion)
	return &pod
}

func TestObserverWithPersistence(t *testing.T) {
	mockClient := newMockDynamicClient()
	storageClient := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")

	cfg := Config{
		Config: k8sinventory.Config{
			Gvr: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespaces: []string{"default"},
		},
	}

	receivedEventsChan := make(chan *apiWatch.Event, 10)

	obs, err := New(mockClient, cfg, zap.NewNop(), storageClient, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})

	require.NoError(t, err)
	require.NotNil(t, obs.checkpointer)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

	time.Sleep(time.Millisecond * 100)

	// Create a pod
	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{"env": "test"}, "100"),
	)

	// Wait for event
	select {
	case <-receivedEventsChan:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Stop the observer; the deferred final flush in startCheckpointFlusher
	// ensures the latest resourceVersion is persisted before wg.Wait() returns.
	close(stopChan)
	wg.Wait()

	// Verify resourceVersion was persisted
	checkpointer := newCheckpointer(storageClient, zap.NewNop())
	rv, err := checkpointer.GetCheckpoint(t.Context(), "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "100", rv)
}

func TestObserverWithoutStorage(t *testing.T) {
	mockClient := newMockDynamicClient()

	cfg := Config{
		Config: k8sinventory.Config{
			Gvr: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespaces: []string{"default"},
		},
	}

	receivedEventsChan := make(chan *apiWatch.Event, 10)

	// No storage client passed — checkpointer should not be initialized
	obs, err := New(mockClient, cfg, zap.NewNop(), nil, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})

	require.NoError(t, err)
	assert.Nil(t, obs.checkpointer)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

	time.Sleep(time.Millisecond * 100)

	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{"env": "test"}, "100"),
	)

	select {
	case <-receivedEventsChan:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}

	close(stopChan)
	wg.Wait()
}

func TestObserverPersistenceNilStorage(t *testing.T) {
	mockClient := newMockDynamicClient()

	cfg := Config{
		Config: k8sinventory.Config{
			Gvr: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespaces: []string{"default"},
		},
	}

	receivedEventsChan := make(chan *apiWatch.Event, 10)

	// Pass nil storage client
	obs, err := New(mockClient, cfg, zap.NewNop(), nil, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})

	require.NoError(t, err)
	assert.Nil(t, obs.checkpointer) // Should not be initialized with nil storage

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

	time.Sleep(time.Millisecond * 100)

	// Create a pod
	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{"env": "test"}, "100"),
	)

	// Should still work without errors
	select {
	case <-receivedEventsChan:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}

	close(stopChan)
	wg.Wait()
}

func TestObserverPersistenceClusterWideWatch(t *testing.T) {
	mockClient := newMockDynamicClient()
	storageClient := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")

	cfg := Config{
		Config: k8sinventory.Config{
			Gvr: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespaces: []string{}, // Empty - cluster-wide watch
		},
	}

	receivedEventsChan := make(chan *apiWatch.Event, 10)

	obs, err := New(mockClient, cfg, zap.NewNop(), storageClient, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

	time.Sleep(time.Millisecond * 100)

	// Create pods in different namespaces
	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{"env": "test"}, "100"),
		generatePod("pod2", "other", map[string]any{"env": "prod"}, "101"),
	)

	// Wait for events
	for i := range 2 {
		select {
		case <-receivedEventsChan:
			// success
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for event %d", i+1)
		}
	}

	time.Sleep(time.Millisecond * 100)

	close(stopChan)
	wg.Wait()

	// Verify single key for cluster-wide watch (no namespace suffix)
	checkpointer := newCheckpointer(storageClient, zap.NewNop())
	rv, err := checkpointer.GetCheckpoint(t.Context(), "", "pods")
	require.NoError(t, err)
	assert.NotEmpty(t, rv) // Should have a value

	// Verify key format
	key := checkpointer.getCheckpointKey("", "pods")
	assert.Equal(t, "latestResourceVersion/pods", key)
}

func TestObserverPersistenceMultipleNamespaces(t *testing.T) {
	mockClient := newMockDynamicClient()
	storageClient := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")

	cfg := Config{
		Config: k8sinventory.Config{
			Gvr: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespaces: []string{"default", "other"},
		},
	}

	receivedEventsChan := make(chan *apiWatch.Event, 10)

	obs, err := New(mockClient, cfg, zap.NewNop(), storageClient, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

	time.Sleep(time.Millisecond * 100)

	// Create pods in different namespaces
	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{"env": "test"}, "100"),
		generatePod("pod2", "other", map[string]any{"env": "prod"}, "200"),
	)

	// Wait for events
	for i := range 2 {
		select {
		case <-receivedEventsChan:
			// success
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for event %d", i+1)
		}
	}

	// Stop the observer; the deferred final flush in startCheckpointFlusher
	// ensures the latest resourceVersion is persisted before wg.Wait() returns.
	close(stopChan)
	wg.Wait()

	// Verify separate keys for each namespace
	checkpointer := newCheckpointer(storageClient, zap.NewNop())

	rv1, err := checkpointer.GetCheckpoint(t.Context(), "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "100", rv1)

	rv2, err := checkpointer.GetCheckpoint(t.Context(), "other", "pods")
	require.NoError(t, err)
	assert.Equal(t, "200", rv2)

	// Verify key formats
	key1 := checkpointer.getCheckpointKey("default", "pods")
	assert.Equal(t, "latestResourceVersion/pods.default", key1)

	key2 := checkpointer.getCheckpointKey("other", "pods")
	assert.Equal(t, "latestResourceVersion/pods.other", key2)
}

func TestGetResourceVersion(t *testing.T) {
	// Tests are grouped by which source of resourceVersion is active.
	// Note: config resource_version and storage-based persistence are mutually
	// exclusive at the receiver config level, so no test combines both.

	t.Run("with persistence", func(t *testing.T) {
		tests := []struct {
			name             string
			persistedVersion string
			listVersion      string
			expectedVersion  string
		}{
			{
				name:             "persisted RV exists - use it",
				persistedVersion: "200",
				listVersion:      "100",
				expectedVersion:  "200",
			},
			{
				name:             "persisted RV is zero - fall back to list",
				persistedVersion: "0",
				listVersion:      "250",
				expectedVersion:  "250",
			},
			{
				name:            "no persisted RV - fetch from list and persist it",
				listVersion:     "300",
				expectedVersion: "300",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockClient := newMockDynamicClient()
				if tt.listVersion != "" {
					mockClient.setListResourceVersion(tt.listVersion)
				}

				storageClient := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
				if tt.persistedVersion != "" {
					cp := newCheckpointer(storageClient, zap.NewNop())
					require.NoError(t, cp.SetCheckpoint(t.Context(), "default", "pods", tt.persistedVersion))
					require.NoError(t, cp.Flush(t.Context()))
				}

				cfg := Config{
					Config: k8sinventory.Config{
						Gvr:        schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
						Namespaces: []string{"default"},
					},
				}

				obs, err := New(mockClient, cfg, zap.NewNop(), storageClient, nil)
				require.NoError(t, err)

				resource := mockClient.Resource(cfg.Gvr)
				version, err := obs.getResourceVersion(t.Context(), resource.Namespace("default"), "default")
				require.NoError(t, err)
				assert.Equal(t, tt.expectedVersion, version)

				// When no persisted RV was set, verify the list version was persisted.
				if tt.persistedVersion == "" && tt.listVersion != "" {
					cp := newCheckpointer(storageClient, zap.NewNop())
					persisted, err := cp.GetCheckpoint(t.Context(), "default", "pods")
					require.NoError(t, err)
					assert.Equal(t, tt.expectedVersion, persisted, "list version should have been persisted")
				}
			})
		}
	})

	t.Run("with config resource version (no persistence)", func(t *testing.T) {
		tests := []struct {
			name            string
			configVersion   string
			listVersion     string
			expectedVersion string
		}{
			{
				name:            "config RV set - use it",
				configVersion:   "150",
				listVersion:     "100",
				expectedVersion: "150",
			},
			{
				name:            "config RV is zero - fall back to list",
				configVersion:   "0",
				listVersion:     "100",
				expectedVersion: "100",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockClient := newMockDynamicClient()
				if tt.listVersion != "" {
					mockClient.setListResourceVersion(tt.listVersion)
				}

				cfg := Config{
					Config: k8sinventory.Config{
						Gvr:             schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
						Namespaces:      []string{"default"},
						ResourceVersion: tt.configVersion,
					},
				}

				obs, err := New(mockClient, cfg, zap.NewNop(), nil, nil)
				require.NoError(t, err)

				resource := mockClient.Resource(cfg.Gvr)
				version, err := obs.getResourceVersion(t.Context(), resource.Namespace("default"), "default")
				require.NoError(t, err)
				assert.Equal(t, tt.expectedVersion, version)
			})
		}
	})

	t.Run("from list (no persistence, no config)", func(t *testing.T) {
		tests := []struct {
			name            string
			listVersion     string
			expectedVersion string
		}{
			{
				name:            "list version returned",
				listVersion:     "100",
				expectedVersion: "100",
			},
			{
				name:            "empty list version - use default",
				listVersion:     "",
				expectedVersion: "1",
			},
			{
				name:            "zero list version - use default",
				listVersion:     "0",
				expectedVersion: "1",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockClient := newMockDynamicClient()
				if tt.listVersion != "" {
					mockClient.setListResourceVersion(tt.listVersion)
				}

				cfg := Config{
					Config: k8sinventory.Config{
						Gvr:        schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
						Namespaces: []string{"default"},
					},
				}

				obs, err := New(mockClient, cfg, zap.NewNop(), nil, nil)
				require.NoError(t, err)

				resource := mockClient.Resource(cfg.Gvr)
				version, err := obs.getResourceVersion(t.Context(), resource.Namespace("default"), "default")
				require.NoError(t, err)
				assert.Equal(t, tt.expectedVersion, version)
			})
		}
	})
}
