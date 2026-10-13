// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinalityguardianprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cardinalityguardianprocessor"

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// metricKind is the metric type and temporality a tracker has seen. Observe
// mode uses it to pick a recommendation that is safe for that type.
type metricKind uint8

const (
	kindUnknown metricKind = iota
	kindGauge
	kindDeltaSum
	kindCumulativeSum
	kindHistogram
	kindExponentialHistogram
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
	case kindHistogram:
		return "histogram"
	case kindExponentialHistogram:
		return "exponential_histogram"
	case kindSummary:
		return "summary"
	case kindMixed:
		return "mixed"
	default:
		return "unknown"
	}
}

// observeMetrics records the data-point attributes of md. It only reads md,
// which the fanout consumer may share between pipelines as read-only.
func (p *cardinalityProcessor) observeMetrics(md pmetric.Metrics) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		var service string
		if v, ok := rms.At(i).Resource().Attributes().Get("service.name"); ok {
			service = v.AsString()
		}
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				p.observeMetric(service, ms.At(k))
			}
		}
	}
}

func (p *cardinalityProcessor) observeMetric(service string, m pmetric.Metric) {
	name := m.Name()
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps := m.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			p.observeAttributes(name, kindGauge, service, dps.At(i).Attributes())
		}
	case pmetric.MetricTypeSum:
		// Unspecified temporality counts as cumulative, which gets the safer recommendation.
		kind := kindCumulativeSum
		if m.Sum().AggregationTemporality() == pmetric.AggregationTemporalityDelta {
			kind = kindDeltaSum
		}
		dps := m.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			p.observeAttributes(name, kind, service, dps.At(i).Attributes())
		}
	case pmetric.MetricTypeHistogram:
		dps := m.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			p.observeAttributes(name, kindHistogram, service, dps.At(i).Attributes())
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := m.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			p.observeAttributes(name, kindExponentialHistogram, service, dps.At(i).Attributes())
		}
	case pmetric.MetricTypeSummary:
		dps := m.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			p.observeAttributes(name, kindSummary, service, dps.At(i).Attributes())
		}
	}
}

// observeAttributes tracks every attribute, including never_drop_labels, so
// the report can include them.
func (p *cardinalityProcessor) observeAttributes(metricName string, kind metricKind, service string, attrs pcommon.Map) {
	attrs.Range(func(k string, v pcommon.Value) bool {
		if t := p.getOrCreateTracker(metricName, k); t != nil {
			t.insertObserved(hashAttrValue(v), kind, service)
		}
		return true
	})
}

// insertObserved never resets kind or multiService, so once a tracker sees a
// second type or service it keeps the safer advice until it is evicted.
func (t *tracker) insertObserved(hashVal uint64, kind metricKind, service string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.insertLocked(hashVal)
	if t.kind == kindUnknown {
		t.kind, t.service = kind, service
		return
	}
	if t.kind != kind {
		t.kind = kindMixed
	}
	if t.service != service {
		t.multiService = true
	}
}

// finding is a tracker that matched HIGH or GROWING in the epoch that ended.
type finding struct {
	metricName       string
	labelKey         string
	kind             metricKind
	distinct         uint64
	delta            uint64
	growingEvaluated bool
	high             bool
	growing          bool
	protected        bool
	multiService     bool
}

func saturatingSub(a, b uint64) uint64 {
	if a <= b {
		return 0
	}
	return a - b
}

// collectFindings evaluates one shard snapshot before tracker.rotate() resets
// it. Every finding counts in flagged; buf keeps the top ones.
func (p *cardinalityProcessor) collectFindings(entries []trackerEntry, buf []finding, flagged int) ([]finding, int) {
	for _, e := range entries {
		e.t.mu.Lock()
		// cachedCurr can lag up to estimateInterval-1 inserts; refresh it so the
		// report and the next epoch's cachedPrev are exact.
		if e.t.insertCount > 0 {
			e.t.cachedCurr = e.t.current.Estimate()
		}
		f := finding{
			metricName: e.key.metricName,
			labelKey:   e.key.attrKey,
			kind:       e.t.kind,
			distinct:   e.t.cachedCurr,
			delta:      saturatingSub(e.t.cachedCurr, e.t.cachedPrev),
			// GROWING needs a full, active previous epoch: a tracker's first epoch has
			// none, its second compares against a partial one, and after an idle epoch
			// cachedPrev is 0.
			growingEvaluated: e.t.rotations >= 2 && e.t.idleEpochs == 0,
			multiService:     e.t.multiService,
		}
		e.t.mu.Unlock()
		if !p.evaluate(&f) {
			continue
		}
		flagged++
		buf = keepTopFindings(buf, f, p.config.TopOffendersCount)
	}
	return buf, flagged
}

// evaluate sets the statuses of f and reports whether it is a finding.
func (p *cardinalityProcessor) evaluate(f *finding) bool {
	f.high = f.distinct >= uint64(p.config.HighCardinalityThreshold)
	f.growing = f.growingEvaluated && f.delta > p.getLimit(f.metricName)
	f.protected = p.isProtected(f.labelKey)
	return f.high || f.growing
}

// compareFindings orders by distinct values, highest first, then by name.
func compareFindings(a, b finding) int {
	return cmp.Or(
		cmp.Compare(b.distinct, a.distinct),
		strings.Compare(a.metricName, b.metricName),
		strings.Compare(a.labelKey, b.labelKey),
	)
}

// keepTopFindings adds f to buf, keeping the n highest-ranked findings.
func keepTopFindings(buf []finding, f finding, n int) []finding {
	if n <= 0 {
		return buf
	}
	if len(buf) < n {
		return append(buf, f)
	}
	last := 0
	for i := 1; i < len(buf); i++ {
		if compareFindings(buf[i], buf[last]) > 0 {
			last = i
		}
	}
	if compareFindings(f, buf[last]) < 0 {
		buf[last] = f
	}
	return buf
}

func (f finding) statuses() []string {
	s := make([]string, 0, 3)
	if f.growing {
		s = append(s, "GROWING")
	}
	if f.high {
		s = append(s, "HIGH")
	}
	if f.protected {
		s = append(s, "PROTECTED")
	}
	return s
}

// recommend returns the advice for f and, for the types strip_and_reaggregate
// itself merges (delta sums and gauges), the OTTL statements that apply it.
func recommend(f finding) (string, []string) {
	if f.protected {
		return fmt.Sprintf("Label %s is in never_drop_labels; reduce its values at the source (instrumentation).", f.labelKey), nil
	}
	if f.multiService {
		return fmt.Sprintf("Metric %s arrives from more than one service.name, and this count combines them. Check label %s per service before removing it.", f.metricName, f.labelKey), nil
	}
	source := fmt.Sprintf("Reduce the values of label %s on metric %s at the source (instrumentation).", f.labelKey, f.metricName)
	switch f.kind {
	case kindDeltaSum, kindGauge:
		if !ottlSafe(f.labelKey) || !ottlSafe(f.metricName) {
			return source, nil
		}
		if f.kind == kindDeltaSum {
			return fmt.Sprintf("Remove label %s from metric %s and re-aggregate with sum. Apply ottl_statements in a transformprocessor as two metric_statements groups, in order.", f.labelKey, f.metricName),
				ottlStatements(f, "sum")
		}
		return fmt.Sprintf("Remove label %s from metric %s and re-aggregate. Apply ottl_statements in a transformprocessor as two metric_statements groups, in order. Replace <fn> with max, min, mean or median.", f.labelKey, f.metricName),
			ottlStatements(f, "<fn>")
	default:
		return source + fmt.Sprintf(" Removing it in the collector merges %s series, which is unsafe when the merged series arrive in different batches or at different collectors.", f.kind), nil
	}
}

// ottlSafe reports whether %q quotes s without escapes, so the result is also a
// valid OTTL string literal.
func ottlSafe(s string) bool {
	return !strings.ContainsFunc(s, func(r rune) bool {
		return r == '"' || r == '\\' || !unicode.IsPrint(r)
	})
}

// ottlStatements are applied as two statement groups: the delete in datapoint
// context, then the aggregation in metric context.
func ottlStatements(f finding, fn string) []string {
	return []string{
		fmt.Sprintf(`delete_key(datapoint.attributes, %q) where metric.name == %q`, f.labelKey, f.metricName),
		fmt.Sprintf(`aggregate_on_attributes(%q) where metric.name == %q`, fn, f.metricName),
	}
}

// logReport logs a summary and one Warn line per finding: names, counts and
// statuses, never attribute values.
func (p *cardinalityProcessor) logReport(findings []finding, flagged int, limitReached bool) {
	if flagged == 0 {
		p.logger.Debug("Cardinality report: no findings")
		return
	}
	slices.SortFunc(findings, compareFindings)
	p.logger.Info("Cardinality report",
		zap.Int("epoch_duration_seconds", p.config.EpochDurationSeconds),
		zap.Int64("trackers_active", p.trackerCount.Load()),
		zap.Int("flagged", flagged),
		zap.Int("shown", len(findings)),
		zap.Bool("tracker_limit_reached", limitReached))
	for _, f := range findings {
		text, stmts := recommend(f)
		fields := []zap.Field{
			zap.String("metric", f.metricName),
			zap.String("label", f.labelKey),
			zap.String("metric_type", f.kind.String()),
			zap.String("status", strings.Join(f.statuses(), ",")),
			zap.Uint64("distinct_values", f.distinct),
		}
		if f.growingEvaluated {
			fields = append(fields, zap.Uint64("delta", f.delta))
		}
		fields = append(fields, zap.String("recommendation", text))
		if len(stmts) > 0 {
			fields = append(fields, zap.Strings("ottl_statements", stmts))
		}
		p.logger.Warn("High-cardinality recommendation", fields...)
	}
}
