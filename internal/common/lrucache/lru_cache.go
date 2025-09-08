// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lrucache // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/lrucache"

import (
	"container/list"
	"sync"
)

const defaultCapacity = 100

type Cache interface {
	Get(key any) (any, bool)
	Add(key, value any)
}

type LRUCache struct {
	capacity int
	ll       *list.List
	cache    map[any]*list.Element
	mtx      sync.Mutex
}

type entry struct {
	key   any
	value any
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
		cache:    make(map[any]*list.Element),
	}

	for _, o := range opts {
		o(c)
	}

	return c
}

func (c *LRUCache) Get(key any) (value any, ok bool) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if ele, hit := c.cache[key]; hit {
		c.ll.MoveToFront(ele)
		return ele.Value.(*entry).value, true
	}
	return
}

func (c *LRUCache) Add(key, value any) {
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
