// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package informer

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiWatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory/checkpoint"
)

var (
	podsGVR       = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	configmapsGVR = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
)

// newFakeClient creates a fake dynamic client pre-seeded with pods.
// Returns the client and an addObj helper.
func newFakeClient(t *testing.T, objects ...*unstructured.Unstructured) (*fake.FakeDynamicClient, func(*unstructured.Unstructured)) {
	t.Helper()
	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{podsGVR: "PodList"}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)
	for _, obj := range objects {
		_, err := client.Resource(podsGVR).Namespace(obj.GetNamespace()).Create(
			t.Context(), obj, metav1.CreateOptions{},
		)
		require.NoError(t, err)
	}
	addObj := func(obj *unstructured.Unstructured) {
		_, err := client.Resource(podsGVR).Namespace(obj.GetNamespace()).Create(
			t.Context(), obj, metav1.CreateOptions{},
		)
		require.NoError(t, err)
	}
	return client, addObj
}

// newFakeClientWithMutations is like newFakeClient but also returns update and delete helpers.
func newFakeClientWithMutations(t *testing.T, objects ...*unstructured.Unstructured) (
	*fake.FakeDynamicClient,
	func(*unstructured.Unstructured),
	func(name, namespace string),
) {
	t.Helper()
	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{podsGVR: "PodList"}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)
	for _, obj := range objects {
		_, err := client.Resource(podsGVR).Namespace(obj.GetNamespace()).Create(
			t.Context(), obj, metav1.CreateOptions{},
		)
		require.NoError(t, err)
	}
	updateObj := func(obj *unstructured.Unstructured) {
		_, err := client.Resource(podsGVR).Namespace(obj.GetNamespace()).Update(
			t.Context(), obj, metav1.UpdateOptions{},
		)
		require.NoError(t, err)
	}
	deleteObj := func(name, namespace string) {
		err := client.Resource(podsGVR).Namespace(namespace).Delete(
			t.Context(), name, metav1.DeleteOptions{},
		)
		require.NoError(t, err)
	}
	return client, updateObj, deleteObj
}

func makePod(name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"name":      name,
				"namespace": "default",
			},
		},
	}
	u.SetResourceVersion("1")
	return u
}

func TestPullModeEmitsSnapshotOnStart(t *testing.T) {
	t.Parallel()
	client, _ := newFakeClient(t, makePod("pod1"))
	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	var mu sync.Mutex
	var received []*unstructured.UnstructuredList

	obs, err := NewPull(reg, PullConfig{
		Config:           k8sinventory.Config{Gvr: podsGVR},
		CacheSyncTimeout: 5 * time.Second,
		Interval:         100 * time.Millisecond,
	}, zap.NewNop(), func(list *unstructured.UnstructuredList) {
		mu.Lock()
		received = append(received, list)
		mu.Unlock()
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	stopCh, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
	t.Cleanup(func() { close(stopCh); wg.Wait() })

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) > 0
	}, 5*time.Second, 10*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, received[0].Items, 1)
	assert.Equal(t, "pod1", received[0].Items[0].GetName())
}

func TestPullModeEmitsOnInterval(t *testing.T) {
	t.Parallel()
	client, _ := newFakeClient(t, makePod("pod1"))
	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	var mu sync.Mutex
	snapshots := 0

	obs, err := NewPull(reg, PullConfig{
		Config:           k8sinventory.Config{Gvr: podsGVR},
		CacheSyncTimeout: 5 * time.Second,
		Interval:         50 * time.Millisecond,
	}, zap.NewNop(), func(_ *unstructured.UnstructuredList) {
		mu.Lock()
		snapshots++
		mu.Unlock()
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	stopCh, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
	t.Cleanup(func() { close(stopCh); wg.Wait() })

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return snapshots >= 3
	}, 5*time.Second, 10*time.Millisecond)
}

func TestWatchModeIncludeInitialStateTrue(t *testing.T) {
	t.Parallel()
	client, _ := newFakeClient(t, makePod("pod1"))
	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	var mu sync.Mutex
	var events []apiWatch.Event

	obs, err := NewWatch(reg, WatchConfig{
		Config:              k8sinventory.Config{Gvr: podsGVR},
		CacheSyncTimeout:    5 * time.Second,
		IncludeInitialState: true,
	}, zap.NewNop(), func(ev *apiWatch.Event) {
		mu.Lock()
		events = append(events, *ev)
		mu.Unlock()
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	stopCh, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
	t.Cleanup(func() { close(stopCh); wg.Wait() })

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) > 0
	}, 5*time.Second, 10*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, apiWatch.Added, events[0].Type)
}

func TestWatchModeIncludeInitialStateFalse(t *testing.T) {
	t.Parallel()
	client, addObj := newFakeClient(t, makePod("existing-pod"))
	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	var mu sync.Mutex
	var events []apiWatch.Event

	obs, err := NewWatch(reg, WatchConfig{
		Config:              k8sinventory.Config{Gvr: podsGVR},
		CacheSyncTimeout:    5 * time.Second,
		IncludeInitialState: false,
	}, zap.NewNop(), func(ev *apiWatch.Event) {
		mu.Lock()
		events = append(events, *ev)
		mu.Unlock()
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	stopCh, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
	t.Cleanup(func() { close(stopCh); wg.Wait() })

	addObj(makePod("new-pod"))

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) > 0
	}, 5*time.Second, 10*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	for _, ev := range events {
		u := ev.Object.(*unstructured.Unstructured)
		assert.Equal(t, "new-pod", u.GetName(), "existing-pod must not appear when include_initial_state=false")
	}
}

// TestTwoObserversIndependent verifies that two observers watching different GVRs
// on the same client do not cross-contaminate event delivery.
func TestTwoObserversIndependent(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		podsGVR:       "PodList",
		configmapsGVR: "ConfigMapList",
	}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)

	pod := makePod("pod1")
	_, err := client.Resource(podsGVR).Namespace("default").Create(t.Context(), pod, metav1.CreateOptions{})
	require.NoError(t, err)

	cm := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "cm1", "namespace": "default"},
	}}
	cm.SetResourceVersion("1")
	_, err = client.Resource(configmapsGVR).Namespace("default").Create(t.Context(), cm, metav1.CreateOptions{})
	require.NoError(t, err)

	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	var (
		mu              sync.Mutex
		podsReceived    []*unstructured.UnstructuredList
		configsReceived []*unstructured.UnstructuredList
	)

	obs1, err := NewPull(reg, PullConfig{
		Config:           k8sinventory.Config{Gvr: podsGVR},
		CacheSyncTimeout: 5 * time.Second,
		Interval:         50 * time.Millisecond,
	}, zap.NewNop(), func(list *unstructured.UnstructuredList) {
		mu.Lock()
		podsReceived = append(podsReceived, list)
		mu.Unlock()
	})
	require.NoError(t, err)

	obs2, err := NewPull(reg, PullConfig{
		Config:           k8sinventory.Config{Gvr: configmapsGVR},
		CacheSyncTimeout: 5 * time.Second,
		Interval:         50 * time.Millisecond,
	}, zap.NewNop(), func(list *unstructured.UnstructuredList) {
		mu.Lock()
		configsReceived = append(configsReceived, list)
		mu.Unlock()
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	stopCh1, err := obs1.Start(t.Context(), &wg)
	require.NoError(t, err)
	stopCh2, err := obs2.Start(t.Context(), &wg)
	require.NoError(t, err)
	t.Cleanup(func() { close(stopCh1); close(stopCh2); wg.Wait() })

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(podsReceived) > 0 && len(configsReceived) > 0
	}, 5*time.Second, 10*time.Millisecond, "both observers must receive their respective objects")

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, "pod1", podsReceived[0].Items[0].GetName())
	assert.Equal(t, "cm1", configsReceived[0].Items[0].GetName())
}

func TestWatchModeExcludeWatchType(t *testing.T) {
	t.Parallel()
	client, _ := newFakeClient(t, makePod("pod1"))
	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	var mu sync.Mutex
	var eventTypes []apiWatch.EventType

	obs, err := NewWatch(reg, WatchConfig{
		Config:              k8sinventory.Config{Gvr: podsGVR},
		CacheSyncTimeout:    5 * time.Second,
		IncludeInitialState: true,
		Exclude:             map[apiWatch.EventType]bool{apiWatch.Deleted: true},
	}, zap.NewNop(), func(ev *apiWatch.Event) {
		mu.Lock()
		eventTypes = append(eventTypes, ev.Type)
		mu.Unlock()
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	stopCh, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
	t.Cleanup(func() { close(stopCh); wg.Wait() })

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return slices.Contains(eventTypes, apiWatch.Added)
	}, 5*time.Second, 10*time.Millisecond, "expected Added event for pre-existing pod")

	mu.Lock()
	defer mu.Unlock()
	for _, et := range eventTypes {
		assert.NotEqual(t, apiWatch.Deleted, et, "Deleted events must be filtered by Exclude map")
	}
}

func TestStartCacheSyncContextCancelled(t *testing.T) {
	t.Parallel()
	client, _ := newFakeClient(t, makePod("pod1"))
	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	obs, err := NewPull(reg, PullConfig{
		Config:           k8sinventory.Config{Gvr: podsGVR},
		CacheSyncTimeout: 5 * time.Second,
		Interval:         time.Hour,
	}, zap.NewNop(), func(_ *unstructured.UnstructuredList) {})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // cancel before Start so cache sync is immediately aborted

	var wg sync.WaitGroup
	_, err = obs.Start(ctx, &wg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "aborted")
}

func TestWatchModeModifiedEvent(t *testing.T) {
	t.Parallel()
	pod := makePod("pod1")
	client, updateObj, _ := newFakeClientWithMutations(t, pod)
	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	var mu sync.Mutex
	var events []apiWatch.Event

	obs, err := NewWatch(reg, WatchConfig{
		Config:              k8sinventory.Config{Gvr: podsGVR},
		CacheSyncTimeout:    5 * time.Second,
		IncludeInitialState: false,
	}, zap.NewNop(), func(ev *apiWatch.Event) {
		mu.Lock()
		events = append(events, *ev)
		mu.Unlock()
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	stopCh, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
	t.Cleanup(func() { close(stopCh); wg.Wait() })

	updated := pod.DeepCopy()
	updated.SetResourceVersion("2")
	updated.SetLabels(map[string]string{"updated": "true"})
	updateObj(updated)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, ev := range events {
			if ev.Type == apiWatch.Modified {
				return true
			}
		}
		return false
	}, 5*time.Second, 10*time.Millisecond, "expected Modified event")
}

func TestWatchModeDeletedEvent(t *testing.T) {
	t.Parallel()
	pod := makePod("pod1")
	client, _, deleteObj := newFakeClientWithMutations(t, pod)
	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	var mu sync.Mutex
	var events []apiWatch.Event

	obs, err := NewWatch(reg, WatchConfig{
		Config:              k8sinventory.Config{Gvr: podsGVR},
		CacheSyncTimeout:    5 * time.Second,
		IncludeInitialState: false,
	}, zap.NewNop(), func(ev *apiWatch.Event) {
		mu.Lock()
		events = append(events, *ev)
		mu.Unlock()
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	stopCh, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
	t.Cleanup(func() { close(stopCh); wg.Wait() })

	deleteObj("pod1", "default")

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, ev := range events {
			if ev.Type == apiWatch.Deleted {
				return true
			}
		}
		return false
	}, 5*time.Second, 10*time.Millisecond, "expected Deleted event")
}

// Closing Start's stopCh must stop watch event delivery, not just the pull ticker.
func TestWatchModeStopChStopsEventDelivery(t *testing.T) {
	t.Parallel()
	client, addObj := newFakeClient(t)
	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	var mu sync.Mutex
	var events []apiWatch.Event

	obs, err := NewWatch(reg, WatchConfig{
		Config:              k8sinventory.Config{Gvr: podsGVR},
		CacheSyncTimeout:    5 * time.Second,
		IncludeInitialState: false,
	}, zap.NewNop(), func(ev *apiWatch.Event) {
		mu.Lock()
		events = append(events, *ev)
		mu.Unlock()
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	stopCh, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)

	addObj(makePod("pod-before-stop"))
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) == 1
	}, 5*time.Second, 10*time.Millisecond, "expected event for pod-before-stop")

	close(stopCh)
	wg.Wait()

	// Handlers are removed after stopCh close; new objects must not produce events.
	addObj(makePod("pod-after-stop"))
	assert.Never(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) > 1
	}, 300*time.Millisecond, 20*time.Millisecond, "events delivered after stopCh close")
}

func TestHandleWatchEventTombstone(t *testing.T) {
	t.Parallel()
	client, _ := newFakeClient(t)
	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	var mu sync.Mutex
	var events []apiWatch.Event

	obs, err := NewWatch(reg, WatchConfig{
		Config:           k8sinventory.Config{Gvr: podsGVR},
		CacheSyncTimeout: 5 * time.Second,
	}, zap.NewNop(), func(ev *apiWatch.Event) {
		mu.Lock()
		events = append(events, *ev)
		mu.Unlock()
	})
	require.NoError(t, err)

	pod := makePod("tombstone-pod")
	tombstone := cache.DeletedFinalStateUnknown{Key: "default/tombstone-pod", Obj: pod}
	obs.handleWatchEvent(apiWatch.Deleted, tombstone, "")

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, events, 1)
	assert.Equal(t, apiWatch.Deleted, events[0].Type)
	assert.Equal(t, "tombstone-pod", events[0].Object.(*unstructured.Unstructured).GetName())
}

func newTestStorageClient(t *testing.T) *storagetest.TestClient {
	t.Helper()
	return storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
}

// TestCheckpointFirstRunEmitsAll verifies that on first startup (no persisted checkpoint),
// all objects in the initial list are emitted as ADDED events.
func TestCheckpointFirstRunEmitsAll(t *testing.T) {
	t.Parallel()

	client, _ := newFakeClient(t, makePodWithRV("pod1", "10"), makePodWithRV("pod2", "20"))
	storageClient := newTestStorageClient(t)
	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	var mu sync.Mutex
	var events []apiWatch.Event

	obs, err := NewWatch(reg, WatchConfig{
		Config:              k8sinventory.Config{Gvr: podsGVR},
		CacheSyncTimeout:    5 * time.Second,
		IncludeInitialState: true,
		StorageClient:       storageClient,
	}, zap.NewNop(), func(ev *apiWatch.Event) {
		mu.Lock()
		events = append(events, *ev)
		mu.Unlock()
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	stopCh, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
	t.Cleanup(func() { close(stopCh); wg.Wait() })

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) >= 2
	}, 5*time.Second, 10*time.Millisecond, "expected 2 ADDED events on first run")

	mu.Lock()
	defer mu.Unlock()
	names := make([]string, 0, len(events))
	for _, ev := range events {
		assert.Equal(t, apiWatch.Added, ev.Type)
		names = append(names, ev.Object.(*unstructured.Unstructured).GetName())
	}
	assert.ElementsMatch(t, []string{"pod1", "pod2"}, names)
}

// TestCheckpointRestartSkipsSeenObjects verifies that on restart with a persisted checkpoint,
// objects whose resourceVersion is ≤ the checkpoint are skipped, while newer ones are emitted.
func TestCheckpointRestartSkipsSeenObjects(t *testing.T) {
	t.Parallel()

	client, _ := newFakeClient(t,
		makePodWithRV("pod1", "10"), // RV 10 — already seen
		makePodWithRV("pod2", "30"), // RV 30 — new since checkpoint
	)

	// Pre-seed checkpoint at RV 20 (pod1 seen, pod2 not yet seen).
	storageClient := newTestStorageClient(t)
	chk := checkpoint.New(storageClient, zap.NewNop())
	require.NoError(t, chk.SetCheckpoint(t.Context(), "", podsGVR.Resource, "20"))
	require.NoError(t, chk.Flush(t.Context()))

	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	var mu sync.Mutex
	var events []apiWatch.Event

	obs, err := NewWatch(reg, WatchConfig{
		Config:              k8sinventory.Config{Gvr: podsGVR},
		CacheSyncTimeout:    5 * time.Second,
		IncludeInitialState: true,
		StorageClient:       storageClient,
	}, zap.NewNop(), func(ev *apiWatch.Event) {
		mu.Lock()
		events = append(events, *ev)
		mu.Unlock()
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	stopCh, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
	t.Cleanup(func() { close(stopCh); wg.Wait() })

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) >= 1
	}, 5*time.Second, 10*time.Millisecond, "expected at least 1 ADDED event")

	assert.Never(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) > 1
	}, 200*time.Millisecond, 10*time.Millisecond, "only pod2 (RV 30) should be emitted")

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, events, 1)
	assert.Equal(t, apiWatch.Added, events[0].Type)
	assert.Equal(t, "pod2", events[0].Object.(*unstructured.Unstructured).GetName())
}

// Duplicate namespaces in Config.Namespaces must not cause duplicate event delivery.
func TestWatchModeDeduplicatesNamespaces(t *testing.T) {
	t.Parallel()
	client, addObj := newFakeClient(t)
	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	var mu sync.Mutex
	var events []apiWatch.Event

	obs, err := NewWatch(reg, WatchConfig{
		Config: k8sinventory.Config{
			Gvr:        podsGVR,
			Namespaces: []string{"default", "default"},
		},
		CacheSyncTimeout:    5 * time.Second,
		IncludeInitialState: false,
	}, zap.NewNop(), func(ev *apiWatch.Event) {
		mu.Lock()
		events = append(events, *ev)
		mu.Unlock()
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	stopCh, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
	t.Cleanup(func() { close(stopCh); wg.Wait() })

	addObj(makePod("pod1"))

	assert.Never(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) > 1
	}, 300*time.Millisecond, 20*time.Millisecond, "duplicate namespaces must not produce duplicate events")

	mu.Lock()
	assert.Len(t, events, 1)
	mu.Unlock()
}

// If Start fails during cache sync, registered handlers must be removed so
// the shared informer doesn't keep firing into a stopped observer.
func TestWatchModeStartFailureUnregistersHandlers(t *testing.T) {
	t.Parallel()
	client, addObj := newFakeClient(t)
	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	var calls atomic.Int32
	obs, err := NewWatch(reg, WatchConfig{
		Config:           k8sinventory.Config{Gvr: podsGVR},
		CacheSyncTimeout: 1 * time.Nanosecond, // force sync failure
	}, zap.NewNop(), func(*apiWatch.Event) {
		calls.Add(1)
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	_, err = obs.Start(t.Context(), &wg)
	require.Error(t, err, "Start must fail with an unrealistic cache sync timeout")

	addObj(makePod("late-pod"))

	assert.Never(t, func() bool {
		return calls.Load() > 0
	}, 300*time.Millisecond, 20*time.Millisecond, "handlers must be removed after Start failure")
}

func makePodWithRV(name, rv string) *unstructured.Unstructured {
	u := makePod(name)
	u.SetResourceVersion(rv)
	return u
}

// Checkpoint must be buffered only after watchHandler returns so a crash
// mid-processing replays the event instead of skipping it.
func TestCheckpointAdvancesAfterHandler(t *testing.T) {
	t.Parallel()

	client, addObj := newFakeClient(t)
	storageClient := newTestStorageClient(t)
	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	// Handler blocks on `release` and signals `entered` when it is invoked.
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	var releaseOnce sync.Once
	releaseHandler := func() { releaseOnce.Do(func() { close(release) }) }

	obs, err := NewWatch(reg, WatchConfig{
		Config:              k8sinventory.Config{Gvr: podsGVR},
		CacheSyncTimeout:    5 * time.Second,
		IncludeInitialState: false,
		StorageClient:       storageClient,
	}, zap.NewNop(), func(*apiWatch.Event) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	stopCh, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
	t.Cleanup(func() { releaseHandler(); close(stopCh); wg.Wait() })

	addObj(makePodWithRV("pod1", "77"))

	// Wait until the handler is blocked mid-callback.
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("watchHandler was not called")
	}

	// While the handler is blocked, storage must not have RV 77 yet. (It may
	// already hold the post-sync checkpoint of the initial, empty list.)
	require.NoError(t, obs.cp.Flush(t.Context()))
	verifier := checkpoint.New(storageClient, zap.NewNop())
	rv, err := verifier.GetCheckpoint(t.Context(), "", podsGVR.Resource)
	require.NoError(t, err)
	assert.NotEqual(t, "77", rv, "checkpoint must not advance before handler returns")

	// Unblock the handler; RV 77 should now flow into storage.
	releaseHandler()

	require.Eventually(t, func() bool {
		if err := obs.cp.Flush(t.Context()); err != nil {
			return false
		}
		rv, err := verifier.GetCheckpoint(t.Context(), "", podsGVR.Resource)
		return err == nil && rv == "77"
	}, 5*time.Second, 10*time.Millisecond, "checkpoint must advance after handler returns")
}

func TestSharedFactoryPullStartsBeforeWatch(t *testing.T) {
	t.Parallel()

	client, _ := newFakeClient(t,
		makePodWithRV("pod-seen", "10"), // already covered by pre-seeded checkpoint
		makePodWithRV("pod-new", "30"),  // must still be emitted
	)

	storageClient := newTestStorageClient(t)
	// Pre-seed checkpoint at RV 20 so pod-seen is deduped but pod-new is not.
	chk := checkpoint.New(storageClient, zap.NewNop())
	require.NoError(t, chk.SetCheckpoint(t.Context(), "", podsGVR.Resource, "20"))
	require.NoError(t, chk.Flush(t.Context()))

	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	pullObs, err := NewPull(reg, PullConfig{
		Config:           k8sinventory.Config{Gvr: podsGVR},
		CacheSyncTimeout: 5 * time.Second,
		Interval:         time.Hour,
	}, zap.NewNop(), func(*unstructured.UnstructuredList) {})
	require.NoError(t, err)

	var mu sync.Mutex
	var events []apiWatch.Event
	watchObs, err := NewWatch(reg, WatchConfig{
		Config:              k8sinventory.Config{Gvr: podsGVR},
		CacheSyncTimeout:    5 * time.Second,
		IncludeInitialState: true,
		StorageClient:       storageClient,
	}, zap.NewNop(), func(ev *apiWatch.Event) {
		mu.Lock()
		events = append(events, *ev)
		mu.Unlock()
	})
	require.NoError(t, err)

	// Start pull first: this starts the shared factory before the watch
	// observer loads its checkpoint. Handler registration in watchObs.Start
	// must happen after Load so AlreadySeen() dedups correctly.
	var wg sync.WaitGroup
	pullStop, err := pullObs.Start(t.Context(), &wg)
	require.NoError(t, err)
	t.Cleanup(func() { close(pullStop) })

	watchStop, err := watchObs.Start(t.Context(), &wg)
	require.NoError(t, err)
	t.Cleanup(func() { close(watchStop); wg.Wait() })

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) >= 1
	}, 5*time.Second, 10*time.Millisecond, "expected pod-new ADDED event")

	assert.Never(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) > 1
	}, 300*time.Millisecond, 20*time.Millisecond, "pod-seen must be deduped by checkpoint")

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, events, 1)
	assert.Equal(t, "pod-new", events[0].Object.(*unstructured.Unstructured).GetName())
}

// TestCheckpointFlushOnCtxCancel verifies that buffered checkpoints are flushed
// when ctx is cancelled, not only when stopCh is closed.
func TestCheckpointFlushOnCtxCancel(t *testing.T) {
	t.Parallel()

	client, _ := newFakeClient(t, makePodWithRV("pod1", "42"))
	storageClient := newTestStorageClient(t)
	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	obs, err := NewWatch(reg, WatchConfig{
		Config:              k8sinventory.Config{Gvr: podsGVR},
		CacheSyncTimeout:    5 * time.Second,
		IncludeInitialState: true,
		StorageClient:       storageClient,
	}, zap.NewNop(), func(*apiWatch.Event) {})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	stopCh, err := obs.Start(ctx, &wg)
	require.NoError(t, err)

	// Buffer a RV in memory without flushing, then cancel ctx to trigger
	// shutdown down the ctx.Done() path in runCheckpointFlusher.
	require.NoError(t, obs.cp.SetCheckpoint(t.Context(), "", podsGVR.Resource, "100"))
	cancel()
	close(stopCh)
	wg.Wait()

	verifier := checkpoint.New(storageClient, zap.NewNop())
	rv, err := verifier.GetCheckpoint(t.Context(), "", podsGVR.Resource)
	require.NoError(t, err)
	assert.Equal(t, "100", rv, "buffered checkpoint must be flushed on ctx cancel")
}
