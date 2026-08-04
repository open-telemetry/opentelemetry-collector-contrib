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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dynamicsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dynamicsamplingprocessor/internal/sampler"
)

// rootSpanConditionRuleLabel is the sentinel value stamped on the
// ProcessorDynamicSamplingOttlEvalErrors counter's `rule` label when the
// root_span_condition expression fails to evaluate. Chosen to sort separately
// from user-defined rule names and to be recognizable as processor-owned.
const rootSpanConditionRuleLabel = "_root_span_condition"

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
	triggerEviction     triggerSource = "eviction"
)

// evictionRuleLabel is the sentinel `rule` label used on decision metrics for
// traces decided by the probabilistic eviction policy, which bypasses rule
// evaluation entirely. Same convention as rootSpanConditionRuleLabel.
const evictionRuleLabel = "_eviction"

// Precomputed measurement options for fixed label values, so hot paths avoid
// rebuilding attribute sets per call.
var (
	unmatchedRuleAttr       = metric.WithAttributes(attribute.String("rule", "unmatched"))
	evictionRuleAttr        = metric.WithAttributes(attribute.String("rule", evictionRuleLabel))
	triggerRootSpanAttr     = metric.WithAttributes(attribute.String("trigger", string(triggerRootSpan)))
	triggerTraceTimeoutAttr = metric.WithAttributes(attribute.String("trigger", string(triggerTraceTimeout)))
	triggerEvictionAttr     = metric.WithAttributes(attribute.String("trigger", string(triggerEviction)))
)

func triggerAttr(source triggerSource) metric.MeasurementOption {
	switch source {
	case triggerRootSpan:
		return triggerRootSpanAttr
	case triggerTraceTimeout:
		return triggerTraceTimeoutAttr
	default:
		return triggerEvictionAttr
	}
}

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
	// arrival records traceIDs in first-seen order so eviction can pick the
	// oldest pending trace. Entries are appended exactly once per trace and
	// lazily skipped when the trace has already been decided (popped ids are
	// checked against the traces map). Guarded by mu.
	arrival []pcommon.TraceID

	rootSpanCond         *ottl.Condition[*ottlspan.TransformContext]
	rootSpanCondEvalErrs metric.Int64Counter
	rootSpanCondAttrSet  metric.MeasurementOption
	// rootSpanFastPath is set when the effective root-span condition is the
	// default IsRootSpan(), letting the per-span check skip OTTL entirely.
	rootSpanFastPath bool

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

	rules, err := buildRules(cfg, set.TelemetrySettings, tb.ProcessorDynamicSamplingOttlEvalErrors)
	if err != nil {
		return nil, err
	}

	warnUnreachableRules(set.Logger, cfg.Rules)

	rootSpanCond, err := compileRootSpanCondition(cfg.effectiveRootSpanCondition(), set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	cache, err := newDecisionCache(cfg.DecisionCache)
	if err != nil {
		return nil, err
	}

	return &dynamicSamplingProcessor{
		logger:               set.Logger,
		telemetry:            tb,
		cfg:                  cfg,
		next:                 next,
		traces:               make(map[pcommon.TraceID]*pendingTrace),
		timers:               make(map[pcommon.TraceID]*time.Timer),
		rules:                rules,
		cache:                cache,
		rootSpanCond:         rootSpanCond,
		rootSpanCondEvalErrs: tb.ProcessorDynamicSamplingOttlEvalErrors,
		rootSpanCondAttrSet:  metric.WithAttributes(attribute.String("rule", rootSpanConditionRuleLabel)),
		rootSpanFastPath:     cfg.effectiveRootSpanCondition() == defaultRootSpanCondition,
	}, nil
}

// compileRootSpanCondition parses the operator-supplied (or defaulted) OTTL
// expression once and returns the compiled condition for per-span evaluation.
// The same ottlspan parser configuration is used as for rule conditions so the
// two share the same function set and path-context conventions.
func compileRootSpanCondition(expr string, settings component.TelemetrySettings) (*ottl.Condition[*ottlspan.TransformContext], error) {
	parser, err := ottlspan.NewParser(filterottl.StandardSpanFuncs(), settings, ottlspan.EnablePathContextNames())
	if err != nil {
		return nil, fmt.Errorf("root_span_condition: build OTTL parser: %w", err)
	}
	cond, err := parser.ParseCondition(expr)
	if err != nil {
		return nil, fmt.Errorf("root_span_condition: %w", err)
	}
	return cond, nil
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

func buildRules(cfg *Config, settings component.TelemetrySettings, evalErrs metric.Int64Counter) ([]*rule, error) {
	rules := make([]*rule, 0, len(cfg.Rules))
	for i := range cfg.Rules {
		rc := &cfg.Rules[i]
		s, keyFields, err := newSamplerForRule(rc)
		if err != nil {
			return nil, fmt.Errorf("rule %q: %w", rc.Name, err)
		}
		r, err := compileRule(rc, s, keyFields, settings, evalErrs)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}

func newSamplerForRule(rc *RuleConfig) (sampler.Sampler, []string, error) {
	sc := rc.Sampler
	switch sc.Type {
	case AlwaysSample:
		return sampler.NewAlwaysSample(), nil, nil
	case Deterministic:
		s, err := sampler.NewDeterministic(sc.SamplingPercentage)
		return s, nil, err
	case EMADynamic:
		s, err := sampler.NewEMADynamic(sampler.EMADynamicConfig{
			GoalSamplingPercentage: sc.GoalSamplingPercentage,
			AdjustmentInterval:     sc.AdjustmentInterval,
			Weight:                 sc.Weight,
			MaxKeys:                sc.MaxKeys,
		})
		return s, append([]string(nil), sc.KeyAttributes...), err
	case EMAThroughput:
		s, err := sampler.NewEMAThroughput(sampler.EMAThroughputConfig{
			GoalThroughputPerSec: sc.GoalThroughputPerSec,
			AdjustmentInterval:   sc.AdjustmentInterval,
			Weight:               sc.Weight,
			MaxKeys:              sc.MaxKeys,
		})
		return s, append([]string(nil), sc.KeyAttributes...), err
	case WindowedThroughput:
		s, err := sampler.NewWindowedThroughput(sampler.WindowedThroughputConfig{
			GoalThroughputPerSec: float64(sc.GoalThroughputPerSec),
			UpdateFrequency:      sc.UpdateFrequency,
			LookbackFrequency:    sc.LookbackFrequency,
			MaxKeys:              sc.MaxKeys,
		})
		return s, append([]string(nil), sc.KeyAttributes...), err
	default:
		return nil, nil, fmt.Errorf("unknown sampler type %q", sc.Type)
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
	var evicted []*pendingTrace
	triggered := make(map[pcommon.TraceID]struct{})
	// dropped memoizes not-sampled cache hits so repeat late spans of a
	// dropped trace skip the LRU lookup within a batch.
	dropped := make(map[pcommon.TraceID]struct{})

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
				// The pending map is checked before the decision cache: spans
				// of buffered traces (the common case) would otherwise pay
				// two guaranteed-miss LRU lookups, and the caches never hold
				// a trace that is still pending.
				pt, exists := p.traces[id]
				if !exists {
					if b, ok := lateBuckets[id]; ok {
						dstSS := findOrAppendScopeSpans(b.rs, ss)
						span.CopyTo(dstSS.Spans().AppendEmpty())
						continue
					}
					if _, ok := dropped[id]; ok {
						continue
					}
					md, sampled, found := p.cache.lookup(id)
					if found {
						if !sampled {
							if dropped == nil {
								dropped = make(map[pcommon.TraceID]struct{})
							}
							dropped[id] = struct{}{}
							continue
						}
						rsCopy := ptrace.NewResourceSpans()
						rs.Resource().CopyTo(rsCopy.Resource())
						rsCopy.SetSchemaUrl(rs.SchemaUrl())
						b := struct {
							rs ptrace.ResourceSpans
							md cachedDecision
						}{rs: rsCopy, md: md}
						lateBuckets[id] = b
						dstSS := findOrAppendScopeSpans(b.rs, ss)
						span.CopyTo(dstSS.Spans().AppendEmpty())
						continue
					}
					pt = &pendingTrace{
						traceID:   id,
						firstSeen: now,
					}
					p.traces[id] = pt
					p.arrival = append(p.arrival, id)
					newTraces = append(newTraces, id)
				}
				pt.spanCount++
				// hasRootSpan only matters until the trace triggers, so skip
				// the condition for spans arriving during decision_delay.
				if !pt.hasRootSpan && !pt.triggered && p.evalRootSpanCondition(ctx, rs, ss, span) {
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

	// Enforce the buffer cap after the whole batch is attached, so an evicted
	// trace always carries every span seen so far (evicting mid-batch would
	// decide it without the spans still sitting in this batch's buckets, and
	// a trace evicted then re-seen within one batch would split in two). The
	// buffer may transiently exceed NumTraces by the number of new traces in
	// a single batch.
	for len(p.traces) > p.cfg.NumTraces {
		ev := p.evictOldestLocked(ctx)
		if ev == nil {
			break
		}
		evicted = append(evicted, ev)
	}

	// The arrival list is only consumed by eviction, so during normal
	// operation (buffer never full) entries for decided traces would
	// accumulate forever. Compact it once stale entries dominate; amortized
	// O(1) per trace.
	if len(p.arrival) > arrivalCompactionFactor*len(p.traces)+arrivalCompactionFloor {
		p.compactArrivalLocked()
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
		// Skip storing the timer if we're shutting down, if the trace was
		// evicted within this same batch (created and then displaced before
		// its timer was armed), or if a timer already exists (trace evicted
		// and re-created within one batch lists the id in newTraces twice).
		_, stillPending := p.traces[traceID]
		_, hasTimer := p.timers[traceID]
		if p.stopped || !stillPending || hasTimer {
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
		// Drop late forwards and evicted traces if we're shutting down rather
		// than push into the downstream consumer.
		return nil
	}

	// Decide evicted traces outside the lock. Bounded work: at most one
	// eviction per new trace in the batch.
	for _, pt := range evicted {
		p.decideEvicted(ctx, pt)
	}

	for _, lf := range lateForwards {
		p.stampLateBatch(ctx, lf.td, lf.md)
		if err := p.next.ConsumeTraces(ctx, lf.td); err != nil {
			p.logger.Error("forwarding late span failed", zap.Error(err), zap.Stringer("traceID", lf.traceID))
		}
	}
	return nil
}

// Compaction tuning for the arrival list. Not configurable: entries are
// 16-byte traceIDs, so the factor bounds the list's memory at a fraction of a
// percent of the buffered span data it shadows, and the floor only prevents
// thrashing on a near-empty buffer. Compaction cost is amortized O(1) per
// trace at any factor > 1.
const (
	arrivalCompactionFactor = 2
	arrivalCompactionFloor  = 1024
)

// compactArrivalLocked rebuilds the arrival order in place, dropping ids
// whose traces have already been decided, preserving first-seen order for the
// rest. The caller must hold p.mu.
func (p *dynamicSamplingProcessor) compactArrivalLocked() {
	live := p.arrival[:0]
	for _, id := range p.arrival {
		if _, ok := p.traces[id]; ok {
			live = append(live, id)
		}
	}
	p.arrival = live
}

// evictOldestLocked removes and returns the oldest pending trace, canceling
// its trace_timeout timer. Returns nil if nothing is evictable. The caller
// must hold p.mu.
func (p *dynamicSamplingProcessor) evictOldestLocked(ctx context.Context) *pendingTrace {
	for len(p.arrival) > 0 {
		id := p.arrival[0]
		p.arrival = p.arrival[1:]
		// Reclaim the backing array once the live window is a small fraction
		// of it, so popped head entries don't pin memory indefinitely.
		if cap(p.arrival) > 1024 && len(p.arrival)*4 < cap(p.arrival) {
			p.arrival = append([]pcommon.TraceID(nil), p.arrival...)
		}
		pt, ok := p.traces[id]
		if !ok {
			// Already decided; its arrival entry was left behind by design.
			continue
		}
		delete(p.traces, id)
		if t, ok := p.timers[id]; ok {
			if t.Stop() {
				p.wg.Done()
			}
			delete(p.timers, id)
		}
		p.telemetry.ProcessorDynamicSamplingTracesEvicted.Add(ctx, 1)
		return pt
	}
	return nil
}

// evalRootSpanCondition returns true if the operator-configured OTTL boolean
// expression matches the given span. Evaluation errors are counted on
// ProcessorDynamicSamplingOttlEvalErrors under the sentinel
// rootSpanConditionRuleLabel so operators can distinguish them from rule
// condition errors, and treated as a non-match so a broken expression can't
// silently trigger every trace.
//
// When the condition is the default IsRootSpan() the OTTL machinery is
// bypassed: the function is exactly ParentSpanID().IsEmpty() and cannot
// error, and this runs per span inside the ConsumeTraces critical section,
// so the ~40x cheaper direct check matters (15ns vs 0.4ns per span).
func (p *dynamicSamplingProcessor) evalRootSpanCondition(ctx context.Context, rs ptrace.ResourceSpans, ss ptrace.ScopeSpans, span ptrace.Span) bool {
	if p.rootSpanFastPath {
		return span.ParentSpanID().IsEmpty()
	}
	tCtx := ottlspan.NewTransformContextPtr(rs, ss, span)
	ok, err := p.rootSpanCond.Eval(ctx, tCtx)
	tCtx.Close()
	if err != nil {
		if p.rootSpanCondEvalErrs != nil {
			p.rootSpanCondEvalErrs.Add(ctx, 1, p.rootSpanCondAttrSet)
		}
		p.logger.Debug("root_span_condition evaluation failed", zap.Error(err))
		return false
	}
	return ok
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
	p.telemetry.ProcessorDynamicSamplingDecisionTriggers.Add(context.Background(), 1, triggerAttr(source))
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
	emptyTS := serializedEmptyTraceState(md.threshold)
	for _, rs := range td.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				span.Attributes().PutStr(ruleAttributeKey, md.ruleName)
				p.updateTraceState(ctx, span, md.threshold, emptyTS)
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

	p.decideTrace(ctx, pt)
}

// decideTrace evaluates rules for an already-popped pending trace and either
// forwards or drops its spans. Shared by the timer-driven decide path and the
// evaluate eviction policy.
func (p *dynamicSamplingProcessor) decideTrace(ctx context.Context, pt *pendingTrace) {
	matchedRule, rate := p.evaluate(ctx, pt)
	if matchedRule == nil {
		// No matching rule and no catch-all: drop the trace.
		p.telemetry.ProcessorDynamicSamplingTracesDropped.Add(ctx, 1, unmatchedRuleAttr)
		p.cache.recordNotSampled(pt.traceID)
		return
	}

	ruleAttr := matchedRule.ruleAttrSet

	upstreamTh, randomness := p.readIncomingSampling(ctx, pt.spans, pt.traceID)
	effectiveTh, err := effectiveThreshold(upstreamTh, rate)
	if err != nil {
		// The error path is unreachable in practice (rate is clamped to >= 1
		// before we get here, giving a valid probability), but if it did occur
		// falling back to the upstream threshold preserves whatever decision
		// upstream already made rather than dropping the trace outright.
		p.logger.Debug("effective threshold calculation failed, falling back to upstream",
			zap.Error(err), zap.Stringer("traceID", pt.traceID))
		effectiveTh = upstreamTh
	}
	p.finishDecision(ctx, pt, matchedRule.name, ruleAttr, effectiveTh, randomness)
}

// finishDecision applies an already-composed effective threshold: records the
// sample-rate histogram, performs the keep/drop check, updates the decision
// cache, and forwards sampled traces. Shared by every decision path.
func (p *dynamicSamplingProcessor) finishDecision(ctx context.Context, pt *pendingTrace, ruleName string, ruleAttr metric.MeasurementOption, effectiveTh sampling.Threshold, randomness sampling.Randomness) {
	// Record the effective (post-composition) rate rather than the raw sampler
	// rate: under equalizing, an upstream stricter than the sampler's rate caps
	// what we emit, and the histogram should reflect that.
	p.telemetry.ProcessorDynamicSamplingDecisionSampleRate.Record(ctx, int64(effectiveTh.AdjustedCount()), ruleAttr)
	if !effectiveTh.ShouldSample(randomness) {
		p.telemetry.ProcessorDynamicSamplingTracesDropped.Add(ctx, 1, ruleAttr)
		p.cache.recordNotSampled(pt.traceID)
		return
	}

	p.cache.recordSampled(pt.traceID, cachedDecision{ruleName: ruleName, threshold: effectiveTh})
	annotated := p.assembleTrace(ctx, pt.spans, ruleName, effectiveTh)
	p.telemetry.ProcessorDynamicSamplingTracesSampled.Add(ctx, 1, ruleAttr)
	if err := p.next.ConsumeTraces(ctx, annotated); err != nil {
		p.logger.Error("forwarding sampled trace failed", zap.Error(err), zap.Stringer("traceID", pt.traceID))
	}
}

// decideEvicted decides a trace displaced by buffer pressure, per the
// configured eviction policy. The trace may be incomplete; decision_delay is
// deliberately skipped because there is no room to wait.
func (p *dynamicSamplingProcessor) decideEvicted(ctx context.Context, pt *pendingTrace) {
	p.telemetry.ProcessorDynamicSamplingDecisionTriggers.Add(ctx, 1, triggerEvictionAttr)
	if p.cfg.Eviction.Policy == EvictionProbabilistic {
		p.decideEvictedProbabilistic(ctx, pt)
		return
	}
	p.decideTrace(ctx, pt)
}

// decideEvictedProbabilistic sheds an evicted trace with constant-time work:
// no rule evaluation, just the configured probability composed with any
// upstream threshold. Kept traces still carry a correct ot=th, so downstream
// weighting remains accurate even under pressure.
func (p *dynamicSamplingProcessor) decideEvictedProbabilistic(ctx context.Context, pt *pendingTrace) {
	evAttr := evictionRuleAttr
	upstreamTh, randomness := p.readIncomingSampling(ctx, pt.spans, pt.traceID)
	effectiveTh := upstreamTh
	if ours, err := sampling.ProbabilityToThreshold(p.cfg.Eviction.SamplingPercentage / 100); err == nil {
		if !sampling.ThresholdGreater(upstreamTh, ours) {
			effectiveTh = ours
		}
	} else {
		// Unreachable with a validated config; fall back to the upstream
		// threshold rather than dropping outright.
		p.logger.Debug("eviction threshold calculation failed, falling back to upstream",
			zap.Error(err), zap.Stringer("traceID", pt.traceID))
	}
	p.finishDecision(ctx, pt, evictionRuleLabel, evAttr, effectiveTh, randomness)
}

// evaluate returns the first matching rule and the sample rate it produced.
func (p *dynamicSamplingProcessor) evaluate(ctx context.Context, pt *pendingTrace) (*rule, int) {
	for _, r := range p.rules {
		if !r.matches(ctx, pt.spans) {
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
	emptyTS := serializedEmptyTraceState(threshold)
	out := ptrace.NewTraces()
	for _, rs := range spans {
		dst := out.ResourceSpans().AppendEmpty()
		rs.CopyTo(dst)
		for _, ss := range dst.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				span.Attributes().PutStr(ruleAttributeKey, ruleName)
				p.updateTraceState(ctx, span, threshold, emptyTS)
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
// emptyTS is the precomputed serialized tracestate to apply when the span has
// no incoming tracestate; the threshold is trace-constant, so callers compute
// it once per decision instead of running the parse/update/serialize round
// trip per span. It must equal what that round trip produces for empty input.
func (*dynamicSamplingProcessor) updateTraceState(_ context.Context, span ptrace.Span, threshold sampling.Threshold, emptyTS string) {
	raw := span.TraceState().AsRaw()
	if raw == "" {
		span.TraceState().FromRaw(emptyTS)
		return
	}
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

// serializedEmptyTraceState returns the tracestate emitted for spans with no
// incoming tracestate, byte-identical to serializing a parsed empty state
// after UpdateTValueWithSampling(threshold).
func serializedEmptyTraceState(threshold sampling.Threshold) string {
	return "ot=th:" + threshold.TValue()
}
