// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maps // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/maps"

import (
	"context"
	"errors"

	"github.com/puzpuzpuz/xsync/v4"
	"go.opentelemetry.io/collector/extension/xextension/storage"
)

// Persisted wraps a lock-free [Parallel] map with a [storage.Client] backing
// store. The in-memory cache remains the hot path (lock-free reads), while the
// backing store provides durability across restarts.
//
// On cache miss, Persisted performs a read-through from storage so that
// previously persisted state is recovered transparently. Updated values are
// written back lazily via [Persisted.Flush].
//
// Only entries that have been accessed (and thus potentially mutated) since the
// last flush are re-serialized, avoiding redundant I/O for unchanged streams.
type Persisted[K comparable, V any] struct {
	cache   *Parallel[K, V]
	store   storage.Client
	prefix  string
	hashKey func(K) string
	encode  func(V) ([]byte, error)
	decode  func([]byte) (V, error)

	// dirty tracks keys that were accessed (and potentially mutated) since the
	// last Flush. Only these entries are re-serialized on the next flush.
	dirty *xsync.Map[K, struct{}]
}

// NewPersisted creates a [Persisted] map backed by a [storage.Client].
//
// Parameters:
//   - hashKey converts a map key to a unique storage-friendly string.
//   - encode serializes a value to bytes for storage.
//   - decode deserializes bytes back into a value.
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
		dirty:   xsync.NewMap[K, struct{}](),
	}
}

// LoadOrStore returns the existing value for k if present in the cache.
// On a cache miss it performs a read-through from the backing storage. If a
// persisted value is found it is loaded into the cache and returned. Otherwise
// def is stored in the cache (subject to the size limit).
//
// Every successful access marks the key as dirty because the caller is expected
// to mutate the returned value (e.g. aggregation). Only dirty entries are
// flushed to storage.
//
// The return semantics match [Parallel.LoadOrStore]: use [Exceeded] to check
// whether the store was rejected due to the size limit.
func (m *Persisted[K, V]) LoadOrStore(ctx context.Context, k K, def V) (V, bool) {
	// Fast path: lock-free cache lookup
	if v, ok := m.cache.elems.Load(k); ok {
		m.dirty.Store(k, struct{}{})
		return v, true
	}

	// Cold path: read-through from storage.
	data, getErr := m.store.Get(ctx, m.storageKey(k))
	if getErr != nil {
		// Transient storage failure on a new key. Return the Exceeded
		// sentinel so the caller safely drops this data point rather than
		// caching a zero default that would overwrite the real persisted
		// cumulative value on the next Flush.
		var zero V
		return zero, false
	}

	if len(data) > 0 {
		if recovered, err := m.decode(data); err == nil {
			// Use recovered value as the default so that the cache stores
			// the previously persisted state rather than a zero value.
			def = recovered
		}
		// Decode failure on existing data: proceed with original def.
		// The corrupt entry will be overwritten on the next Flush.
	}

	v, loaded := m.cache.LoadOrStore(k, def)
	// Mark dirty unconditionally. In the exceeded case the key won't be in
	// the cache, so the phantom dirty entry is harmlessly ignored by Flush.
	m.dirty.Store(k, struct{}{})
	return v, loaded
}

// LoadAndDelete removes k from both the in-memory cache and the backing
// storage. The returned value and loaded flag match [Parallel.LoadAndDelete].
func (m *Persisted[K, V]) LoadAndDelete(ctx context.Context, k K) (V, bool) {
	v, loaded := m.cache.LoadAndDelete(k)
	if loaded {
		m.dirty.Delete(k)
		_ = m.store.Delete(ctx, m.storageKey(k))
	}
	return v, loaded
}

// FlushResult contains the outcome of a [Persisted.Flush] operation.
type FlushResult struct {
	// Keys holds the hash keys of all entries currently in the cache,
	// collected during the flush iteration at no extra cost.
	Keys []string
	// Err is the combined error from encoding and batch storage operations.
	Err error
}

// Flush writes dirty cached entries to the backing storage. Only entries that
// were accessed since the last Flush are re-serialized, significantly reducing
// I/O when only a fraction of streams are active per flush period.
//
// It also returns all current cache keys (collected in the same iteration) so
// callers can persist the stale index without an additional full scan.
func (m *Persisted[K, V]) Flush(ctx context.Context) FlushResult {
	var (
		ops     []*storage.Operation
		encErrs []error
		keys    []string
	)

	// Collect all cache keys for the stale index, and encode only dirty ones.
	m.cache.Range(func(k K, v V) bool {
		hk := m.hashKey(k)
		keys = append(keys, hk)

		if _, isDirty := m.dirty.LoadAndDelete(k); !isDirty {
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

	// Clean up phantom dirty entries left by exceeded stores. These keys
	// are in the dirty set but not in the cache, so Flush never drains them.
	m.dirty.Range(func(k K, _ struct{}) bool {
		if _, ok := m.cache.elems.Load(k); !ok {
			m.dirty.Delete(k)
		}
		return true
	})

	return FlushResult{
		Keys: keys,
		Err:  errors.Join(append(encErrs, batchErr)...),
	}
}

// Keys returns the hash keys of all entries currently in the cache.
func (m *Persisted[K, V]) Keys() []string {
	var keys []string
	m.cache.Range(func(k K, _ V) bool {
		keys = append(keys, m.hashKey(k))
		return true
	})
	return keys
}

// DeleteByHashKey removes an entry from storage by its hash key. This is used
// to clean up orphaned entries whose original map key cannot be reconstructed.
func (m *Persisted[K, V]) DeleteByHashKey(ctx context.Context, hashKey string) error {
	return m.store.Delete(ctx, m.prefix+"/"+hashKey)
}

func (m *Persisted[K, V]) storageKey(k K) string {
	return m.prefix + "/" + m.hashKey(k)
}
