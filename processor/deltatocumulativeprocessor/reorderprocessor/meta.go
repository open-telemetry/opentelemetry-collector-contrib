// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reorderprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/reorderprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

type Meta struct {
	resources map[identity.Resource]pcommon.Resource
	scopes    map[identity.Scope]pcommon.InstrumentationScope
	metrics   map[identity.Metric]pmetric.Metric
}

func (t Meta) Store(res pcommon.Resource, sc pcommon.InstrumentationScope, m pmetric.Metric) {
	rid := identity.OfResource(res)
	if _, ok := t.resources[rid]; !ok {
		t.resources[rid] = res
	}

	sid := identity.OfScope(rid, sc)
	if _, ok := t.scopes[sid]; !ok {
		t.scopes[sid] = sc
	}

	mid := identity.OfMetric(sid, m)
	if _, ok := t.metrics[mid]; !ok {
		t.metrics[mid] = m
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

func (mt Meta) Metrics(ids ...identity.Metric) Table {
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
