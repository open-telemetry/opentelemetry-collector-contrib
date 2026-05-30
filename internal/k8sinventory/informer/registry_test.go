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
	reg := NewFactoryRegistry(client)
	reg.Get("default", "", "")

	require.NotPanics(t, reg.Shutdown)
	require.NotPanics(t, reg.Shutdown)
}

func TestRegistryStopChClosesOnShutdown(t *testing.T) {
	t.Parallel()
	client, _ := newFakeClient(t)
	reg := NewFactoryRegistry(client)

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
	reg := NewFactoryRegistry(client)
	t.Cleanup(reg.Shutdown)

	const n = 32
	var wg sync.WaitGroup
	results := make([]any, n)
	for i := range n {
		wg.Go(func() {
			results[i] = reg.Get("default", "app=foo", "")
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
	reg := NewFactoryRegistry(client)
	t.Cleanup(reg.Shutdown)

	const n = 16
	var wg sync.WaitGroup
	results := make([]any, n)
	for i := range n {
		wg.Go(func() {
			// Distinct label selector per goroutine forces a distinct factoryKey.
			results[i] = reg.Get("default", "app="+string(rune('a'+i)), "")
		})
	}
	wg.Wait()

	seen := make(map[any]struct{}, n)
	for _, f := range results {
		seen[f] = struct{}{}
	}
	assert.Len(t, seen, n, "distinct scopes must each produce a distinct factory")
}

func TestRegistryGetAfterShutdownPanics(t *testing.T) {
	t.Parallel()
	client, _ := newFakeClient(t)
	reg := NewFactoryRegistry(client)
	reg.Shutdown()

	assert.Panics(t, func() {
		reg.Get("default", "", "")
	})
}

// Pins the registry's sharing contract: same scope => same factory, different scope => different factory.
func TestRegistrySharesFactoryAcrossObservers(t *testing.T) {
	t.Parallel()
	client, _ := newFakeClient(t)
	reg := NewFactoryRegistry(client)
	t.Cleanup(reg.Shutdown)

	f1 := reg.Get("default", "app=foo", "")
	f2 := reg.Get("default", "app=foo", "")
	assert.Same(t, f1, f2, "same scope must return the same factory instance")

	f3 := reg.Get("default", "app=bar", "")
	assert.NotSame(t, f1, f3, "different label selector must produce a different factory")

	f4 := reg.Get("other", "app=foo", "")
	assert.NotSame(t, f1, f4, "different namespace must produce a different factory")
}
