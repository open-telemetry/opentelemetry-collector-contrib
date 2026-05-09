// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maps // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/maps"

import (
	"context"

	"go.opentelemetry.io/collector/extension/xextension/storage"
)

// WriteThrough wraps a lock-free [Parallel] map with a [storage.Client] that
// is written on every [WriteThrough.Store] call. The in-memory cache ensures
// concurrent callers within the same process receive the same pointer (required
// for mutex-based aggregation safety), while the immediate write-through
// guarantees persistence without waiting for a periodic flush.
//
// On a cache miss, WriteThrough performs a read-through from storage so that
// previously persisted state is recovered transparently — identical to
// [Persisted]. The key difference is that mutated values must be persisted via
// an explicit [WriteThrough.Store] call rather than a lazy batch flush.
type WriteThrough[K comparable, V any] struct {
	cache   *Parallel[K, V]
	store   storage.Client
	prefix  string
	hashKey func(K) string
	encode  func(V) ([]byte, error)
	decode  func([]byte) (V, error)
}

// NewWriteThrough creates a [WriteThrough] map backed by a [storage.Client].
// Values are cached in memory for concurrent safety (same pointer to all
// goroutines). Every mutation must be followed by [WriteThrough.Store] to
// persist the update.
func NewWriteThrough[K comparable, V any](
	client storage.Client,
	prefix string,
	limit Context,
	hashKey func(K) string,
	encode func(V) ([]byte, error),
	decode func([]byte) (V, error),
) *WriteThrough[K, V] {
	return &WriteThrough[K, V]{
		cache:   New[K, V](limit),
		store:   client,
		prefix:  prefix,
		hashKey: hashKey,
		encode:  encode,
		decode:  decode,
	}
}

// LoadOrStore returns the existing value for k if present in the cache.
// On a cache miss it performs a read-through from the backing storage. If a
// persisted value is found it is loaded into the cache and returned. Otherwise
// def is stored in the cache (subject to the size limit).
//
// The return semantics match [Parallel.LoadOrStore]: use [Exceeded] to check
// whether the store was rejected due to the size limit.
//
// If a storage read fails for a new key (not yet cached), LoadOrStore returns
// the [Exceeded] sentinel to safely drop the data point rather than risk
// overwriting existing cumulative state with a zero default.
func (m *WriteThrough[K, V]) LoadOrStore(ctx context.Context, k K, def V) (V, bool) {
	// Fast path: cache hit — return existing pointer for concurrent safety.
	if v, ok := m.cache.elems.Load(k); ok {
		return v, true
	}

	// Cold path: read-through from storage.
	sk := m.storageKey(k)
	data, getErr := m.store.Get(ctx, sk)

	if getErr != nil {
		// Transient storage failure on a new key. We cannot distinguish
		// between "key doesn't exist" and "key exists but Get failed".
		// Return the Exceeded sentinel so the caller safely drops this
		// data point rather than aggregating on a zero default and
		// overwriting existing cumulative state via Store.
		var zero V
		return zero, false
	}

	if len(data) > 0 {
		if recovered, err := m.decode(data); err == nil {
			def = recovered
		}
		// Decode failure on existing data: proceed with original def.
		// The corrupt entry will be overwritten on the next Store.
	}

	// Write the default to storage only when the key is truly new.
	if len(data) == 0 {
		if encoded, err := m.encode(def); err == nil {
			_ = m.store.Set(ctx, sk, encoded)
		}
	}

	v, loaded := m.cache.LoadOrStore(k, def)
	return v, loaded
}

// Store writes v to the backing storage immediately. This must be called after
// mutating a value returned by [WriteThrough.LoadOrStore] to persist the update.
func (m *WriteThrough[K, V]) Store(ctx context.Context, k K, v V) error {
	data, err := m.encode(v)
	if err != nil {
		return err
	}
	return m.store.Set(ctx, m.storageKey(k), data)
}

// LoadAndDelete removes k from both the in-memory cache and the backing
// storage. The returned value and loaded flag match [Parallel.LoadAndDelete].
func (m *WriteThrough[K, V]) LoadAndDelete(ctx context.Context, k K) (V, bool) {
	v, loaded := m.cache.LoadAndDelete(k)
	if loaded {
		_ = m.store.Delete(ctx, m.storageKey(k))
	}
	return v, loaded
}

// Keys returns the hash keys of all entries currently managed by this instance.
func (m *WriteThrough[K, V]) Keys() []string {
	var keys []string
	m.cache.Range(func(k K, _ V) bool {
		keys = append(keys, m.hashKey(k))
		return true
	})
	return keys
}

// DeleteByHashKey removes an entry from storage by its hash key. This is used
// to clean up orphaned entries whose original map key cannot be reconstructed.
func (m *WriteThrough[K, V]) DeleteByHashKey(ctx context.Context, hashKey string) error {
	return m.store.Delete(ctx, m.prefix+"/"+hashKey)
}

func (m *WriteThrough[K, V]) storageKey(k K) string {
	return m.prefix + "/" + m.hashKey(k)
}
