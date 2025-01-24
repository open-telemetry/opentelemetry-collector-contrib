// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cache // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/cache"

import (
	"encoding/binary"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// lruDecisionCache implements Cache as a simple LRU cache.
// It holds trace IDs that had sampling decisions made on them.
// It does not specify the type of sampling decision that was made, only that
// a decision was made for an ID. You need separate DecisionCaches for caching
// sampled and not sampled trace IDs.
type lruDecisionCache[V any] struct {
	cache *lru.Cache[uint64, V]
}

var _ Cache[any] = (*lruDecisionCache[any])(nil)

// NewLRUDecisionCache returns a new lruDecisionCache.
// The size parameter indicates the amount of keys the cache will hold before it
// starts evicting the least recently used key.
func NewLRUDecisionCache[V any](size int) (Cache[V], error) {
	c, err := lru.New[uint64, V](size)
	if err != nil {
		return nil, err
	}
	return &lruDecisionCache[V]{cache: c}, nil
}

func (c *lruDecisionCache[V]) Get(id pcommon.TraceID) (V, bool) {
	return c.cache.Get(rightHalfTraceID(id))
}

func (c *lruDecisionCache[V]) Put(id pcommon.TraceID, v V) bool {
	_ = c.cache.Add(rightHalfTraceID(id), v)
	return true
}

// Delete is no-op since LRU relies on least recently used key being evicting automatically
func (c *lruDecisionCache[V]) Delete(_ pcommon.TraceID) {}

func rightHalfTraceID(id pcommon.TraceID) uint64 {
	return binary.LittleEndian.Uint64(id[8:])
}
