// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helper

import (
	"fmt"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestIPResolverCacheLookup(t *testing.T) {
	resolver := NewIpResolver()
	resolver.cache["127.0.0.1"] = cacheEntry{
		hostname:   "definitely invalid hostname",
		expireTime: time.Now().Add(time.Hour),
	}

	require.Equal(t, "definitely invalid hostname", resolver.GetHostFromIp("127.0.0.1"))
}

func TestIPResolverCacheInvalidation(t *testing.T) {
	resolver := NewIpResolver()

	resolver.cache["127.0.0.1"] = cacheEntry{
		hostname:   "definitely invalid hostname",
		expireTime: time.Now().Add(-1 * time.Hour),
	}

	resolver.Stop()
	resolver.invalidateCache()

	hostname := resolver.lookupIpAddr("127.0.0.1")
	require.Equal(t, hostname, resolver.GetHostFromIp("127.0.0.1"))
}

func TestIPResolver100Hits(t *testing.T) {
	resolver := NewIpResolver()
	resolver.cache["127.0.0.1"] = cacheEntry{
		hostname:   "definitely invalid hostname",
		expireTime: time.Now().Add(time.Hour),
	}

	for i := 0; i < 100; i++ {
		require.Equal(t, "definitely invalid hostname", resolver.GetHostFromIp("127.0.0.1"))
	}
}

func TestIPResolverWithMultipleStops(t *testing.T) {
	resolver := NewIpResolver()

	resolver.Stop()
	resolver.Stop()
}

func TestSizes(t *testing.T) {
	fmt.Printf("string %v \n", unsafe.Sizeof(""))
	fmt.Printf("cache entry %v \n", unsafe.Sizeof(cacheEntry{}))
	fmt.Printf("time %v \n", unsafe.Sizeof(time.Now()))
	fmt.Printf("time %v \n", unsafe.Sizeof(time.Now()))
}
