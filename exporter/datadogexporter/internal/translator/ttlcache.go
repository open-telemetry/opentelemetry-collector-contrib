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

package translator

import (
	"sort"
	"strings"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

type TTLCache struct {
	cache *gocache.Cache
}

// numberCounter keeps the value of a number
// monotonic counter at a given point in time
type numberCounter struct {
	ts    uint64
	value float64
}

func NewTTLCache(sweepInterval int64, deltaTTL int64) *TTLCache {
	cache := gocache.New(time.Duration(deltaTTL)*time.Second, time.Duration(sweepInterval)*time.Second)
	return &TTLCache{cache}
}

// metricDimensionsToMapKey maps name and tags to a string to use as an identifier
// The tags order does not matter
func (*TTLCache) metricDimensionsToMapKey(name string, tags []string) string {
	const separator string = "}{" // These are invalid in tags
	dimensions := append(tags, name)
	sort.Strings(dimensions)
	return strings.Join(dimensions, separator)
}

// putAndGetDiff submits a new value for a given metric and returns the difference with the
// last submitted value (ordered by timestamp). The diff value is only valid if `ok` is true.
func (t *TTLCache) putAndGetDiff(name string, tags []string, ts uint64, val float64) (dx float64, ok bool) {
	key := t.metricDimensionsToMapKey(name, tags)
	if c, found := t.cache.Get(key); found {
		cnt := c.(numberCounter)
		if cnt.ts > ts {
			// We were given a point older than the one in memory so we drop it
			// We keep the existing point in memory since it is the most recent
			return 0, false
		}
		// if dx < 0, we assume there was a reset, thus we save the point
		// but don't export it (it's the first one so we can't do a delta)
		dx = val - cnt.value
		ok = dx >= 0
	}

	t.cache.Set(key, numberCounter{ts, val}, gocache.DefaultExpiration)
	return
}
