// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cache // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/cache"

import (
	lru "github.com/hashicorp/golang-lru/v2"
)

type lruBlueprintCache[V any] struct {
	cache *lru.Cache[uint64, V]
}

var _ Cache[any] = (*lruBlueprintCache[any])(nil)

func (c *lruBlueprintCache[V]) Get(hash uint64) (V, bool) {
	return c.cache.Get(hash)
}

func (c *lruBlueprintCache[V]) Add(hash uint64, v V) {
	c.cache.Add(hash, v)
}

func (c *lruBlueprintCache[V]) Delete(hash uint64) {
	c.cache.Remove(hash)
}

func NewLRUBlueprintCache[V any](size int) (Cache[V], error) {
	c, err := lru.New[uint64, V](size)
	if err != nil {
		return nil, err
	}
	return &lruBlueprintCache[V]{cache: c}, nil
}
