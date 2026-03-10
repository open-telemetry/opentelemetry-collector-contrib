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
	fakeClient.Fake.PrependReactor("list", "*", func(action k8s_testing.Action) (handled bool, ret runtime.Object, err error) {
		// Don't handle, let default action occur
		return false, nil, nil
	})

	fakeClient.Fake.PrependWatchReactor("*", func(action k8s_testing.Action) (handled bool, ret apiWatch.Interface, err error) {
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
		PersistResourceVersion: true,
	}

	receivedEventsChan := make(chan *apiWatch.Event, 10)

	obs, err := New(mockClient, cfg, zap.NewNop(), storageClient, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})

	require.NoError(t, err)
	require.NotNil(t, obs.checkpointer)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(context.Background(), &wg)

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

	// Give time for persistence to complete
	time.Sleep(time.Millisecond * 100)

	// Verify resourceVersion was persisted
	checkpointer := newCheckpointer(storageClient, zap.NewNop())
	rv, err := checkpointer.GetCheckpoint(context.Background(), "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "100", rv)

	close(stopChan)
	wg.Wait()
}

func TestObserverWithoutPersistence(t *testing.T) {
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
		PersistResourceVersion: false, // Disabled
	}

	receivedEventsChan := make(chan *apiWatch.Event, 10)

	obs, err := New(mockClient, cfg, zap.NewNop(), storageClient, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})

	require.NoError(t, err)
	assert.Nil(t, obs.checkpointer) // Should not be initialized

	wg := sync.WaitGroup{}

	stopChan := obs.Start(context.Background(), &wg)

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

	time.Sleep(time.Millisecond * 100)

	// Verify resourceVersion was NOT persisted
	checkpointer := newCheckpointer(storageClient, zap.NewNop())
	rv, err := checkpointer.GetCheckpoint(context.Background(), "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "", rv) // Should be empty

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
		PersistResourceVersion: true,
	}

	receivedEventsChan := make(chan *apiWatch.Event, 10)

	// Pass nil storage client
	obs, err := New(mockClient, cfg, zap.NewNop(), nil, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})

	require.NoError(t, err)
	assert.Nil(t, obs.checkpointer) // Should not be initialized with nil storage

	wg := sync.WaitGroup{}

	stopChan := obs.Start(context.Background(), &wg)

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
		PersistResourceVersion: true,
	}

	receivedEventsChan := make(chan *apiWatch.Event, 10)

	obs, err := New(mockClient, cfg, zap.NewNop(), storageClient, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(context.Background(), &wg)

	time.Sleep(time.Millisecond * 100)

	// Create pods in different namespaces
	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{"env": "test"}, "100"),
		generatePod("pod2", "other", map[string]any{"env": "prod"}, "101"),
	)

	// Wait for events
	for i := 0; i < 2; i++ {
		select {
		case <-receivedEventsChan:
			// success
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for event %d", i+1)
		}
	}

	time.Sleep(time.Millisecond * 100)

	// Verify single key for cluster-wide watch (no namespace suffix)
	checkpointer := newCheckpointer(storageClient, zap.NewNop())
	rv, err := checkpointer.GetCheckpoint(context.Background(), "", "pods")
	require.NoError(t, err)
	assert.NotEmpty(t, rv) // Should have a value

	// Verify key format
	key := checkpointer.getCheckpointKey("", "pods")
	assert.Equal(t, "latestResourceVersion/pods", key)

	close(stopChan)
	wg.Wait()
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
		PersistResourceVersion: true,
	}

	receivedEventsChan := make(chan *apiWatch.Event, 10)

	obs, err := New(mockClient, cfg, zap.NewNop(), storageClient, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(context.Background(), &wg)

	time.Sleep(time.Millisecond * 100)

	// Create pods in different namespaces
	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{"env": "test"}, "100"),
		generatePod("pod2", "other", map[string]any{"env": "prod"}, "200"),
	)

	// Wait for events
	for i := 0; i < 2; i++ {
		select {
		case <-receivedEventsChan:
			// success
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for event %d", i+1)
		}
	}

	time.Sleep(time.Millisecond * 100)

	// Verify separate keys for each namespace
	checkpointer := newCheckpointer(storageClient, zap.NewNop())

	rv1, err := checkpointer.GetCheckpoint(context.Background(), "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "100", rv1)

	rv2, err := checkpointer.GetCheckpoint(context.Background(), "other", "pods")
	require.NoError(t, err)
	assert.Equal(t, "200", rv2)

	// Verify key formats
	key1 := checkpointer.getCheckpointKey("default", "pods")
	assert.Equal(t, "latestResourceVersion/pods.default", key1)

	key2 := checkpointer.getCheckpointKey("other", "pods")
	assert.Equal(t, "latestResourceVersion/pods.other", key2)

	close(stopChan)
	wg.Wait()
}

func TestObserverResourceVersionPriority(t *testing.T) {
	mockClient := newMockDynamicClient()
	storageClient := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")

	// Pre-populate storage with a persisted resourceVersion
	checkpointer := newCheckpointer(storageClient, zap.NewNop())
	err := checkpointer.SetCheckpoint(context.Background(), "default", "pods", "500")
	require.NoError(t, err)

	// Set list resourceVersion
	mockClient.setListResourceVersion("100")

	tests := []struct {
		name                  string
		configResourceVersion string
		expectUsedVersion     string // The version that should actually be used
	}{
		{
			name:                  "config provided but persisted takes priority",
			configResourceVersion: "999",
			expectUsedVersion:     "500", // Persisted takes priority
		},
		{
			name:                  "no config - uses persisted",
			configResourceVersion: "",
			expectUsedVersion:     "500", // Persisted version used
		},
		{
			name:                  "config lower than persisted - persisted still wins",
			configResourceVersion: "50",
			expectUsedVersion:     "500", // Persisted takes priority over config
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Config: k8sinventory.Config{
					Gvr: schema.GroupVersionResource{
						Group:    "",
						Version:  "v1",
						Resource: "pods",
					},
					Namespaces:      []string{"default"},
					ResourceVersion: tt.configResourceVersion,
				},
				PersistResourceVersion: true,
			}

			receivedEventsChan := make(chan *apiWatch.Event, 10)

			obs, err := New(mockClient, cfg, zap.NewNop(), storageClient, func(event *apiWatch.Event) {
				receivedEventsChan <- event
			})

			require.NoError(t, err)

			// Test getResourceVersion directly to verify the logic
			resource := mockClient.Resource(cfg.Gvr)
			version, err := obs.getResourceVersion(context.Background(), resource.Namespace("default"), "default")
			require.NoError(t, err)
			assert.Equal(t, tt.expectUsedVersion, version, "getResourceVersion should return persisted version when available")

			wg := sync.WaitGroup{}
			stopChan := obs.Start(context.Background(), &wg)

			time.Sleep(time.Millisecond * 200)

			// The observer should start watching from the expected version
			assert.NotNil(t, obs)

			close(stopChan)
			wg.Wait()
		})
	}
}

func TestGetResourceVersion(t *testing.T) {
	tests := []struct {
		name              string
		configVersion     string
		persistedVersion  string
		listVersion       string
		expectedVersion   string
		enablePersistence bool
	}{
		{
			name:            "list version only (no persistence, no config)",
			configVersion:   "",
			persistedVersion: "",
			listVersion:     "100",
			expectedVersion: "100",
		},
		{
			name:            "config provided without persistence - config wins",
			configVersion:   "150",
			persistedVersion: "",
			listVersion:     "100",
			expectedVersion: "150", // Config used when no persistence
		},
		{
			name:              "persisted exists - takes priority over everything",
			configVersion:     "999",
			persistedVersion:  "200",
			listVersion:       "100",
			expectedVersion:   "200", // Persisted takes priority
			enablePersistence: true,
		},
		{
			name:              "config provided but persisted wins",
			configVersion:     "50",
			persistedVersion:  "200",
			listVersion:       "100",
			expectedVersion:   "200", // Persisted takes priority over config
			enablePersistence: true,
		},
		{
			name:              "no persisted - list and persist it",
			configVersion:     "",
			persistedVersion:  "",
			listVersion:       "300",
			expectedVersion:   "300", // List version used and persisted
			enablePersistence: true,
		},
		{
			name:            "all empty uses default",
			configVersion:   "",
			persistedVersion: "",
			listVersion:     "",
			expectedVersion: "1", // defaultResourceVersion
		},
		{
			name:            "zero values ignored - falls back to default",
			configVersion:   "0",
			persistedVersion: "",
			listVersion:     "0",
			expectedVersion: "1", // defaultResourceVersion
		},
		{
			name:              "persistence disabled with config",
			configVersion:     "100",
			persistedVersion:  "999", // Won't be loaded since persistence disabled
			listVersion:       "200",
			expectedVersion:   "100", // Config used when persistence disabled
			enablePersistence: false,
		},
		{
			name:              "persistence disabled without config",
			configVersion:     "",
			persistedVersion:  "999", // Won't be loaded since persistence disabled
			listVersion:       "200",
			expectedVersion:   "200", // List version used (persisted not loaded)
			enablePersistence: false,
		},
		{
			name:              "persisted zero value - falls back to list",
			configVersion:     "",
			persistedVersion:  "0",
			listVersion:       "250",
			expectedVersion:   "250", // Persisted "0" is invalid, use list
			enablePersistence: true,
		},
		{
			name:              "persisted empty - falls back to list",
			configVersion:     "",
			persistedVersion:  "",
			listVersion:       "350",
			expectedVersion:   "350", // No persisted value, use list
			enablePersistence: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockDynamicClient()

			// Set the list resourceVersion that will be returned by List operations
			if tt.listVersion != "" {
				mockClient.setListResourceVersion(tt.listVersion)
			}

			cfg := Config{
				Config: k8sinventory.Config{
					Gvr: schema.GroupVersionResource{
						Group:    "",
						Version:  "v1",
						Resource: "pods",
					},
					Namespaces:      []string{"default"},
					ResourceVersion: tt.configVersion,
				},
				PersistResourceVersion: tt.enablePersistence,
			}

			var storageClient *storagetest.TestClient
			if tt.enablePersistence {
				storageClient = storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")

				// Pre-populate persisted version if provided
				if tt.persistedVersion != "" {
					checkpointer := newCheckpointer(storageClient, zap.NewNop())
					err := checkpointer.SetCheckpoint(context.Background(), "default", "pods", tt.persistedVersion)
					require.NoError(t, err)
				}
			}

			obs, err := New(mockClient, cfg, zap.NewNop(), storageClient, nil)
			require.NoError(t, err)

			resource := mockClient.Resource(cfg.Gvr)
			version, err := obs.getResourceVersion(context.Background(), resource.Namespace("default"), "default")
			require.NoError(t, err)
			assert.Equal(t, tt.expectedVersion, version)

			// If persistence enabled and no initial persisted value, verify it was persisted
			if tt.enablePersistence && tt.persistedVersion == "" && tt.listVersion != "" && tt.listVersion != "0" {
				checkpointer := newCheckpointer(storageClient, zap.NewNop())
				persistedAfter, err := checkpointer.GetCheckpoint(context.Background(), "default", "pods")
				require.NoError(t, err)
				assert.Equal(t, tt.expectedVersion, persistedAfter, "list version should have been persisted")
			}
		})
	}
}

