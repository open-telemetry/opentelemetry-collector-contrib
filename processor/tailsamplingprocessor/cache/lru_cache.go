// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cache // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/cache"

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
type lruDecisionCache struct {
	cache *lru.Cache[uint64, DecisionMetadata]
}

var _ Cache = (*lruDecisionCache)(nil)

// NewLRUDecisionCache returns a new lruDecisionCache.
// The size parameter indicates the amount of keys the cache will hold before it
// starts evicting the least recently used key.
func NewLRUDecisionCache(size int) (Cache, error) {
	c, err := lru.New[uint64, DecisionMetadata](size)
	if err != nil {
		return nil, err
	}
	return &lruDecisionCache{cache: c}, nil
}

func (c *lruDecisionCache) Get(id pcommon.TraceID) (DecisionMetadata, bool) {
	return c.cache.Get(rightHalfTraceID(id))
}

func (c *lruDecisionCache) Put(id pcommon.TraceID, metadata DecisionMetadata) {
	_ = c.cache.Add(rightHalfTraceID(id), metadata)
}

func rightHalfTraceID(id pcommon.TraceID) uint64 {
	return binary.LittleEndian.Uint64(id[8:])
}
