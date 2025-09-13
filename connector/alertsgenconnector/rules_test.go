// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsgenconnector

import (
	"regexp"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ------------------------------
// compileRules / compileRule / buildExpr
// ------------------------------

func TestCompileRules_RoutesBySignal(t *testing.T) {
	cfg := &Config{
		WindowSize: 200 * time.Millisecond,
		Rules: []RuleCfg{
			{
				Name:     "tr",
				Signal:   "traces",
				Severity: "warn",
				Window:   100 * time.Millisecond,
				Step:     50 * time.Millisecond,
				Expr:     ExprCfg{Type: "count_over_time", Op: ">", Value: 0},
			},
			{
				Name:     "lg",
				Signal:   "logs",
				Severity: "warn",
				Expr:     ExprCfg{Type: "avg_over_time", Op: ">", Value: 1},
			},
			{
				Name:     "mt",
				Signal:   "metrics",
				Severity: "warn",
				Expr:     ExprCfg{Type: "rate_over_time", Op: ">=", Value: 1},
			},
		},
	}

	rs, err := compileRules(cfg)
	require.NoError(t, err)
	require.NotNil(t, rs)

	require.Len(t, rs.traceRules, 1)
	require.Len(t, rs.logRules, 1)
	require.Len(t, rs.metricRules, 1)

	// Defaults for Window/Step applied when zero in rule.
	assert.Equal(t, cfg.WindowSize, rs.logRules[0].window)
	assert.Equal(t, cfg.WindowSize, rs.logRules[0].step)
}

func TestCompileRule_Validates(t *testing.T) {
	// missing name
	_, err := compileRule(RuleCfg{Signal: "traces", Expr: ExprCfg{Type: "count_over_time"}}, &Config{WindowSize: time.Second})
	require.Error(t, err)

	// missing expr.type
	_, err = compileRule(RuleCfg{Name: "x", Signal: "traces"}, &Config{WindowSize: time.Second})
	require.Error(t, err)

	// bad selector regex
	_, err = compileRule(RuleCfg{
		Name: "x", Signal: "traces",
		Select: map[string]string{"service.name": "("},
		Expr:   ExprCfg{Type: "count_over_time"},
	}, &Config{WindowSize: time.Second})
	require.Error(t, err)

	// good one
	cr, err := compileRule(RuleCfg{
		Name:     "ok",
		Signal:   "traces",
		Severity: "warn",
		Window:   250 * time.Millisecond,
		Step:     100 * time.Millisecond,
		GroupBy:  []string{"service.name"},
		Select:   map[string]string{"service.name": ".*"},
		Expr:     ExprCfg{Type: "count_over_time", Op: ">", Value: 0},
	}, &Config{WindowSize: time.Second})
	require.NoError(t, err)
	require.NotNil(t, cr)
	assert.Equal(t, 250*time.Millisecond, cr.window)
	assert.Equal(t, 100*time.Millisecond, cr.step)
	assert.Equal(t, "warn", cr.severity)
}

func TestBuildExprKinds(t *testing.T) {
	tests := []struct {
		name string
		ec   ExprCfg
		kind string
	}{
		{"avg", ExprCfg{Type: "avg_over_time"}, "avg"},
		{"rate", ExprCfg{Type: "rate_over_time"}, "rate"},
		{"count", ExprCfg{Type: "count_over_time"}, "count"},
		{"quantile", ExprCfg{Type: "latency_quantile_over_time", Quantile: 0.9}, "quantile"},
		{"absent", ExprCfg{Type: "absent_over_time"}, "absent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, err := buildExpr(tt.ec)
			require.NoError(t, err)
			assert.Equal(t, tt.kind, e.kind())
		})
	}

	_, err := buildExpr(ExprCfg{Type: "nope"})
	require.Error(t, err)
}

// ------------------------------
// cmp operator semantics
// ------------------------------

func TestCmpOperators(t *testing.T) {
	assert.True(t, cmp(">", 2, 1))
	assert.True(t, cmp(">=", 2, 2))
	assert.True(t, cmp("<", 1, 2))
	assert.True(t, cmp("<=", 2, 2))
	assert.True(t, cmp("==", 3, 3))
	assert.True(t, cmp("!=", 3, 4))

	assert.False(t, cmp(">", 1, 2))
	assert.False(t, cmp(">=", 1, 2))
	assert.False(t, cmp("<", 2, 1))
	assert.False(t, cmp("<=", 1, 0))
	assert.False(t, cmp("==", 3, 4))
	assert.False(t, cmp("!=", 3, 3))

	assert.False(t, cmp("<?>", 1, 1))
}

// ------------------------------
// Expression evaluation
// ------------------------------

func TestExprCount(t *testing.T) {
	tr := []traceRow{{}, {}, {}}
	lg := []logRow{{}, {}}
	mt := []metricRow{{}, {}, {}, {}}

	e := &exprCount{Field: "any", Op: ">", Threshold: 0}

	v, ok := e.evaluateTrace(tr)
	require.True(t, ok)
	assert.Equal(t, float64(3), v)

	v, ok = e.evaluateLog(lg)
	require.True(t, ok)
	assert.Equal(t, float64(2), v)

	v, ok = e.evaluateMetric(mt)
	require.True(t, ok)
	assert.Equal(t, float64(4), v)
}

func TestExprAvg(t *testing.T) {
	tr := []traceRow{
		{durationNs: 10},
		{durationNs: 20},
		{durationNs: 30},
	}
	mt := []metricRow{
		{value: 2.5}, {value: 3.5},
	}
	e := &exprAvg{Field: "value", Op: ">", Threshold: 0}

	v, ok := e.evaluateTrace(tr)
	require.True(t, ok)
	assert.InDelta(t, 20.0, v, 1e-9)

	v, ok = e.evaluateMetric(mt)
	require.True(t, ok)
	assert.InDelta(t, 3.0, v, 1e-9)

	// logs path returns count (as implemented) and ok when len>0
	v, ok = e.evaluateLog([]logRow{{}, {}})
	require.True(t, ok)
	assert.Equal(t, float64(2), v)
}

func TestExprRate(t *testing.T) {
	e := &exprRate{Field: "value", Op: ">=", Threshold: 2}

	v, ok := e.evaluateTrace([]traceRow{{}, {}, {}})
	require.True(t, ok)
	assert.Equal(t, float64(3), v)
	assert.True(t, e.compare(v))

	v, ok = e.evaluateLog([]logRow{{}})
	require.True(t, ok)
	assert.Equal(t, float64(1), v)

	v, ok = e.evaluateMetric([]metricRow{})
	assert.False(t, ok)
	assert.Equal(t, float64(0), v)
}

func TestExprAbsent(t *testing.T) {
	e := &exprAbsent{}
	v, ok := e.evaluateTrace(nil)
	assert.True(t, ok)
	assert.Equal(t, float64(0), v)

	v, ok = e.evaluateLog([]logRow{{}})
	assert.False(t, ok)
	assert.Equal(t, float64(0), v)

	v, ok = e.evaluateMetric(nil)
	assert.True(t, ok)
	assert.Equal(t, float64(0), v)
}

func TestExprQuantile_NearestRank_TracesAndMetrics(t *testing.T) {
	// traces durations: 100..1000
	tr := make([]traceRow, 10)
	for i := 0; i < 10; i++ {
		tr[i] = traceRow{durationNs: float64((i + 1) * 100)}
	}
	// shuffle to ensure sort is required
	sort.Slice(tr, func(i, j int) bool { return (i%2 == 0) && (j%2 == 1) })

	// With the current implementation, P90 of 10 values = 900 (rank=ceil(0.9*10)=9 -> index 8)
	e := &exprQuantile{Field: "duration_ns", Q: 0.9, Op: ">=", Threshold: 900}
	v, ok := e.evaluateTrace(tr)
	require.True(t, ok)
	assert.InDelta(t, 900.0, v, 1.0)
	assert.True(t, e.compare(v))

	// metrics values: 1..6
	mt := []metricRow{{value: 1}, {value: 2}, {value: 3}, {value: 4}, {value: 5}, {value: 6}}

	e = &exprQuantile{Field: "value", Q: 1.0}
	v, ok = e.evaluateMetric(mt)
	require.True(t, ok)
	assert.InDelta(t, 6.0, v, 1e-9)

	e = &exprQuantile{Field: "value", Q: 0.5}
	v, ok = e.evaluateMetric(mt)
	require.True(t, ok)
	assert.InDelta(t, 3.0, v, 1e-9) // ceil(0.5*6)=3 → index 2 → 3
}

// ------------------------------
// Selection & Grouping helpers
// ------------------------------

func TestMatchSel(t *testing.T) {
	lbls := map[string]string{"service.name": "svc-a", "env": "prod"}
	sel := map[string]*regexp.Regexp{
		"service.name": regexp.MustCompile(`^svc-.*$`),
		"env":          regexp.MustCompile(`^(prod|stg)$`),
	}
	assert.True(t, matchSel(lbls, sel))

	sel["env"] = regexp.MustCompile(`^dev$`)
	assert.False(t, matchSel(lbls, sel))
}

func TestGroupingTraceLogMetric(t *testing.T) {
	// trace
	tr := []traceRow{
		{attrs: map[string]string{"service.name": "svc-a", "env": "prod"}},
		{attrs: map[string]string{"service.name": "svc-b", "env": "prod"}},
		{attrs: map[string]string{"service.name": "svc-a", "env": "prod"}},
	}
	grTr := groupTraceRows(tr, []string{"service.name"})
	require.Len(t, grTr, 2)

	// log
	lg := []logRow{
		{attrs: map[string]string{"level": "ERROR", "service.name": "svc-a"}},
		{attrs: map[string]string{"level": "INFO", "service.name": "svc-a"}},
		{attrs: map[string]string{"level": "ERROR", "service.name": "svc-b"}},
	}
	grLg := groupLogRows(lg, []string{"service.name"})
	require.Len(t, grLg, 2)

	// metric
	mt := []metricRow{
		{attrs: map[string]string{"host": "h1"}},
		{attrs: map[string]string{"host": "h1"}},
		{attrs: map[string]string{"host": "h2"}},
	}
	grMt := groupMetricRows(mt, []string{"host"})
	require.Len(t, grMt, 2)
}

func TestCanonKeyAndPick(t *testing.T) {
	lbls := map[string]string{"b": "2", "a": "1"}
	key := canonKey(lbls)
	// order-insensitive; must produce "a=1|b=2"
	assert.Equal(t, "a=1|b=2", key)

	got := pick(map[string]string{"a": "1", "b": "2", "c": "3"}, []string{"b", "a"})
	assert.Equal(t, map[string]string{"a": "1", "b": "2"}, got)
}

func TestFingerprintDeterminism(t *testing.T) {
	lbls1 := map[string]string{"a": "1", "b": "2"}
	lbls2 := map[string]string{"b": "2", "a": "1"}
	assert.Equal(t, fingerprint("r", lbls1), fingerprint("r", lbls2))
	assert.NotEqual(t, fingerprint("r", lbls1), fingerprint("r2", lbls1))
}

// ------------------------------
// Window filters
// ------------------------------

func TestFilterWindows(t *testing.T) {
	now := time.Now()
	w := 200 * time.Millisecond

	tr := []traceRow{
		{ts: now.Add(-w - 10*time.Millisecond)},
		{ts: now.Add(-w + 10*time.Millisecond)},
		{ts: now.Add(-50 * time.Millisecond)},
	}
	gotTr := filterWindowTr(tr, now, w)
	require.Len(t, gotTr, 2)

	lg := []logRow{
		{ts: now.Add(-w - time.Millisecond)},
		{ts: now.Add(-w + time.Millisecond)},
	}
	require.Len(t, filterWindowLg(lg, now, w), 1)

	mt := []metricRow{
		{ts: now.Add(-w)},                     // equal to cutoff -> excluded (strict After)
		{ts: now.Add(-w - time.Nanosecond)},   // before cutoff -> excluded
		{ts: now.Add(-10 * time.Millisecond)}, // within window -> included
	}
	require.Len(t, filterWindowMt(mt, now, w), 1)
}

// ------------------------------
// ruleSet.transition state machine
// ------------------------------

func TestTransition_FiringAfterFor_AndResolved(t *testing.T) {
	// compiled rule that fires when count_over_time > 0 with for=100ms
	cr, err := compileRule(RuleCfg{
		Name:     "count>0",
		Signal:   "traces",
		Severity: "critical",
		For:      100 * time.Millisecond,
		Window:   100 * time.Millisecond,
		Expr:     ExprCfg{Type: "count_over_time", Op: ">", Value: 0},
		GroupBy:  []string{"service.name"},
	}, &Config{WindowSize: 100 * time.Millisecond})
	require.NoError(t, err)

	rs := &ruleSet{
		cfg:   &Config{},
		state: map[uint64]*alertState{},
	}

	labels := map[string]string{"service.name": "svc-a"}
	fp := fingerprint(cr.cfg.Name, labels)

	now := time.Now()
	var events []alertEvent
	var metrics []stateMetric

	// First transition: condition true, but 'for' not yet satisfied → no firing
	rs.transition(now, cr, fp, labels, 1.0, true, &events, &metrics)
	require.Len(t, events, 0)
	require.Len(t, metrics, 1)
	assert.Equal(t, int64(0), metrics[0].Active)

	// After 'for' has elapsed, condition still true → becomes active and emits "firing"
	events = nil
	metrics = nil
	rs.transition(now.Add(120*time.Millisecond), cr, fp, labels, 1.0, true, &events, &metrics)
	require.Len(t, events, 1)
	assert.Equal(t, "firing", events[0].State)
	require.Len(t, metrics, 1)
	assert.Equal(t, int64(1), metrics[0].Active)

	// Now resolve: condition false → emits "resolved"
	events = nil
	metrics = nil
	rs.transition(now.Add(200*time.Millisecond), cr, fp, labels, 0.0, false, &events, &metrics)
	require.Len(t, events, 1)
	assert.Equal(t, "resolved", events[0].State)
	require.Len(t, metrics, 1)
	assert.Equal(t, int64(0), metrics[0].Active)
}

// ------------------------------
// restoreFromTSDB(nil) is no-op
// ------------------------------

func TestRestoreFromTSDB_NilNoop(t *testing.T) {
	rs := &ruleSet{state: map[uint64]*alertState{}}
	err := rs.restoreFromTSDB(nil)
	require.NoError(t, err)
}
