// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package staleness // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/staleness"

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

// Map is an abstraction over a map
type Map[T any] interface {
	// Load the value at key. If it does not exist, the boolean will be false and the value returned will be the zero value
	Load(key identity.Stream) (T, bool)
	// Store the given key value pair in the map
	Store(key identity.Stream, value T)
	// Remove the value at key from the map
	Delete(key identity.Stream)
	// Items returns an iterator function that in future go version can be used with range
	// See: https://go.dev/wiki/RangefuncExperiment
	Items() func(yield func(identity.Stream, T) bool) bool
}

// RawMap implementation

var _ Map[time.Time] = (*RawMap[identity.Stream, time.Time])(nil)

// RawMap is an implementation of the Map interface using a standard golang map
type RawMap[K comparable, V any] map[K]V

func (rm *RawMap[K, V]) Load(key K) (V, bool) {
	value, ok := (*rm)[key]
	return value, ok
}

func (rm *RawMap[K, V]) Store(key K, value V) {
	(*rm)[key] = value
}

func (rm *RawMap[K, V]) Delete(key K) {
	delete(*rm, key)
}

func (rm *RawMap[K, V]) Items() func(yield func(K, V) bool) bool {
	return func(yield func(K, V) bool) bool {
		for k, v := range *rm {
			if !yield(k, v) {
				break
			}
		}
		return false
	}
}
