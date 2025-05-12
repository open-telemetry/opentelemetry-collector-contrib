package cache

import (
	"fmt"
	"sync"
	"time"

	pkgzcache "github.com/go-pkgz/expirable-cache/v3"
)

// Cache wraps two expirable caches: primary and secondary
type Cache struct {
	primaryCache   pkgzcache.Cache[string, string]
	secondaryCache pkgzcache.Cache[string, string]
}

var (
	instance *Cache
	once     sync.Once
)

const (
	DEFAULT_PRIMARY_CACHE_EXPIRATION_INTERVAL   = 5 * time.Minute
	DEFAULT_SECONDARY_CACHE_EXPIRATION_INTERVAL = 2 * time.Minute
	DEFAULT_PRIMARY_CACHE_SIZE                  = 5000
	DEFAULT_SECONDARY_CACHE_SIZE                = 5000
)

// GetCache returns the singleton cache instance, initializing it only once with the given parameters.
func GetCacheInstance(primarySize int, primaryTTL time.Duration, secondarySize int, secondaryTTL time.Duration) *Cache {
	once.Do(func() {
		instance = &Cache{
			primaryCache: pkgzcache.NewCache[string, string]().
				WithMaxKeys(primarySize).
				WithTTL(primaryTTL),

			secondaryCache: pkgzcache.NewCache[string, string]().
				WithMaxKeys(secondarySize).
				WithTTL(secondaryTTL),
		}
	})
	return instance
}

// GetFromPrimary retrieves a value from the primary cache
func (c *Cache) GetFromPrimary(key string) (string, error) {
	if val, ok := c.primaryCache.Get(key); ok {
		return val, nil
	}
	return "", fmt.Errorf("key '%s' not found in primary cache", key)
}

// GetFromSecondary retrieves a value from the secondary cache
func (c *Cache) GetFromSecondary(key string) (string, error) {
	if val, ok := c.secondaryCache.Get(key); ok {
		return val, nil
	}
	return "", fmt.Errorf("key '%s' not found in secondary cache", key)
}

// AddToPrimary adds a key-value pair to the primary cache using default TTL
func (c *Cache) AddToPrimary(key, value string) {
	c.primaryCache.Set(key, value, 0)
}

// AddToPrimaryWithTTL adds a key-value pair to the primary cache with custom TTL
func (c *Cache) AddToPrimaryWithTTL(key, value string, ttl time.Duration) {
	c.primaryCache.Set(key, value, ttl)
}

// AddToSecondary adds a key-value pair to the secondary cache using default TTL
func (c *Cache) AddToSecondary(key, value string) {
	c.secondaryCache.Set(key, value, 0)
}

// AddToSecondaryWithTTL adds a key-value pair to the secondary cache with custom TTL
func (c *Cache) AddToSecondaryWithTTL(key, value string, ttl time.Duration) {
	c.secondaryCache.Set(key, value, ttl)
}

// Clear removes all items from both caches
func (c *Cache) Clear() {
	c.primaryCache.Purge()
	c.secondaryCache.Purge()
}

// Size returns the total number of items in both caches
func (c *Cache) Size() int {
	return c.primaryCache.Len() + c.secondaryCache.Len()
}

// PrimarySize returns the number of items in the primary cache
func (c *Cache) PrimarySize() int {
	return c.primaryCache.Len()
}

// SecondarySize returns the number of items in the secondary cache
func (c *Cache) SecondarySize() int {
	return c.secondaryCache.Len()
}
