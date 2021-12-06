package spanmetricsprocessor

import lru "github.com/hashicorp/golang-lru"

// Cache is consist of an LRU cache and the evicted items from the LRU cache
// this data structure makes sure all the cached item can be retrieved either from the LRU cache or the evictedItems map
type Cache struct {
	*lru.Cache
	evictedItems map[interface{}]interface{}
}

// NewCache create Cache
func NewCache(size int) (c *Cache, e error) {
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

// RemoveEvictedItems clean all the evicted items
func (c *Cache) RemoveEvictedItems() {
	c.evictedItems = make(map[interface{}]interface{})
}

// Get retrieve item from the LRU or evicted Items
func (c *Cache) Get(key interface{}) (value interface{}, ok bool) {
	val, okay := c.Cache.Get(key)
	if !okay {
		val, okay = c.evictedItems[key]
		return val, okay
	}
	return val, okay
}
