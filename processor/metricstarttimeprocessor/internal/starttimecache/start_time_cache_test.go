// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package starttimecache // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/starttimecache"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

func TestStartTimeCache_NewStartTimeCache(t *testing.T) {
	gcInterval := time.Minute
	stc := NewStartTimeCache(gcInterval)

	assert.NotNil(t, stc)
	assert.Equal(t, gcInterval, stc.gcInterval)
	assert.WithinDuration(t, time.Now(), stc.lastGC, time.Second)
	assert.Empty(t, stc.resourceMap)
}

func TestStartTimeCache_Get(t *testing.T) {
	stc := NewStartTimeCache(time.Minute)
	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("k1", "v1")
	resourceHash := pdatautil.MapHash(resourceAttrs)

	tsm := stc.Get(resourceHash)
	assert.NotNil(t, tsm)
	assert.True(t, tsm.Mark)

	tsm2 := stc.Get(resourceHash)
	assert.Equal(t, tsm, tsm2)
	assert.True(t, tsm2.Mark)
}

func TestStartTimeCache_MaybeGC(t *testing.T) {
	stc := NewStartTimeCache(time.Millisecond)
	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("k1", "v1")
	resourceHash := pdatautil.MapHash(resourceAttrs)
	resourceAttrs2 := pcommon.NewMap()
	resourceAttrs2.PutStr("k2", "v2")
	resourceHash2 := pdatautil.MapHash(resourceAttrs2)

	tsm := stc.Get(resourceHash)
	tsm2 := stc.Get(resourceHash2)

	metric := pmetric.NewMetric()
	metric.SetName("test_metric")
	metric.SetEmptyGauge()
	attrs := pcommon.NewMap()
	attrs.PutStr("k1", "v1")
	tsi, found := tsm.Get(metric, attrs)
	tsi2, found2 := tsm2.Get(metric, attrs)
	assert.False(t, found)
	assert.False(t, found2)

	// Expect no GC.
	stc.MaybeGC()
	assert.True(t, tsm.Mark)
	assert.True(t, tsi.Mark)
	assert.True(t, tsm2.Mark)
	assert.True(t, tsi2.Mark)

	// Sleep for the GC interval. Expect the next GC to unmark all timeseriesInfo and resourceMap entries.
	time.Sleep(stc.gcInterval)
	stc.MaybeGC()
	time.Sleep(time.Millisecond) // Waiting for goroutine to complete.

	assert.False(t, tsm.Mark)
	assert.False(t, tsi.Mark)
	assert.False(t, tsm2.Mark)
	assert.False(t, tsi2.Mark)

	// Sleep for the GC interval. Expect the next GC to delete the resourceMap entries.
	time.Sleep(stc.gcInterval)
	stc.gc()
	assert.Empty(t, stc.resourceMap)

	tsm4 := stc.Get(resourceHash)
	assert.NotNil(t, tsm4)
	assert.True(t, tsm4.Mark)
}
