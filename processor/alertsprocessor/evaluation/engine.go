package evaluation

import (
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

	Logs   *LogsRule   `yaml:"logs,omitempty"`
	Traces *TracesRule `yaml:"traces,omitempty"`
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
	log         *zap.Logger
	logsRules   []Rule
	tracesRules []Rule
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
			// TODO: metrics rules in future
		}
	}
	return e
}

func (e *Engine) RunMetrics(_ []pmetric.Metrics, _ time.Time) []Result { return nil }

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

// Helpers

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

func matchSpan(rule *TracesRule, res pcommon.Resource, sp ptrace.Span) bool {
	if rule == nil { return false }
	if rule.LatencyMillisGT > 0 {
		dur := int64(sp.EndTimestamp()-sp.StartTimestamp())/1_000_000
		if dur <= int64(rule.LatencyMillisGT) { return false }
	}
	if rule.StatusNotOK {
		if sp.Status().Code() == ptrace.StatusCodeOk { return false }
	}
	if len(rule.AttrEquals) > 0 {
		attrs := sp.Attributes()
		resAttrs := res.Attributes()
		for k, v := range rule.AttrEquals {
			if !attrEquals(attrs, resAttrs, k, v) { return false }
		}
	}
	return true
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

func merge(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a { out[k] = v }
	for k, v := range b { out[k] = v }
	return out
}
