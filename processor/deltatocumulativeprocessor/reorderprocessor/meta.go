// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reorderprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/reorderprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/reorderprocessor/internal/gmetric"
)

type Meta struct {
	resources map[identity.Resource]pcommon.Resource
	scopes    map[identity.Scope]pcommon.InstrumentationScope
	metrics   map[identity.Metric]pmetric.Metric
}

func (mt Meta) Store(rm gmetric.ResourceMetric) {
	rid := identity.OfResource(rm.Resource)
	if _, ok := mt.resources[rid]; !ok {
		mt.resources[rid] = rm.Resource
	}

	sid := identity.OfScope(rid, rm.Scope)
	if _, ok := mt.scopes[sid]; !ok {
		mt.scopes[sid] = rm.Scope
	}

	mid := identity.OfMetric(sid, rm.Metric)
	if _, ok := mt.metrics[mid]; !ok {
		mt.metrics[mid] = rm.Metric
	}
}

func (mt Meta) Select(ids ...identity.Metric) Table {
	pms := pmetric.NewMetrics()

	var (
		rms = make(map[identity.Resource]pmetric.ResourceMetrics)
		sms = make(map[identity.Scope]pmetric.ScopeMetrics)
		ms  = make(map[identity.Metric]pmetric.Metric)
	)

	for _, id := range ids {
		if _, ok := ms[id]; ok {
			continue
		}

		rid := id.Resource()
		rm, ok := rms[rid]
		if !ok {
			rm = pms.ResourceMetrics().AppendEmpty()
			mt.resources[rid].CopyTo(rm.Resource())
			rms[rid] = rm
		}

		sid := id.Scope()
		sm, ok := sms[sid]
		if !ok {
			sm = rm.ScopeMetrics().AppendEmpty()
			mt.scopes[sid].CopyTo(sm.Scope())
			sms[sid] = sm
		}

		m := sm.Metrics().AppendEmpty()
		mt.metrics[id].CopyTo(m)
		ms[id] = m
	}

	return Table{
		metrics: pms,
		lookup:  ms,
	}
}

type Table struct {
	metrics pmetric.Metrics
	lookup  map[identity.Metric]pmetric.Metric
}

func (t Table) MoveTo(into pmetric.Metrics) {
	t.metrics.ResourceMetrics().MoveAndAppendTo(into.ResourceMetrics())
	clear(t.lookup)
}

func (t Table) Lookup(id identity.Metric) pmetric.Metric {
	return t.lookup[id]
}
