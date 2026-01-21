// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

type MetricsTranslator struct {
	sync.RWMutex
	buildInfo         component.BuildInfo
	lastTs            map[identity.Stream]pcommon.Timestamp
	stringPool        *StringPool
	seriesIdleTimeout time.Duration
}

func NewMetricsTranslator(buildInfo component.BuildInfo, seriesIdleTimeout time.Duration) *MetricsTranslator {
	return &MetricsTranslator{
		buildInfo:         buildInfo,
		lastTs:            make(map[identity.Stream]pcommon.Timestamp),
		stringPool:        newStringPool(),
		seriesIdleTimeout: seriesIdleTimeout,
	}
}

func (mt *MetricsTranslator) streamHasTimestamp(stream identity.Stream) (pcommon.Timestamp, bool) {
	mt.RLock()
	defer mt.RUnlock()
	ts, ok := mt.lastTs[stream]
	return ts, ok
}

func (mt *MetricsTranslator) updateLastTsForStream(stream identity.Stream, ts pcommon.Timestamp) {
	mt.Lock()
	defer mt.Unlock()
	mt.lastTs[stream] = ts
}

// Prune recreates the map keeping only the recent items.
// It returns the number of removed items.
func (mt *MetricsTranslator) Prune() int {
	// If the timeout is 0, the feature is disabled.
	// Return 0 immediately to preserve legacy behavior (keep all series).
	if mt.seriesIdleTimeout == 0 {
		return 0
	}
	// Full Lock is required here because we are swapping the entire map reference.
	// During this process, no one can read or write.
	mt.Lock()
	defer mt.Unlock()

	now := time.Now()
	originalSize := len(mt.lastTs)

	// Optimization: if the map is empty, do nothing.
	if originalSize == 0 {
		return 0
	}

	// Create a new map.
	// We let it grow organically to avoid allocating memory for the stale data.
	newMap := make(map[identity.Stream]pcommon.Timestamp)

	for stream, ts := range mt.lastTs {
		// Convert pcommon timestamp (nanos) to time.Time.
		tsTime := time.Unix(0, int64(ts))

		// If the age is less than the max idle time, keep it.
		if now.Sub(tsTime) < mt.seriesIdleTimeout {
			newMap[stream] = ts
		}
	}

	// Swap the pointer. The old map (mt.lastTs) loses the reference.
	// The Go Garbage Collector will detect this and release all memory allocated
	// by the old buckets.
	mt.lastTs = newMap

	// Reset the string pool as well since we cleaned up the streams.
	mt.stringPool = newStringPool()

	// Return the number of removed items.
	return originalSize - len(newMap)
}
