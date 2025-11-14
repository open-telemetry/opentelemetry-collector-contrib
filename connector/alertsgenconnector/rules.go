// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsgenconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector"

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/state"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/telemetry"
)

type ruleSet struct {
	cfg *Config

	traceRules  []*compiledRule
	logRules    []*compiledRule
	metricRules []*compiledRule

	stateMu sync.Mutex
	state   map[uint64]*alertState

	// telemetry (may be nil; set by engine)
	mx *telemetry.Metrics
}

type compiledRule struct {
	cfg      RuleCfg
	sel      map[string]*regexp.Regexp
	expr     evalExpr
	window   time.Duration
	step     time.Duration
	forDur   time.Duration
	groupBy  []string
	severity string
}

type evalExpr interface {
	kind() string
	evaluateTrace(rows []traceRow) (float64, bool)
	evaluateLog(rows []logRow) (float64, bool)
	evaluateMetric(rows []metricRow) (float64, bool)
	compare(val float64) bool
}

type alertState struct {
	Rule        string
	FP          uint64
	Labels      map[string]string
	Severity    string
	LastValue   float64
	FirstExceed time.Time
	Active      bool
}

func compileRules(cfg *Config) (*ruleSet, error) {
	rs := &ruleSet{cfg: cfg, state: map[uint64]*alertState{}}
	for _, rc := range cfg.Rules {
		cr, err := compileRule(rc, cfg)
		if err != nil {
			return nil, err
		}
		switch strings.ToLower(rc.Signal) {
		case "traces":
			rs.traceRules = append(rs.traceRules, cr)
		case "logs":
			rs.logRules = append(rs.logRules, cr)
		case "metrics":
			rs.metricRules = append(rs.metricRules, cr)
		default:
			return nil, fmt.Errorf("unknown signal %q", rc.Signal)
		}
	}
	return rs, nil
}

func compileRule(rc RuleCfg, cfg *Config) (*compiledRule, error) {
	if rc.Name == "" {
		return nil, errors.New("rule.name required")
	}
	if rc.Window == 0 {
		rc.Window = cfg.WindowSize
	}
	if rc.Step == 0 {
		rc.Step = cfg.WindowSize
	}
	if rc.Expr.Type == "" {
		return nil, fmt.Errorf("rule %s expr.type required", rc.Name)
	}

	sel := map[string]*regexp.Regexp{}
	for k, v := range rc.Select {
		re, err := regexp.Compile(v)
		if err != nil {
			return nil, fmt.Errorf("rule %s: bad select regex for %s: %v", rc.Name, k, err)
		}
		sel[k] = re
	}

	expr, err := buildExpr(rc.Expr)
	if err != nil {
		return nil, fmt.Errorf("rule %s: %v", rc.Name, err)
	}

	return &compiledRule{
		cfg:      rc,
		sel:      sel,
		expr:     expr,
		window:   rc.Window,
		step:     rc.Step,
		forDur:   rc.For,
		groupBy:  rc.GroupBy,
		severity: rc.Severity,
	}, nil
}

func buildExpr(ec ExprCfg) (evalExpr, error) {
	switch ec.Type {
	case "avg_over_time":
		return &exprAvg{Field: ec.Field, Op: ec.Op, Threshold: ec.Value}, nil
	case "rate_over_time":
		return &exprRate{Field: ec.Field, Op: ec.Op, Threshold: ec.Value}, nil
	case "count_over_time":
		return &exprCount{Field: ec.Field, Op: ec.Op, Threshold: ec.Value}, nil
	case "latency_quantile_over_time":
		return &exprQuantile{Field: ec.Field, Q: ec.Quantile, Op: ec.Op, Threshold: ec.Value}, nil
	case "absent_over_time":
		return &exprAbsent{}, nil
	default:
		return nil, fmt.Errorf("unsupported expr.type %q", ec.Type)
	}
}

func cmp(op string, a, b float64) bool {
	switch op {
	case ">":
		return a > b
	case ">=":
		return a >= b
	case "<":
		return a < b
	case "<=":
		return a <= b
	case "==":
		return a == b
	case "!=":
		return a != b
	default:
		return false
	}
}

type exprAvg struct {
	Field, Op string
	Threshold float64
}

func (e *exprAvg) kind() string { return "avg" }
func (e *exprAvg) evaluateTrace(rows []traceRow) (float64, bool) {
	if len(rows) == 0 {
		return 0, false
	}
	var sum float64
	for _, r := range rows {
		sum += r.durationNs
	}
	return sum / float64(len(rows)), true
}

func (e *exprAvg) evaluateLog(rows []logRow) (float64, bool) {
	if len(rows) == 0 {
		return 0, false
	}
	return float64(len(rows)), true
}

func (e *exprAvg) evaluateMetric(rows []metricRow) (float64, bool) {
	if len(rows) == 0 {
		return 0, false
	}
	var sum float64
	for _, r := range rows {
		sum += r.value
	}
	return sum / float64(len(rows)), true
}
func (e *exprAvg) compare(v float64) bool { return cmp(e.Op, v, e.Threshold) }

type exprRate struct {
	Field, Op string
	Threshold float64
}

func (e *exprRate) kind() string { return "rate" }
func (e *exprRate) evaluateTrace(rows []traceRow) (float64, bool) {
	return float64(len(rows)), len(rows) > 0
}

func (e *exprRate) evaluateLog(rows []logRow) (float64, bool) {
	return float64(len(rows)), len(rows) > 0
}

func (e *exprRate) evaluateMetric(rows []metricRow) (float64, bool) {
	return float64(len(rows)), len(rows) > 0
}
func (e *exprRate) compare(v float64) bool { return cmp(e.Op, v, e.Threshold) }

type exprCount struct {
	Field, Op string
	Threshold float64
}

func (e *exprCount) kind() string { return "count" }
func (e *exprCount) evaluateTrace(rows []traceRow) (float64, bool) {
	return float64(len(rows)), len(rows) > 0
}

func (e *exprCount) evaluateLog(rows []logRow) (float64, bool) {
	return float64(len(rows)), len(rows) > 0
}

func (e *exprCount) evaluateMetric(rows []metricRow) (float64, bool) {
	return float64(len(rows)), len(rows) > 0
}
func (e *exprCount) compare(v float64) bool { return cmp(e.Op, v, e.Threshold) }

type exprQuantile struct {
	Field     string
	Q         float64
	Op        string
	Threshold float64
}

func (e *exprQuantile) kind() string { return "quantile" }
func (e *exprQuantile) evaluateTrace(rows []traceRow) (float64, bool) {
	if len(rows) == 0 {
		return 0, false
	}
	vals := make([]float64, len(rows))
	for i, r := range rows {
		vals[i] = r.durationNs
	}
	sort.Float64s(vals)
	// nearest-rank (1-based): rank = ceil(Q * N); idx = rank - 1
	rank := int(math.Ceil(float64(len(vals)) * e.Q))
	if rank < 1 {
		rank = 1
	}
	if rank > len(vals) {
		rank = len(vals)
	}
	idx := rank - 1
	return vals[idx], true
}

func (e *exprQuantile) evaluateLog(rows []logRow) (float64, bool) {
	return float64(len(rows)), len(rows) > 0
}

func (e *exprQuantile) evaluateMetric(rows []metricRow) (float64, bool) {
	if len(rows) == 0 {
		return 0, false
	}
	vals := make([]float64, len(rows))
	for i, r := range rows {
		vals[i] = r.value
	}
	sort.Float64s(vals)
	// nearest-rank (1-based): rank = ceil(Q * N); idx = rank - 1
	rank := int(math.Ceil(float64(len(vals)) * e.Q))
	if rank < 1 {
		rank = 1
	}
	if rank > len(vals) {
		rank = len(vals)
	}
	idx := rank - 1
	return vals[idx], true
}
func (e *exprQuantile) compare(v float64) bool { return cmp(e.Op, v, e.Threshold) }

type exprAbsent struct{}

func (e *exprAbsent) kind() string                                    { return "absent" }
func (e *exprAbsent) evaluateTrace(rows []traceRow) (float64, bool)   { return 0, len(rows) == 0 }
func (e *exprAbsent) evaluateLog(rows []logRow) (float64, bool)       { return 0, len(rows) == 0 }
func (e *exprAbsent) evaluateMetric(rows []metricRow) (float64, bool) { return 0, len(rows) == 0 }
func (e *exprAbsent) compare(v float64) bool                          { return true }

// ---- grouping containers (avoid using map[string]string as map keys) ----

type trGroup struct {
	Labels map[string]string
	Rows   []traceRow
}
type lgGroup struct {
	Labels map[string]string
	Rows   []logRow
}
type mtGroup struct {
	Labels map[string]string
	Rows   []metricRow
}

// ---- evaluation pass ----

func (rs *ruleSet) evaluate(now time.Time, ing *ingester) ([]alertEvent, []stateMetric) {
	tr, lg, mt := ing.drain()
	var events []alertEvent
	var metrics []stateMetric

	handle := func(cr *compiledRule, fp uint64, labels map[string]string, value float64, ok bool) {
		if cr.expr.kind() == "absent" && ok {
			rs.transition(now, cr, fp, labels, 0, true, &events, &metrics)
			return
		}
		if !ok {
			return
		}
		fire := cr.expr.compare(value)
		rs.transition(now, cr, fp, labels, value, fire, &events, &metrics)
	}

	ctx := context.Background()

	// traces rules
	for _, cr := range rs.traceRules {
		evBefore := len(events)
		start := time.Now()

		spans := filterTraceRows(tr, cr.sel)
		groups := groupTraceRows(spans, cr.groupBy)
		for _, g := range groups {
			rows := filterWindowTr(g.Rows, now, cr.window)
			if len(rows) == 0 && cr.expr.kind() != "absent" {
				continue
			}
			val, ok := cr.expr.evaluateTrace(rows)
			handle(cr, fingerprint(cr.cfg.Name, g.Labels), g.Labels, val, ok)
		}

		dur := time.Since(start)
		if rs.mx != nil {
			rs.mx.RecordEvaluation(ctx, dur)
			if ev := len(events) - evBefore; ev > 0 {
				rs.mx.RecordEvents(ctx, ev)
			}
		}
	}

	// logs rules
	for _, cr := range rs.logRules {
		evBefore := len(events)
		start := time.Now()

		rows0 := filterLogRows(lg, cr.sel)
		groups := groupLogRows(rows0, cr.groupBy)
		for _, g := range groups {
			rows := filterWindowLg(g.Rows, now, cr.window)
			if len(rows) == 0 && cr.expr.kind() != "absent" {
				continue
			}
			val, ok := cr.expr.evaluateLog(rows)
			handle(cr, fingerprint(cr.cfg.Name, g.Labels), g.Labels, val, ok)
		}

		dur := time.Since(start)
		if rs.mx != nil {
			rs.mx.RecordEvaluation(ctx, dur)
			if ev := len(events) - evBefore; ev > 0 {
				rs.mx.RecordEvents(ctx, ev)
			}
		}
	}

	// metrics rules
	for _, cr := range rs.metricRules {
		evBefore := len(events)
		start := time.Now()

		rows0 := filterMetricRows(mt, cr.sel)
		groups := groupMetricRows(rows0, cr.groupBy)
		for _, g := range groups {
			rows := filterWindowMt(g.Rows, now, cr.window)
			if len(rows) == 0 && cr.expr.kind() != "absent" {
				continue
			}
			val, ok := cr.expr.evaluateMetric(rows)
			handle(cr, fingerprint(cr.cfg.Name, g.Labels), g.Labels, val, ok)
		}

		dur := time.Since(start)
		if rs.mx != nil {
			rs.mx.RecordEvaluation(ctx, dur)
			if ev := len(events) - evBefore; ev > 0 {
				rs.mx.RecordEvents(ctx, ev)
			}
		}
	}

	return events, metrics
}

func filterWindowTr(rows []traceRow, now time.Time, w time.Duration) []traceRow {
	if w <= 0 {
		return rows
	}
	cutoff := now.Add(-w)
	out := rows[:0]
	for _, r := range rows {
		if r.ts.After(cutoff) {
			out = append(out, r)
		}
	}
	return out
}

func filterWindowLg(rows []logRow, now time.Time, w time.Duration) []logRow {
	if w <= 0 {
		return rows
	}
	cutoff := now.Add(-w)
	out := rows[:0]
	for _, r := range rows {
		if r.ts.After(cutoff) {
			out = append(out, r)
		}
	}
	return out
}

func filterWindowMt(rows []metricRow, now time.Time, w time.Duration) []metricRow {
	if w <= 0 {
		return rows
	}
	cutoff := now.Add(-w)
	out := rows[:0]
	for _, r := range rows {
		if r.ts.After(cutoff) {
			out = append(out, r)
		}
	}
	return out
}

func filterTraceRows(rows []traceRow, sel map[string]*regexp.Regexp) []traceRow {
	if len(sel) == 0 {
		return rows
	}
	out := make([]traceRow, 0, len(rows))
	for _, r := range rows {
		if matchSel(r.attrs, sel) {
			out = append(out, r)
		}
	}
	return out
}

func filterLogRows(rows []logRow, sel map[string]*regexp.Regexp) []logRow {
	if len(sel) == 0 {
		return rows
	}
	out := make([]logRow, 0, len(rows))
	for _, r := range rows {
		if matchSel(r.attrs, sel) {
			out = append(out, r)
		}
	}
	return out
}

func filterMetricRows(rows []metricRow, sel map[string]*regexp.Regexp) []metricRow {
	if len(sel) == 0 {
		return rows
	}
	out := make([]metricRow, 0, len(rows))
	for _, r := range rows {
		if matchSel(r.attrs, sel) {
			out = append(out, r)
		}
	}
	return out
}

func matchSel(attrs map[string]string, sel map[string]*regexp.Regexp) bool {
	for k, re := range sel {
		if v, ok := attrs[k]; !ok || !re.MatchString(v) {
			return false
		}
	}
	return true
}

// ---- grouping helpers (using canonical string bucket keys) ----

func groupTraceRows(rows []traceRow, keys []string) []trGroup {
	buckets := map[string]*trGroup{}
	for _, r := range rows {
		lbls := pick(r.attrs, keys)
		key := canonKey(lbls)
		g := buckets[key]
		if g == nil {
			g = &trGroup{Labels: lbls}
			buckets[key] = g
		}
		g.Rows = append(g.Rows, r)
	}
	out := make([]trGroup, 0, len(buckets))
	for _, g := range buckets {
		out = append(out, *g)
	}
	return out
}

func groupLogRows(rows []logRow, keys []string) []lgGroup {
	buckets := map[string]*lgGroup{}
	for _, r := range rows {
		lbls := pick(r.attrs, keys)
		key := canonKey(lbls)
		g := buckets[key]
		if g == nil {
			g = &lgGroup{Labels: lbls}
			buckets[key] = g
		}
		g.Rows = append(g.Rows, r)
	}
	out := make([]lgGroup, 0, len(buckets))
	for _, g := range buckets {
		out = append(out, *g)
	}
	return out
}

func groupMetricRows(rows []metricRow, keys []string) []mtGroup {
	buckets := map[string]*mtGroup{}
	for _, r := range rows {
		lbls := pick(r.attrs, keys)
		key := canonKey(lbls)
		g := buckets[key]
		if g == nil {
			g = &mtGroup{Labels: lbls}
			buckets[key] = g
		}
		g.Rows = append(g.Rows, r)
	}
	out := make([]mtGroup, 0, len(buckets))
	for _, g := range buckets {
		out = append(out, *g)
	}
	return out
}

func canonKey(lbls map[string]string) string {
	ks := make([]string, 0, len(lbls))
	for k := range lbls {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	parts := make([]string, 0, len(ks))
	for _, k := range ks {
		parts = append(parts, k+"="+lbls[k])
	}
	return strings.Join(parts, "|")
}

func pick(attrs map[string]string, keys []string) map[string]string {
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		if v, ok := attrs[k]; ok {
			out[k] = v
		}
	}
	return out
}

func fpMap(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	out := make([]string, 0, len(ks)*2)
	for _, k := range ks {
		out = append(out, k, m[k])
	}
	return out
}

func fingerprint(rule string, labels map[string]string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(rule))
	for _, s := range fpMap(labels) {
		h.Write([]byte{0xff})
		h.Write([]byte(s))
	}
	return h.Sum64()
}

type alertEvent struct {
	Rule     string
	State    string // firing|resolved|no_data
	Severity string
	Labels   map[string]string
	Value    float64
	Window   string
	For      string
}

type stateMetric struct {
	Rule      string
	Labels    map[string]string
	Severity  string
	Active    int64
	LastValue float64
}

func (rs *ruleSet) transition(now time.Time, cr *compiledRule, fp uint64, labels map[string]string, value float64, cond bool, events *[]alertEvent, metrics *[]stateMetric) {
	rs.stateMu.Lock()
	defer rs.stateMu.Unlock()
	st, ok := rs.state[fp]
	if !ok {
		st = &alertState{Rule: cr.cfg.Name, FP: fp, Labels: labels, Severity: cr.severity}
		rs.state[fp] = st
	}
	st.LastValue = value
	if cond {
		if !st.Active {
			if st.FirstExceed.IsZero() {
				st.FirstExceed = now
			}
			if now.Sub(st.FirstExceed) >= cr.forDur {
				st.Active = true
				*events = append(*events, alertEvent{
					Rule: cr.cfg.Name, State: "firing", Severity: cr.severity, Labels: labels,
					Value: value, Window: cr.window.String(), For: cr.forDur.String(),
				})
			}
		}
	} else {
		if st.Active {
			st.Active = false
			st.FirstExceed = time.Time{}
			*events = append(*events, alertEvent{
				Rule: cr.cfg.Name, State: "resolved", Severity: cr.severity, Labels: labels,
				Value: value, Window: cr.window.String(), For: cr.forDur.String(),
			})
		} else {
			st.FirstExceed = time.Time{}
		}
	}
	*metrics = append(*metrics, stateMetric{
		Rule: cr.cfg.Name, Labels: labels, Severity: cr.severity, Active: boolToInt(st.Active), LastValue: value,
	})
}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// TSDB state restore (best effort): consult active firing states and pre-populate 'Active' if takeover conditions are met
func (rs *ruleSet) restoreFromTSDB(syncer *state.TSDBSyncer) error {
	if syncer == nil {
		return nil
	}
	for _, cr := range append(append([]*compiledRule{}, rs.traceRules...), append(rs.logRules, rs.metricRules...)...) {
		actives, _ := syncer.QueryActive(cr.cfg.Name)
		rs.stateMu.Lock()
		for fp, entry := range actives {
			if st, ok := rs.state[fp]; ok {
				st.Active = entry.Active
				st.FirstExceed = time.Now().Add(-entry.ForDuration)
			} else {
				rs.state[fp] = &alertState{
					Rule: cr.cfg.Name, FP: fp, Labels: entry.Labels, Severity: cr.severity,
					Active: entry.Active, FirstExceed: time.Now().Add(-entry.ForDuration),
				}
			}
		}
		rs.stateMu.Unlock()
	}
	return nil
}
