// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation // import "github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/evaluation"

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

/* =========================
   Rules & Results
   ========================= */

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

// MetricsRule aggregates over the sliding window.
// Aggregation: count|sum|avg|rate|percentile
// - For percentile, set Percentile (e.g., 0.95) and Threshold.
// - For sum/avg/rate, set Threshold.
// - For count, set CountThreshold.
type MetricsRule struct {
	MetricName     string            `yaml:"metric_name"`
	Aggregation    string            `yaml:"aggregation"` // count|sum|avg|rate|percentile
	Percentile     float64           `yaml:"percentile,omitempty"`
	Threshold      float64           `yaml:"threshold,omitempty"`
	AttrEquals     map[string]string `yaml:"attr_equals"`
	GroupBy        []string          `yaml:"group_by"`
	CountThreshold int               `yaml:"count_threshold"`
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

/* =========================
   Engine
   ========================= */

type Engine struct {
	log          *zap.Logger
	logsRules    []Rule
	tracesRules  []Rule
	metricsRules []Rule
}

func NewEngine(src Sources, log *zap.Logger) *Engine {
	e := &Engine{log: log}
	rules := loadRules(src.Files, log)
	for _, in := range src.Inline {
		rules = append(rules, in)
	}
	for _, r := range rules {
		switch strings.ToLower(strings.TrimSpace(r.Signal)) {
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

/* =========================
   Metrics evaluation
   ========================= */

func (e *Engine) RunMetrics(window []pmetric.Metrics, ts time.Time) []Result {
	if len(e.metricsRules) == 0 || len(window) == 0 {
		return nil
	}
	var out []Result

	for _, r := range e.metricsRules {
		aggKind := strings.ToLower(strings.TrimSpace(r.Metrics.Aggregation))
		if aggKind == "" {
			aggKind = "count"
		}

		// Accumulators per group key
		type acc struct {
			count   int
			sum     float64
			firstV  float64
			firstTS time.Time
			lastV   float64
			lastTS  time.Time
			labels  map[string]string
		}
		accs := map[string]*acc{}

		// For percentile aggregations we collect per‑dp quantiles and
		// later average them per group (simple, stable across changing bounds).
		qvals := map[string][]float64{} // key -> list of quantile values

		for _, md := range window {
			for i := 0; i < md.ResourceMetrics().Len(); i++ {
				rm := md.ResourceMetrics().At(i)
				res := rm.Resource()
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						m := sm.Metrics().At(k)
						if m.Name() != r.Metrics.MetricName {
							continue
						}
						switch m.Type() {
						case pmetric.MetricTypeGauge:
							dps := m.Gauge().DataPoints()
							for x := 0; x < dps.Len(); x++ {
								dp := dps.At(x)
								if !metricAttrsMatch(r.Metrics.AttrEquals, res, dp.Attributes()) {
									continue
								}
								key, lab := metricGroupKey(r.Metrics.GroupBy, res, dp.Attributes(), m.Name())
								a := accs[key]
								if a == nil {
									a = &acc{labels: lab, firstV: dp.DoubleValue(), firstTS: dp.Timestamp().AsTime()}
									accs[key] = a
								}
								a.count++
								a.sum += dp.DoubleValue()
								a.lastV = dp.DoubleValue()
								a.lastTS = dp.Timestamp().AsTime()
							}

						case pmetric.MetricTypeSum:
							sum := m.Sum()
							dps := sum.DataPoints()
							for x := 0; x < dps.Len(); x++ {
								dp := dps.At(x)
								if !metricAttrsMatch(r.Metrics.AttrEquals, res, dp.Attributes()) {
									continue
								}
								key, lab := metricGroupKey(r.Metrics.GroupBy, res, dp.Attributes(), m.Name())
								a := accs[key]
								v := dp.DoubleValue()
								if a == nil {
									a = &acc{labels: lab, firstV: v, firstTS: dp.Timestamp().AsTime()}
									accs[key] = a
								}
								a.count++
								a.sum += v
								a.lastV = v
								a.lastTS = dp.Timestamp().AsTime()
							}

						case pmetric.MetricTypeHistogram:
							dps := m.Histogram().DataPoints()
							for x := 0; x < dps.Len(); x++ {
								dp := dps.At(x)
								if !metricAttrsMatch(r.Metrics.AttrEquals, res, dp.Attributes()) {
									continue
								}
								key, lab := metricGroupKey(r.Metrics.GroupBy, res, dp.Attributes(), m.Name())
								switch aggKind {
								case "percentile":
									q := r.Metrics.Percentile
									if q <= 0 || q >= 1 {
										continue
									}
									val := explicitHistPercentile(dp, q)
									qvals[key] = append(qvals[key], val)
									if _, ok := accs[key]; !ok {
										accs[key] = &acc{labels: lab}
									}
								case "sum", "avg":
									a := accs[key]
									if a == nil {
										a = &acc{labels: lab}
										accs[key] = a
									}
									a.sum += dp.Sum()
									a.count += int(dp.Count())
								default:
									// count is ambiguous for hist; ignore
								}
							}

						case pmetric.MetricTypeExponentialHistogram:
							dps := m.ExponentialHistogram().DataPoints()
							for x := 0; x < dps.Len(); x++ {
								dp := dps.At(x)
								if !metricAttrsMatch(r.Metrics.AttrEquals, res, dp.Attributes()) {
									continue
								}
								key, lab := metricGroupKey(r.Metrics.GroupBy, res, dp.Attributes(), m.Name())
								switch aggKind {
								case "percentile":
									q := r.Metrics.Percentile
									if q <= 0 || q >= 1 {
										continue
									}
									val := expHistPercentile(dp, q)
									qvals[key] = append(qvals[key], val)
									if _, ok := accs[key]; !ok {
										accs[key] = &acc{labels: lab}
									}
								case "sum", "avg":
									a := accs[key]
									if a == nil {
										a = &acc{labels: lab}
										accs[key] = a
									}
									a.sum += dp.Sum()
									a.count += int(dp.Count())
								default:
									// ignore
								}
							}
						}
					}
				}
			}
		}

		// Build instances per aggregation kind.
		var insts []Instance
		switch aggKind {
		case "count":
			thr := r.Metrics.CountThreshold
			if thr <= 0 {
				thr = 1
			}
			for k, a := range accs {
				if a.count >= thr {
					insts = append(insts, Instance{
						RuleID:      r.ID,
						Fingerprint: k,
						Labels:      merge(a.labels, map[string]string{"signal": "metrics", "metric": r.Metrics.MetricName, "alertname": pick(r.Name, r.ID)}),
						Active:      true,
						Value:       float64(a.count),
					})
				}
			}

		case "sum":
			thr := r.Metrics.Threshold
			for k, a := range accs {
				if a.sum >= thr {
					insts = append(insts, Instance{
						RuleID:      r.ID,
						Fingerprint: k,
						Labels:      merge(a.labels, map[string]string{"signal": "metrics", "metric": r.Metrics.MetricName, "alertname": pick(r.Name, r.ID)}),
						Active:      true,
						Value:       a.sum,
					})
				}
			}

		case "avg":
			thr := r.Metrics.Threshold
			for k, a := range accs {
				if a.count == 0 {
					continue
				}
				avg := a.sum / float64(a.count)
				if avg >= thr {
					insts = append(insts, Instance{
						RuleID:      r.ID,
						Fingerprint: k,
						Labels:      merge(a.labels, map[string]string{"signal": "metrics", "metric": r.Metrics.MetricName, "alertname": pick(r.Name, r.ID)}),
						Active:      true,
						Value:       avg,
					})
				}
			}

		case "rate":
			thr := r.Metrics.Threshold
			for k, a := range accs {
				// Basic rate for cumulative monotonic sums:
				// (last - first) / seconds
				dur := a.lastTS.Sub(a.firstTS).Seconds()
				if dur <= 0 {
					continue
				}
				rate := (a.lastV - a.firstV) / dur
				if rate >= thr {
					insts = append(insts, Instance{
						RuleID:      r.ID,
						Fingerprint: k,
						Labels:      merge(a.labels, map[string]string{"signal": "metrics", "metric": r.Metrics.MetricName, "alertname": pick(r.Name, r.ID)}),
						Active:      true,
						Value:       rate,
					})
				}
			}

		case "percentile":
			thr := r.Metrics.Threshold
			for k, list := range qvals {
				if len(list) == 0 {
					continue
				}
				// Use simple average of per‑dp quantiles within window.
				sum := 0.0
				for _, v := range list {
					sum += v
				}
				avg := sum / float64(len(list))
				a := accs[k]
				if avg >= thr {
					insts = append(insts, Instance{
						RuleID:      r.ID,
						Fingerprint: k,
						Labels:      merge(a.labels, map[string]string{"signal": "metrics", "metric": r.Metrics.MetricName, "alertname": pick(r.Name, r.ID)}),
						Active:      true,
						Value:       avg,
					})
				}
			}
		}

		if len(insts) > 0 {
			out = append(out, Result{Rule: r, At: ts, Signal: "metrics", Instances: insts})
		}
	}

	return out
}

/* =========================
   Logs evaluation
   ========================= */

func (e *Engine) RunLogs(w []plog.Logs, ts time.Time) []Result {
	if len(e.logsRules) == 0 || len(w) == 0 {
		return nil
	}
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
						if !matchLog(r.Logs, e.log, res, lr) {
							continue
						}
						key, lab := logGroupKey(r.Logs.GroupBy, res, lr)
						counts[key]++
						if _, ok := labelsForKey[key]; !ok {
							labelsForKey[key] = lab
						}
					}
				}
			}
		}
		threshold := r.Logs.CountThreshold
		if threshold <= 0 {
			threshold = 1
		}
		var insts []Instance
		for k, c := range counts {
			if c >= threshold {
				insts = append(insts, Instance{
					RuleID:      r.ID,
					Fingerprint: k,
					Labels:      merge(labelsForKey[k], map[string]string{"signal": "logs", "alertname": pick(r.Name, r.ID)}),
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

/* =========================
   Traces evaluation
   ========================= */

func (e *Engine) RunTraces(w []ptrace.Traces, ts time.Time) []Result {
	if len(e.tracesRules) == 0 || len(w) == 0 {
		return nil
	}
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
						if !matchSpan(r.Traces, res, sp) {
							continue
						}
						key, lab := traceGroupKey(r.Traces.GroupBy, res, sp)
						counts[key]++
						if _, ok := labelsForKey[key]; !ok {
							labelsForKey[key] = lab
						}
					}
				}
			}
		}
		threshold := r.Traces.CountThreshold
		if threshold <= 0 {
			threshold = 1
		}
		var insts []Instance
		for k, c := range counts {
			if c >= threshold {
				insts = append(insts, Instance{
					RuleID:      r.ID,
					Fingerprint: k,
					Labels:      merge(labelsForKey[k], map[string]string{"signal": "traces", "alertname": pick(r.Name, r.ID)}),
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

/* =========================
   Helpers
   ========================= */

func loadRules(files RuleFiles, log *zap.Logger) []Rule {
	var out []Rule
	for _, pat := range files.Include {
		matches, _ := filepath.Glob(pat)
		for _, path := range matches {
			b, err := os.ReadFile(path)
			if err != nil {
				if log != nil {
					log.Warn("failed to read rule file", zap.String("file", path), zap.Error(err))
				}
				continue
			}
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

func pick(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func matchLog(rule *LogsRule, log *zap.Logger, res pcommon.Resource, lr plog.LogRecord) bool {
	if rule == nil {
		return false
	}
	// WARN if Body is non-string (requested behavior)
	if lr.Body().Type() != pcommon.ValueTypeStr && log != nil {
		log.Warn("non-string log body encountered; converting to string for body_contains",
			zap.String("body_type", lr.Body().Type().String()))
	}
	if rule.SeverityAtLeast != "" {
		if lr.SeverityNumber() < severityToNumber(rule.SeverityAtLeast) {
			return false
		}
	}
	if s := rule.BodyContains; s != "" {
		if !strings.Contains(strings.ToLower(lr.Body().AsString()), strings.ToLower(s)) {
			return false
		}
	}
	if len(rule.AttrEquals) > 0 {
		attrs := lr.Attributes()
		resAttrs := res.Attributes()
		for k, v := range rule.AttrEquals {
			if !attrEquals(attrs, resAttrs, k, v) {
				return false
			}
		}
	}
	return true
}

func logGroupKey(keys []string, res pcommon.Resource, lr plog.LogRecord) (string, map[string]string) {
	if len(keys) == 0 {
		return "all", map[string]string{}
	}
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

func matchSpan(rule *TracesRule, res pcommon.Resource, sp ptrace.Span) bool {
	if rule == nil {
		return false
	}
	if rule.LatencyMillisGT > 0 {
		dur := int64(sp.EndTimestamp()-sp.StartTimestamp()) / 1_000_000
		if dur <= int64(rule.LatencyMillisGT) {
			return false
		}
	}
	if rule.StatusNotOK {
		if sp.Status().Code() == ptrace.StatusCodeOk {
			return false
		}
	}
	if len(rule.AttrEquals) > 0 {
		attrs := sp.Attributes()
		resAttrs := res.Attributes()
		for k, v := range rule.AttrEquals {
			if !attrEquals(attrs, resAttrs, k, v) {
				return false
			}
		}
	}
	return true
}

func traceGroupKey(keys []string, res pcommon.Resource, sp ptrace.Span) (string, map[string]string) {
	if len(keys) == 0 {
		return "all", map[string]string{}
	}
	attrs := sp.Attributes()
	rattrs := res.Attributes()
	parts := make([]string, 0, len(keys))
	out := map[string]string{}
	for _, k := range keys {
		val := ""
		if k == "span.name" {
			val = sp.Name()
		} else {
			val = findAttr(attrs, rattrs, k)
		}
		parts = append(parts, k+"="+val)
		out[k] = val
	}
	return strings.Join(parts, "|"), out
}

func severityToNumber(s string) plog.SeverityNumber {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "TRACE":
		return plog.SeverityNumberTrace
	case "DEBUG":
		return plog.SeverityNumberDebug
	case "INFO":
		return plog.SeverityNumberInfo
	case "WARN", "WARNING":
		return plog.SeverityNumberWarn
	case "ERROR":
		return plog.SeverityNumberError
	case "FATAL":
		return plog.SeverityNumberFatal
	default:
		return plog.SeverityNumberUnspecified
	}
}

func attrEquals(attrs, resAttrs pcommon.Map, key, want string) bool {
	if v, ok := attrs.Get(key); ok {
		return v.AsString() == want
	}
	if v, ok := resAttrs.Get(key); ok {
		return v.AsString() == want
	}
	return false
}

func findAttr(attrs, resAttrs pcommon.Map, key string) string {
	if v, ok := attrs.Get(key); ok {
		return v.AsString()
	}
	if v, ok := resAttrs.Get(key); ok {
		return v.AsString()
	}
	return ""
}

func merge(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func metricAttrsMatch(want map[string]string, res pcommon.Resource, dpAttrs pcommon.Map) bool {
	if len(want) == 0 {
		return true
	}
	for k, v := range want {
		if val, ok := dpAttrs.Get(k); ok {
			if val.AsString() != v {
				return false
			}
			continue
		}
		if val, ok := res.Attributes().Get(k); ok {
			if val.AsString() != v {
				return false
			}
			continue
		}
		return false
	}
	return true
}

func metricGroupKey(keys []string, res pcommon.Resource, dpAttrs pcommon.Map, metricName string) (string, map[string]string) {
	if len(keys) == 0 {
		return "all", map[string]string{"metric": metricName}
	}
	parts := make([]string, 0, len(keys))
	out := map[string]string{"metric": metricName}
	rattrs := res.Attributes()
	for _, k := range keys {
		val := ""
		if v, ok := dpAttrs.Get(k); ok {
			val = v.AsString()
		} else if v, ok := rattrs.Get(k); ok {
			val = v.AsString()
		}
		parts = append(parts, k+"="+val)
		out[k] = val
	}
	return strings.Join(parts, "|"), out
}

/* =========================
   Percentile helpers
   ========================= */

func explicitHistPercentile(dp pmetric.HistogramDataPoint, q float64) float64 {
	// pdata slices use Len()/At(i), not built-in len().
	bounds := dp.ExplicitBounds()
	counts := dp.BucketCounts()

	nb := bounds.Len() // number of explicit bounds
	nc := counts.Len() // number of buckets (= nb + 1)
	if nc == 0 {
		return 0
	}

	// Total samples across all buckets
	var total uint64
	for i := 0; i < nc; i++ {
		total += counts.At(i)
	}
	if total == 0 {
		return 0
	}
	target := uint64(math.Ceil(float64(total) * q))

	var cum uint64
	var lastUpper float64 = math.Inf(1) // in case we fall through to the last bucket

	for i := 0; i < nc; i++ {
		c := counts.At(i)

		// For bucket i:
		//   lower = (i == 0)     ?  -Inf or 0 (we'll use 0 as practical lower)
		//          (i > 0)       ?  bounds[i-1]
		//   upper = (i < nb)     ?  bounds[i]
		//          (i == nb)     ?  +Inf
		var lower float64
		if i == 0 {
			lower = 0 // practical floor; adjust if you track a true min elsewhere
		} else {
			lower = bounds.At(i - 1)
		}
		upper := math.Inf(1)
		if i < nb {
			upper = bounds.At(i)
			lastUpper = upper
		}

		// If the target quantile lies in this bucket
		if cum+c >= target {
			if c == 0 || math.IsInf(upper, 1) {
				// No internal resolution or open-ended bucket: return lower bound
				return lower
			}
			posInBucket := float64(target-cum) / float64(c)
			return lower + (upper-lower)*posInBucket
		}
		cum += c
	}

	// Fell through: return the upper bound of the last finite bucket if any,
	// otherwise +Inf (lastUpper carries last finite bound we saw).
	return lastUpper
}

func expHistPercentile(dp pmetric.ExponentialHistogramDataPoint, q float64) float64 {
	// Positive side only for now (common latency/size metrics are positive).
	pb := dp.Positive()
	if pb.BucketCounts().Len() == 0 {
		return 0
	}

	// base = 2^(2^-scale) per OTel exp-hist definition
	base := math.Pow(2, math.Pow(2, -float64(dp.Scale())))

	counts := pb.BucketCounts()
	off := int64(pb.Offset()) // <-- normalize to int64 once

	var total uint64
	for i := 0; i < counts.Len(); i++ {
		total += counts.At(i)
	}
	if total == 0 {
		return 0
	}
	target := uint64(math.Ceil(float64(total) * q))

	var cum uint64
	for i := 0; i < counts.Len(); i++ {
		c := counts.At(i)
		k := int64(i) + off // <-- all int64 now
		lower := math.Pow(base, float64(k))
		upper := math.Pow(base, float64(k+1))
		if cum+c >= target {
			if c == 0 {
				return lower
			}
			posInBucket := float64(target-cum) / float64(c)
			return lower + (upper-lower)*posInBucket
		}
		cum += c
	}

	// Fell off the end: return upper bound of last bucket.
	k := int64(counts.Len()-1) + off
	return math.Pow(base, float64(k+1))
}
