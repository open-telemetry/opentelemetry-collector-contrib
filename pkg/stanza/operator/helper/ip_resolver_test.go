// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIPResolverCacheLookup(t *testing.T) {
	resolver := NewIPResolver()
	resolver.cache["127.0.0.1"] = cacheEntry{
		hostname:   "definitely invalid hostname",
		expireTime: time.Now().Add(time.Hour),
	}

	require.Equal(t, "definitely invalid hostname", resolver.GetHostFromIP("127.0.0.1"))
}

func TestIPResolverCacheInvalidation(t *testing.T) {
	resolver := NewIPResolver()

	resolver.cache["127.0.0.1"] = cacheEntry{
		hostname:   "definitely invalid hostname",
		expireTime: time.Now().Add(-1 * time.Hour),
	}

	resolver.Stop()
	resolver.invalidateCache()

	hostname := resolver.lookupIPAddr("127.0.0.1")
	require.Equal(t, hostname, resolver.GetHostFromIP("127.0.0.1"))
}

func TestIPResolver100Hits(t *testing.T) {
	resolver := NewIPResolver()
	resolver.cache["127.0.0.1"] = cacheEntry{
		hostname:   "definitely invalid hostname",
		expireTime: time.Now().Add(time.Hour),
	}

	for i := 0; i < 100; i++ {
		require.Equal(t, "definitely invalid hostname", resolver.GetHostFromIP("127.0.0.1"))
	}
}

func TestIPResolverWithMultipleStops(t *testing.T) {
	resolver := NewIPResolver()

	resolver.Stop()
	resolver.Stop()
}
