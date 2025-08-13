// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapointstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/datapointstorage"

import (
	"sync"
	"time"
)

// Notes on garbage collection (gc):
//
// Resource level gc:
// The collector will likely execute in a long running service whose lifetime may exceed
// the lifetimes of many of the jobs that it is collecting from. In order to keep the StartTimeCache from
// leaking memory for entries of no-longer existing jobs, the StartTimeCache needs to remove entries that
// haven't been accessed for a long period of time.
//
// Timeseries-level gc:
// Some resources that the collector is collecting from may export timeseries based on metrics
// from other resources (e.g. cAdvisor). In order to keep the timeseriesMap from leaking memory for entries
// of no-longer existing resources, the timeseriesMap for each resource needs to remove entries that haven't
// been accessed for a long period of time.
//
// The gc strategy uses a standard mark-and-sweep approach - each time a timeseriesMap is accessed,
// it is marked. Similarly, each time a timeseriesInfo is accessed, it is also marked.
//
// At the end of each StartTimeCache.Get(), if the last time the StartTimeCache was gc'd exceeds the 'gcInterval',
// the StartTimeCache is locked and any timeseriesMaps that are unmarked are removed from the StartTimeCache
// otherwise the timeseriesMap is gc'd
//
// The gc for the timeseriesMap is straightforward - the map is locked and, for each timeseriesInfo
// in the map, if it has not been marked, it is removed otherwise it is unmarked.
//
// Alternative Strategies
// 1. If the resource-level gc doesn't run often enough, or runs too often, a separate go routine can
//    be spawned at StartTimeCache creation time that gc's at periodic intervals. This approach potentially
//    adds more contention and latency to each scrape so the current approach is used. Note that
//    the go routine will need to be cancelled upon Shutdown().
// 2. If the gc of each timeseriesMap during the gc of the StartTimeCache causes too much contention,
//    the gc of timeseriesMaps can be moved to the end of MetricsAdjuster().AdjustMetricSlice(). This
//    approach requires adding 'lastGC' Time and (potentially) a gcInterval duration to
//    timeseriesMap so the current approach is used instead.

// Cache maps from a resource to a map of timeseries instances for the resource.
type Cache struct {
	sync.RWMutex
	// The mutex is used to protect access to the member fields. It is acquired for most of
	// get() and also acquired by gc().

	gcInterval  time.Duration
	lastGC      time.Time
	resourceMap map[[16]byte]*TimeseriesMap
}

// NewCache creates a new (empty) JobsMap.
func NewCache(gcInterval time.Duration) *Cache {
	return &Cache{gcInterval: gcInterval, lastGC: time.Now(), resourceMap: make(map[[16]byte]*TimeseriesMap)}
}

// Remove jobs and timeseries that have aged out.
func (c *Cache) gc() {
	c.Lock()
	defer c.Unlock()
	// once the structure is locked, confirm that gc() is still necessary
	if time.Since(c.lastGC) > c.gcInterval {
		for sig, tsm := range c.resourceMap {
			tsm.RLock()
			tsmNotMarked := !tsm.Mark
			// take a read lock here, no need to get a full lock as we have a lock on the JobsMap
			tsm.RUnlock()
			if tsmNotMarked {
				delete(c.resourceMap, sig)
			} else {
				// a full lock will be obtained in here, if required.
				tsm.GC()
			}
		}
		c.lastGC = time.Now()
	}
}

// Speculatively check if gc() is necessary, recheck once the structure is locked
func (c *Cache) MaybeGC() {
	c.RLock()
	defer c.RUnlock()
	if time.Since(c.lastGC) > c.gcInterval {
		go c.gc()
	}
}

// Fetches the TimeseriesMap for the given resource hash. Returns a new empty tsm and false if there was
// a cache miss.
// To add a datapoint to the cache, callers can add to the returned tsm directly.
func (c *Cache) Get(resourceHash [16]byte) (*TimeseriesMap, bool) {
	// a read lock is taken here as we will not need to modify resourceMap if the target timeseriesMap is available.
	c.RLock()
	tsm, ok := c.resourceMap[resourceHash]
	c.RUnlock()
	defer c.MaybeGC()
	if ok {
		tsm.Mark = true
		return tsm, ok
	}
	c.Lock()
	defer c.Unlock()
	// Now that we've got an exclusive lock, check once more to ensure an entry wasn't created in the interim
	// and then create a new timeseriesMap if required.
	tsm2, ok2 := c.resourceMap[resourceHash]
	if !ok2 {
		tsm2 = newTimeseriesMap()
		c.resourceMap[resourceHash] = tsm2
	}
	return tsm2, ok
}
