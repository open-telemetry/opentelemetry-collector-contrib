// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cache // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor/internal/cache"

import (
	lru "github.com/hashicorp/golang-lru"
)

// Cache consists of an LRU cache and the evicted items from the LRU cache.
// This data structure makes sure all the cached items can be retrieved either from the LRU cache or the evictedItems
// map. In spanmetricsprocessor's use case, we need to hold all the items during the current processing step for
// building the metrics. The evicted items can/should be safely removed once the metrics are built from the current
// batch of spans.
type Cache struct {
	*lru.Cache
	evictedItems map[interface{}]interface{}
}

// NewCache creates a Cache.
func NewCache(size int) (*Cache, error) {
	evictedItems := make(map[interface{}]interface{})
	lruCache, err := lru.NewWithEvict(size, func(key interface{}, value interface{}) {
		evictedItems[key] = value
	})
	if err != nil {
		return nil, err
	}

	return &Cache{
		Cache:        lruCache,
		evictedItems: evictedItems,
	}, nil
}

// RemoveEvictedItems cleans all the evicted items.
func (c *Cache) RemoveEvictedItems() {
	// we need to keep the original pointer to evictedItems map as it is used in the closure of lru.NewWithEvict
	for k := range c.evictedItems {
		delete(c.evictedItems, k)
	}
}

// Get retrieves an item from the LRU cache or evicted items.
func (c *Cache) Get(key interface{}) (interface{}, bool) {
	if val, ok := c.Cache.Get(key); ok {
		return val, ok
	}
	val, ok := c.evictedItems[key]
	return val, ok
}

// Purge removes all the items from the LRU cache and evicted items.
func (c *Cache) Purge() {
	c.Cache.Purge()
	c.RemoveEvictedItems()
}
