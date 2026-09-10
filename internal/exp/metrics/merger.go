// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

// Merger incrementally merges pmetric.Metrics into an accumulated destination.
// It has the same merging semantics as Merge, but caches the identities of the
// destination's ResourceMetrics / ScopeMetrics / Metrics entries between calls,
// so merging N sources into one destination costs O(N) identity computations
// instead of the O(N^2) that repeated calls to Merge incur (Merge re-hashes the
// whole destination on every call).
//
// The zero value is not usable; use NewMerger.
//
// Like Merge, any duplicate entries that already exist in the destination are
// not combined: subsequent merges apply to the first matching entry.
type Merger struct {
	dest      pmetric.Metrics
	resources map[identity.Resource]*mergerResource
}

type mergerResource struct {
	rm     pmetric.ResourceMetrics
	id     identity.Resource
	scopes map[identity.Scope]*mergerScope
}

type mergerScope struct {
	sm      pmetric.ScopeMetrics
	id      identity.Scope
	metrics map[identity.Metric]pmetric.Metric
}

// NewMerger returns a Merger accumulating into dest. Any data already present
// in dest is indexed once, up front.
func NewMerger(dest pmetric.Metrics) *Merger {
	m := &Merger{
		dest:      dest,
		resources: make(map[identity.Resource]*mergerResource),
	}
	for i := 0; i < dest.ResourceMetrics().Len(); i++ {
		m.indexResource(dest.ResourceMetrics().At(i))
	}
	return m
}

// Metrics returns the destination the Merger accumulates into.
func (m *Merger) Metrics() pmetric.Metrics {
	return m.dest
}

// Merge merges md into the destination. md is not modified.
func (m *Merger) Merge(md pmetric.Metrics) {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rmB := md.ResourceMetrics().At(i)
		resourceID := identity.OfResource(rmB.Resource())

		mr, ok := m.resources[resourceID]
		if !ok {
			// We didn't find a match. Add it to dest and index the copy so
			// later merges find it without re-hashing.
			newRM := m.dest.ResourceMetrics().AppendEmpty()
			rmB.CopyTo(newRM)
			m.indexResource(newRM)
			continue
		}

		m.mergeResourceMetrics(mr, rmB)
	}
}

func (m *Merger) mergeResourceMetrics(mr *mergerResource, rmB pmetric.ResourceMetrics) {
	for i := 0; i < rmB.ScopeMetrics().Len(); i++ {
		smB := rmB.ScopeMetrics().At(i)
		scopeID := identity.OfScope(mr.id, smB.Scope())

		ms, ok := mr.scopes[scopeID]
		if !ok {
			newSM := mr.rm.ScopeMetrics().AppendEmpty()
			smB.CopyTo(newSM)
			mr.indexScope(newSM)
			continue
		}

		m.mergeScopeMetrics(ms, smB)
	}
}

func (*Merger) mergeScopeMetrics(ms *mergerScope, smB pmetric.ScopeMetrics) {
	for i := 0; i < smB.Metrics().Len(); i++ {
		mB := smB.Metrics().At(i)
		metricID := identity.OfMetric(ms.id, mB)

		mA, ok := ms.metrics[metricID]
		if !ok {
			newM := ms.sm.Metrics().AppendEmpty()
			mB.CopyTo(newM)
			ms.indexMetric(newM)
			continue
		}

		//exhaustive:enforce
		switch mA.Type() {
		case pmetric.MetricTypeGauge:
			mergeDataPoints(mA.Gauge().DataPoints(), mB.Gauge().DataPoints())
		case pmetric.MetricTypeSum:
			mergeDataPoints(mA.Sum().DataPoints(), mB.Sum().DataPoints())
		case pmetric.MetricTypeHistogram:
			mergeDataPoints(mA.Histogram().DataPoints(), mB.Histogram().DataPoints())
		case pmetric.MetricTypeExponentialHistogram:
			mergeDataPoints(mA.ExponentialHistogram().DataPoints(), mB.ExponentialHistogram().DataPoints())
		case pmetric.MetricTypeSummary:
			mergeDataPoints(mA.Summary().DataPoints(), mB.Summary().DataPoints())
		}
	}
}

// indexResource indexes rm and its subtree. If an entry with the same identity
// is already indexed, the existing entry is kept (first match wins, matching
// Merge's linear-scan behavior on duplicate destination entries).
func (m *Merger) indexResource(rm pmetric.ResourceMetrics) {
	id := identity.OfResource(rm.Resource())
	if _, ok := m.resources[id]; ok {
		return
	}
	mr := &mergerResource{
		rm:     rm,
		id:     id,
		scopes: make(map[identity.Scope]*mergerScope),
	}
	for i := 0; i < rm.ScopeMetrics().Len(); i++ {
		mr.indexScope(rm.ScopeMetrics().At(i))
	}
	m.resources[id] = mr
}

func (mr *mergerResource) indexScope(sm pmetric.ScopeMetrics) {
	id := identity.OfScope(mr.id, sm.Scope())
	if _, ok := mr.scopes[id]; ok {
		return
	}
	ms := &mergerScope{
		sm:      sm,
		id:      id,
		metrics: make(map[identity.Metric]pmetric.Metric),
	}
	for i := 0; i < sm.Metrics().Len(); i++ {
		ms.indexMetric(sm.Metrics().At(i))
	}
	mr.scopes[id] = ms
}

func (ms *mergerScope) indexMetric(metric pmetric.Metric) {
	id := identity.OfMetric(ms.id, metric)
	if _, ok := ms.metrics[id]; ok {
		return
	}
	ms.metrics[id] = metric
}
