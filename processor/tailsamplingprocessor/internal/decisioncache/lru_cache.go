package decisioncache

import (
	"encoding/binary"
	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"sync"
)

// lruDecisionCache implements DecisionCache as a simple LRU cache
type lruDecisionCache struct {
	cache *lru.Cache[uint64, bool]
	sync.RWMutex
}

var _ DecisionCache = (*lruDecisionCache)(nil)

func NewLRUDecisionCache(size int, onEvict func(uint64, bool)) (DecisionCache, error) {
	cache, err := lru.NewWithEvict[uint64, bool](size, onEvict)
	if err != nil {
		return nil, err
	}
	return &lruDecisionCache{cache: cache}, nil
}

func (c *lruDecisionCache) Get(id pcommon.TraceID) bool {
	c.RLock()
	defer c.RUnlock()
	result, _ := c.cache.Get(rightHalfTraceId(id))
	return result
}

func (c *lruDecisionCache) Put(id pcommon.TraceID) error {
	c.Lock()
	defer c.Unlock()
	_ = c.cache.Add(rightHalfTraceId(id), true)
	return nil
}

func rightHalfTraceId(id pcommon.TraceID) uint64 {
	return binary.LittleEndian.Uint64(id[8:])
}
