// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"

import (
	"encoding/json"
	"sort"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// snapshot is the normalized form of actual metrics: plain data, no matchers.
// It is the right-hand side of every comparison in assert.go, and the value
// WriteAssertionFile serializes.
//
// The assertion types in document.go are the left-hand side and hold matchers
// instead of values. Both describe the same file format: the assertion side
// additionally accepts the operator suffixes that this side never produces.
//
// Fields are exported because yaml.v3 only marshals exported fields; the tags
// and field order define WriteAssertionFile's output.
type snapshot struct {
	Version   int                `yaml:"version"`
	Signal    string             `yaml:"signal"`
	Resources []resourceSnapshot `yaml:"resources"`
}

type resourceSnapshot struct {
	Attributes map[string]any  `yaml:"attributes,omitempty"`
	Scopes     []scopeSnapshot `yaml:"scopes"`
}

type scopeSnapshot struct {
	Name    string           `yaml:"name,omitempty"`
	Version string           `yaml:"version,omitempty"`
	Metrics []metricSnapshot `yaml:"metrics"`
}

type metricSnapshot struct {
	Name        string              `yaml:"name"`
	Type        string              `yaml:"type"`
	Unit        string              `yaml:"unit,omitempty"`
	Temporality string              `yaml:"temporality,omitempty"`
	Monotonic   *bool               `yaml:"monotonic,omitempty"`
	Datapoints  []datapointSnapshot `yaml:"datapoints,omitempty"`
}

type datapointSnapshot struct {
	Attributes     map[string]any `yaml:"attributes,omitempty"`
	IntValue       *int64         `yaml:"int_value,omitempty"`
	DoubleValue    *float64       `yaml:"double_value,omitempty"`
	Count          *uint64        `yaml:"count,omitempty"`
	Sum            *float64       `yaml:"sum,omitempty"`
	ExplicitBounds *[]float64     `yaml:"explicit_bounds,omitempty"`
	BucketCounts   []uint64       `yaml:"bucket_counts,omitempty"`
	Min            *float64       `yaml:"min,omitempty"`
	Max            *float64       `yaml:"max,omitempty"`
}

// normalize produces the snapshot form of m, capturing every field the
// snapshot can represent. Dropping fields a snapshot file should not pin is
// the write path's job, in project.
//
// Normalization merges compatible resources (by resource attributes), scopes
// (by name+version) and metrics (by name) so that batch boundaries do not
// influence the assertion. Datapoints are keyed by their attribute values for
// order-insensitive comparison, so datapoints sharing an attribute set collapse
// into one logical series.
func normalize(m pmetric.Metrics) *snapshot {
	type metricAgg struct {
		metric     metricSnapshot
		datapoints map[string]datapointSnapshot
	}
	type scopeKey struct {
		name, version string
	}
	type scopeAgg struct {
		metrics map[string]*metricAgg
	}
	type resourceAgg struct {
		attrs  map[string]any
		scopes map[scopeKey]*scopeAgg
	}

	resourceByKey := map[string]*resourceAgg{}

	rms := m.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		attrs := attrMapToRaw(rm.Resource().Attributes())
		rk := canonKey(attrs)
		rAgg, ok := resourceByKey[rk]
		if !ok {
			rAgg = &resourceAgg{attrs: attrs, scopes: map[scopeKey]*scopeAgg{}}
			resourceByKey[rk] = rAgg
		}

		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			sk := scopeKey{name: sm.Scope().Name(), version: sm.Scope().Version()}
			sAgg, ok := rAgg.scopes[sk]
			if !ok {
				sAgg = &scopeAgg{metrics: map[string]*metricAgg{}}
				rAgg.scopes[sk] = sAgg
			}

			ms := sm.Metrics()
			for k := 0; k < ms.Len(); k++ {
				metric := ms.At(k)
				mAgg, ok := sAgg.metrics[metric.Name()]
				if !ok {
					mAgg = &metricAgg{
						metric:     buildMetricSnapshot(metric),
						datapoints: map[string]datapointSnapshot{},
					}
					sAgg.metrics[metric.Name()] = mAgg
				}
				for _, extDP := range extractDatapoints(metric) {
					dp := datapointSnapshot{
						Attributes:     attrMapToRaw(extDP.attributes),
						IntValue:       extDP.intValue,
						DoubleValue:    extDP.doubleValue,
						Count:          extDP.count,
						Sum:            extDP.sum,
						ExplicitBounds: extDP.explicitBounds,
						BucketCounts:   extDP.bucketCounts,
						Min:            extDP.minVal,
						Max:            extDP.maxVal,
					}
					mAgg.datapoints[canonKey(dp.Attributes)] = dp
				}
			}
		}
	}

	out := &snapshot{Version: documentVersion, Signal: "metrics"}
	for _, rk := range sortedKeys(resourceByKey) {
		rAgg := resourceByKey[rk]
		res := resourceSnapshot{Attributes: rAgg.attrs}

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
			scope := scopeSnapshot{Name: sk.name, Version: sk.version}
			for _, name := range sortedKeys(sAgg.metrics) {
				mAgg := sAgg.metrics[name]
				metric := mAgg.metric
				for _, dpk := range sortedKeys(mAgg.datapoints) {
					metric.Datapoints = append(metric.Datapoints, mAgg.datapoints[dpk])
				}
				scope.Metrics = append(scope.Metrics, metric)
			}
			res.Scopes = append(res.Scopes, scope)
		}
		out.Resources = append(out.Resources, res)
	}
	return out
}

// sortedKeys returns m's keys in ascending order, so that the snapshot is
// deterministic regardless of map iteration order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func buildMetricSnapshot(metric pmetric.Metric) metricSnapshot {
	s := metricSnapshot{
		Name: metric.Name(),
		Unit: metric.Unit(),
		Type: metricTypeString(metric.Type()),
	}
	switch metric.Type() {
	case pmetric.MetricTypeSum:
		sum := metric.Sum()
		s.Temporality = temporalityString(sum.AggregationTemporality())
		mono := sum.IsMonotonic()
		s.Monotonic = &mono
	case pmetric.MetricTypeHistogram:
		s.Temporality = temporalityString(metric.Histogram().AggregationTemporality())
	case pmetric.MetricTypeExponentialHistogram:
		s.Temporality = temporalityString(metric.ExponentialHistogram().AggregationTemporality())
	}
	return s
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

type extractedDatapoint struct {
	attributes     pcommon.Map
	intValue       *int64
	doubleValue    *float64
	count          *uint64
	sum            *float64
	minVal         *float64
	maxVal         *float64
	explicitBounds *[]float64
	bucketCounts   []uint64
}

func extractDatapoints(metric pmetric.Metric) []extractedDatapoint {
	var out []extractedDatapoint
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps := metric.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			edp := extractedDatapoint{attributes: dp.Attributes()}
			edp.intValue, edp.doubleValue = extractValue(dp)
			out = append(out, edp)
		}
	case pmetric.MetricTypeSum:
		dps := metric.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			edp := extractedDatapoint{attributes: dp.Attributes()}
			edp.intValue, edp.doubleValue = extractValue(dp)
			out = append(out, edp)
		}
	case pmetric.MetricTypeHistogram:
		dps := metric.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			edp := extractedDatapoint{
				attributes: dp.Attributes(),
			}
			count := dp.Count()
			edp.count = &count
			if dp.HasSum() {
				sumVal := dp.Sum()
				edp.sum = &sumVal
			}
			if dp.HasMin() {
				minVal := dp.Min()
				edp.minVal = &minVal
			}
			if dp.HasMax() {
				maxVal := dp.Max()
				edp.maxVal = &maxVal
			}
			bounds := dp.ExplicitBounds().AsRaw()
			edp.explicitBounds = &bounds
			if dp.BucketCounts().Len() > 0 {
				edp.bucketCounts = dp.BucketCounts().AsRaw()
			}
			out = append(out, edp)
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := metric.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			out = append(out, extractedDatapoint{attributes: dps.At(i).Attributes()})
		}
	case pmetric.MetricTypeSummary:
		dps := metric.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			out = append(out, extractedDatapoint{attributes: dps.At(i).Attributes()})
		}
	}
	return out
}

func extractValue(dp pmetric.NumberDataPoint) (intVal *int64, doubleVal *float64) {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		v := dp.IntValue()
		return &v, nil
	case pmetric.NumberDataPointValueTypeDouble:
		v := dp.DoubleValue()
		return nil, &v
	}
	return nil, nil
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
