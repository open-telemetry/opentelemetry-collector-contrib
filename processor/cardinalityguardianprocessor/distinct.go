// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinalityguardianprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cardinalityguardianprocessor"

import (
	"cmp"
	"slices"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// metricKind is the metric type and temporality a tracker has seen. It decides
// which recommendation is safe for that type.
type metricKind uint8

const (
	kindUnknown metricKind = iota
	kindGauge
	kindDeltaSum
	kindCumulativeSum
	kindDeltaHistogram
	kindCumulativeHistogram
	kindDeltaExponentialHistogram
	kindCumulativeExponentialHistogram
	kindSummary
	// kindMixed marks a metric name seen with more than one kind.
	kindMixed
)

func (k metricKind) String() string {
	switch k {
	case kindGauge:
		return "gauge"
	case kindDeltaSum:
		return "delta_sum"
	case kindCumulativeSum:
		return "cumulative_sum"
	case kindDeltaHistogram:
		return "delta_histogram"
	case kindCumulativeHistogram:
		return "cumulative_histogram"
	case kindDeltaExponentialHistogram:
		return "delta_exponential_histogram"
	case kindCumulativeExponentialHistogram:
		return "cumulative_exponential_histogram"
	case kindSummary:
		return "summary"
	case kindMixed:
		return "mixed"
	default:
		return "unknown"
	}
}

// kindOf returns the kind of m. Unspecified temporality counts as cumulative,
// which gets the safer recommendation.
func kindOf(m pmetric.Metric) metricKind {
	pick := func(t pmetric.AggregationTemporality, delta, cumulative metricKind) metricKind {
		if t == pmetric.AggregationTemporalityDelta {
			return delta
		}
		return cumulative
	}
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		return kindGauge
	case pmetric.MetricTypeSum:
		return pick(m.Sum().AggregationTemporality(), kindDeltaSum, kindCumulativeSum)
	case pmetric.MetricTypeHistogram:
		return pick(m.Histogram().AggregationTemporality(), kindDeltaHistogram, kindCumulativeHistogram)
	case pmetric.MetricTypeExponentialHistogram:
		return pick(m.ExponentialHistogram().AggregationTemporality(), kindDeltaExponentialHistogram, kindCumulativeExponentialHistogram)
	case pmetric.MetricTypeSummary:
		return kindSummary
	default:
		return kindUnknown
	}
}

// metricInfo is the metric kind and service.name of the data points being
// counted. Trackers record it to choose a recommendation.
type metricInfo struct {
	kind    metricKind
	service string
}

// serviceName returns the service.name resource attribute, or "" if it is absent.
func serviceName(res pcommon.Resource) string {
	if v, ok := res.Attributes().Get("service.name"); ok {
		return v.AsString()
	}
	return ""
}

// recordMetrics tracks the data-point attributes of md, skipping
// never_drop_labels as the enforcing path does. It must only read md: the
// fanout consumer may share it between pipelines as read-only.
func (p *cardinalityProcessor) recordMetrics(md pmetric.Metrics) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		service := serviceName(rms.At(i).Resource())
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				mInfo := metricInfo{kind: kindOf(m), service: service}
				forEachPointAttributes(m, func(attrs pcommon.Map) {
					p.recordAttributes(m.Name(), mInfo, attrs)
				})
			}
		}
	}
}

func forEachPointAttributes(m pmetric.Metric, fn func(pcommon.Map)) {
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		for i, dps := 0, m.Gauge().DataPoints(); i < dps.Len(); i++ {
			fn(dps.At(i).Attributes())
		}
	case pmetric.MetricTypeSum:
		for i, dps := 0, m.Sum().DataPoints(); i < dps.Len(); i++ {
			fn(dps.At(i).Attributes())
		}
	case pmetric.MetricTypeHistogram:
		for i, dps := 0, m.Histogram().DataPoints(); i < dps.Len(); i++ {
			fn(dps.At(i).Attributes())
		}
	case pmetric.MetricTypeExponentialHistogram:
		for i, dps := 0, m.ExponentialHistogram().DataPoints(); i < dps.Len(); i++ {
			fn(dps.At(i).Attributes())
		}
	case pmetric.MetricTypeSummary:
		for i, dps := 0, m.Summary().DataPoints(); i < dps.Len(); i++ {
			fn(dps.At(i).Attributes())
		}
	}
}

func (p *cardinalityProcessor) recordAttributes(metricName string, mInfo metricInfo, attrs pcommon.Map) {
	attrs.Range(func(k string, v pcommon.Value) bool {
		if p.isProtected(k) {
			return true
		}
		// shouldDrop inserts the value; its decision is unused in read-only mode.
		p.shouldDrop(metricName, k, v, mInfo)
		return true
	})
}

// distinctEntry is a tracker's distinct-value count for the epoch that ended.
type distinctEntry struct {
	metricName   string
	labelKey     string
	distinct     uint64
	kind         metricKind
	multiService bool
}

// compareDistinct orders by distinct values, highest first, then by name.
func compareDistinct(a, b distinctEntry) int {
	return cmp.Or(
		cmp.Compare(b.distinct, a.distinct),
		strings.Compare(a.metricName, b.metricName),
		strings.Compare(a.labelKey, b.labelKey),
	)
}

// keepTopDistinct adds d to buf, keeping the n highest-ranked entries.
func keepTopDistinct(buf []distinctEntry, d distinctEntry, n int) []distinctEntry {
	if len(buf) < n {
		return append(buf, d)
	}
	last := 0
	for i := 1; i < len(buf); i++ {
		if compareDistinct(buf[i], buf[last]) > 0 {
			last = i
		}
	}
	if compareDistinct(d, buf[last]) < 0 {
		buf[last] = d
	}
	return buf
}

// publishTopDistinct stores buf, sorted, for the telemetry callback. An empty
// buf clears the previous epoch's entries.
func (p *cardinalityProcessor) publishTopDistinct(buf []distinctEntry) {
	slices.SortFunc(buf, compareDistinct)
	p.topDistinctMu.Lock()
	p.topDistinct = buf
	p.topDistinctMu.Unlock()
}

// recommendation is the README's action code for a (metric, label) pair.
// Removing a label merges series, which is safe in the collector only for
// gauges and delta temporality; a count that combines services needs a
// per-service check first.
func recommendation(kind metricKind, multiService bool) string {
	if multiService {
		return "check_per_service"
	}
	switch kind {
	case kindGauge, kindDeltaSum, kindDeltaHistogram, kindDeltaExponentialHistogram:
		return "remove_and_reaggregate"
	default:
		return "reduce_at_source"
	}
}
