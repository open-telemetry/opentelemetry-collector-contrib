// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory/checkpoint"
)

func setRetryBackoffs(observer *Observer, delay time.Duration) {
	observer.resourceVersionRetryBackoff.Duration = delay
	observer.resourceVersionRetryBackoff.Jitter = 0
	observer.watchRestartBackoff.Duration = delay
	observer.watchRestartBackoff.Jitter = 0
}

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

	stopChan, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
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

	stopChan, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
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

	stopChan, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
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

	stopChan, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
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

	stopChan, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
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

	stopChan, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
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

	stopChan, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
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

	stopChan, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
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

func TestObserverRetriesResourceVersionRelistPastBackoffCap(t *testing.T) {
	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}: "PodList",
	}
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)

	listCalls := 0
	fakeClient.PrependReactor("list", "pods", func(_ k8s_testing.Action) (bool, runtime.Object, error) {
		listCalls++
		switch listCalls {
		case 1:
			return true, podListWithResourceVersion("100"), nil
		case 2, 3, 4, 5:
			return true, nil, errors.New("transient list error")
		default:
			return true, podListWithResourceVersion("200"), nil
		}
	})

	watchResourceVersions := make(chan string, 2)
	watchCalls := 0
	fakeClient.PrependWatchReactor("pods", func(action k8s_testing.Action) (bool, apiWatch.Interface, error) {
		watchCalls++
		watchAction := action.(k8s_testing.WatchAction)
		watchResourceVersions <- watchAction.GetWatchRestrictions().ResourceVersion

		watcher := apiWatch.NewFakeWithChanSize(1, false)
		switch watchCalls {
		case 1:
			go watcher.Error(resourceVersionExpiredStatus())
		case 2:
			go watcher.Add(generatePod("pod-after-relist", "default", nil, "201"))
		}
		return true, watcher, nil
	})

	cfg := Config{
		Config: k8sinventory.Config{
			Gvr:        schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Namespaces: []string{"default"},
		},
	}

	receivedEventsChan := make(chan *apiWatch.Event, 1)
	obs, err := New(mockDynamicClient{client: fakeClient}, cfg, zap.NewNop(), nil, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})
	require.NoError(t, err)
	setRetryBackoffs(obs, time.Millisecond)
	obs.resourceVersionRetryBackoff.Cap = 2 * time.Millisecond

	wg := sync.WaitGroup{}
	stopChan, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)

	select {
	case event := <-receivedEventsChan:
		assert.Equal(t, apiWatch.Added, event.Type)
	case <-time.After(2 * time.Second):
		close(stopChan)
		wg.Wait()
		t.Fatal("timeout waiting for event after relist retry")
	}

	close(stopChan)
	wg.Wait()

	assert.Equal(t, "100", <-watchResourceVersions)
	assert.Equal(t, "200", <-watchResourceVersions)
	assert.GreaterOrEqual(t, listCalls, 6)
}

func TestResourceVersionRetryStopsPromptly(t *testing.T) {
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{gvr: "PodList"},
	)

	listCalled := make(chan struct{}, 1)
	fakeClient.PrependReactor("list", "pods", func(_ k8s_testing.Action) (bool, runtime.Object, error) {
		listCalled <- struct{}{}
		return true, nil, errors.New("persistent list error")
	})

	obs, err := New(
		mockDynamicClient{client: fakeClient},
		Config{Config: k8sinventory.Config{Gvr: gvr}},
		zap.NewNop(),
		nil,
		nil,
	)
	require.NoError(t, err)
	setRetryBackoffs(obs, time.Hour)

	stopperChan := make(chan struct{})
	retryDone := make(chan error, 1)
	go func() {
		_, err := obs.getResourceVersionWithRetry(
			t.Context(),
			fakeClient.Resource(gvr),
			"",
			"",
			stopperChan,
		)
		retryDone <- err
	}()

	<-listCalled
	close(stopperChan)

	select {
	case err := <-retryDone:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("resourceVersion retry did not stop promptly")
	}
}

func TestObserverBypassesPersistedResourceVersionAfter410(t *testing.T) {
	storageClient := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	cp := checkpoint.New(storageClient, zap.NewNop())
	require.NoError(t, cp.SetCheckpoint(t.Context(), "default", "pods", "50"))
	require.NoError(t, cp.Flush(t.Context()))

	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}: "PodList",
	}
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)
	fakeClient.PrependReactor("list", "pods", func(_ k8s_testing.Action) (bool, runtime.Object, error) {
		return true, podListWithResourceVersion("200"), nil
	})

	watchResourceVersions := make(chan string, 2)
	watchCalls := 0
	fakeClient.PrependWatchReactor("pods", func(action k8s_testing.Action) (bool, apiWatch.Interface, error) {
		watchCalls++
		watchAction := action.(k8s_testing.WatchAction)
		watchResourceVersions <- watchAction.GetWatchRestrictions().ResourceVersion

		watcher := apiWatch.NewFakeWithChanSize(1, false)
		switch watchCalls {
		case 1:
			go watcher.Error(resourceVersionExpiredStatus())
		case 2:
			go watcher.Add(generatePod("pod-after-relist", "default", nil, "201"))
		}
		return true, watcher, nil
	})

	cfg := Config{
		Config: k8sinventory.Config{
			Gvr:        schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Namespaces: []string{"default"},
		},
	}

	receivedEventsChan := make(chan *apiWatch.Event, 1)
	obs, err := New(mockDynamicClient{client: fakeClient}, cfg, zap.NewNop(), storageClient, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})
	require.NoError(t, err)
	setRetryBackoffs(obs, 10*time.Millisecond)

	wg := sync.WaitGroup{}
	stopChan, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)

	select {
	case event := <-receivedEventsChan:
		assert.Equal(t, apiWatch.Added, event.Type)
	case <-time.After(2 * time.Second):
		close(stopChan)
		wg.Wait()
		t.Fatal("timeout waiting for event after stale checkpoint was deleted")
	}

	close(stopChan)
	wg.Wait()

	assert.Equal(t, "50", <-watchResourceVersions)
	assert.Equal(t, "200", <-watchResourceVersions)
}

func TestObserverNonStatusWatchErrorDoesNotPanic(t *testing.T) {
	cfg := Config{Config: k8sinventory.Config{Gvr: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}}}
	receivedEventsChan := make(chan *apiWatch.Event, 1)
	obs, err := New(newMockDynamicClient(), cfg, zap.NewNop(), nil, func(event *apiWatch.Event) {
		receivedEventsChan <- event
	})
	require.NoError(t, err)

	watchCalls := 0
	watchFunc := func(context.Context, v1.ListOptions) (apiWatch.Interface, error) {
		watchCalls++
		watcher := apiWatch.NewFakeWithChanSize(1, false)
		switch watchCalls {
		case 1:
			// Not a *metav1.Status: exercises the defensive FromObject() path.
			go watcher.Error(generatePod("not-a-status", "default", nil, "1"))
		case 2:
			go watcher.Add(generatePod("pod-after-error", "default", nil, "2"))
		case 3:
			return nil, errors.New("unexpected extra watch call")
		}
		return watcher, nil
	}

	stopperChan := make(chan struct{})
	doneChan := make(chan struct{})
	go func() {
		defer close(doneChan)
		assert.NotPanics(t, func() {
			obs.doWatch(t.Context(), "1", "default", watchFunc, stopperChan, func(string) {})
		})
	}()

	select {
	case event := <-receivedEventsChan:
		assert.Equal(t, apiWatch.Added, event.Type)
	case <-time.After(2 * time.Second):
		close(stopperChan)
		<-doneChan
		t.Fatal("timeout waiting for event after non-status error")
	}

	close(stopperChan)
	<-doneChan
}

// TestObserverForbiddenKeepsPersistedResourceVersion verifies that a 403
// Forbidden watch error (unlike a 410) does not clear the persisted
// resourceVersion: the watch is restarted from the same checkpoint.
func TestObserverForbiddenKeepsPersistedResourceVersion(t *testing.T) {
	storageClient := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	cp := checkpoint.New(storageClient, zap.NewNop())
	require.NoError(t, cp.SetCheckpoint(t.Context(), "default", "pods", "50"))
	require.NoError(t, cp.Flush(t.Context()))

	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}: "PodList",
	}
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)
	fakeClient.PrependReactor("list", "pods", func(_ k8s_testing.Action) (bool, runtime.Object, error) {
		return true, podListWithResourceVersion("200"), nil
	})

	watchResourceVersions := make(chan string, 2)
	watchCalls := 0
	fakeClient.PrependWatchReactor("pods", func(action k8s_testing.Action) (bool, apiWatch.Interface, error) {
		watchCalls++
		watchAction := action.(k8s_testing.WatchAction)
		watchResourceVersions <- watchAction.GetWatchRestrictions().ResourceVersion

		if watchCalls == 1 {
			// A Forbidden error returned directly from the watch call (as
			// opposed to an error event on the channel) is what client-go's
			// RetryWatcher forwards as a watch.Error event before stopping.
			return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", errors.New("forbidden"))
		}
		return true, apiWatch.NewFakeWithChanSize(1, false), nil
	})

	cfg := Config{
		Config: k8sinventory.Config{
			Gvr:        schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Namespaces: []string{"default"},
		},
	}

	events := make(chan *apiWatch.Event, 16)
	obs, err := New(mockDynamicClient{client: fakeClient}, cfg, zap.NewNop(), storageClient, func(e *apiWatch.Event) {
		select {
		case events <- e:
		default:
		}
	})
	require.NoError(t, err)
	setRetryBackoffs(obs, 10*time.Millisecond)

	wg := sync.WaitGroup{}
	stopChan, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)

	var rv1, rv2 string
	select {
	case rv1 = <-watchResourceVersions:
	case <-time.After(2 * time.Second):
		close(stopChan)
		wg.Wait()
		t.Fatal("timeout waiting for first watch call")
	}
	select {
	case rv2 = <-watchResourceVersions:
	case <-time.After(2 * time.Second):
		close(stopChan)
		wg.Wait()
		t.Fatal("timeout waiting for second watch call")
	}

	close(stopChan)
	wg.Wait()

	close(events)
	for e := range events {
		assert.NotEqual(t, apiWatch.Error, e.Type, "non-410 watch errors must not reach handleWatchEventFunc")
	}

	assert.Equal(t, "50", rv1, "first watch should start from the persisted resourceVersion")
	assert.Equal(t, "50", rv2, "watch restarted after Forbidden should reuse the persisted resourceVersion, not the list RV")

	rv, err := cp.GetCheckpoint(t.Context(), "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "50", rv, "persisted checkpoint must survive a non-410 watch error")
}

// TestWatchRestartBackoffStopsPromptly verifies that closing stopperChan while
// startWatch is sleeping between watch restarts (not blocked inside doWatch)
// interrupts the backoff promptly instead of waiting out the full delay.
func TestWatchRestartBackoffStopsPromptly(t *testing.T) {
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{gvr: "PodList"},
	)

	watchCalled := make(chan struct{}, 1)
	fakeClient.PrependWatchReactor("pods", func(_ k8s_testing.Action) (bool, apiWatch.Interface, error) {
		select {
		case watchCalled <- struct{}{}:
		default:
		}
		watcher := apiWatch.NewFakeWithChanSize(1, false)
		// An object without a resourceVersion makes client-go's RetryWatcher
		// give up immediately, so doWatch returns (false, false) right away
		// and startWatch enters its restart backoff sleep.
		go watcher.Add(generatePod("no-rv", "default", nil, ""))
		return true, watcher, nil
	})

	obs, err := New(
		mockDynamicClient{client: fakeClient},
		Config{Config: k8sinventory.Config{Gvr: gvr, Namespaces: []string{"default"}, ResourceVersion: "1"}},
		zap.NewNop(),
		nil,
		nil,
	)
	require.NoError(t, err)
	obs.watchRestartBackoff.Duration = time.Hour
	obs.watchRestartBackoff.Jitter = 0

	wg := sync.WaitGroup{}
	stopChan, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)

	select {
	case <-watchCalled:
	case <-time.After(2 * time.Second):
		close(stopChan)
		wg.Wait()
		t.Fatal("timeout waiting for first watch call")
	}
	// Give doWatch time to return so startWatch is sleeping in the backoff,
	// not blocked on its own stopperChan select inside doWatch.
	time.Sleep(50 * time.Millisecond)

	close(stopChan)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("startWatch did not stop promptly during restart backoff")
	}
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
	stopChan, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)

	close(stopChan)
	wg.Wait()

	cp := checkpoint.New(storageClient, zap.NewNop())
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
	cp := checkpoint.New(storageClient, zap.NewNop())
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

func podListWithResourceVersion(resourceVersion string) *unstructured.UnstructuredList {
	list := &unstructured.UnstructuredList{}
	list.SetResourceVersion(resourceVersion)
	return list
}

func resourceVersionExpiredStatus() *v1.Status {
	return &v1.Status{
		Status:  v1.StatusFailure,
		Reason:  v1.StatusReasonExpired,
		Message: "The resourceVersion for the provided watch is too old.",
		Code:    http.StatusGone,
	}
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

	stopChan, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
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
	checkpointer := checkpoint.New(storageClient, zap.NewNop())
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

	stopChan, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
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

	stopChan, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
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

	stopChan, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
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
	checkpointer := checkpoint.New(storageClient, zap.NewNop())
	rv, err := checkpointer.GetCheckpoint(t.Context(), "", "pods")
	require.NoError(t, err)
	assert.NotEmpty(t, rv) // Should have a value
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

	stopChan, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
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
	checkpointer := checkpoint.New(storageClient, zap.NewNop())

	rv1, err := checkpointer.GetCheckpoint(t.Context(), "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "100", rv1)

	rv2, err := checkpointer.GetCheckpoint(t.Context(), "other", "pods")
	require.NoError(t, err)
	assert.Equal(t, "200", rv2)
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
					cp := checkpoint.New(storageClient, zap.NewNop())
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
				version, err := obs.getResourceVersion(t.Context(), resource.Namespace("default"), "default", cfg.ResourceVersion)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedVersion, version)

				// When no persisted RV was set, verify the list version was persisted.
				if tt.persistedVersion == "" && tt.listVersion != "" {
					cp := checkpoint.New(storageClient, zap.NewNop())
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
				version, err := obs.getResourceVersion(t.Context(), resource.Namespace("default"), "default", cfg.ResourceVersion)
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
				version, err := obs.getResourceVersion(t.Context(), resource.Namespace("default"), "default", cfg.ResourceVersion)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedVersion, version)
			})
		}
	})
}

type watchTestClient struct {
	client    *fake.FakeDynamicClient
	listCalls atomic.Int32

	mu       sync.Mutex
	watchRVs map[string][]string
}

// newWatchTestClient returns a fake client whose pod lists return listRVs in
// order, repeating the last one, and whose pod watches are set up by watchFn.
// An error from watchFn fails the watch call.
func newWatchTestClient(watchFn func(ns, rv string, w *apiWatch.FakeWatcher) error, listRVs ...string) *watchTestClient {
	tc := &watchTestClient{watchRVs: map[string][]string{}}
	tc.client = fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		{Version: "v1", Resource: "pods"}: "PodList",
	})
	tc.client.PrependReactor("list", "pods", func(k8s_testing.Action) (bool, runtime.Object, error) {
		i := min(int(tc.listCalls.Add(1)), len(listRVs)) - 1
		return true, podListWithResourceVersion(listRVs[i]), nil
	})
	tc.client.PrependWatchReactor("pods", func(action k8s_testing.Action) (bool, apiWatch.Interface, error) {
		watchAction := action.(k8s_testing.WatchAction)
		ns, rv := watchAction.GetNamespace(), watchAction.GetWatchRestrictions().ResourceVersion
		tc.mu.Lock()
		tc.watchRVs[ns] = append(tc.watchRVs[ns], rv)
		tc.mu.Unlock()
		w := apiWatch.NewFakeWithChanSize(1, false)
		if err := watchFn(ns, rv, w); err != nil {
			return true, nil, err
		}
		return true, w, nil
	})
	return tc
}

// watched returns the resourceVersions that watches in ns started from.
func (tc *watchTestClient) watched(ns string) []string {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return slices.Clone(tc.watchRVs[ns])
}

// runPodWatch runs an observer with fast restarts that re-lists after five
// restarts without progress, and stops it once every namespace has delivered
// an event.
func runPodWatch(t *testing.T, tc *watchTestClient, storageClient storage.Client, namespaces []string, configRV string) (*Observer, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zapcore.InfoLevel)
	cfg := Config{Config: k8sinventory.Config{
		Gvr:             schema.GroupVersionResource{Version: "v1", Resource: "pods"},
		Namespaces:      namespaces,
		ResourceVersion: configRV,
	}}
	events := make(chan *apiWatch.Event, 16)
	obs, err := New(mockDynamicClient{client: tc.client}, cfg, zap.New(core), storageClient, func(e *apiWatch.Event) {
		events <- e
	})
	require.NoError(t, err)
	setRetryBackoffs(obs, time.Millisecond)
	obs.noProgressRestarts = 5

	wg := sync.WaitGroup{}
	stopChan, err := obs.Start(t.Context(), &wg)
	require.NoError(t, err)
	for range namespaces {
		select {
		case e := <-events:
			assert.Equal(t, apiWatch.Added, e.Type)
		case <-time.After(15 * time.Second):
			close(stopChan)
			wg.Wait()
			t.Fatal("timeout waiting for watch event")
		}
	}
	close(stopChan)
	wg.Wait()
	return obs, logs
}

// TestObserverRelistsStuckWatch verifies that a watch that client-go's
// RetryWatcher would retry forever from the same resourceVersion is restarted
// from a fresh one, without touching the shared config.
func TestObserverRelistsStuckWatch(t *testing.T) {
	t.Parallel()
	const stuckLog = "watch is stuck, grabbing new resource version"
	stuck := []string{"50", "50", "50", "50", "50", "200"}
	emptyWatch := func(w *apiWatch.FakeWatcher) error {
		w.Stop()
		return nil
	}
	for _, tt := range []struct {
		name        string
		namespaces  []string
		persistedRV string
		configRV    string
		onStale     func(w *apiWatch.FakeWatcher) error
		wantWatched []string
		wantLog     string
		wantLevel   zapcore.Level
	}{
		{
			name:        "empty_watch_with_checkpoint",
			namespaces:  []string{"default"},
			persistedRV: "50",
			onStale:     emptyWatch,
			wantWatched: stuck,
			wantLog:     stuckLog,
			wantLevel:   zapcore.WarnLevel,
		},
		{
			name:       "failing_watch_with_config_version",
			namespaces: []string{"default"},
			configRV:   "50",
			onStale: func(*apiWatch.FakeWatcher) error {
				return apierrors.NewInternalError(errors.New("watch failed"))
			},
			wantWatched: stuck,
			wantLog:     stuckLog,
			wantLevel:   zapcore.WarnLevel,
		},
		{
			name:       "too_large_resource_version",
			namespaces: []string{"default"},
			configRV:   "50",
			onStale: func(*apiWatch.FakeWatcher) error {
				err := apierrors.NewTimeoutError("Too large resource version: 50, current: 40", 1)
				err.ErrStatus.Details.Causes = []v1.StatusCause{{Type: v1.CauseTypeResourceVersionTooLarge}}
				return err
			},
			wantWatched: stuck,
			wantLog:     stuckLog,
			wantLevel:   zapcore.WarnLevel,
		},
		{
			name:       "resource_expired",
			namespaces: []string{"default"},
			configRV:   "50",
			onStale: func(*apiWatch.FakeWatcher) error {
				return apierrors.NewResourceExpired("too old resource version: 50 (200)")
			},
			wantWatched: []string{"50", "200"},
			wantLog:     "received a 410, grabbing new resource version",
			wantLevel:   zapcore.InfoLevel,
		},
		{
			name:        "concurrent_namespaces",
			namespaces:  []string{"ns-a", "ns-b"},
			configRV:    "50",
			onStale:     emptyWatch,
			wantWatched: stuck,
			wantLog:     stuckLog,
			wantLevel:   zapcore.WarnLevel,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var storageClient storage.Client
			if tt.persistedRV != "" {
				storageClient = storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
				cp := checkpoint.New(storageClient, zap.NewNop())
				require.NoError(t, cp.SetCheckpoint(t.Context(), "default", "pods", tt.persistedRV))
				require.NoError(t, cp.Flush(t.Context()))
			}
			tc := newWatchTestClient(func(ns, rv string, w *apiWatch.FakeWatcher) error {
				if rv == "50" {
					return tt.onStale(w)
				}
				go w.Add(generatePod("pod-"+ns, ns, nil, "201"))
				return nil
			}, "200")

			obs, logs := runPodWatch(t, tc, storageClient, tt.namespaces, tt.configRV)
			for _, ns := range tt.namespaces {
				assert.Equal(t, tt.wantWatched, tc.watched(ns), ns)
			}
			assert.Equal(t, len(tt.namespaces), int(tc.listCalls.Load()))
			assert.Equal(t, len(tt.namespaces), logs.FilterMessage(tt.wantLog).FilterLevelExact(tt.wantLevel).Len())
			assert.Equal(t, tt.configRV, obs.config.ResourceVersion)
			if storageClient != nil {
				rv, err := checkpoint.New(storageClient, zap.NewNop()).GetCheckpoint(t.Context(), "default", "pods")
				require.NoError(t, err)
				assert.Equal(t, "201", rv)
			}
		})
	}
}

// TestObserverKeepsResourceVersionWhileAPIServerUnavailable verifies that a
// blip of network errors or 403, 429, 503 and 504 responses longer than the
// stall threshold keeps retrying from the same resourceVersion and keeps the
// checkpoint.
func TestObserverKeepsResourceVersionWhileAPIServerUnavailable(t *testing.T) {
	t.Parallel()
	dialErr := func(err error) error {
		return &url.Error{Op: "Get", URL: "https://10.0.0.1/api/v1/namespaces/default/pods", Err: &net.OpError{Op: "dial", Net: "tcp", Err: err}}
	}
	inBand := func(status v1.Status) func(*apiWatch.FakeWatcher) error {
		return func(w *apiWatch.FakeWatcher) error {
			w.Error(&status)
			return nil
		}
	}
	for _, tt := range []struct {
		name      string
		onFailure func(w *apiWatch.FakeWatcher) error
	}{
		{
			name: "connection_refused",
			onFailure: func(*apiWatch.FakeWatcher) error {
				return dialErr(os.NewSyscallError("connect", syscall.ECONNREFUSED))
			},
		},
		{
			name: "no_such_host",
			onFailure: func(*apiWatch.FakeWatcher) error {
				return dialErr(&net.DNSError{Err: "no such host", Name: "kubernetes.default.svc", IsNotFound: true})
			},
		},
		{
			name: "too_many_requests",
			onFailure: func(*apiWatch.FakeWatcher) error {
				return apierrors.NewTooManyRequests("throttled", 0)
			},
		},
		{
			name: "service_unavailable",
			onFailure: func(*apiWatch.FakeWatcher) error {
				return apierrors.NewServiceUnavailable("apiserver is shutting down")
			},
		},
		{
			name: "gateway_timeout",
			onFailure: func(*apiWatch.FakeWatcher) error {
				return apierrors.NewTimeoutError("request timed out", 0)
			},
		},
		{
			name:      "in_band_too_many_requests",
			onFailure: inBand(apierrors.NewTooManyRequests("storage is (re)initializing", 1).Status()),
		},
		{
			name:      "in_band_forbidden",
			onFailure: inBand(apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", errors.New("forbidden")).Status()),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			storageClient := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
			var attempts atomic.Int32
			tc := newWatchTestClient(func(_, rv string, w *apiWatch.FakeWatcher) error {
				if attempts.Add(1) <= 6 {
					return tt.onFailure(w)
				}
				go w.Add(generatePod("pod", "default", nil, rv+"1"))
				return nil
			}, "100", "300")

			_, logs := runPodWatch(t, tc, storageClient, []string{"default"}, "")
			assert.Equal(t, slices.Repeat([]string{"100"}, 7), tc.watched("default"))
			assert.Equal(t, int32(1), tc.listCalls.Load())
			assert.Empty(t, logs.Filter(func(e observer.LoggedEntry) bool { return e.Level >= zapcore.WarnLevel }).All())
			// 1001 comes from the watch at 100; a re-list would have watched from 300.
			rv, err := checkpoint.New(storageClient, zap.NewNop()).GetCheckpoint(t.Context(), "default", "pods")
			require.NoError(t, err)
			assert.Equal(t, "1001", rv)
		})
	}
}

func TestStallDetector(t *testing.T) {
	internalErr := apierrors.NewInternalError(errors.New("watch failed"))
	unavailableErr := apierrors.NewServiceUnavailable("apiserver is shutting down")
	expiredErr := apierrors.NewResourceExpired("too old resource version: 50 (200)")
	type attempt struct {
		rv    string
		after time.Duration
		err   error
	}
	for _, tt := range []struct {
		name     string
		attempts []attempt
		stalled  bool
	}{
		{
			name:     "short_restarts_from_same_version",
			attempts: []attempt{{"50", 0, internalErr}, {"50", time.Second, internalErr}, {"50", time.Second, internalErr}},
			stalled:  true,
		},
		{
			name:     "moved_version_resets_count",
			attempts: []attempt{{"50", 0, internalErr}, {"50", time.Second, internalErr}, {"51", time.Second, internalErr}, {"51", time.Second, internalErr}},
		},
		{
			name:     "long_attempt_resets_count",
			attempts: []attempt{{"50", 0, internalErr}, {"50", time.Second, internalErr}, {"50", time.Minute, internalErr}, {"50", time.Second, internalErr}},
		},
		{
			name:     "unavailable_attempt_is_not_counted",
			attempts: []attempt{{"50", 0, unavailableErr}, {"50", time.Second, internalErr}, {"50", time.Second, internalErr}},
		},
		{
			name:     "unavailable_flag_is_cleared",
			attempts: []attempt{{"50", 0, unavailableErr}, {"50", time.Second, internalErr}, {"50", time.Second, internalErr}, {"50", time.Second, internalErr}},
			stalled:  true,
		},
		{
			name:     "resource_expired",
			attempts: []attempt{{"50", 0, expiredErr}},
			stalled:  true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			var watchErr error
			calls := 0
			d := newStallDetector(2)
			d.now = func() time.Time { return now }
			watchFunc := d.wrap(func(context.Context, v1.ListOptions) (apiWatch.Interface, error) {
				calls++
				return nil, watchErr
			})
			for _, a := range tt.attempts {
				now, watchErr = now.Add(a.after), a.err
				_, err := watchFunc(t.Context(), v1.ListOptions{ResourceVersion: a.rv})
				require.Error(t, err)
			}
			select {
			case <-d.detected:
				require.True(t, tt.stalled, "unexpected stall: %v", d.err)
				// RetryWatcher may retry once more before doWatch stops it.
				callsBefore := calls
				_, err := watchFunc(t.Context(), v1.ListOptions{ResourceVersion: "50"})
				assert.Equal(t, d.err, err)
				assert.Equal(t, callsBefore, calls)
			default:
				assert.False(t, tt.stalled, "expected a stall")
			}
		})
	}
}

type stopCountingWatcher struct {
	*apiWatch.FakeWatcher
	stops atomic.Int32
}

func (w *stopCountingWatcher) Stop() {
	w.stops.Add(1)
	w.FakeWatcher.Stop()
}

func TestStallDetectorForwardStop(t *testing.T) {
	for _, tt := range []struct {
		name    string
		pending bool
	}{
		{name: "idle"},
		{name: "blocked_on_send", pending: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				inner := &stopCountingWatcher{FakeWatcher: apiWatch.NewFakeWithChanSize(1, false)}
				if tt.pending {
					inner.Add(generatePod("pod", "default", nil, "1"))
				}
				w := newStallDetector(1).forward(inner)
				synctest.Wait()
				w.Stop()
				w.Stop()
				synctest.Wait()
				_, ok := <-w.ResultChan()
				assert.False(t, ok, "result channel should be closed")
				assert.Equal(t, int32(1), inner.stops.Load())
			})
		})
	}
}
