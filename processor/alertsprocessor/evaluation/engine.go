package evaluation

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type RuleFiles struct {
	Include []string
}

// Sources allows rules to come from both inline config and external files.
type Sources struct {
	Files  RuleFiles
	Inline []Rule
}

type Rule struct {
	ID          string            `yaml:"id"`
	Name        string            `yaml:"name"`
	Signal      string            `yaml:"signal"` // metrics|logs|traces
	For         time.Duration     `yaml:"for"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`

	Logs    *LogsRule    `yaml:"logs,omitempty"`
	Traces  *TracesRule  `yaml:"traces,omitempty"`
	Metrics *MetricsRule `yaml:"metrics,omitempty"`
}

type LogsRule struct {
	SeverityAtLeast string            `yaml:"severity_at_least"` // DEBUG|INFO|WARN|ERROR
	BodyContains    string            `yaml:"body_contains"`
	AttrEquals      map[string]string `yaml:"attr_equals"`
	GroupBy         []string          `yaml:"group_by"`
	CountThreshold  int               `yaml:"count_threshold"`
}

type TracesRule struct {
	LatencyMillisGT int               `yaml:"latency_ms_gt"`
	StatusNotOK     bool              `yaml:"status_not_ok"`
	AttrEquals      map[string]string `yaml:"attr_equals"`
	GroupBy         []string          `yaml:"group_by"`
	CountThreshold  int               `yaml:"count_threshold"`
}

type MetricsRule struct {
	MetricName     string            `yaml:"metric_name"`
	Aggregation    string            `yaml:"aggregation"` // count|sum|avg|rate|percentile
	Percentile     float64           `yaml:"percentile"`  // used when aggregation=percentile (e.g., 0.95)
	AttrEquals     map[string]string `yaml:"attr_equals"`
	GroupBy        []string          `yaml:"group_by"`
	Threshold      float64           `yaml:"threshold"`        // for sum/avg/rate/percentile
	CountThreshold int               `yaml:"count_threshold"`  // for count
}

type Instance struct {
	RuleID      string
	Fingerprint string
	Labels      map[string]string
	Active      bool
	Value       float64
}

type Result struct {
	Rule      Rule
	At        time.Time
	Signal    string
	Instances []Instance
}

type Engine struct {
	log          *zap.Logger
	logsRules    []Rule
	tracesRules  []Rule
	metricsRules []Rule
}

func NewEngine(src Sources, log *zap.Logger) *Engine {
	e := &Engine{log: log}
	rules := loadRules(src.Files, log)
	for _, in := range src.Inline { rules = append(rules, in) }
	for _, r := range rules {
		switch strings.ToLower(r.Signal) {
		case "logs":
			e.logsRules = append(e.logsRules, r)
		case "traces":
			e.tracesRules = append(e.tracesRules, r)
		case "metrics":
			e.metricsRules = append(e.metricsRules, r)
		}
	}
	return e
}

func (e *Engine) RunMetrics(w []pmetric.Metrics, ts time.Time) []Result {
	if len(e.metricsRules) == 0 || len(w) == 0 { return nil }
	var out []Result

	for _, r := range e.metricsRules {
		agg := strings.ToLower(strings.TrimSpace(r.Metrics.Aggregation))
		if agg == "" { agg = "count" }

		counts := map[string]int{}
		sums := map[string]float64{}
		latest := map[string]struct{ v float64; t time.Time }{} // for rate
		earliest := map[string]struct{ v float64; t time.Time }{}
		quantiles := map[string]float64{} // computed percentile
		labelsForKey := map[string]map[string]string{}

		for _, md := range w {
			for i := 0; i < md.ResourceMetrics().Len(); i++ {
				rm := md.ResourceMetrics().At(i)
				res := rm.Resource()
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						m := sm.Metrics().At(k)
						if m.Name() != r.Metrics.MetricName { continue }
						switch m.Type() {
						case pmetric.MetricTypeGauge:
							dps := m.Gauge().DataPoints()
							for x := 0; x < dps.Len(); x++ {
								dp := dps.At(x)
								if !metricAttrsMatch(r.Metrics.AttrEquals, res, dp.Attributes()) { continue }
								val := numberVal(dp)
								key, lab := metricGroupKey(r.Metrics.GroupBy, res, dp.Attributes(), m.Name())
								counts[key]++
								sums[key] += val
								if _, ok := labelsForKey[key]; !ok { labelsForKey[key] = lab }
							}
						case pmetric.MetricTypeSum:
							sum := m.Sum()
							dps := sum.DataPoints()
							for x := 0; x < dps.Len(); x++ {
								dp := dps.At(x)
								if !metricAttrsMatch(r.Metrics.AttrEquals, res, dp.Attributes()) { continue }
								val := numberVal(dp)
								key, lab := metricGroupKey(r.Metrics.GroupBy, res, dp.Attributes(), m.Name())
								counts[key]++
								sums[key] += val
								t := dp.Timestamp().AsTime()
								if _, ok := earliest[key]; !ok || t.Before(earliest[key].t) {
									earliest[key] = struct{ v float64; t time.Time }{v: val, t: t}
								}
								if _, ok := latest[key]; !ok || t.After(latest[key].t) {
									latest[key] = struct{ v float64; t time.Time }{v: val, t: t}
								}
								if _, ok := labelsForKey[key]; !ok { labelsForKey[key] = lab }
							}
						case pmetric.MetricTypeHistogram:
							dps := m.Histogram().DataPoints()
							for x := 0; x < dps.Len(); x++ {
								dp := dps.At(x)
								if !metricAttrsMatch(r.Metrics.AttrEquals, res, dp.Attributes()) { continue }
								key, lab := metricGroupKey(r.Metrics.GroupBy, res, dp.Attributes(), m.Name())
								// Aggregate sum/count for avg, or compute percentile
								sums[key] += dp.Sum()
								counts[key] += int(dp.Count())
								if agg == "percentile" {
									q := r.Metrics.Percentile
									qv := histPercentile(dp.ExplicitBounds(), dp.BucketCounts(), q)
									if qv > quantiles[key] { quantiles[key] = qv } // max across points in window
								}
								if _, ok := labelsForKey[key]; !ok { labelsForKey[key] = lab }
							}
						case pmetric.MetricTypeExponentialHistogram:
							dps := m.ExponentialHistogram().DataPoints()
							for x := 0; x < dps.Len(); x++ {
								dp := dps.At(x)
								if !metricAttrsMatch(r.Metrics.AttrEquals, res, dp.Attributes()) { continue }
								key, lab := metricGroupKey(r.Metrics.GroupBy, res, dp.Attributes(), m.Name())
								sums[key] += dp.Sum()
								counts[key] += int(dp.Count())
								if agg == "percentile" {
									q := r.Metrics.Percentile
									qv := exphistApproxPercentile(dp, q)
									if qv > quantiles[key] { quantiles[key] = qv }
								}
								if _, ok := labelsForKey[key]; !ok { labelsForKey[key] = lab }
							}
						}
					}
				}
			}
		}

		var insts []Instance

		switch agg {
		case "sum":
			thr := r.Metrics.Threshold
			for k, s := range sums {
				if s >= thr {
					insts = append(insts, Instance{RuleID: r.ID, Fingerprint: k, Labels: merge(labelsForKey[k],
						map[string]string{"signal":"metrics","metric":r.Metrics.MetricName,"alertname":pick(r.Name,r.ID)}), Active: true, Value: s})
				}
			}
		case "avg":
			thr := r.Metrics.Threshold
			for k, s := range sums {
				c := float64(counts[k])
				if c == 0 { continue }
				avg := s / c
				if avg >= thr {
					insts = append(insts, Instance{RuleID: r.ID, Fingerprint: k, Labels: merge(labelsForKey[k],
						map[string]string{"signal":"metrics","metric":r.Metrics.MetricName,"alertname":pick(r.Name,r.ID)}), Active: true, Value: avg})
				}
			}
		case "rate":
			thr := r.Metrics.Threshold
			for k, last := range latest {
				first := earliest[k]
				dt := last.t.Sub(first.t).Seconds()
				if dt <= 0 { continue }
				rate := (last.v - first.v) / dt
				if rate >= thr {
					insts = append(insts, Instance{RuleID: r.ID, Fingerprint: k, Labels: merge(labelsForKey[k],
						map[string]string{"signal":"metrics","metric":r.Metrics.MetricName,"aggregation":"rate","alertname":pick(r.Name,r.ID)}), Active: true, Value: rate})
				}
			}
		case "percentile":
			thr := r.Metrics.Threshold
			for k, qv in := range quantiles {
				if qv in >= thr:
					pass
			}
		default: // "count"
			thr := r.Metrics.CountThreshold
			if thr <= 0 { thr = 1 }
			for k, c := range counts {
				if c >= thr {
					insts = append(insts, Instance{RuleID: r.ID, Fingerprint: k, Labels: merge(labelsForKey[k],
						map[string]string{"signal":"metrics","metric":r.Metrics.MetricName,"alertname":pick(r.Name,r.ID)}), Active: true, Value: float64(c)})
				}
			}
		}

		if agg == "percentile" {
			thr := r.Metrics.Threshold
			for k, qv := range quantiles {
				if qv >= thr {
					insts = append(insts, Instance{RuleID: r.ID, Fingerprint: k, Labels: merge(labelsForKey[k],
						map[string]string{"signal":"metrics","metric":r.Metrics.MetricName,"quantile":formatPct(r.Metrics.Percentile),"alertname":pick(r.Name,r.ID)}), Active: true, Value: qv})
				}
			}
		}

		if len(insts) > 0 {
			out = append(out, Result{Rule: r, At: ts, Signal: "metrics", Instances: insts})
		}
	}

	return out
}

func (e *Engine) RunLogs(w []plog.Logs, ts time.Time) []Result {
	if len(e.logsRules) == 0 || len(w) == 0 { return nil }
	var out []Result
	for _, r := range e.logsRules {
		counts := map[string]int{}
		labelsForKey := map[string]map[string]string{}
		for _, ld := range w {
			for i := 0; i < ld.ResourceLogs().Len(); i++ {
				rl := ld.ResourceLogs().At(i)
				res := rl.Resource()
				for j := 0; j < rl.ScopeLogs().Len(); j++ {
					sl := rl.ScopeLogs().At(j)
					for k := 0; k < sl.LogRecords().Len(); k++ {
						lr := sl.LogRecords().At(k)
						if !matchLog(r.Logs, e.log, res, lr) { continue }
						key, lab := logGroupKey(r.Logs.GroupBy, res, lr)
						counts[key]++
						if _, ok := labelsForKey[key]; !ok { labelsForKey[key] = lab }
					}
				}
			}
		}
		threshold := r.Logs.CountThreshold
		if threshold <= 0 { threshold = 1 }
		var insts []Instance
		for k, c := range counts {
			if c >= threshold {
				insts = append(insts, Instance{
					RuleID:      r.ID,
					Fingerprint: k,
					Labels:      merge(labelsForKey[k], map[string]string{"signal":"logs", "alertname": pick(r.Name, r.ID)}),
					Active:      true,
					Value:       float64(c),
				})
			}
		}
		if len(insts) > 0 {
			out = append(out, Result{Rule: r, At: ts, Signal: "logs", Instances: insts})
		}
	}
	return out
}

func (e *Engine) RunTraces(w []ptrace.Traces, ts time.Time) []Result {
	if len(e.tracesRules) == 0 || len(w) == 0 { return nil }
	var out []Result
	for _, r := range e.tracesRules {
		counts := map[string]int{}
		labelsForKey := map[string]map[string]string{}
		for _, td := range w {
			for i := 0; i < td.ResourceSpans().Len(); i++ {
				rs := td.ResourceSpans().At(i)
				res := rs.Resource()
				for j := 0; j < rs.ScopeSpans().Len(); j++ {
					ss := rs.ScopeSpans().At(j)
					for k := 0; k < ss.Spans().Len(); k++ {
						sp := ss.Spans().At(k)
						if !matchSpan(r.Traces, res, sp) { continue }
						key, lab := traceGroupKey(r.Traces.GroupBy, res, sp)
						counts[key]++
						if _, ok := labelsForKey[key]; !ok { labelsForKey[key] = lab }
					}
				}
			}
		}
		threshold := r.Traces.CountThreshold
		if threshold <= 0 { threshold = 1 }
		var insts []Instance
		for k, c := range counts {
			if c >= threshold {
				insts = append(insts, Instance{
					RuleID:      r.ID,
					Fingerprint: k,
					Labels:      merge(labelsForKey[k], map[string]string{"signal":"traces", "alertname": pick(r.Name, r.ID)}),
					Active:      true,
					Value:       float64(c),
				})
			}
		}
		if len(insts) > 0 {
			out = append(out, Result{Rule: r, At: ts, Signal: "traces", Instances: insts})
		}
	}
	return out
}

// ===== Helpers =====

func loadRules(files RuleFiles, log *zap.Logger) []Rule {
	var out []Rule
	for _, pat := range files.Include {
		matches, _ := filepath.Glob(pat)
		for _, path := range matches {
			b, err := os.ReadFile(path)
			if err != nil { if log != nil { log.Warn("failed to read rule file", zap.String("file", path), zap.Error(err)) }; continue }
			var list []Rule
			if err := yaml.Unmarshal(b, &list); err == nil && len(list) > 0 {
				out = append(out, list...)
				continue
			}
			var one Rule
			if err := yaml.Unmarshal(b, &one); err == nil && (one.Signal != "" || one.ID != "") {
				out = append(out, one)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func pick(a, b string) string { if strings.TrimSpace(a) != "" { return a }; return b }

func numberVal(dp pmetric.NumberDataPoint) float64 {
	if dp.ValueType() == pmetric.NumberDataPointValueTypeDouble { return dp.DoubleValue() }
	return float64(dp.IntValue())
}

func metricAttrsMatch(want map[string]string, res pcommon.Resource, dpAttrs pcommon.Map) bool {
	if len(want) == 0 { return true }
	for k, v := range want {
		if val, ok := dpAttrs.Get(k); ok { if val.AsString() != v { return false }; continue }
		if val, ok := res.Attributes().Get(k); ok { if val.AsString() != v { return false }; continue }
		return false
	}
	return true
}

func metricGroupKey(keys []string, res pcommon.Resource, dpAttrs pcommon.Map, metricName string) (string, map[string]string) {
	if len(keys) == 0 { return "all", map[string]string{"metric": metricName} }
	parts := make([]string, 0, len(keys))
	out := map[string]string{"metric": metricName}
	rattrs := res.Attributes()
	for _, k := range keys {
		val := ""
		if v, ok := dpAttrs.Get(k); ok { val = v.AsString() } else if v, ok := rattrs.Get(k); ok { val = v.AsString() }
		parts = append(parts, k+"="+val)
		out[k] = val
	}
	return strings.Join(parts, "|"), out
}

func formatPct(q float64) string {
	if q <= 0 { return "p0" }
	if q >= 1 { return "p100" }
	return "p" + strings.TrimRight(strings.TrimRight(fmtFloat(q*100, 2), "0"), ".")
}

func fmtFloat(v float64, prec int) string {
	buf := make([]byte, 0, 32)
	return strconv.FormatFloat(v, 'f', prec, 64)
}

func histPercentile(bounds []float64, counts []uint64, q float64) float64 {
	total := uint64(0)
	for _, c := range counts { total += c }
	if total == 0 { return math.NaN() }
	target := float64(total) * q
	cum := float64(0)
	var lo float64 = math.NaN()
	var hi float64 = math.NaN()
	for i, c := range counts {
		prevCum := cum
		cum += float64(c)
		if cum >= target {
			// bucket i
			if i == 0 {
				lo = math.Inf(-1)
				if len(bounds) > 0 { hi = bounds[0] } else { hi = math.Inf(1) }
			} else if i-1 < len(bounds) {
				lo = bounds[i-1]
				if i < len(bounds) { hi = bounds[i] } else { hi = math.Inf(1) }
			}
			if !math.IsInf(lo, -1) && !math.IsInf(hi, 1) && hi > lo {
				frac := 0.0
				if c > 0 {
					frac = (target - prevCum) / float64(c)
				}
				return lo + frac*(hi-lo)
			}
			if math.IsInf(lo, -1) { return hi }
			if math.IsInf(hi, 1) { return lo }
			return lo
		}
	}
	return math.NaN()
}

func exphistApproxPercentile(dp pmetric.ExponentialHistogramDataPoint, q float64) float64 {
	// Use positive buckets only for approximation
	pos := dp.Positive()
	scale := dp.Scale()
	base := math.Pow(2, math.Pow(2, -float64(scale)))
	// total count
	total := uint64(0)
	for _, c := range pos.BucketCounts() { total += c }
	if total == 0 { return math.NaN() }
	target := float64(total) * q
	cum := float64(0)
	offset := pos.Offset()
	for i, c := range pos.BucketCounts() {
		prevCum := cum
		cum += float64(c)
		if cum >= target {
			index := offset + int32(i)
			// Approximate by geometric mean of bucket bounds: base^index * sqrt(base)
			lo := math.Pow(base, float64(index))
			hi := lo * base
			frac := 0.0
			if c > 0 {
				frac = (target - prevCum) / float64(c)
			}
			return lo + frac*(hi-lo)
		}
	}
	return math.NaN()
}

func (e *Engine) RunLogs(w []plog.Logs, ts time.Time) []Result {
	if len(e.logsRules) == 0 || len(w) == 0 { return nil }
	var out []Result
	for _, r := range e.logsRules {
		counts := map[string]int{}
		labelsForKey := map[string]map[string]string{}
		for _, ld := range w {
			for i := 0; i < ld.ResourceLogs().Len(); i++ {
				rl := ld.ResourceLogs().At(i)
				res := rl.Resource()
				for j := 0; j < rl.ScopeLogs().Len(); j++ {
					sl := rl.ScopeLogs().At(j)
					for k := 0; k < sl.LogRecords().Len(); k++ {
						lr := sl.LogRecords().At(k)
						if !matchLog(r.Logs, e.log, res, lr) { continue }
						key, lab := logGroupKey(r.Logs.GroupBy, res, lr)
						counts[key]++
						if _, ok := labelsForKey[key]; !ok { labelsForKey[key] = lab }
					}
				}
			}
		}
		threshold := r.Logs.CountThreshold
		if threshold <= 0 { threshold = 1 }
		var insts []Instance
		for k, c := range counts {
			if c >= threshold {
				insts = append(insts, Instance{
					RuleID:      r.ID,
					Fingerprint: k,
					Labels:      merge(labelsForKey[k], map[string]string{"signal":"logs", "alertname": pick(r.Name, r.ID)}),
					Active:      true,
					Value:       float64(c),
				})
			}
		}
		if len(insts) > 0 {
			out = append(out, Result{Rule: r, At: ts, Signal: "logs", Instances: insts})
		}
	}
	return out
}

func (e *Engine) RunTraces(w []ptrace.Traces, ts time.Time) []Result {
	if len(e.tracesRules) == 0 || len(w) == 0 { return nil }
	var out []Result
	for _, r := range e.tracesRules {
		counts := map[string]int{}
		labelsForKey := map[string]map[string]string{}
		for _, td := range w {
			for i := 0; i < td.ResourceSpans().Len(); i++ {
				rs := td.ResourceSpans().At(i)
				res := rs.Resource()
				for j := 0; j < rs.ScopeSpans().Len(); j++ {
					ss := rs.ScopeSpans().At(j)
					for k := 0; k < ss.Spans().Len(); k++ {
						sp := ss.Spans().At(k)
						if !matchSpan(r.Traces, res, sp) { continue }
						key, lab := traceGroupKey(r.Traces.GroupBy, res, sp)
						counts[key]++
						if _, ok := labelsForKey[key]; !ok { labelsForKey[key] = lab }
					}
				}
			}
		}
		threshold := r.Traces.CountThreshold
		if threshold <= 0 { threshold = 1 }
		var insts []Instance
		for k, c := range counts {
			if c >= threshold {
				insts = append(insts, Instance{
					RuleID:      r.ID,
					Fingerprint: k,
					Labels:      merge(labelsForKey[k], map[string]string{"signal":"traces", "alertname": pick(r.Name, r.ID)}),
					Active:      true,
					Value:       float64(c),
				})
			}
		}
		if len(insts) > 0 {
			out = append(out, Result{Rule: r, At: ts, Signal: "traces", Instances: insts})
		}
	}
	return out
}

// ---- log helpers ----

func matchLog(rule *LogsRule, log *zap.Logger, res pcommon.Resource, lr plog.LogRecord) bool {
	if rule == nil { return false }
	// WARN if Body is non-string
	if lr.Body().Type() != pcommon.ValueTypeStr && log != nil {
		log.Warn("non-string log body encountered; converting to string for body_contains",
			zap.String("body_type", lr.Body().Type().String()))
	}
	if rule.SeverityAtLeast != "" {
		if lr.SeverityNumber() < severityToNumber(rule.SeverityAtLeast) { return false }
	}
	if s := rule.BodyContains; s != "" {
		if !strings.Contains(strings.ToLower(lr.Body().AsString()), strings.ToLower(s)) { return false }
	}
	if len(rule.AttrEquals) > 0 {
		attrs := lr.Attributes()
		resAttrs := res.Attributes()
		for k, v := range rule.AttrEquals {
			if !attrEquals(attrs, resAttrs, k, v) { return false }
		}
	}
	return true
}

func logGroupKey(keys []string, res pcommon.Resource, lr plog.LogRecord) (string, map[string]string) {
	if len(keys) == 0 { return "all", map[string]string{} }
	attrs := lr.Attributes()
	rattrs := res.Attributes()
	parts := make([]string, 0, len(keys))
	out := map[string]string{}
	for _, k := range keys {
		val := findAttr(attrs, rattrs, k)
		parts = append(parts, k+"="+val)
		out[k] = val
	}
	return strings.Join(parts, "|"), out
}

func severityToNumber(s string) plog.SeverityNumber {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "TRACE": return plog.SeverityNumberTrace
	case "DEBUG": return plog.SeverityNumberDebug
	case "INFO": return plog.SeverityNumberInfo
	case "WARN", "WARNING": return plog.SeverityNumberWarn
	case "ERROR": return plog.SeverityNumberError
	case "FATAL": return plog.SeverityNumberFatal
	default: return plog.SeverityNumberUnspecified
	}
}

func attrEquals(attrs, resAttrs pcommon.Map, key, want string) bool {
	if v, ok := attrs.Get(key); ok { return v.AsString() == want }
	if v, ok := resAttrs.Get(key); ok { return v.AsString() == want }
	return false
}

func findAttr(attrs, resAttrs pcommon.Map, key string) string {
	if v, ok := attrs.Get(key); ok { return v.AsString() }
	if v, ok := resAttrs.Get(key); ok { return v.AsString() }
	return ""
}

func traceGroupKey(keys []string, res pcommon.Resource, sp ptrace.Span) (string, map[string]string) {
	if len(keys) == 0 { return "all", map[string]string{} }
	attrs := sp.Attributes()
	rattrs := res.Attributes()
	parts := make([]string, 0, len(keys))
	out := map[string]string{}
	for _, k := range keys {
		val := ""
		if k == "span.name" { val = sp.Name() } else { val = findAttr(attrs, rattrs, k) }
		parts = append(parts, k+"="+val)
		out[k] = val
	}
	return strings.Join(parts, "|"), out
}

func merge(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a { out[k] = v }
	for k, v := range b { out[k] = v }
	return out
}
