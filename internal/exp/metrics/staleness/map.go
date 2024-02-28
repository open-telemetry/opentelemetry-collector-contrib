// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package staleness // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/staleness"

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/streams"
)

// RawMap implementation
var _ streams.Map[time.Time] = (*RawMap[identity.Stream, time.Time])(nil)

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

func (rm *RawMap[K, V]) Len() int {
	return len(*rm)
}
