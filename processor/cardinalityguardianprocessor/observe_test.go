// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinalityguardianprocessor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

var allKinds = []metricKind{kindGauge, kindDeltaSum, kindCumulativeSum, kindHistogram, kindExponentialHistogram, kindSummary}

// newObserveProcessor returns an observe-mode processor with a Debug-level log
// observer. mutate adjusts the default test config; next defaults to a sink.
func newObserveProcessor(t *testing.T, mutate func(*Config), next consumer.Metrics) (*cardinalityProcessor, *observer.ObservedLogs) {
	t.Helper()
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 100,
		EpochDurationSeconds:        300,
		EnforcementMode:             EnforcementObserve,
		TopOffendersCount:           10,
		HighCardinalityThreshold:    1000,
	}
	if mutate != nil {
		mutate(cfg)
	}
	require.NoError(t, cfg.Validate())
	if next == nil {
		next = new(consumertest.MetricsSink)
	}
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)
	p := proc.(*cardinalityProcessor)
	core, logs := observer.New(zap.DebugLevel)
	p.logger = zap.New(core)
	return p, logs
}

func testValues(prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%s-%d", prefix, i)
	}
	return out
}

// kindBatch returns one metric of the given kind with one data point per value.
// Each point carries label=value and, unless label is "region", region=r1.
func kindBatch(kind metricKind, metricName, label string, values []string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName(metricName)
	var newPoint func() pcommon.Map
	switch kind {
	case kindGauge:
		g := m.SetEmptyGauge()
		newPoint = func() pcommon.Map {
			dp := g.DataPoints().AppendEmpty()
			dp.SetIntValue(1)
			return dp.Attributes()
		}
	case kindDeltaSum, kindCumulativeSum:
		s := m.SetEmptySum()
		if kind == kindDeltaSum {
			s.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		} else {
			s.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		}
		newPoint = func() pcommon.Map {
			dp := s.DataPoints().AppendEmpty()
			dp.SetIntValue(1)
			return dp.Attributes()
		}
	case kindHistogram:
		h := m.SetEmptyHistogram()
		h.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		newPoint = func() pcommon.Map {
			dp := h.DataPoints().AppendEmpty()
			dp.SetCount(1)
			dp.ExplicitBounds().FromRaw([]float64{1})
			dp.BucketCounts().FromRaw([]uint64{1, 0})
			return dp.Attributes()
		}
	case kindExponentialHistogram:
		e := m.SetEmptyExponentialHistogram()
		e.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		newPoint = func() pcommon.Map {
			dp := e.DataPoints().AppendEmpty()
			dp.SetCount(1)
			return dp.Attributes()
		}
	case kindSummary:
		s := m.SetEmptySummary()
		newPoint = func() pcommon.Map {
			dp := s.DataPoints().AppendEmpty()
			dp.SetCount(1)
			return dp.Attributes()
		}
	}
	for _, v := range values {
		attrs := newPoint()
		attrs.PutStr(label, v)
		if label != "region" {
			attrs.PutStr("region", "r1")
		}
	}
	return md
}

func protoBytes(t *testing.T, md pmetric.Metrics) []byte {
	t.Helper()
	b, err := (&pmetric.ProtoMarshaler{}).MarshalMetrics(md)
	require.NoError(t, err)
	return b
}

func consume(t *testing.T, p *cardinalityProcessor, md pmetric.Metrics) {
	t.Helper()
	require.NoError(t, p.ConsumeMetrics(t.Context(), md))
}

// findingsOf returns the fields of every finding line, in order.
func findingsOf(logs *observer.ObservedLogs) []map[string]any {
	entries := logs.FilterMessage("High-cardinality recommendation").All()
	out := make([]map[string]any, len(entries))
	for i := range entries {
		out[i] = entries[i].ContextMap()
	}
	return out
}

func findingsFor(logs *observer.ObservedLogs, metric, label string) []map[string]any {
	var out []map[string]any
	for _, f := range findingsOf(logs) {
		if f["metric"] == metric && f["label"] == label {
			out = append(out, f)
		}
	}
	return out
}

func TestObserve_DoesNotMutate(t *testing.T) {
	for _, kind := range allKinds {
		t.Run(kind.String(), func(t *testing.T) {
			p, logs := newObserveProcessor(t, func(c *Config) {
				c.HighCardinalityThreshold = 5
				c.MaxCardinalityDeltaPerEpoch = 1
				c.MaxTrackerCount = 1 // region is rejected
				c.NeverDropLabels = []string{"region"}
			}, nil)
			// The value count grows every epoch, so user_id ends up HIGH and GROWING.
			for epoch := range 4 {
				md := kindBatch(kind, "m", "user_id", testValues(fmt.Sprint(epoch), 50*(epoch+1)))
				want := pmetric.NewMetrics()
				md.CopyTo(want)
				md.MarkReadOnly() // any mutation panics
				consume(t, p, md)
				p.rotate()
				assert.Equal(t, protoBytes(t, want), protoBytes(t, md))
			}
			assert.Positive(t, p.trackersRejected.Load())
			got := findingsFor(logs, "m", "user_id")
			require.NotEmpty(t, got)
			assert.Equal(t, "GROWING,HIGH", got[len(got)-1]["status"])
		})
	}
}

func TestObserve_PassesSameBatch(t *testing.T) {
	var got pmetric.Metrics
	next, err := consumer.NewMetrics(func(_ context.Context, md pmetric.Metrics) error {
		got = md
		return nil
	})
	require.NoError(t, err)
	p, _ := newObserveProcessor(t, nil, next)
	md := kindBatch(kindGauge, "m", "user_id", testValues("u", 3))
	consume(t, p, md)

	// A write through the received value is visible in md only if it is the same batch.
	got.ResourceMetrics().At(0).Resource().Attributes().PutStr("probe", "x")
	_, ok := md.ResourceMetrics().At(0).Resource().Attributes().Get("probe")
	assert.True(t, ok)
}

func TestObserve_ReturnsNextConsumerError(t *testing.T) {
	errBoom := errors.New("boom")
	p, _ := newObserveProcessor(t, nil, consumertest.NewErr(errBoom))
	err := p.ConsumeMetrics(t.Context(), kindBatch(kindGauge, "m", "user_id", testValues("u", 3)))
	assert.ErrorIs(t, err, errBoom)
}

func TestObserve_Capabilities(t *testing.T) {
	for mode, wantMutates := range map[EnforcementMode]bool{
		EnforcementObserve:             false,
		"OBSERVE":                      false,
		EnforcementTagOnly:             true,
		EnforcementOverflowAttribute:   true,
		EnforcementStripAndReaggregate: true,
	} {
		t.Run(string(mode), func(t *testing.T) {
			p, _ := newObserveProcessor(t, func(c *Config) { c.EnforcementMode = mode }, nil)
			assert.Equal(t, wantMutates, p.Capabilities().MutatesData)
		})
	}
}

func TestObserve_UnusualInput(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	p, _ := newObserveProcessor(t, nil, sink)
	consume(t, p, pmetric.NewMetrics())

	md := pmetric.NewMetrics()
	ms := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	ms.AppendEmpty().SetName("empty_type")
	dps := ms.AppendEmpty().SetEmptyGauge().DataPoints()
	dps.AppendEmpty() // no attributes
	attrs := dps.AppendEmpty().Attributes()
	attrs.PutInt("i", 1)
	attrs.PutBool("b", true)
	attrs.PutDouble("d", 1.5)
	attrs.PutEmptyBytes("bytes").FromRaw([]byte{1, 2})
	attrs.PutEmptyMap("map").PutStr("k", "v")
	attrs.PutEmptySlice("slice").AppendEmpty().SetStr("v")
	want := pmetric.NewMetrics()
	md.CopyTo(want)
	md.MarkReadOnly()

	consume(t, p, md)
	p.rotate()
	assert.Equal(t, protoBytes(t, want), protoBytes(t, md))
	assert.Len(t, sink.AllMetrics(), 2)
	assert.Equal(t, int64(6), p.trackerCount.Load())
}

func TestObserve_ConcurrentConsumeAndRotate(t *testing.T) {
	p, _ := newObserveProcessor(t, func(c *Config) { c.HighCardinalityThreshold = 5 }, nil)
	var wg sync.WaitGroup
	for g := range 4 {
		wg.Go(func() {
			for i := range 200 {
				md := kindBatch(allKinds[(g+i)%len(allKinds)], fmt.Sprintf("m%d", g), "user_id", testValues(fmt.Sprintf("%d-%d", g, i), 20))
				assert.NoError(t, p.ConsumeMetrics(t.Context(), md))
			}
		})
	}
	for range 50 {
		p.rotate()
	}
	wg.Wait()
}

func TestSaturatingSub(t *testing.T) {
	assert.Equal(t, uint64(5), saturatingSub(10, 5))
	assert.Equal(t, uint64(0), saturatingSub(5, 10)) // falling count: no wrap-around
}

func TestEvaluate(t *testing.T) {
	p, _ := newObserveProcessor(t, func(c *Config) {
		c.MaxCardinalityDeltaPerEpoch = 50
		c.MetricOverrides = map[string]int{"wide": 500}
	}, nil)
	tests := []struct {
		name          string
		f             finding
		want          bool
		high, growing bool
	}{
		{"healthy", finding{metricName: "m", distinct: 10, growingEvaluated: true}, false, false, false},
		{"high at threshold", finding{metricName: "m", distinct: 1000, growingEvaluated: true}, true, true, false},
		{"growing", finding{metricName: "m", distinct: 300, delta: 200, growingEvaluated: true}, true, false, true},
		{"growth at limit is not growing", finding{metricName: "m", distinct: 150, delta: 50, growingEvaluated: true}, false, false, false},
		{"growth not evaluated", finding{metricName: "m", distinct: 300, delta: 300}, false, false, false},
		{"metric override respected", finding{metricName: "wide", distinct: 300, delta: 200, growingEvaluated: true}, false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.f
			assert.Equal(t, tt.want, p.evaluate(&f))
			assert.Equal(t, tt.high, f.high)
			assert.Equal(t, tt.growing, f.growing)
		})
	}
}

func TestRecommend(t *testing.T) {
	f := func(kind metricKind) finding {
		return finding{metricName: "checkout_requests", labelKey: "error_type", kind: kind, high: true}
	}

	text, stmts := recommend(f(kindDeltaSum))
	assert.Equal(t, "Remove label error_type from metric checkout_requests and re-aggregate with sum. Apply ottl_statements in a transformprocessor as two metric_statements groups, in order.", text)
	assert.Equal(t, []string{
		`delete_key(datapoint.attributes, "error_type") where metric.name == "checkout_requests"`,
		`aggregate_on_attributes("sum") where metric.name == "checkout_requests"`,
	}, stmts)

	text, stmts = recommend(f(kindGauge))
	assert.Equal(t, "Remove label error_type from metric checkout_requests and re-aggregate. Apply ottl_statements in a transformprocessor as two metric_statements groups, in order. Replace <fn> with max, min, mean or median.", text)
	assert.Equal(t, `aggregate_on_attributes("<fn>") where metric.name == "checkout_requests"`, stmts[1])

	for _, kind := range []metricKind{kindCumulativeSum, kindHistogram, kindExponentialHistogram, kindSummary, kindMixed} {
		text, stmts = recommend(f(kind))
		assert.Equal(t, "Reduce the values of label error_type on metric checkout_requests at the source (instrumentation). Removing it in the collector merges "+kind.String()+" series, which is unsafe when the merged series arrive in different batches or at different collectors.", text, kind.String())
		assert.Nil(t, stmts, kind.String())
	}

	s := f(kindDeltaSum)
	s.multiService = true
	text, stmts = recommend(s)
	assert.Equal(t, "Metric checkout_requests arrives from more than one service.name, and this count combines them. Check label error_type per service before removing it.", text)
	assert.Nil(t, stmts)

	p := f(kindDeltaSum)
	p.protected = true
	text, stmts = recommend(p)
	assert.Equal(t, "Label error_type is in never_drop_labels; reduce its values at the source (instrumentation).", text)
	assert.Nil(t, stmts)

	for _, name := range []string{`bad"label`, `bad\label`, "bad\nlabel", "bad\u200blabel"} {
		u := f(kindDeltaSum)
		u.labelKey = name
		text, stmts = recommend(u)
		assert.Nil(t, stmts, name)
		assert.Contains(t, text, "at the source (instrumentation).", name)
	}
}

func TestReport_HighSparseRange(t *testing.T) {
	p, logs := newObserveProcessor(t, nil, nil) // threshold 1000
	consume(t, p, kindBatch(kindDeltaSum, "a", "user_id", testValues("u", 1100)))
	consume(t, p, kindBatch(kindDeltaSum, "b", "user_id", testValues("u", 900)))
	consume(t, p, kindBatch(kindGauge, "mix", "user_id", testValues("u", 1100)))
	consume(t, p, kindBatch(kindDeltaSum, "mix", "user_id", testValues("u", 1100)))
	p.rotate()

	got := findingsFor(logs, "a", "user_id")
	require.Len(t, got, 1)
	assert.Equal(t, "HIGH", got[0]["status"])
	assert.NotContains(t, got[0], "delta", "delta is omitted while GROWING is not evaluated")
	assert.Len(t, got[0]["ottl_statements"], 2)
	assert.Empty(t, findingsFor(logs, "b", "user_id"))

	mixed := findingsFor(logs, "mix", "user_id")
	require.Len(t, mixed, 1)
	assert.Equal(t, "mixed", mixed[0]["metric_type"])
	assert.NotContains(t, mixed[0], "ottl_statements")

	summary := logs.FilterMessage("Cardinality report").All()
	require.Len(t, summary, 1)
	assert.Equal(t, zap.InfoLevel, summary[0].Level)
	assert.Equal(t, zap.WarnLevel, logs.FilterMessage("High-cardinality recommendation").All()[0].Level)
	assert.Empty(t, logs.FilterMessage("Top cardinality offender").All())
}

func TestReport_HighDenseRange(t *testing.T) {
	p, logs := newObserveProcessor(t, func(c *Config) {
		c.HighCardinalityThreshold = 20_000
		c.MaxCardinalityDeltaPerEpoch = 1_000_000
	}, nil)
	consume(t, p, kindBatch(kindGauge, "a", "user_id", testValues("u", 22_000)))
	consume(t, p, kindBatch(kindGauge, "b", "user_id", testValues("u", 18_000)))
	p.rotate()
	assert.Len(t, findingsFor(logs, "a", "user_id"), 1)
	assert.Empty(t, findingsFor(logs, "b", "user_id"))
}

// The hot path last refreshes its cached estimate at insert 1024 of 1087; the
// report must still count all 1087.
func TestReport_CountsEveryInsertSinceLastRefresh(t *testing.T) {
	p, logs := newObserveProcessor(t, func(c *Config) { c.HighCardinalityThreshold = 1050 }, nil)
	consume(t, p, kindBatch(kindDeltaSum, "m", "user_id", testValues("u", 1087)))
	p.rotate()
	got := findingsFor(logs, "m", "user_id")
	require.Len(t, got, 1)
	assert.InDelta(t, 1087, got[0]["distinct_values"], 5)
}

func growingProcessor(t *testing.T) (*cardinalityProcessor, *observer.ObservedLogs) {
	return newObserveProcessor(t, func(c *Config) {
		c.MaxCardinalityDeltaPerEpoch = 50
		c.HighCardinalityThreshold = 1_000_000
	}, nil)
}

func TestReport_GrowingNeedsFullPreviousEpoch(t *testing.T) {
	p, logs := growingProcessor(t)
	consume(t, p, kindBatch(kindDeltaSum, "m", "user_id", testValues("a", 10)))
	p.rotate() // first epoch: no previous epoch
	consume(t, p, kindBatch(kindDeltaSum, "m", "user_id", testValues("b", 300)))
	p.rotate() // second epoch: previous epoch was partial
	assert.Empty(t, findingsFor(logs, "m", "user_id"))

	consume(t, p, kindBatch(kindDeltaSum, "m", "user_id", testValues("c", 600)))
	p.rotate()
	got := findingsFor(logs, "m", "user_id")
	require.Len(t, got, 1)
	assert.Equal(t, "GROWING", got[0]["status"])
	assert.Contains(t, got[0], "delta")
}

func TestReport_GrowingNotEvaluatedAfterIdleEpoch(t *testing.T) {
	p, logs := growingProcessor(t)
	for range 3 {
		consume(t, p, kindBatch(kindDeltaSum, "m", "user_id", testValues("a", 10)))
		p.rotate()
	}
	p.rotate() // idle epoch
	consume(t, p, kindBatch(kindDeltaSum, "m", "user_id", testValues("b", 300)))
	p.rotate()
	assert.Empty(t, findingsFor(logs, "m", "user_id"))
}

func TestReport_RecreatedTrackerStartsAgain(t *testing.T) {
	p, logs := growingProcessor(t)
	consume(t, p, kindBatch(kindDeltaSum, "m", "user_id", testValues("a", 10)))
	for range 3 {
		p.rotate() // active, then two idle epochs: evicted
	}
	require.Equal(t, int64(0), p.trackerCount.Load())

	consume(t, p, kindBatch(kindDeltaSum, "m", "user_id", testValues("b", 300)))
	p.rotate()
	consume(t, p, kindBatch(kindDeltaSum, "m", "user_id", testValues("c", 600)))
	p.rotate()
	assert.Empty(t, findingsFor(logs, "m", "user_id"))
}

func TestReport_Protected(t *testing.T) {
	p, logs := newObserveProcessor(t, func(c *Config) { c.NeverDropLabels = []string{"region", "host"} }, nil)
	consume(t, p, kindBatch(kindDeltaSum, "m", "host", testValues("h", 1100)))
	p.rotate()

	got := findingsFor(logs, "m", "host")
	require.Len(t, got, 1)
	assert.Equal(t, "HIGH,PROTECTED", got[0]["status"])
	assert.NotContains(t, got[0], "ottl_statements")
	assert.Empty(t, findingsFor(logs, "m", "region"), "healthy protected label is not reported")
}

func TestReport_OrderingAndTruncation(t *testing.T) {
	p, logs := newObserveProcessor(t, func(c *Config) { c.TopOffendersCount = 2 }, nil)
	for _, m := range []struct {
		name string
		n    int
	}{{"m3", 1100}, {"m2b", 1300}, {"m1", 1500}, {"m2", 1300}} {
		consume(t, p, kindBatch(kindGauge, m.name, "user_id", testValues("u", m.n)))
	}
	p.rotate()

	findings := findingsOf(logs)
	require.Len(t, findings, 2)
	assert.Equal(t, "m1", findings[0]["metric"])
	assert.Equal(t, "m2", findings[1]["metric"]) // tie with m2b broken by name
	summary := logs.FilterMessage("Cardinality report").All()[0].ContextMap()
	assert.Equal(t, int64(4), summary["flagged"])
	assert.Equal(t, int64(2), summary["shown"])
}

func TestReport_TrackerLimitFlag(t *testing.T) {
	p, logs := newObserveProcessor(t, func(c *Config) {
		c.MaxTrackerCount = 2 // user_id and region of m1
		c.HighCardinalityThreshold = 5
	}, nil)
	consume(t, p, kindBatch(kindGauge, "m1", "user_id", testValues("u", 10)))
	consume(t, p, kindBatch(kindGauge, "m2", "user_id", testValues("u", 10)))
	p.rotate()
	consume(t, p, kindBatch(kindGauge, "m1", "user_id", testValues("u", 10)))
	p.rotate()

	summaries := logs.FilterMessage("Cardinality report").All()
	require.Len(t, summaries, 2)
	limitReached := func(e observer.LoggedEntry) bool {
		v, ok := e.ContextMap()["tracker_limit_reached"].(bool)
		require.True(t, ok)
		return v
	}
	assert.True(t, limitReached(summaries[0]))
	assert.False(t, limitReached(summaries[1]))
}

func TestReport_QuietWhenNothingFlagged(t *testing.T) {
	p, logs := newObserveProcessor(t, nil, nil)
	consume(t, p, kindBatch(kindGauge, "m", "user_id", testValues("u", 10)))
	p.rotate()
	assert.Empty(t, logs.FilterMessage("Cardinality report").All())
	assert.Len(t, logs.FilterMessage("Cardinality report: no findings").All(), 1)
}

func TestReport_NeverLogsAttributeValues(t *testing.T) {
	p, logs := newObserveProcessor(t, func(c *Config) {
		c.HighCardinalityThreshold = 5
		c.MaxTrackerCount = 2
	}, nil)
	consume(t, p, kindBatch(kindDeltaSum, "m", "user_id", testValues("secret", 50)))
	consume(t, p, kindBatch(kindDeltaSum, "m2", "user_id", testValues("secret", 50)))
	p.rotate()
	require.NotEmpty(t, findingsOf(logs))
	for _, e := range logs.All() {
		assert.NotContains(t, e.Message, "secret-")
		assert.NotContains(t, fmt.Sprint(e.ContextMap()), "secret-")
	}
}

func TestReport_DefaultFindingsSurviveLogSampling(t *testing.T) {
	p, _ := newObserveProcessor(t, func(c *Config) { c.HighCardinalityThreshold = 5 }, nil) // top_offenders_count 10
	// The collector's default sampling: per message, the first 10 per tick, then every 100th.
	core, logs := observer.New(zap.DebugLevel)
	p.logger = zap.New(zapcore.NewSamplerWithOptions(core, 10*time.Second, 10, 100))
	for i := range 15 {
		consume(t, p, kindBatch(kindGauge, fmt.Sprintf("m%d", i), "user_id", testValues("u", 10)))
	}
	p.rotate()

	summary := logs.FilterMessage("Cardinality report").All()
	require.Len(t, summary, 1)
	assert.Equal(t, int64(15), summary[0].ContextMap()["flagged"])
	assert.Equal(t, int64(10), summary[0].ContextMap()["shown"])
	assert.Len(t, findingsOf(logs), 10)
}

func TestReport_SharedMetricAcrossServices(t *testing.T) {
	p, logs := newObserveProcessor(t, nil, nil)
	md := kindBatch(kindDeltaSum, "m", "user_id", testValues("a", 600))
	md.ResourceMetrics().At(0).Resource().Attributes().PutStr("service.name", "checkout")
	other := kindBatch(kindDeltaSum, "m", "user_id", testValues("b", 600))
	other.ResourceMetrics().At(0).Resource().Attributes().PutStr("service.name", "payments")
	other.ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())
	consume(t, p, md)
	p.rotate()

	got := findingsFor(logs, "m", "user_id")
	require.Len(t, got, 1)
	assert.Contains(t, got[0]["recommendation"], "more than one service.name")
	assert.NotContains(t, got[0], "ottl_statements")
}

func TestKeepTopFindings_ZeroLimit(t *testing.T) {
	assert.Empty(t, keepTopFindings(nil, finding{distinct: 1}, 0))
}
