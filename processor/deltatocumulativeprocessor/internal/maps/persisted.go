// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maps // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/maps"

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/extension/xextension/storage"
)

// Persisted reads through storage on cache misses and writes snapshots on Flush.
type Persisted[K comparable, V any] struct {
	mu      sync.RWMutex
	cache   *Parallel[K, V]
	store   storage.Client
	prefix  string
	hashKey func(K) string
	encode  func(V) ([]byte, error)
	decode  func([]byte) (V, error)
}

// NewPersisted creates a storage-backed cache.
func NewPersisted[K comparable, V any](
	client storage.Client,
	prefix string,
	limit Context,
	hashKey func(K) string,
	encode func(V) ([]byte, error),
	decode func([]byte) (V, error),
) *Persisted[K, V] {
	return &Persisted[K, V]{
		cache:   New[K, V](limit),
		store:   client,
		prefix:  prefix,
		hashKey: hashKey,
		encode:  encode,
		decode:  decode,
	}
}

// LoadOrStore caches the stored value or def; loaded reports an existing cache entry.
// Capacity rejection returns [ErrLimitExceeded].
func (m *Persisted[K, V]) LoadOrStore(ctx context.Context, k K, def V) (V, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if v, ok := m.cache.elems.Load(k); ok {
		return v, true, nil
	}
	data, err := m.store.Get(ctx, m.storageKey(k))
	if err != nil {
		var zero V
		return zero, false, fmt.Errorf("read state: %w", err)
	}
	if len(data) > 0 {
		recovered, err := m.decode(data)
		if err != nil {
			var zero V
			return zero, false, fmt.Errorf("decode state: %w", err)
		}
		def = recovered
	}
	return m.cache.LoadOrStore(ctx, k, def)
}

// LoadAndDelete retains the cached entry when deletion fails so cleanup can retry.
func (m *Persisted[K, V]) LoadAndDelete(ctx context.Context, k K) (V, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v, loaded := m.cache.elems.Load(k); loaded {
		if err := m.store.Delete(ctx, m.storageKey(k)); err != nil {
			return v, false, err
		}
	}
	return m.cache.LoadAndDelete(ctx, k)
}

// Flush snapshots only the selected keys, which the caller must index before writing.
func (m *Persisted[K, V]) Flush(ctx context.Context, keys []string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	selected := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		selected[key] = struct{}{}
	}
	var (
		ops     []*storage.Operation
		encErrs []error
	)
	m.cache.Range(func(k K, v V) bool {
		hk := m.hashKey(k)
		if _, ok := selected[hk]; !ok {
			return true
		}
		data, err := m.encode(v)
		if err != nil {
			encErrs = append(encErrs, err)
			return true
		}
		ops = append(ops, storage.SetOperation(m.prefix+"/"+hk, data))
		return true
	})
	var batchErr error
	if len(ops) > 0 {
		batchErr = m.store.Batch(ctx, ops...)
	}
	return errors.Join(append(encErrs, batchErr)...)
}

// Keys returns the storage hashes of cached entries.
func (m *Persisted[K, V]) Keys() []string {
	var keys []string
	m.cache.Range(func(k K, _ V) bool {
		keys = append(keys, m.hashKey(k))
		return true
	})
	return keys
}

// DeleteByHashKey deletes stored state without evicting the cache.
func (m *Persisted[K, V]) DeleteByHashKey(ctx context.Context, hashKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.store.Delete(ctx, m.prefix+"/"+hashKey)
}

func (m *Persisted[K, V]) storageKey(k K) string {
	return m.prefix + "/" + m.hashKey(k)
}
