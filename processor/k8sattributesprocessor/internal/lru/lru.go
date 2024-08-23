package lru

import (
	"fmt"
	"sync"
	"time"

	cache "github.com/hashicorp/golang-lru/v2/expirable"
)

type Cache struct {
	lrucache *cache.LRU[string, string]
}

const (
	DEFAULT_CACHE_SIZE                = 256
	DEFAULT_CACHE_EXPIRATION_INTERVAL = 10 * time.Minute
)

var (
	once     sync.Once
	instance *Cache
)

func New(size int, expirationInterval time.Duration) *Cache {
	return &Cache{lrucache: cache.NewLRU[string, string](size, nil, expirationInterval)}
}

func GetInstance(size int, expirationInterval time.Duration) *Cache {
	once.Do(func() {
		instance = New(size, expirationInterval)
	})
	return instance
}

func (c *Cache) Get(key string) (string, bool) {
	return c.lrucache.Get(key)
}

func (c *Cache) Add(key string, value string) {
	c.lrucache.Add(key, value)
}

func (c *Cache) AddEvicted(key string, value string) (evicted bool) {
	return c.lrucache.Add(key, value)
}

func (c *Cache) PrintKeys() {
	fmt.Println(c.lrucache.Keys())
}
func (c *Cache) PrintValues() {
	fmt.Println(c.lrucache.Values())
}
