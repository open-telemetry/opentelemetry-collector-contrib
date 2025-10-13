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
	resolver.Stop()
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

	for range 100 {
		require.Equal(t, "definitely invalid hostname", resolver.GetHostFromIP("127.0.0.1"))
	}
	resolver.Stop()
}

func TestIPResolverWithMultipleStops(_ *testing.T) {
	resolver := NewIPResolver()

	resolver.Stop()
	resolver.Stop()
}
