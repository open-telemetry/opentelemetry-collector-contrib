package lru

import (
	"sync"

	cache "github.com/hashicorp/golang-lru/v2"
)

type Cache struct {
	lru *cache.Cache[string, string]
}

var (
	once     sync.Once
	instance *Cache
	err      error
)

func New(size int) (*Cache, error) {
	cache, err := cache.New[string, string](size)
	if err != nil {
		return nil, err
	}
	return &Cache{lru: cache}, nil
}

func GetInstance(size int) (*Cache, error) {
	once.Do(func() {
		instance, err = New(size)
	})
	return instance, err
}

func (c *Cache) Get(key string) (string, bool) {
	return c.lru.Get(key)
}

func (c *Cache) Add(key string, value string) {
	c.lru.Add(key, value)
}

func (c *Cache) AddEvicted(key string, value string) (evicted bool) {
	return c.lru.Add(key, value)
}
