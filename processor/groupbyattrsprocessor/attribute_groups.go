// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbyattrsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

type resourceCacheKey struct {
	origin   [16]byte
	required [16]byte
}

// resourceIndex resolves which grouped Resource a record belongs to. byPair skips the merge for
// an already known group; byMerged is still needed because different origin resources can merge
// into the same attribute set.
type resourceIndex struct {
	byPair   map[resourceCacheKey]int
	byMerged map[[16]byte]int
}

func newResourceIndex() resourceIndex {
	return resourceIndex{
		byPair:   make(map[resourceCacheKey]int),
		byMerged: make(map[[16]byte]int),
	}
}

func (ri *resourceIndex) lookup(key resourceCacheKey) (int, bool) {
	idx, ok := ri.byPair[key]
	return idx, ok
}

// resolve maps key onto the group identified by mergedHash, calling newIndex to append one when
// mergedHash is new.
func (ri *resourceIndex) resolve(key resourceCacheKey, mergedHash [16]byte, newIndex func() int) int {
	idx, ok := ri.byMerged[mergedHash]
	if !ok {
		idx = newIndex()
		ri.byMerged[mergedHash] = idx
	}
	ri.byPair[key] = idx
	return idx
}

type tracesGroup struct {
	traces ptrace.Traces
	index  resourceIndex
}

func newTracesGroup() *tracesGroup {
	return &tracesGroup{traces: ptrace.NewTraces(), index: newResourceIndex()}
}

// findOrCreateResource searches for a Resource with matching attributes and returns it. If nothing is found, it is being created
func (tg *tracesGroup) findOrCreateResourceSpans(originResource pcommon.Resource, originHash [16]byte, requiredAttributes pcommon.Map) ptrace.ResourceSpans {
	rss := tg.traces.ResourceSpans()

	key := resourceCacheKey{origin: originHash, required: pdatautil.MapHash(requiredAttributes)}
	if idx, ok := tg.index.lookup(key); ok {
		return rss.At(idx)
	}

	referenceResource := buildReferenceResource(originResource, requiredAttributes)
	referenceResourceHash := pdatautil.MapHash(referenceResource.Attributes())

	idx := tg.index.resolve(key, referenceResourceHash, func() int {
		rs := rss.AppendEmpty()
		referenceResource.MoveTo(rs.Resource())
		return rss.Len() - 1
	})
	return rss.At(idx)
}

type metricsGroup struct {
	metrics pmetric.Metrics
	index   resourceIndex
}

func newMetricsGroup() *metricsGroup {
	return &metricsGroup{metrics: pmetric.NewMetrics(), index: newResourceIndex()}
}

// findOrCreateResourceMetrics searches for a Resource with matching attributes and returns it. If nothing is found, it is being created
func (mg *metricsGroup) findOrCreateResourceMetrics(originResource pcommon.Resource, originHash [16]byte, requiredAttributes pcommon.Map) pmetric.ResourceMetrics {
	rms := mg.metrics.ResourceMetrics()

	key := resourceCacheKey{origin: originHash, required: pdatautil.MapHash(requiredAttributes)}
	if idx, ok := mg.index.lookup(key); ok {
		return rms.At(idx)
	}

	referenceResource := buildReferenceResource(originResource, requiredAttributes)
	referenceResourceHash := pdatautil.MapHash(referenceResource.Attributes())

	idx := mg.index.resolve(key, referenceResourceHash, func() int {
		rm := rms.AppendEmpty()
		referenceResource.MoveTo(rm.Resource())
		return rms.Len() - 1
	})
	return rms.At(idx)
}

type logsGroup struct {
	logs  plog.Logs
	index resourceIndex
}

// newLogsGroup returns new logsGroup with predefined capacity
func newLogsGroup() *logsGroup {
	return &logsGroup{logs: plog.NewLogs(), index: newResourceIndex()}
}

// findOrCreateResourceLogs searches for a Resource with matching attributes and returns it. If nothing is found, it is being created
func (lg *logsGroup) findOrCreateResourceLogs(originResource pcommon.Resource, originHash [16]byte, requiredAttributes pcommon.Map) plog.ResourceLogs {
	rls := lg.logs.ResourceLogs()

	key := resourceCacheKey{origin: originHash, required: pdatautil.MapHash(requiredAttributes)}
	if idx, ok := lg.index.lookup(key); ok {
		return rls.At(idx)
	}

	referenceResource := buildReferenceResource(originResource, requiredAttributes)
	referenceResourceHash := pdatautil.MapHash(referenceResource.Attributes())

	idx := lg.index.resolve(key, referenceResourceHash, func() int {
		rl := rls.AppendEmpty()
		referenceResource.MoveTo(rl.Resource())
		return rls.Len() - 1
	})
	return rls.At(idx)
}

func instrumentationLibrariesEqual(il1, il2 pcommon.InstrumentationScope) bool {
	return il1.Name() == il2.Name() && il1.Version() == il2.Version()
}

// matchingScopeSpans searches for a ptrace.ScopeSpans instance matching
// given InstrumentationScope. If nothing is found, it creates a new one
func matchingScopeSpans(rl ptrace.ResourceSpans, library pcommon.InstrumentationScope) ptrace.ScopeSpans {
	ilss := rl.ScopeSpans()
	for i := 0; i < ilss.Len(); i++ {
		ils := ilss.At(i)
		if instrumentationLibrariesEqual(ils.Scope(), library) {
			return ils
		}
	}

	ils := ilss.AppendEmpty()
	library.CopyTo(ils.Scope())
	return ils
}

// matchingScopeLogs searches for a plog.ScopeLogs instance matching
// given InstrumentationScope. If nothing is found, it creates a new one
func matchingScopeLogs(rl plog.ResourceLogs, library pcommon.InstrumentationScope) plog.ScopeLogs {
	ills := rl.ScopeLogs()
	for i := 0; i < ills.Len(); i++ {
		sl := ills.At(i)
		if instrumentationLibrariesEqual(sl.Scope(), library) {
			return sl
		}
	}

	sl := ills.AppendEmpty()
	library.CopyTo(sl.Scope())
	return sl
}

// matchingScopeMetrics searches for a pmetric.ScopeMetrics instance matching
// given InstrumentationScope. If nothing is found, it creates a new one
func matchingScopeMetrics(rm pmetric.ResourceMetrics, library pcommon.InstrumentationScope) pmetric.ScopeMetrics {
	ilms := rm.ScopeMetrics()
	for i := 0; i < ilms.Len(); i++ {
		ilm := ilms.At(i)
		if instrumentationLibrariesEqual(ilm.Scope(), library) {
			return ilm
		}
	}

	ilm := ilms.AppendEmpty()
	library.CopyTo(ilm.Scope())
	return ilm
}

// buildReferenceResource returns a new resource that we'll be looking for in existing Resources
// as a merge of the Attributes of the original Resource with the requested Attributes.
func buildReferenceResource(originResource pcommon.Resource, requiredAttributes pcommon.Map) pcommon.Resource {
	referenceResource := pcommon.NewResource()
	originResource.Attributes().CopyTo(referenceResource.Attributes())
	for k, v := range requiredAttributes.All() {
		v.CopyTo(referenceResource.Attributes().PutEmpty(k))
	}
	return referenceResource
}
