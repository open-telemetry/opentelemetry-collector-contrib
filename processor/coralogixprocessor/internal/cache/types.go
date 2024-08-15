// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cache

// Cache is a cache using a uint64 as the key and any generic type as the value.
type Cache[V any] interface {
	// Get returns the value for the given blueprint hash, and a boolean to indicate whether the hash was found.
	// If the hash is not present, the zero value is returned.
	Get(hash uint64) (V, bool)
	// Add adds the value for a given id
	Add(hash uint64, v V)
	// Delete deletes the value for the given id
	Delete(hash uint64)
}
