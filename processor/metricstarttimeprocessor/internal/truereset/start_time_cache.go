// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package truereset // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/truereset"

import (
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
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
// At the end of each StartTimeCache.get(), if the last time the StartTimeCache was gc'd exceeds the 'gcInterval',
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

// timeseriesInfo contains the information necessary to adjust from the initial point and to detect resets.
type timeseriesInfo struct {
	mark bool

	number    numberInfo
	histogram histogramInfo
	summary   summaryInfo
}

type numberInfo struct {
	startTime     pcommon.Timestamp
	previousValue float64
}

type histogramInfo struct {
	startTime     pcommon.Timestamp
	previousCount uint64
	previousSum   float64
}

type summaryInfo struct {
	startTime     pcommon.Timestamp
	previousCount uint64
	previousSum   float64
}

type timeseriesKey struct {
	name           string
	attributes     [16]byte
	aggTemporality pmetric.AggregationTemporality
}

// timeseriesMap maps from a timeseries instance (metric * label values) to the timeseries info for
// the instance.
type timeseriesMap struct {
	sync.RWMutex
	// The mutex is used to protect access to the member fields. It is acquired for the entirety of
	// AdjustMetricSlice() and also acquired by gc().

	mark   bool
	tsiMap map[timeseriesKey]*timeseriesInfo
}

// Get the timeseriesInfo for the timeseries associated with the metric and label values.
func (tsm *timeseriesMap) get(metric pmetric.Metric, kv pcommon.Map) (*timeseriesInfo, bool) {
	// This should only be invoked be functions called (directly or indirectly) by AdjustMetricSlice().
	// The lock protecting tsm.tsiMap is acquired there.
	name := metric.Name()
	key := timeseriesKey{
		name:       name,
		attributes: getAttributesSignature(kv),
	}
	switch metric.Type() {
	case pmetric.MetricTypeHistogram:
		// There are 2 types of Histograms whose aggregation temporality needs distinguishing:
		// * CumulativeHistogram
		// * GaugeHistogram
		key.aggTemporality = metric.Histogram().AggregationTemporality()
	case pmetric.MetricTypeExponentialHistogram:
		// There are 2 types of ExponentialHistograms whose aggregation temporality needs distinguishing:
		// * CumulativeHistogram
		// * GaugeHistogram
		key.aggTemporality = metric.ExponentialHistogram().AggregationTemporality()
	}

	tsm.mark = true
	tsi, ok := tsm.tsiMap[key]
	if !ok {
		tsi = &timeseriesInfo{}
		tsm.tsiMap[key] = tsi
	}
	tsi.mark = true
	return tsi, ok
}

// Create a unique string signature for attributes values sorted by attribute keys.
func getAttributesSignature(m pcommon.Map) [16]byte {
	clearedMap := pcommon.NewMap()
	m.Range(func(k string, attrValue pcommon.Value) bool {
		value := attrValue.Str()
		if value != "" {
			clearedMap.PutStr(k, value)
		}
		return true
	})
	return pdatautil.MapHash(clearedMap)
}

// Remove timeseries that have aged out.
func (tsm *timeseriesMap) gc() {
	tsm.Lock()
	defer tsm.Unlock()
	// this shouldn't happen under the current gc() strategy
	if !tsm.mark {
		return
	}
	for ts, tsi := range tsm.tsiMap {
		if !tsi.mark {
			delete(tsm.tsiMap, ts)
		} else {
			tsi.mark = false
		}
	}
	tsm.mark = false
}

func newTimeseriesMap() *timeseriesMap {
	return &timeseriesMap{mark: true, tsiMap: map[timeseriesKey]*timeseriesInfo{}}
}

// StartTimeCache maps from a resource to a map of timeseries instances for the resource.
type StartTimeCache struct {
	sync.RWMutex
	// The mutex is used to protect access to the member fields. It is acquired for most of
	// get() and also acquired by gc().

	gcInterval  time.Duration
	lastGC      time.Time
	resourceMap map[[16]byte]*timeseriesMap
}

// NewStartTimeCache creates a new (empty) JobsMap.
func NewStartTimeCache(gcInterval time.Duration) *StartTimeCache {
	return &StartTimeCache{gcInterval: gcInterval, lastGC: time.Now(), resourceMap: make(map[[16]byte]*timeseriesMap)}
}

// Remove jobs and timeseries that have aged out.
func (c *StartTimeCache) gc() {
	c.Lock()
	defer c.Unlock()
	// once the structure is locked, confirm that gc() is still necessary
	if time.Since(c.lastGC) > c.gcInterval {
		for sig, tsm := range c.resourceMap {
			tsm.RLock()
			tsmNotMarked := !tsm.mark
			// take a read lock here, no need to get a full lock as we have a lock on the JobsMap
			tsm.RUnlock()
			if tsmNotMarked {
				delete(c.resourceMap, sig)
			} else {
				// a full lock will be obtained in here, if required.
				tsm.gc()
			}
		}
		c.lastGC = time.Now()
	}
}

func (c *StartTimeCache) maybeGC() {
	// speculatively check if gc() is necessary, recheck once the structure is locked
	c.RLock()
	defer c.RUnlock()
	if time.Since(c.lastGC) > c.gcInterval {
		go c.gc()
	}
}

func (c *StartTimeCache) get(resourceHash [16]byte) *timeseriesMap {
	// a read lock is taken here as we will not need to modify jobsMap if the target timeseriesMap is available.
	c.RLock()
	tsm, ok := c.resourceMap[resourceHash]
	c.RUnlock()
	defer c.maybeGC()
	if ok {
		return tsm
	}
	c.Lock()
	defer c.Unlock()
	// Now that we've got an exclusive lock, check once more to ensure an entry wasn't created in the interim
	// and then create a new timeseriesMap if required.
	tsm2, ok2 := c.resourceMap[resourceHash]
	if ok2 {
		return tsm2
	}
	tsm2 = newTimeseriesMap()
	c.resourceMap[resourceHash] = tsm2
	return tsm2
}
