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

type tracesGroup struct {
	traces         ptrace.Traces
	resourceHashes [][16]byte
}

func newTracesGroup() *tracesGroup {
	return &tracesGroup{traces: ptrace.NewTraces()}
}

// findOrCreateResource searches for a Resource with matching attributes and returns it. If nothing is found, it is being created
func (tg *tracesGroup) findOrCreateResourceSpans(originResource pcommon.Resource, requiredAttributes pcommon.Map) ptrace.ResourceSpans {
	referenceResource := buildReferenceResource(originResource, requiredAttributes)
	referenceResourceHash := pdatautil.MapHash(referenceResource.Attributes())

	rss := tg.traces.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		if tg.resourceHashes[i] == referenceResourceHash {
			return rss.At(i)
		}
	}

	rs := tg.traces.ResourceSpans().AppendEmpty()
	referenceResource.MoveTo(rs.Resource())
	tg.resourceHashes = append(tg.resourceHashes, referenceResourceHash)
	return rs
}

type metricsGroup struct {
	metrics        pmetric.Metrics
	resourceHashes [][16]byte
}

func newMetricsGroup() *metricsGroup {
	return &metricsGroup{metrics: pmetric.NewMetrics()}
}

// findOrCreateResourceMetrics searches for a Resource with matching attributes and returns it. If nothing is found, it is being created
func (mg *metricsGroup) findOrCreateResourceMetrics(originResource pcommon.Resource, requiredAttributes pcommon.Map) pmetric.ResourceMetrics {
	referenceResource := buildReferenceResource(originResource, requiredAttributes)
	referenceResourceHash := pdatautil.MapHash(referenceResource.Attributes())

	rms := mg.metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		if mg.resourceHashes[i] == referenceResourceHash {
			return rms.At(i)
		}
	}

	rm := mg.metrics.ResourceMetrics().AppendEmpty()
	referenceResource.MoveTo(rm.Resource())
	mg.resourceHashes = append(mg.resourceHashes, referenceResourceHash)
	return rm

}

type logsGroup struct {
	logs           plog.Logs
	resourceHashes [][16]byte
}

// newLogsGroup returns new logsGroup with predefined capacity
func newLogsGroup() *logsGroup {
	return &logsGroup{logs: plog.NewLogs()}
}

// findOrCreateResourceLogs searches for a Resource with matching attributes and returns it. If nothing is found, it is being created
func (lg *logsGroup) findOrCreateResourceLogs(originResource pcommon.Resource, requiredAttributes pcommon.Map) plog.ResourceLogs {
	referenceResource := buildReferenceResource(originResource, requiredAttributes)
	referenceResourceHash := pdatautil.MapHash(referenceResource.Attributes())

	rls := lg.logs.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		if lg.resourceHashes[i] == referenceResourceHash {
			return rls.At(i)
		}
	}

	rl := lg.logs.ResourceLogs().AppendEmpty()
	referenceResource.MoveTo(rl.Resource())
	lg.resourceHashes = append(lg.resourceHashes, referenceResourceHash)
	return rl
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
	requiredAttributes.Range(func(k string, v pcommon.Value) bool {
		v.CopyTo(referenceResource.Attributes().PutEmpty(k))
		return true
	})
	return referenceResource
}
