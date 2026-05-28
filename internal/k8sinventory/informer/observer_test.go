// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package informer

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiWatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory"
)

var (
	podsGVR       = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	configmapsGVR = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
)

// newFakeFactory creates a fake factory pre-seeded with objects. For pods only.
// Returns the factory, an addObj function, and a deleteObj function.
func newFakeFactory(t *testing.T, objects ...*unstructured.Unstructured) (dynamicinformer.DynamicSharedInformerFactory, func(*unstructured.Unstructured)) {
	t.Helper()
	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{podsGVR: "PodList"}
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)
	for _, obj := range objects {
		_, err := fakeClient.Resource(podsGVR).Namespace(obj.GetNamespace()).Create(
			t.Context(), obj, metav1.CreateOptions{},
		)
		require.NoError(t, err)
	}
	factory := dynamicinformer.NewDynamicSharedInformerFactory(fakeClient, 0)
	addObj := func(obj *unstructured.Unstructured) {
		_, err := fakeClient.Resource(podsGVR).Namespace(obj.GetNamespace()).Create(
			t.Context(), obj, metav1.CreateOptions{},
		)
		require.NoError(t, err)
	}
	return factory, addObj
}

// newFakeFactoryWithMutations is like newFakeFactory but also returns update and delete helpers.
func newFakeFactoryWithMutations(t *testing.T, objects ...*unstructured.Unstructured) (
	dynamicinformer.DynamicSharedInformerFactory,
	func(*unstructured.Unstructured),
	func(*unstructured.Unstructured),
	func(name, namespace string),
) {
	t.Helper()
	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{podsGVR: "PodList"}
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)
	for _, obj := range objects {
		_, err := fakeClient.Resource(podsGVR).Namespace(obj.GetNamespace()).Create(
			t.Context(), obj, metav1.CreateOptions{},
		)
		require.NoError(t, err)
	}
	factory := dynamicinformer.NewDynamicSharedInformerFactory(fakeClient, 0)
	addObj := func(obj *unstructured.Unstructured) {
		_, err := fakeClient.Resource(podsGVR).Namespace(obj.GetNamespace()).Create(
			t.Context(), obj, metav1.CreateOptions{},
		)
		require.NoError(t, err)
	}
	updateObj := func(obj *unstructured.Unstructured) {
		_, err := fakeClient.Resource(podsGVR).Namespace(obj.GetNamespace()).Update(
			t.Context(), obj, metav1.UpdateOptions{},
		)
		require.NoError(t, err)
	}
	deleteObj := func(name, namespace string) {
		err := fakeClient.Resource(podsGVR).Namespace(namespace).Delete(
			t.Context(), name, metav1.DeleteOptions{},
		)
		require.NoError(t, err)
	}
	return factory, addObj, updateObj, deleteObj
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
	factory, _ := newFakeFactory(t, makePod("pod1"))

	var mu sync.Mutex
	var received []*unstructured.UnstructuredList

	obs, err := New(
		[]dynamicinformer.DynamicSharedInformerFactory{factory},
		Config{
			Config:           k8sinventory.Config{Gvr: podsGVR},
			Mode:             k8sinventory.PullMode,
			CacheSyncTimeout: 5 * time.Second,
			Interval:         100 * time.Millisecond,
		},
		zap.NewNop(),
		func(list *unstructured.UnstructuredList) {
			mu.Lock()
			received = append(received, list)
			mu.Unlock()
		},
		nil,
	)
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
	factory, _ := newFakeFactory(t, makePod("pod1"))

	var mu sync.Mutex
	snapshots := 0

	obs, err := New(
		[]dynamicinformer.DynamicSharedInformerFactory{factory},
		Config{
			Config:           k8sinventory.Config{Gvr: podsGVR},
			Mode:             k8sinventory.PullMode,
			CacheSyncTimeout: 5 * time.Second,
			Interval:         50 * time.Millisecond,
		},
		zap.NewNop(),
		func(_ *unstructured.UnstructuredList) {
			mu.Lock()
			snapshots++
			mu.Unlock()
		},
		nil,
	)
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
	// Pod exists BEFORE informer starts — AddFunc must fire.
	factory, _ := newFakeFactory(t, makePod("pod1"))

	var mu sync.Mutex
	var events []apiWatch.Event

	obs, err := New(
		[]dynamicinformer.DynamicSharedInformerFactory{factory},
		Config{
			Config:              k8sinventory.Config{Gvr: podsGVR},
			Mode:                k8sinventory.WatchMode,
			CacheSyncTimeout:    5 * time.Second,
			IncludeInitialState: true,
		},
		zap.NewNop(),
		nil,
		func(ev *apiWatch.Event) {
			mu.Lock()
			events = append(events, *ev)
			mu.Unlock()
		},
	)
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
	// Pre-existing pod must be suppressed; isInInitialList=true filters it out.
	factory, addObj := newFakeFactory(t, makePod("existing-pod"))

	var mu sync.Mutex
	var events []apiWatch.Event

	obs, err := New(
		[]dynamicinformer.DynamicSharedInformerFactory{factory},
		Config{
			Config:              k8sinventory.Config{Gvr: podsGVR},
			Mode:                k8sinventory.WatchMode,
			CacheSyncTimeout:    5 * time.Second,
			IncludeInitialState: false,
		},
		zap.NewNop(),
		nil,
		func(ev *apiWatch.Event) {
			mu.Lock()
			events = append(events, *ev)
			mu.Unlock()
		},
	)
	require.NoError(t, err)

	var wg sync.WaitGroup
	stopCh, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
	t.Cleanup(func() { close(stopCh); wg.Wait() })

	// Add a new pod AFTER start (after cache sync) — this one SHOULD appear.
	addObj(makePod("new-pod"))

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) > 0
	}, 5*time.Second, 10*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	// Must only see "new-pod", not "existing-pod".
	for _, ev := range events {
		u := ev.Object.(*unstructured.Unstructured)
		assert.Equal(t, "new-pod", u.GetName(), "existing-pod must not appear when include_initial_state=false")
	}
}

// TestTwoObserversSharingFactory verifies that a shared factory does not cross-contaminate
// event delivery between observers watching different GVRs.
func TestTwoObserversSharingFactory(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		podsGVR:       "PodList",
		configmapsGVR: "ConfigMapList",
	}
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)
	factory := dynamicinformer.NewDynamicSharedInformerFactory(fakeClient, 0)

	pod := makePod("pod1")
	_, err := fakeClient.Resource(podsGVR).Namespace("default").Create(t.Context(), pod, metav1.CreateOptions{})
	require.NoError(t, err)

	cm := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "cm1", "namespace": "default"},
	}}
	cm.SetResourceVersion("1")
	_, err = fakeClient.Resource(configmapsGVR).Namespace("default").Create(t.Context(), cm, metav1.CreateOptions{})
	require.NoError(t, err)

	var (
		mu              sync.Mutex
		podsReceived    []*unstructured.UnstructuredList
		configsReceived []*unstructured.UnstructuredList
	)

	obs1, err := New(
		[]dynamicinformer.DynamicSharedInformerFactory{factory},
		Config{Config: k8sinventory.Config{Gvr: podsGVR}, Mode: k8sinventory.PullMode, CacheSyncTimeout: 5 * time.Second, Interval: 50 * time.Millisecond},
		zap.NewNop(),
		func(list *unstructured.UnstructuredList) {
			mu.Lock()
			podsReceived = append(podsReceived, list)
			mu.Unlock()
		},
		nil,
	)
	require.NoError(t, err)

	obs2, err := New(
		[]dynamicinformer.DynamicSharedInformerFactory{factory},
		Config{Config: k8sinventory.Config{Gvr: configmapsGVR}, Mode: k8sinventory.PullMode, CacheSyncTimeout: 5 * time.Second, Interval: 50 * time.Millisecond},
		zap.NewNop(),
		func(list *unstructured.UnstructuredList) {
			mu.Lock()
			configsReceived = append(configsReceived, list)
			mu.Unlock()
		},
		nil,
	)
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

	// Pre-populate a pod so we get an Added event — confirms the filter is not over-broad.
	factory, _ := newFakeFactory(t, makePod("pod1"))

	var mu sync.Mutex
	var eventTypes []apiWatch.EventType

	obs, err := New(
		[]dynamicinformer.DynamicSharedInformerFactory{factory},
		Config{
			Config:              k8sinventory.Config{Gvr: podsGVR},
			Mode:                k8sinventory.WatchMode,
			CacheSyncTimeout:    5 * time.Second,
			IncludeInitialState: true,
			Exclude:             map[apiWatch.EventType]bool{apiWatch.Deleted: true},
		},
		zap.NewNop(),
		nil,
		func(ev *apiWatch.Event) {
			mu.Lock()
			eventTypes = append(eventTypes, ev.Type)
			mu.Unlock()
		},
	)
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

func TestStartCacheSyncTimeout(t *testing.T) {
	t.Parallel()
	factory, _ := newFakeFactory(t, makePod("pod1"))

	obs, err := New(
		[]dynamicinformer.DynamicSharedInformerFactory{factory},
		Config{
			Config:           k8sinventory.Config{Gvr: podsGVR},
			Mode:             k8sinventory.PullMode,
			CacheSyncTimeout: 1 * time.Nanosecond, // effectively immediate timeout
			Interval:         time.Hour,
		},
		zap.NewNop(),
		func(_ *unstructured.UnstructuredList) {},
		nil,
	)
	require.NoError(t, err)

	var wg sync.WaitGroup
	_, err = obs.Start(context.Background(), &wg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out waiting for informer cache to sync")
}

func TestWatchModeModifiedEvent(t *testing.T) {
	t.Parallel()
	pod := makePod("pod1")
	factory, _, updateObj, _ := newFakeFactoryWithMutations(t, pod)

	var mu sync.Mutex
	var events []apiWatch.Event

	obs, err := New(
		[]dynamicinformer.DynamicSharedInformerFactory{factory},
		Config{
			Config:              k8sinventory.Config{Gvr: podsGVR},
			Mode:                k8sinventory.WatchMode,
			CacheSyncTimeout:    5 * time.Second,
			IncludeInitialState: false,
		},
		zap.NewNop(),
		nil,
		func(ev *apiWatch.Event) {
			mu.Lock()
			events = append(events, *ev)
			mu.Unlock()
		},
	)
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
	factory, _, _, deleteObj := newFakeFactoryWithMutations(t, pod)

	var mu sync.Mutex
	var events []apiWatch.Event

	obs, err := New(
		[]dynamicinformer.DynamicSharedInformerFactory{factory},
		Config{
			Config:              k8sinventory.Config{Gvr: podsGVR},
			Mode:                k8sinventory.WatchMode,
			CacheSyncTimeout:    5 * time.Second,
			IncludeInitialState: false,
		},
		zap.NewNop(),
		nil,
		func(ev *apiWatch.Event) {
			mu.Lock()
			events = append(events, *ev)
			mu.Unlock()
		},
	)
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

func TestHandleWatchEventTombstone(t *testing.T) {
	t.Parallel()
	factory, _ := newFakeFactory(t)

	var mu sync.Mutex
	var events []apiWatch.Event

	obs, err := New(
		[]dynamicinformer.DynamicSharedInformerFactory{factory},
		Config{
			Config:           k8sinventory.Config{Gvr: podsGVR},
			Mode:             k8sinventory.WatchMode,
			CacheSyncTimeout: 5 * time.Second,
		},
		zap.NewNop(),
		nil,
		func(ev *apiWatch.Event) {
			mu.Lock()
			events = append(events, *ev)
			mu.Unlock()
		},
	)
	require.NoError(t, err)

	pod := makePod("tombstone-pod")
	tombstone := cache.DeletedFinalStateUnknown{Key: "default/tombstone-pod", Obj: pod}
	obs.handleWatchEvent(apiWatch.Deleted, tombstone)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, events, 1)
	assert.Equal(t, apiWatch.Deleted, events[0].Type)
	assert.Equal(t, "tombstone-pod", events[0].Object.(*unstructured.Unstructured).GetName())
}
