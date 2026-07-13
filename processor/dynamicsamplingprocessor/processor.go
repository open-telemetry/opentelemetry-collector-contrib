// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynamicsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dynamicsamplingprocessor"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dynamicsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dynamicsamplingprocessor/internal/sampler"
)

// ruleAttributeKey is the namespaced attribute set on every span in a sampled
// trace to record which rule selected it. This is an interim convention; a
// permanent semantic convention may replace it in the future.
const ruleAttributeKey = "otelcol.processor.dynamic_sampling.rule"

// triggerSource identifies which event caused a pending trace to transition
// from the buffering phase to the decision-delay phase. Reported on the
// decision-trigger counter.
type triggerSource string

const (
	triggerRootSpan     triggerSource = "root_span"
	triggerTraceTimeout triggerSource = "trace_timeout"
)

// pendingTrace holds spans accumulated for a single trace plus its arrival
// metadata. Access is guarded by dynamicSamplingProcessor.mu.
type pendingTrace struct {
	traceID     pcommon.TraceID
	spans       []ptrace.ResourceSpans
	spanCount   int
	firstSeen   time.Time
	hasRootSpan bool
	triggered   bool
}

// dynamicSamplingProcessor implements processor.Traces. It accumulates spans by
// traceID, evaluates rules after decision_wait, and forwards or drops the
// trace based on the matched sampler's rate.
type dynamicSamplingProcessor struct {
	logger    *zap.Logger
	telemetry *metadata.TelemetryBuilder
	cfg       *Config
	next      consumer.Traces

	mu      sync.Mutex
	traces  map[pcommon.TraceID]*pendingTrace
	timers  map[pcommon.TraceID]*time.Timer
	rules   []*rule
	stopped bool
	cache   *decisionCache

	wg sync.WaitGroup
}

var _ processor.Traces = (*dynamicSamplingProcessor)(nil)

// newProcessor builds the processor. The samplers within rules are not started
// here; Start is responsible for that lifecycle.
func newProcessor(set processor.Settings, cfg *Config, next consumer.Traces) (*dynamicSamplingProcessor, error) {
	tb, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	rules, err := buildRules(cfg)
	if err != nil {
		return nil, err
	}

	warnUnreachableRules(set.Logger, cfg.Rules)

	cache, err := newDecisionCache(cfg.DecisionCache)
	if err != nil {
		return nil, err
	}

	return &dynamicSamplingProcessor{
		logger:    set.Logger,
		telemetry: tb,
		cfg:       cfg,
		next:      next,
		traces:    make(map[pcommon.TraceID]*pendingTrace),
		timers:    make(map[pcommon.TraceID]*time.Timer),
		rules:     rules,
		cache:     cache,
	}, nil
}

// warnUnreachableRules logs a warning when a no-conditions catch-all rule is
// placed before other rules. The catch-all matches every trace, so any rules
// after it never run. This is almost always a misconfiguration.
func warnUnreachableRules(logger *zap.Logger, rules []RuleConfig) {
	for i := range rules {
		if len(rules[i].Conditions) == 0 && i < len(rules)-1 {
			logger.Warn(
				"catch-all rule (no conditions) is followed by other rules that will never be reached; move it to the end of the rules list",
				zap.String("rule", rules[i].Name),
				zap.Int("unreachable_rules", len(rules)-i-1),
			)
			return
		}
	}
}

func buildRules(cfg *Config) ([]*rule, error) {
	rules := make([]*rule, 0, len(cfg.Rules))
	for i := range cfg.Rules {
		rc := &cfg.Rules[i]
		s, keyFields, err := newSamplerForRule(rc)
		if err != nil {
			return nil, fmt.Errorf("rule %q: %w", rc.Name, err)
		}
		r, err := compileRule(rc, s, keyFields)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}

func newSamplerForRule(rc *RuleConfig) (sampler.Sampler, []string, error) {
	switch rc.Sampler.Type {
	case AlwaysSample:
		return sampler.NewAlwaysSample(), nil, nil
	case Deterministic:
		s, err := sampler.NewDeterministic(rc.Sampler.Deterministic.SamplingPercentage)
		return s, nil, err
	case EMADynamic:
		c := rc.Sampler.EMADynamic
		s, err := sampler.NewEMADynamic(sampler.EMADynamicConfig{
			GoalSamplingPercentage: c.GoalSamplingPercentage,
			AdjustmentInterval:     c.AdjustmentInterval,
			Weight:                 c.Weight,
			MaxKeys:                c.MaxKeys,
		})
		return s, append([]string(nil), c.KeyFields...), err
	case EMAThroughput:
		c := rc.Sampler.EMAThroughput
		s, err := sampler.NewEMAThroughput(sampler.EMAThroughputConfig{
			GoalThroughputPerSec: c.GoalThroughputPerSec,
			InitialSamplingRate:  c.InitialSamplingRate,
			AdjustmentInterval:   c.AdjustmentInterval,
			Weight:               c.Weight,
			MaxKeys:              c.MaxKeys,
		})
		return s, append([]string(nil), c.KeyFields...), err
	case WindowedThroughput:
		c := rc.Sampler.WindowedThroughput
		s, err := sampler.NewWindowedThroughput(sampler.WindowedThroughputConfig{
			GoalThroughputPerSec: c.GoalThroughputPerSec,
			UpdateFrequency:      c.UpdateFrequency,
			LookbackFrequency:    c.LookbackFrequency,
			MaxKeys:              c.MaxKeys,
		})
		return s, append([]string(nil), c.KeyFields...), err
	default:
		return nil, nil, fmt.Errorf("unknown sampler type %q", rc.Sampler.Type)
	}
}

// Capabilities reports that the processor mutates trace data (it writes the
// TraceState and rule attribute on every sampled span).
func (*dynamicSamplingProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// Start initializes the embedded samplers.
func (p *dynamicSamplingProcessor) Start(context.Context, component.Host) error {
	for _, r := range p.rules {
		if err := r.sampler.Start(); err != nil {
			return fmt.Errorf("rule %q sampler start: %w", r.name, err)
		}
	}
	return nil
}

// Shutdown cancels any pending decision timers, stops samplers, and waits for
// in-flight decisions to drain.
func (p *dynamicSamplingProcessor) Shutdown(context.Context) error {
	p.mu.Lock()
	p.stopped = true
	for id, t := range p.timers {
		if t.Stop() {
			// Stop returned true: the timer was active. The AfterFunc closure
			// will not run, so we need to release its waitgroup slot here.
			p.wg.Done()
		}
		delete(p.timers, id)
	}
	p.mu.Unlock()

	p.wg.Wait()

	var errs error
	for _, r := range p.rules {
		if err := r.sampler.Stop(); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	p.telemetry.Shutdown()
	return errs
}

// ConsumeTraces splits the incoming batch by traceID. Spans whose traceID
// already has a recorded decision in the cache short-circuit the accumulation
// path: sampled traces are forwarded with the original annotations, dropped
// traces are silently discarded. Spans for unknown traces are accumulated and
// the trace's pendingTrace is created on first appearance.
func (p *dynamicSamplingProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	now := time.Now()

	// Bucket spans by (traceID, ResourceSpans) so each span can be routed into
	// either the cache fast path (sampled-forward, dropped-discard) or the
	// pending buffer. We collect deferred work outside the lock.
	type lateSampled struct {
		traceID pcommon.TraceID
		md      cachedDecision
		td      ptrace.Traces
	}
	var lateForwards []lateSampled
	var newTraces []pcommon.TraceID
	triggered := make(map[pcommon.TraceID]struct{})

	p.mu.Lock()
	for _, rs := range td.ResourceSpans().All() {
		// Split the batch by traceID and by decision-cache status so we can
		// stamp sampled-cache hits with the original rule annotations once per
		// batch.
		pendingBuckets := make(map[pcommon.TraceID]ptrace.ResourceSpans)
		lateBuckets := make(map[pcommon.TraceID]struct {
			rs ptrace.ResourceSpans
			md cachedDecision
		})
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				id := span.TraceID()
				md, sampled, found := p.cache.lookup(id)
				if found {
					if !sampled {
						continue
					}
					b, ok := lateBuckets[id]
					if !ok {
						rsCopy := ptrace.NewResourceSpans()
						rs.Resource().CopyTo(rsCopy.Resource())
						rsCopy.SetSchemaUrl(rs.SchemaUrl())
						b = struct {
							rs ptrace.ResourceSpans
							md cachedDecision
						}{rs: rsCopy, md: md}
						lateBuckets[id] = b
					}
					dstSS := findOrAppendScopeSpans(b.rs, ss)
					span.CopyTo(dstSS.Spans().AppendEmpty())
					continue
				}
				pt, exists := p.traces[id]
				if !exists {
					if len(p.traces) >= p.cfg.NumTraces {
						for evictID := range p.traces {
							delete(p.traces, evictID)
							p.telemetry.ProcessorDynamicSamplingTracesEvicted.Add(ctx, 1)
							break
						}
					}
					pt = &pendingTrace{
						traceID:   id,
						firstSeen: now,
					}
					p.traces[id] = pt
					newTraces = append(newTraces, id)
				}
				pt.spanCount++
				if span.ParentSpanID().IsEmpty() {
					pt.hasRootSpan = true
				}
				if _, ok := pendingBuckets[id]; !ok {
					rsCopy := ptrace.NewResourceSpans()
					rs.Resource().CopyTo(rsCopy.Resource())
					rsCopy.SetSchemaUrl(rs.SchemaUrl())
					pendingBuckets[id] = rsCopy
				}
				dstSS := findOrAppendScopeSpans(pendingBuckets[id], ss)
				span.CopyTo(dstSS.Spans().AppendEmpty())
			}
		}
		for id, copied := range pendingBuckets {
			if pt, ok := p.traces[id]; ok {
				pt.spans = append(pt.spans, copied)
				if pt.hasRootSpan && !pt.triggered {
					if p.trigger(id, triggerRootSpan) {
						triggered[id] = struct{}{}
					}
				}
			}
		}
		for id, b := range lateBuckets {
			out := ptrace.NewTraces()
			b.rs.MoveTo(out.ResourceSpans().AppendEmpty())
			lateForwards = append(lateForwards, lateSampled{traceID: id, md: b.md, td: out})
		}
	}

	active := len(p.traces)
	stopped := p.stopped
	p.mu.Unlock()
	p.telemetry.ProcessorDynamicSamplingTracesActive.Record(ctx, int64(active))

	// Schedule the initial trace_timeout timer for brand-new traces that did
	// not get triggered by a root span in this same batch.
	for _, id := range newTraces {
		if _, alreadyTriggered := triggered[id]; alreadyTriggered {
			continue
		}
		traceID := id
		p.wg.Add(1)
		timer := time.AfterFunc(p.cfg.TraceTimeout, func() {
			defer p.wg.Done()
			p.mu.Lock()
			p.trigger(traceID, triggerTraceTimeout)
			p.mu.Unlock()
		})
		p.mu.Lock()
		if p.stopped {
			if timer.Stop() {
				p.wg.Done()
			}
			p.mu.Unlock()
			continue
		}
		p.timers[traceID] = timer
		p.mu.Unlock()
	}

	if stopped {
		// Drop late forwards if we're shutting down rather than push into the
		// downstream consumer.
		return nil
	}

	for _, lf := range lateForwards {
		p.stampLateBatch(ctx, lf.td, lf.md)
		if err := p.next.ConsumeTraces(ctx, lf.td); err != nil {
			p.logger.Error("forwarding late span failed", zap.Error(err), zap.Stringer("traceID", lf.traceID))
		}
	}
	return nil
}

// trigger transitions a pending trace from the buffering phase to the
// decision-delay phase. The caller must hold p.mu. Returns true if a decision
// timer was actually armed (false if the trace was already triggered, missing,
// or the processor has stopped).
func (p *dynamicSamplingProcessor) trigger(id pcommon.TraceID, source triggerSource) bool {
	if p.stopped {
		return false
	}
	pt, ok := p.traces[id]
	if !ok || pt.triggered {
		return false
	}
	pt.triggered = true
	p.telemetry.ProcessorDynamicSamplingDecisionTriggers.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("trigger", string(source))))
	// Cancel the existing timer (trace_timeout). If Stop returns false, the
	// timer's callback is already running or queued; that callback will re-enter
	// trigger under the mutex, see triggered=true, and bail without harm.
	if old, ok := p.timers[id]; ok && old.Stop() {
		p.wg.Done()
	}
	p.wg.Add(1)
	traceID := id
	p.timers[id] = time.AfterFunc(p.cfg.DecisionDelay, func() {
		defer p.wg.Done()
		p.decide(traceID)
	})
	return true
}

// stampLateBatch stamps every span in a late batch with the original rule
// attribution and ot=th TraceState.
func (p *dynamicSamplingProcessor) stampLateBatch(ctx context.Context, td ptrace.Traces, md cachedDecision) {
	for _, rs := range td.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				span.Attributes().PutStr(ruleAttributeKey, md.ruleName)
				p.updateTraceState(ctx, span, md.threshold)
			}
		}
	}
}

// findOrAppendScopeSpans returns the ScopeSpans slot in dst that matches src,
// appending an empty entry if needed. This preserves resource attributes when
// copying spans across batches.
func findOrAppendScopeSpans(dst ptrace.ResourceSpans, src ptrace.ScopeSpans) ptrace.ScopeSpans {
	for _, ss := range dst.ScopeSpans().All() {
		if ss.Scope().Name() == src.Scope().Name() && ss.Scope().Version() == src.Scope().Version() {
			return ss
		}
	}
	out := dst.ScopeSpans().AppendEmpty()
	src.Scope().CopyTo(out.Scope())
	out.SetSchemaUrl(src.SchemaUrl())
	return out
}

// decide pops a trace from the buffer, evaluates rules, and either forwards or
// drops the spans.
func (p *dynamicSamplingProcessor) decide(id pcommon.TraceID) {
	ctx := context.Background()

	p.mu.Lock()
	pt, ok := p.traces[id]
	if !ok {
		p.mu.Unlock()
		return
	}
	delete(p.traces, id)
	delete(p.timers, id)
	p.mu.Unlock()

	matchedRule, rate := p.evaluate(pt)
	if matchedRule == nil {
		// No matching rule and no catch-all: drop the trace.
		p.telemetry.ProcessorDynamicSamplingTracesDropped.Add(ctx, 1,
			metric.WithAttributes(attribute.String("rule", "unmatched")))
		p.cache.recordNotSampled(id)
		return
	}

	ruleAttr := metric.WithAttributes(attribute.String("rule", matchedRule.name))

	upstreamTh, randomness := p.readIncomingSampling(ctx, pt.spans, id)
	effectiveTh, err := effectiveThreshold(upstreamTh, rate)
	if err != nil {
		// The error path is unreachable in practice (rate is clamped to >= 1
		// before we get here, giving a valid probability), but if it did occur
		// falling back to the upstream threshold preserves whatever decision
		// upstream already made rather than dropping the trace outright.
		p.logger.Debug("effective threshold calculation failed, falling back to upstream",
			zap.Error(err), zap.Stringer("traceID", id))
		effectiveTh = upstreamTh
	}
	// Record the effective (post-composition) rate rather than the raw sampler
	// rate: under equalizing, an upstream stricter than the sampler's rate caps
	// what we emit, and the histogram should reflect that.
	p.telemetry.ProcessorDynamicSamplingDecisionSampleRate.Record(ctx, int64(effectiveTh.AdjustedCount()), ruleAttr)
	if !effectiveTh.ShouldSample(randomness) {
		p.telemetry.ProcessorDynamicSamplingTracesDropped.Add(ctx, 1, ruleAttr)
		p.cache.recordNotSampled(id)
		return
	}

	p.cache.recordSampled(id, cachedDecision{ruleName: matchedRule.name, threshold: effectiveTh})
	annotated := p.assembleTrace(ctx, pt.spans, matchedRule.name, effectiveTh)
	p.telemetry.ProcessorDynamicSamplingTracesSampled.Add(ctx, 1, ruleAttr)
	if err := p.next.ConsumeTraces(ctx, annotated); err != nil {
		p.logger.Error("forwarding sampled trace failed", zap.Error(err), zap.Stringer("traceID", id))
	}
}

// evaluate returns the first matching rule and the sample rate it produced.
func (p *dynamicSamplingProcessor) evaluate(pt *pendingTrace) (*rule, int) {
	for _, r := range p.rules {
		if !r.matches(pt.spans) {
			continue
		}
		var key string
		if len(r.keyFields) > 0 {
			key = sampler.ExtractKey(pt.spans, r.keyFields)
		}
		rate := max(r.sampler.GetSampleRate(key, pt.spanCount), 1)
		return r, rate
	}
	return nil, 0
}

// readIncomingSampling scans the accumulated spans for upstream sampling state:
// the strictest observed `ot=th` (or AlwaysSampleThreshold if none), and the
// randomness value (preferring `ot=rv` when present, falling back to the trace
// ID). The first `ot=rv` encountered in span iteration order wins; later
// occurrences are ignored so the decision is stable across a trace even if
// spans disagree. When the accumulated trace carries multiple distinct `ot=rv`
// values, a warning is logged: the tracestate contract requires rv to be
// trace-level and consistent across all spans, so divergence indicates a
// producer-side bug.
//
// Spans whose tracestate fails to parse are counted on
// ProcessorDynamicSamplingIncomingTracestateUnparseable and skipped. This
// runs on every decision path (sampled and dropped) so the counter reflects
// all observed parse failures, not just those on sampled traces.
func (p *dynamicSamplingProcessor) readIncomingSampling(ctx context.Context, spans []ptrace.ResourceSpans, id pcommon.TraceID) (sampling.Threshold, sampling.Randomness) {
	upstream := sampling.AlwaysSampleThreshold
	randomness := sampling.TraceIDToRandomness(id)
	haveRV := false
	rvLogged := false
	for _, rs := range spans {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				raw := span.TraceState().AsRaw()
				if raw == "" {
					continue
				}
				w3c, err := sampling.NewW3CTraceState(raw)
				if err != nil {
					p.telemetry.ProcessorDynamicSamplingIncomingTracestateUnparseable.Add(ctx, 1)
					continue
				}
				ot := w3c.OTelValue()
				if th, ok := ot.TValueThreshold(); ok {
					if sampling.ThresholdGreater(th, upstream) {
						upstream = th
					}
				}
				if rv, ok := ot.RValueRandomness(); ok {
					if !haveRV {
						randomness = rv
						haveRV = true
					} else if !rvLogged && rv != randomness {
						p.logger.Warn(
							"trace has spans with divergent ot=rv values; using the first observed value for the decision",
							zap.Stringer("traceID", id),
						)
						rvLogged = true
					}
				}
			}
		}
	}
	return upstream, randomness
}

// effectiveThreshold returns the threshold to use for the decision, cache
// entry, and emission under equalizing composition: the operator's rate is
// interpreted as population-relative and effective absolute keep is
// min(P_upstream, 1/rate). Concretely, if the rate-derived threshold is
// stricter than upstream we use ours; otherwise upstream caps us.
//
// This matches `processor/probabilisticsamplerprocessor` equalizing mode.
// Metric accuracy downstream is preserved because the emitted `ot=th` is the
// effective threshold; UpdateTValueWithSampling on the emit path additionally
// preserves any per-span incoming threshold that is stricter still.
func effectiveThreshold(upstream sampling.Threshold, rate int) (sampling.Threshold, error) {
	if rate <= 1 {
		return upstream, nil
	}
	ours, err := sampling.ProbabilityToThreshold(1.0 / float64(rate))
	if err != nil {
		return upstream, err
	}
	if sampling.ThresholdGreater(upstream, ours) {
		return upstream, nil
	}
	return ours, nil
}

// assembleTrace combines accumulated ResourceSpans into a single ptrace.Traces
// and stamps every span with the rule attribute and `ot=th` TraceState.
func (p *dynamicSamplingProcessor) assembleTrace(ctx context.Context, spans []ptrace.ResourceSpans, ruleName string, threshold sampling.Threshold) ptrace.Traces {
	out := ptrace.NewTraces()
	for _, rs := range spans {
		dst := out.ResourceSpans().AppendEmpty()
		rs.CopyTo(dst)
		for _, ss := range dst.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				span.Attributes().PutStr(ruleAttributeKey, ruleName)
				p.updateTraceState(ctx, span, threshold)
			}
		}
	}
	return out
}

// updateTraceState parses the existing TraceState, updates the OTel T-value to
// reflect the sampling threshold, and serializes the result back onto the
// span. `UpdateTValueWithSampling` refuses to lower a stricter incoming
// threshold; that is spec-correct and treated as a silent no-op. Parse
// failures on the incoming tracestate are counted in readIncomingSampling
// (which runs on every decision path); here we silently skip the span.
func (*dynamicSamplingProcessor) updateTraceState(_ context.Context, span ptrace.Span, threshold sampling.Threshold) {
	raw := span.TraceState().AsRaw()
	w3c, err := sampling.NewW3CTraceState(raw)
	if err != nil {
		return
	}
	if err := w3c.OTelValue().UpdateTValueWithSampling(threshold); err != nil {
		return
	}
	var sb strings.Builder
	if err := w3c.Serialize(&sb); err != nil {
		return
	}
	span.TraceState().FromRaw(sb.String())
}
