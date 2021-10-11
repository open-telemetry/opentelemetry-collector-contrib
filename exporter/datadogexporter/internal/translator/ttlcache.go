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

const (
	metricKeySeparator = string(byte(0))
)

type ttlCache struct {
	cache *gocache.Cache
}

// numberCounter keeps the value of a number
// monotonic counter at a given point in time
type numberCounter struct {
	ts    uint64
	value float64
}

func newTTLCache(sweepInterval int64, deltaTTL int64) *ttlCache {
	cache := gocache.New(time.Duration(deltaTTL)*time.Second, time.Duration(sweepInterval)*time.Second)
	return &ttlCache{cache}
}

// Uses a logic similar to what is done in the span processor to build metric keys:
// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/b2327211df976e0a57ef0425493448988772a16b/processor/spanmetricsprocessor/processor.go#L353-L387
// TODO: make this a public util function?
func concatDimensionValue(metricKeyBuilder *strings.Builder, value string) {
	metricKeyBuilder.WriteString(value)
	metricKeyBuilder.WriteString(metricKeySeparator)
}

// metricDimensionsToMapKey maps name and tags to a string to use as an identifier
// The tags order does not matter
func (*ttlCache) metricDimensionsToMapKey(name string, tags []string) string {
	var metricKeyBuilder strings.Builder

	dimensions := make([]string, len(tags))
	copy(dimensions, tags)

	dimensions = append(dimensions, name)
	sort.Strings(dimensions)

	for _, dim := range dimensions {
		concatDimensionValue(&metricKeyBuilder, dim)
	}
	return metricKeyBuilder.String()
}

// putAndGetDiff submits a new value for a given metric and returns the difference with the
// last submitted value (ordered by timestamp). The diff value is only valid if `ok` is true.
func (t *ttlCache) putAndGetDiff(name string, tags []string, ts uint64, val float64) (dx float64, ok bool) {
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
