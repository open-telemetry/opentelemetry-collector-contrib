// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package informer

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryShutdownIdempotent(t *testing.T) {
	t.Parallel()
	client, _ := newFakeClient(t)
	reg := NewFactoryRegistry(client, 0)
	_, err := reg.Get("default", "", "")
	require.NoError(t, err)

	require.NotPanics(t, reg.Shutdown)
	require.NotPanics(t, reg.Shutdown)
}

func TestRegistryShutdownConcurrent(t *testing.T) {
	t.Parallel()
	client, _ := newFakeClient(t)
	reg := NewFactoryRegistry(client, 0)

	const n = 16
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			assert.NotPanics(t, reg.Shutdown)
		})
	}
	wg.Wait()
}

func TestRegistryStopChClosesOnShutdown(t *testing.T) {
	t.Parallel()
	client, _ := newFakeClient(t)
	reg := NewFactoryRegistry(client, 0)

	stopCh := reg.StopCh()
	select {
	case <-stopCh:
		t.Fatal("StopCh must not be closed before Shutdown")
	default:
	}

	reg.Shutdown()

	select {
	case <-stopCh:
	case <-time.After(time.Second):
		t.Fatal("StopCh must be closed after Shutdown")
	}
}

func TestRegistryConcurrentGetSameKeyReturnsSameFactory(t *testing.T) {
	t.Parallel()
	client, _ := newFakeClient(t)
	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	const n = 32
	var wg sync.WaitGroup
	results := make([]any, n)
	for i := range n {
		wg.Go(func() {
			f, err := reg.Get("default", "app=foo", "")
			assert.NoError(t, err)
			results[i] = f
		})
	}
	wg.Wait()

	for i := 1; i < n; i++ {
		assert.Same(t, results[0], results[i], "concurrent Get with same scope must return the same factory")
	}
}

func TestRegistryConcurrentGetDifferentKeysProducesDistinctFactories(t *testing.T) {
	t.Parallel()
	client, _ := newFakeClient(t)
	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	const n = 16
	var wg sync.WaitGroup
	results := make([]any, n)
	for i := range n {
		wg.Go(func() {
			// Distinct label selector per goroutine forces a distinct factoryKey.
			f, err := reg.Get("default", "app="+string(rune('a'+i)), "")
			assert.NoError(t, err)
			results[i] = f
		})
	}
	wg.Wait()

	seen := make(map[any]struct{}, n)
	for _, f := range results {
		seen[f] = struct{}{}
	}
	assert.Len(t, seen, n, "distinct scopes must each produce a distinct factory")
}

func TestRegistryGetAfterShutdownReturnsError(t *testing.T) {
	t.Parallel()
	client, _ := newFakeClient(t)
	reg := NewFactoryRegistry(client, 0)
	reg.Shutdown()

	f, err := reg.Get("default", "", "")
	assert.Nil(t, f)
	assert.Error(t, err)
}

func TestRegistrySharesFactoryAcrossObservers(t *testing.T) {
	t.Parallel()
	client, _ := newFakeClient(t)
	reg := NewFactoryRegistry(client, 0)
	t.Cleanup(reg.Shutdown)

	f1, err := reg.Get("default", "app=foo", "")
	require.NoError(t, err)
	f2, err := reg.Get("default", "app=foo", "")
	require.NoError(t, err)
	assert.Same(t, f1, f2, "same scope must return the same factory instance")

	f3, err := reg.Get("default", "app=bar", "")
	require.NoError(t, err)
	assert.NotSame(t, f1, f3, "different label selector must produce a different factory")

	f4, err := reg.Get("other", "app=foo", "")
	require.NoError(t, err)
	assert.NotSame(t, f1, f4, "different namespace must produce a different factory")
}
