package lrucache // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/lrucache"

import (
	"container/list"
	"sync"
)

const defaultCapacity = 100

type Cache interface {
	Get(key interface{}) (interface{}, bool)
	Add(key, value interface{})
}

type LRUCache struct {
	capacity int
	ll       *list.List
	cache    map[interface{}]*list.Element
	mtx      sync.Mutex
}

type entry struct {
	key   interface{}
	value interface{}
}

type option func(*LRUCache)

func WithCapacity(capacity int) func(c *LRUCache) {
	return func(c *LRUCache) {
		c.capacity = capacity
	}
}

func New(opts ...option) *LRUCache {
	c := &LRUCache{
		capacity: defaultCapacity,
		ll:       list.New(),
		cache:    make(map[interface{}]*list.Element),
	}

	for _, o := range opts {
		o(c)
	}

	return c
}

func (c *LRUCache) Get(key interface{}) (value interface{}, ok bool) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if ele, hit := c.cache[key]; hit {
		c.ll.MoveToFront(ele)
		return ele.Value.(*entry).value, true
	}
	return
}

func (c *LRUCache) Add(key, value interface{}) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if ele, hit := c.cache[key]; hit {
		c.ll.MoveToFront(ele)
		ele.Value.(*entry).value = value
		return
	}

	ele := c.ll.PushFront(&entry{key, value})
	c.cache[key] = ele

	if c.ll.Len() > c.capacity {
		c.removeOldest()
	}
}

func (c *LRUCache) removeOldest() {
	ele := c.ll.Back()
	if ele != nil {
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		delete(c.cache, kv.key)
	}
}
