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

// pendingTrace holds spans accumulated for a single trace plus its arrival
// metadata. Access is guarded by dynamicSamplingProcessor.mu.
type pendingTrace struct {
	traceID    pcommon.TraceID
	spans      []ptrace.ResourceSpans
	spanCount  int
	firstSeen  time.Time
	decisionAt time.Time
}

// dynamicSamplingProcessor implements processor.Traces. It accumulates spans by
// traceID, evaluates rules after decision_wait, and forwards or drops the
// trace based on the matched sampler's rate.
type dynamicSamplingProcessor struct {
	logger    *zap.Logger
	telemetry *metadata.TelemetryBuilder
	cfg       *Config
	next      consumer.Traces

	mu       sync.Mutex
	traces   map[pcommon.TraceID]*pendingTrace
	timers   map[pcommon.TraceID]*time.Timer
	rules    []*rule
	stopped  bool

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

	return &dynamicSamplingProcessor{
		logger:    set.Logger,
		telemetry: tb,
		cfg:       cfg,
		next:      next,
		traces:    make(map[pcommon.TraceID]*pendingTrace),
		timers:    make(map[pcommon.TraceID]*time.Timer),
		rules:     rules,
	}, nil
}

func buildRules(cfg *Config) ([]*rule, error) {
	rules := make([]*rule, 0, len(cfg.Rules))
	for _, rc := range cfg.Rules {
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

func newSamplerForRule(rc RuleConfig) (sampler.Sampler, []string, error) {
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

// Start initialises the embedded samplers.
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

// ConsumeTraces splits the incoming batch by traceID, appends spans to the
// accumulation buffer, and schedules a decision after decision_wait.
func (p *dynamicSamplingProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	now := time.Now()
	type pendingDecision struct {
		traceID pcommon.TraceID
		fireAt  time.Time
	}
	var newDecisions []pendingDecision

	p.mu.Lock()
	for _, rs := range td.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				id := span.TraceID()
				pt, exists := p.traces[id]
				if !exists {
					if len(p.traces) >= p.cfg.NumTraces {
						// Evict an arbitrary trace to make room. A future
						// enhancement could LRU-evict the oldest.
						for evictID := range p.traces {
							delete(p.traces, evictID)
							p.telemetry.ProcessorDynamicSamplingTracesEvicted.Add(ctx, 1)
							break
						}
					}
					pt = &pendingTrace{
						traceID:    id,
						firstSeen:  now,
						decisionAt: now.Add(p.cfg.DecisionWait),
					}
					p.traces[id] = pt
					newDecisions = append(newDecisions, pendingDecision{traceID: id, fireAt: pt.decisionAt})
				}
				pt.spanCount++
			}
		}
	}
	// Copy the ResourceSpans payload once per incoming batch so we can release
	// the lock without holding the upstream batch reference. We re-scan in a
	// second pass to bucket the spans onto their pending entries.
	for _, rs := range td.ResourceSpans().All() {
		// Bucket spans by traceID for fast attach.
		buckets := make(map[pcommon.TraceID]ptrace.ResourceSpans)
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				id := span.TraceID()
				if _, ok := buckets[id]; !ok {
					rsCopy := ptrace.NewResourceSpans()
					rs.Resource().CopyTo(rsCopy.Resource())
					rsCopy.SetSchemaUrl(rs.SchemaUrl())
					buckets[id] = rsCopy
				}
				dstSS := findOrAppendScopeSpans(buckets[id], ss)
				span.CopyTo(dstSS.Spans().AppendEmpty())
			}
		}
		for id, copied := range buckets {
			if pt, ok := p.traces[id]; ok {
				pt.spans = append(pt.spans, copied)
			}
		}
	}
	active := len(p.traces)
	p.mu.Unlock()
	p.telemetry.ProcessorDynamicSamplingTracesActive.Record(ctx, int64(active))

	// Schedule any newly-tracked traces for decision. We use time.AfterFunc so
	// the goroutine count stays proportional to active traces, not span volume.
	for _, d := range newDecisions {
		id := d.traceID
		delay := max(time.Until(d.fireAt), 0)
		p.wg.Add(1)
		timer := time.AfterFunc(delay, func() {
			defer p.wg.Done()
			p.decide(id)
		})
		p.mu.Lock()
		// Bail if shutdown raced us between scheduling and registering. Stop
		// the timer and release the waitgroup slot here.
		if p.stopped {
			if timer.Stop() {
				p.wg.Done()
			}
			p.mu.Unlock()
			continue
		}
		p.timers[id] = timer
		p.mu.Unlock()
	}
	return nil
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
		return
	}

	ruleAttr := metric.WithAttributes(attribute.String("rule", matchedRule.name))
	p.telemetry.ProcessorDynamicSamplingDecisionSampleRate.Record(ctx, int64(rate), ruleAttr)

	if !shouldSample(id, rate) {
		p.telemetry.ProcessorDynamicSamplingTracesDropped.Add(ctx, 1, ruleAttr)
		return
	}

	annotated := assembleTrace(pt.spans, matchedRule.name, rate)
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

// shouldSample returns true when the trace should be kept at the given rate.
// rate <= 1 always keeps the trace. Otherwise the decision is deterministic
// using consistent probability sampling against the traceID randomness.
func shouldSample(id pcommon.TraceID, rate int) bool {
	if rate <= 1 {
		return true
	}
	probability := 1.0 / float64(rate)
	threshold, err := sampling.ProbabilityToThreshold(probability)
	if err != nil {
		return false
	}
	return threshold.ShouldSample(sampling.TraceIDToRandomness(id))
}

// assembleTrace combines accumulated ResourceSpans into a single ptrace.Traces
// and stamps every span with the rule attribute and `ot=th` TraceState.
func assembleTrace(spans []ptrace.ResourceSpans, ruleName string, rate int) ptrace.Traces {
	out := ptrace.NewTraces()
	threshold, _ := sampling.ProbabilityToThreshold(1.0 / float64(rate))
	for _, rs := range spans {
		dst := out.ResourceSpans().AppendEmpty()
		rs.CopyTo(dst)
		for _, ss := range dst.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				span.Attributes().PutStr(ruleAttributeKey, ruleName)
				updateTraceState(span, threshold)
			}
		}
	}
	return out
}

// updateTraceState parses the existing TraceState, updates the OTel T-value to
// reflect the sampling threshold, and serialises the result back onto the
// span. Failures fall through silently so we never block a sampled trace on a
// malformed upstream TraceState.
func updateTraceState(span ptrace.Span, threshold sampling.Threshold) {
	w3c, err := sampling.NewW3CTraceState(span.TraceState().AsRaw())
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
