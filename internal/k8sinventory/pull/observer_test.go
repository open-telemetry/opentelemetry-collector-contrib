// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pull

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	k8s_testing "k8s.io/client-go/testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory"
)

func TestObserver(t *testing.T) {
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
		Interval: 1 * time.Second,
	}

	ts := testSink{}

	obs, err := New(mockClient, cfg, zap.NewNop(), ts.receive)

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

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

	require.Eventually(t, func() bool {
		return len(ts.getLastReceivedEventItems()) == 2
	}, 5*time.Second, 10*time.Millisecond, "Observer should receive events")

	close(stopChan)

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
		Interval: 1 * time.Second,
	}

	ts := testSink{}

	obs, err := New(mockClient, cfg, zap.NewNop(), ts.receive)

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{"env": "test"}, "1"),
		generatePod("pod2", "other", map[string]any{"env": "prod"}, "2"),
	)

	require.Eventually(t, func() bool {
		return len(ts.getLastReceivedEventItems()) == 2
	}, 5*time.Second, 10*time.Millisecond, "Observer should receive events from all namespaces")

	close(stopChan)
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
		Interval: 1 * time.Second,
	}

	ts := testSink{}

	obs, err := New(mockClient, cfg, zap.NewNop(), ts.receive)

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{"env": "test"}, "1"),
		generatePod("pod2", "other", map[string]any{"env": "prod"}, "2"),
		generatePod("pod3", "ignored", map[string]any{"env": "dev"}, "3"),
	)

	require.Eventually(t, func() bool {
		return len(ts.getAllReceivedItems()) == 2
	}, 5*time.Second, 10*time.Millisecond, "Observer should receive events from specified namespaces")

	close(stopChan)
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
			ResourceVersion: "1",
		},
		Interval: 1 * time.Second,
	}

	ts := testSink{}

	obs, err := New(mockClient, cfg, zap.NewNop(), ts.receive)

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{"environment": "test"}, "1"),
		generatePod("pod2", "default", map[string]any{"environment": "prod"}, "2"),
	)

	require.Eventually(t, func() bool {
		return len(ts.getLastReceivedEventItems()) == 1
	}, 5*time.Second, 10*time.Millisecond, "Observer should attempt to filter")

	close(stopChan)
	wg.Wait()
}

func TestObserverStop(t *testing.T) {
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
		Interval: 10 * time.Second, // long interval to ensure stop before tick
	}

	ts := testSink{}

	obs, err := New(mockClient, cfg, zap.NewNop(), ts.receive)

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

	// Stop immediately
	close(stopChan)

	wg.Wait()

	// Should not have received anything since stopped before tick
	require.Empty(t, ts.getLastReceivedEventItems())
}

func TestObserverContextCancel(t *testing.T) {
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
		Interval: 10 * time.Second,
	}

	ts := testSink{}

	ctx, cancel := context.WithCancel(t.Context())

	obs, err := New(mockClient, cfg, zap.NewNop(), ts.receive)

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	_ = obs.Start(ctx, &wg)

	cancel()

	wg.Wait()

	require.Empty(t, ts.getLastReceivedEventItems())
}

func TestObserverNoObjects(t *testing.T) {
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
		Interval: 1 * time.Second,
	}

	ts := testSink{}

	obs, err := New(mockClient, cfg, zap.NewNop(), ts.receive)

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

	// Wait for a tick
	time.Sleep(100 * time.Millisecond)

	close(stopChan)

	wg.Wait()

	// Since no objects, should not have called receive
	require.Empty(t, ts.getLastReceivedEventItems())
}

func TestObserverListError(t *testing.T) {
	mockClient := newMockDynamicClient()

	// Make list return error
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
		Interval: 1 * time.Second,
	}

	ts := testSink{}

	obs, err := New(mockClient, cfg, zap.NewNop(), ts.receive)

	require.NoError(t, err)

	wg := sync.WaitGroup{}

	stopChan := obs.Start(t.Context(), &wg)

	// Wait for a tick
	time.Sleep(100 * time.Millisecond)

	close(stopChan)

	wg.Wait()

	// Should not have received anything due to error
	require.Empty(t, ts.getAllReceivedItems())
}

func TestNewTicker(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	ticker := newTicker(ctx, 1*time.Second)
	defer ticker.Stop()

	// Should tick immediately
	select {
	case <-ticker.C:
		// ok
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ticker did not tick immediately")
	}

	// Cancel context
	cancel()

	// Should stop
	select {
	case <-ticker.C:
		t.Fatal("ticker ticked after cancel")
	case <-time.After(100 * time.Millisecond):
		// ok
	}
}

type testSink struct {
	lastEventItems   []unstructured.Unstructured
	allReceivedItems []unstructured.Unstructured
	mtx              sync.RWMutex
}

func (t *testSink) receive(objs *unstructured.UnstructuredList) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	t.lastEventItems = objs.Items

	// deduplicate received items
	found := false

	for _, obj := range objs.Items {
		for _, existing := range t.allReceivedItems {
			if obj.GetName() == existing.GetName() {
				found = true
				break
			}
		}
	}
	if !found {
		t.allReceivedItems = append(t.allReceivedItems, objs.Items...)
	}
}

func (t *testSink) getLastReceivedEventItems() []unstructured.Unstructured {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	return t.lastEventItems
}

func (t *testSink) getAllReceivedItems() []unstructured.Unstructured {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	return t.allReceivedItems
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
