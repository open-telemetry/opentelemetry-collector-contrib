// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookup // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/lookup"

import "time"

// Cache is a placeholder implementation that currently does not retain data.
type Cache struct{}

// NewCache returns a cache stub; the parameters are accepted for compatibility
// but are unused in this skeleton branch.
func NewCache(maxSize int, ttl time.Duration) *Cache {
	_ = maxSize
	_ = ttl
	return &Cache{}
}

// Get always reports a miss.
func (c *Cache) Get(key string) (any, bool) {
	_ = c
	_ = key
	return nil, false
}

// Set is a no-op placeholder.
func (c *Cache) Set(key string, value any) {
	_ = c
	_ = key
	_ = value
}

// Clear is a no-op placeholder.
func (*Cache) Clear() {}
