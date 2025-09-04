package lrucache // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/lrucache"

import (
	"container/list"
	"sync"
)

type Cache struct {
	capacity int
	ll       *list.List
	cache    map[interface{}]*list.Element
	mtx      sync.Mutex
}

type entry struct {
	key   interface{}
	value interface{}
}

func New(capacity int) *Cache {
	return &Cache{
		capacity: capacity,
		ll:       list.New(),
		cache:    make(map[interface{}]*list.Element),
	}
}

func (c *Cache) Get(key interface{}) (value interface{}, ok bool) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if ele, hit := c.cache[key]; hit {
		c.ll.MoveToFront(ele)
		return ele.Value.(*entry).value, true
	}
	return
}

func (c *Cache) Add(key, value interface{}) {
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

func (c *Cache) removeOldest() {
	ele := c.ll.Back()
	if ele != nil {
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		delete(c.cache, kv.key)
	}
}
