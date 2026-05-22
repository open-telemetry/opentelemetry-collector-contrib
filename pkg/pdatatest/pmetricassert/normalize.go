// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"

import (
	"encoding/json"
	"sort"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// normalize produces the identity-only document form of m.
//
// Normalization merges compatible resources (by resource attributes), scopes
// (by name+version) and metrics (by name) so that batch boundaries do not
// influence the assertion. Datapoints are folded into a set keyed by their
// attribute values; duplicate logical MTS entries collapse to one.
func normalize(m pmetric.Metrics) *document {
	type dpKey struct {
		attrs string
	}
	type metricAgg struct {
		assertion  metricAssertion
		datapoints map[dpKey]datapointAssertion
	}
	type scopeKey struct {
		name, version string
	}
	type scopeAgg struct {
		name, version string
		metrics       map[string]*metricAgg
	}
	type resourceAgg struct {
		attrs  map[string]any
		scopes map[scopeKey]*scopeAgg
	}

	resourceByKey := map[string]*resourceAgg{}
	resourceOrder := []string{}

	rms := m.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		attrs := attrMapToRaw(rm.Resource().Attributes())
		rk := canonKey(attrs)
		rAgg, ok := resourceByKey[rk]
		if !ok {
			rAgg = &resourceAgg{attrs: attrs, scopes: map[scopeKey]*scopeAgg{}}
			resourceByKey[rk] = rAgg
			resourceOrder = append(resourceOrder, rk)
		}

		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			sk := scopeKey{name: sm.Scope().Name(), version: sm.Scope().Version()}
			sAgg, ok := rAgg.scopes[sk]
			if !ok {
				sAgg = &scopeAgg{name: sk.name, version: sk.version, metrics: map[string]*metricAgg{}}
				rAgg.scopes[sk] = sAgg
			}

			ms := sm.Metrics()
			for k := 0; k < ms.Len(); k++ {
				metric := ms.At(k)
				mAgg, ok := sAgg.metrics[metric.Name()]
				if !ok {
					mAgg = &metricAgg{
						assertion:  buildMetricAssertion(metric),
						datapoints: map[dpKey]datapointAssertion{},
					}
					sAgg.metrics[metric.Name()] = mAgg
				}
				for _, dpAttrs := range extractDatapointAttributes(metric) {
					raw := attrMapToRaw(dpAttrs)
					key := dpKey{attrs: canonKey(raw)}
					if _, dup := mAgg.datapoints[key]; dup {
						continue
					}
					mAgg.datapoints[key] = datapointAssertion{Attributes: raw}
				}
			}
		}
	}

	doc := &document{Version: documentVersion, Signal: "metrics"}
	sort.Strings(resourceOrder)
	for _, rk := range resourceOrder {
		rAgg := resourceByKey[rk]
		res := resourceAssertion{Attributes: rAgg.attrs}

		scopeKeys := make([]scopeKey, 0, len(rAgg.scopes))
		for k := range rAgg.scopes {
			scopeKeys = append(scopeKeys, k)
		}
		sort.Slice(scopeKeys, func(i, j int) bool {
			if scopeKeys[i].name != scopeKeys[j].name {
				return scopeKeys[i].name < scopeKeys[j].name
			}
			return scopeKeys[i].version < scopeKeys[j].version
		})
		for _, sk := range scopeKeys {
			sAgg := rAgg.scopes[sk]
			scope := scopeAssertion{Name: sAgg.name, Version: sAgg.version}

			metricNames := make([]string, 0, len(sAgg.metrics))
			for n := range sAgg.metrics {
				metricNames = append(metricNames, n)
			}
			sort.Strings(metricNames)
			for _, n := range metricNames {
				mAgg := sAgg.metrics[n]
				metricAssert := mAgg.assertion
				dpList := make([]datapointAssertion, 0, len(mAgg.datapoints))
				for _, dp := range mAgg.datapoints {
					dpList = append(dpList, dp)
				}
				sort.Slice(dpList, func(i, j int) bool {
					return canonKey(dpList[i].Attributes) < canonKey(dpList[j].Attributes)
				})
				metricAssert.Datapoints = dpList
				scope.Metrics = append(scope.Metrics, metricAssert)
			}
			res.Scopes = append(res.Scopes, scope)
		}
		doc.Resources = append(doc.Resources, res)
	}
	return doc
}

func buildMetricAssertion(metric pmetric.Metric) metricAssertion {
	a := metricAssertion{
		Name: metric.Name(),
		Unit: metric.Unit(),
		Type: metricTypeString(metric.Type()),
	}
	switch metric.Type() {
	case pmetric.MetricTypeSum:
		sum := metric.Sum()
		a.Temporality = temporalityString(sum.AggregationTemporality())
		mono := sum.IsMonotonic()
		a.Monotonic = &mono
	case pmetric.MetricTypeHistogram:
		a.Temporality = temporalityString(metric.Histogram().AggregationTemporality())
	case pmetric.MetricTypeExponentialHistogram:
		a.Temporality = temporalityString(metric.ExponentialHistogram().AggregationTemporality())
	}
	return a
}

func metricTypeString(t pmetric.MetricType) string {
	switch t {
	case pmetric.MetricTypeGauge:
		return "gauge"
	case pmetric.MetricTypeSum:
		return "sum"
	case pmetric.MetricTypeHistogram:
		return "histogram"
	case pmetric.MetricTypeExponentialHistogram:
		return "exponential_histogram"
	case pmetric.MetricTypeSummary:
		return "summary"
	default:
		return "empty"
	}
}

func temporalityString(t pmetric.AggregationTemporality) string {
	switch t {
	case pmetric.AggregationTemporalityDelta:
		return "delta"
	case pmetric.AggregationTemporalityCumulative:
		return "cumulative"
	default:
		return "unspecified"
	}
}

func extractDatapointAttributes(metric pmetric.Metric) []pcommon.Map {
	var out []pcommon.Map
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps := metric.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			out = append(out, dps.At(i).Attributes())
		}
	case pmetric.MetricTypeSum:
		dps := metric.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			out = append(out, dps.At(i).Attributes())
		}
	case pmetric.MetricTypeHistogram:
		dps := metric.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			out = append(out, dps.At(i).Attributes())
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := metric.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			out = append(out, dps.At(i).Attributes())
		}
	case pmetric.MetricTypeSummary:
		dps := metric.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			out = append(out, dps.At(i).Attributes())
		}
	}
	return out
}

func attrMapToRaw(m pcommon.Map) map[string]any {
	if m.Len() == 0 {
		return nil
	}
	return m.AsRaw()
}

// canonKey produces a stable string key for a map-like structure. It is used
// for map lookup and for deterministic sort order in the emitted document.
func canonKey(v any) string {
	if v == nil {
		return "null"
	}
	b, err := json.Marshal(sortedAny(v))
	if err != nil {
		return ""
	}
	return string(b)
}

func sortedAny(v any) any {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([][2]any, 0, len(keys))
		for _, k := range keys {
			out = append(out, [2]any{k, sortedAny(t[k])})
		}
		return out
	case []any:
		cp := make([]any, len(t))
		for i, e := range t {
			cp[i] = sortedAny(e)
		}
		return cp
	default:
		return v
	}
}
