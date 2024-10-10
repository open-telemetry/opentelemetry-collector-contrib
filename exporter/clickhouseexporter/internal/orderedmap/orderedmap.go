// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package orderedmap // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/orderedmap"

import (
	"cmp"
	"slices"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
)

// Map is a simple implementation of [column.IterableOrderedMap] interface.
// It is intended to be used as a serdes wrapper for map[K]V and not as a general purpose container.
type Map[K comparable, V any] []entry[K, V]

type entry[K comparable, V any] struct {
	key   K
	value V
}

type iterator[K comparable, V any] struct {
	om Map[K, V]
	i  int
}

func FromMap[M ~map[K]V, K cmp.Ordered, V any](m M) *Map[K, V] {
	return FromMapFunc(m, cmp.Compare)
}

func FromMapFunc[M ~map[K]V, K comparable, V any](m M, compare func(K, K) int) *Map[K, V] {
	om := Map[K, V](make([]entry[K, V], 0, len(m)))
	for k, v := range m {
		om.Put(k, v)
	}
	slices.SortFunc(om, func(i, j entry[K, V]) int { return compare(i.key, j.key) })
	return &om
}

// Collect creates a Map from an iter.Seq2[K,V] iterator.
func Collect[K cmp.Ordered, V any](seq func(yield func(K, V) bool)) *Map[K, V] {
	return CollectFunc(seq, cmp.Compare)
}

// CollectN creates a Map, pre-sized for n entries, from an iter.Seq2[K,V] iterator.
func CollectN[K cmp.Ordered, V any](seq func(yield func(K, V) bool), n int) *Map[K, V] {
	return CollectNFunc(seq, n, cmp.Compare)
}

// CollectFunc creates a Map from an iter.Seq2[K,V] iterator with a custom compare function.
func CollectFunc[K comparable, V any](seq func(yield func(K, V) bool), compare func(K, K) int) *Map[K, V] {
	return CollectNFunc(seq, 8, compare)
}

// CollectNFunc creates a Map, pre-sized for n entries, from an iter.Seq2[K,V] iterator with a custom compare function.
func CollectNFunc[K comparable, V any](seq func(yield func(K, V) bool), n int, compare func(K, K) int) *Map[K, V] {
	om := Map[K, V](make([]entry[K, V], 0, n))
	seq(func(k K, v V) bool {
		om.Put(k, v)
		return true
	})
	slices.SortFunc(om, func(i, j entry[K, V]) int { return compare(i.key, j.key) })
	return &om
}

func (om *Map[K, V]) ToMap() map[K]V {
	m := make(map[K]V, len(*om))
	for _, e := range *om {
		m[e.key] = e.value
	}
	return m
}

// Put is part of [column.IterableOrderedMap] interface, it expects to be called by the driver itself,
// provides no type safety and expects the keys to be given in order.
// It is recommended to use [FromMap] and [Collect] to initialize [Map].
func (om *Map[K, V]) Put(key any, value any) {
	*om = append(*om, entry[K, V]{key.(K), value.(V)})
}

// All is an iter.Seq[K,V] iterator that yields all key-value pairs in order.
func (om *Map[K, V]) All(yield func(k K, v V) bool) {
	for _, e := range *om {
		if !yield(e.key, e.value) {
			return
		}
	}
}

// Keys is an iter.Seq[K] iterator that yields all keys in order.
func (om *Map[K, V]) Keys(yield func(k K) bool) {
	for _, e := range *om {
		if !yield(e.key) {
			return
		}
	}
}

// Values is an iter.Seq[V] iterator that yields all values in key order.
func (om *Map[K, V]) Values(yield func(v V) bool) {
	for _, e := range *om {
		if !yield(e.value) {
			return
		}
	}
}

// Iterator is part of [column.IterableOrderedMap] interface, it expects to be called by the driver itself.
func (om *Map[K, V]) Iterator() column.MapIterator { return &iterator[K, V]{om: *om, i: -1} }

func (i *iterator[K, V]) Next() bool { i.i++; return i.i < len(i.om) }

func (i *iterator[K, V]) Key() any { return i.om[i.i].key }

func (i *iterator[K, V]) Value() any { return i.om[i.i].value }
