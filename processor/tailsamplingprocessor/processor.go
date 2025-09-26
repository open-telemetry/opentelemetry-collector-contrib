// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/timeutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/cache"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/idbatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/telemetry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/tracelimiter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

// policy combines a sampling policy evaluator with the destinations to be
// used for that policy.
type policy struct {
	// name used to identify this policy instance.
	name string
	// evaluator that decides if a trace is sampled or not by this policy instance.
	evaluator samplingpolicy.Evaluator
	// attribute to use in the telemetry to denote the policy.
	attribute metric.MeasurementOption
}

// tailSamplingSpanProcessor handles the incoming trace data and uses the given sampling
// policy to sample traces.
type tailSamplingSpanProcessor struct {
	ctx context.Context

	set       processor.Settings
	telemetry *metadata.TelemetryBuilder
	logger    *zap.Logger

	nextConsumer       consumer.Traces
	maxNumTraces       uint64
	policies           []*policy
	idToTrace          sync.Map
	policyTicker       timeutils.TTicker
	tickerFrequency    time.Duration
	decisionBatcher    idbatcher.Batcher
	sampledIDCache     cache.Cache[bool]
	nonSampledIDCache  cache.Cache[bool]
	traceLimiter       traceLimiter
	numTracesOnMap     *atomic.Uint64
	recordPolicy       bool
	setPolicyMux       sync.Mutex
	pendingPolicy      []PolicyCfg
	sampleOnFirstMatch bool

	host component.Host
}

type traceLimiter interface {
	AcceptTrace(context.Context, pcommon.TraceID, time.Time)
	OnDeleteTrace()
}

// spanAndScope a structure for holding information about span and its instrumentation scope.
// required for preserving the instrumentation library information while sampling.
// We use pointers there to fast find the span in the map.
type spanAndScope struct {
	span                 *ptrace.Span
	instrumentationScope *pcommon.InstrumentationScope
}

var (
	attrDecisionSampled    = metric.WithAttributes(attribute.String("sampled", "true"), attribute.String("decision", "sampled"))
	attrDecisionNotSampled = metric.WithAttributes(attribute.String("sampled", "false"), attribute.String("decision", "not_sampled"))
	attrDecisionDropped    = metric.WithAttributes(attribute.String("sampled", "false"), attribute.String("decision", "dropped"))
	decisionToAttributes   = map[samplingpolicy.Decision]metric.MeasurementOption{
		samplingpolicy.Sampled:          attrDecisionSampled,
		samplingpolicy.NotSampled:       attrDecisionNotSampled,
		samplingpolicy.InvertNotSampled: attrDecisionNotSampled,
		samplingpolicy.InvertSampled:    attrDecisionSampled,
		samplingpolicy.Dropped:          attrDecisionDropped,
	}

	attrSampledTrue  = metric.WithAttributes(attribute.String("sampled", "true"))
	attrSampledFalse = metric.WithAttributes(attribute.String("sampled", "false"))
)

type Option func(*tailSamplingSpanProcessor)

// newTracesProcessor returns a processor.TracesProcessor that will perform tail sampling according to the given
// configuration.
func newTracesProcessor(ctx context.Context, set processor.Settings, nextConsumer consumer.Traces, cfg Config) (processor.Traces, error) {
	telemetrySettings := set.TelemetrySettings
	telemetry, err := metadata.NewTelemetryBuilder(telemetrySettings)
	if err != nil {
		return nil, err
	}
	nopCache := cache.NewNopDecisionCache[bool]()
	sampledDecisions := nopCache
	nonSampledDecisions := nopCache
	if cfg.DecisionCache.SampledCacheSize > 0 {
		sampledDecisions, err = cache.NewLRUDecisionCache[bool](cfg.DecisionCache.SampledCacheSize)
		if err != nil {
			return nil, err
		}
	}
	if cfg.DecisionCache.NonSampledCacheSize > 0 {
		nonSampledDecisions, err = cache.NewLRUDecisionCache[bool](cfg.DecisionCache.NonSampledCacheSize)
		if err != nil {
			return nil, err
		}
	}

	tsp := &tailSamplingSpanProcessor{
		ctx:                ctx,
		set:                set,
		telemetry:          telemetry,
		nextConsumer:       nextConsumer,
		maxNumTraces:       cfg.NumTraces,
		sampledIDCache:     sampledDecisions,
		nonSampledIDCache:  nonSampledDecisions,
		logger:             telemetrySettings.Logger,
		numTracesOnMap:     &atomic.Uint64{},
		sampleOnFirstMatch: cfg.SampleOnFirstMatch,
	}
	tsp.policyTicker = &timeutils.PolicyTicker{OnTickFunc: tsp.samplingPolicyOnTick}

	var tl traceLimiter
	if cfg.BlockOnOverflow {
		tl = tracelimiter.NewBlockingTracesLimiter(cfg.NumTraces)
	} else {
		tl = tracelimiter.NewDropOldTracesLimiter(cfg.NumTraces, tsp.dropTrace)
	}
	tsp.traceLimiter = tl

	for _, opt := range cfg.Options {
		opt(tsp)
	}

	if tsp.tickerFrequency == 0 {
		tsp.tickerFrequency = time.Second
	}

	if tsp.policies == nil {
		tsp.pendingPolicy = cfg.PolicyCfgs
	}

	if tsp.decisionBatcher == nil {
		// this will start a goroutine in the background, so we run it only if everything went
		// well in creating the policies
		numDecisionBatches := math.Max(1, cfg.DecisionWait.Seconds())
		inBatcher, err := idbatcher.New(uint64(numDecisionBatches), cfg.ExpectedNewTracesPerSec, uint64(2*runtime.NumCPU()))
		if err != nil {
			return nil, err
		}
		tsp.decisionBatcher = inBatcher
	}

	return tsp, nil
}

// withDecisionBatcher sets the batcher used to batch trace IDs for policy evaluation.
func withDecisionBatcher(batcher idbatcher.Batcher) Option {
	return func(tsp *tailSamplingSpanProcessor) {
		tsp.decisionBatcher = batcher
	}
}

// withPolicies sets the sampling policies to be used by the processor.
func withPolicies(policies []*policy) Option {
	return func(tsp *tailSamplingSpanProcessor) {
		tsp.policies = policies
	}
}

// withTickerFrequency sets the frequency at which the processor will evaluate the sampling policies.
func withTickerFrequency(frequency time.Duration) Option {
	return func(tsp *tailSamplingSpanProcessor) {
		tsp.tickerFrequency = frequency
	}
}

// WithSampledDecisionCache sets the cache which the processor uses to store recently sampled trace IDs.
func WithSampledDecisionCache(c cache.Cache[bool]) Option {
	return func(tsp *tailSamplingSpanProcessor) {
		tsp.sampledIDCache = c
	}
}

// WithNonSampledDecisionCache sets the cache which the processor uses to store recently non-sampled trace IDs.
func WithNonSampledDecisionCache(c cache.Cache[bool]) Option {
	return func(tsp *tailSamplingSpanProcessor) {
		tsp.nonSampledIDCache = c
	}
}

func withRecordPolicy() Option {
	return func(tsp *tailSamplingSpanProcessor) {
		tsp.recordPolicy = true
	}
}

func getPolicyEvaluator(settings component.TelemetrySettings, cfg *PolicyCfg, policyExtensions map[string]samplingpolicy.Extension) (samplingpolicy.Evaluator, error) {
	switch cfg.Type {
	case Composite:
		return getNewCompositePolicy(settings, &cfg.CompositeCfg, policyExtensions)
	case And:
		return getNewAndPolicy(settings, &cfg.AndCfg, policyExtensions)
	case Drop:
		return getNewDropPolicy(settings, &cfg.DropCfg, policyExtensions)
	default:
		return getSharedPolicyEvaluator(settings, &cfg.sharedPolicyCfg, policyExtensions)
	}
}

func getSharedPolicyEvaluator(settings component.TelemetrySettings, cfg *sharedPolicyCfg, policyExtensions map[string]samplingpolicy.Extension) (samplingpolicy.Evaluator, error) {
	settings.Logger = settings.Logger.With(zap.Any("policy", cfg.Type))

	switch cfg.Type {
	case AlwaysSample:
		return sampling.NewAlwaysSample(settings), nil
	case Latency:
		lfCfg := cfg.LatencyCfg
		return sampling.NewLatency(settings, lfCfg.ThresholdMs, lfCfg.UpperThresholdMs), nil
	case NumericAttribute:
		nafCfg := cfg.NumericAttributeCfg
		var minValuePtr, maxValuePtr *int64
		if nafCfg.MinValue != 0 {
			minValuePtr = &nafCfg.MinValue
		}
		if nafCfg.MaxValue != 0 {
			maxValuePtr = &nafCfg.MaxValue
		}
		return sampling.NewNumericAttributeFilter(settings, nafCfg.Key, minValuePtr, maxValuePtr, nafCfg.InvertMatch), nil
	case Probabilistic:
		pCfg := cfg.ProbabilisticCfg
		return sampling.NewProbabilisticSampler(settings, pCfg.HashSalt, pCfg.SamplingPercentage), nil
	case StringAttribute:
		safCfg := cfg.StringAttributeCfg
		return sampling.NewStringAttributeFilter(settings, safCfg.Key, safCfg.Values, safCfg.EnabledRegexMatching, safCfg.CacheMaxSize, safCfg.InvertMatch), nil
	case StatusCode:
		scfCfg := cfg.StatusCodeCfg
		return sampling.NewStatusCodeFilter(settings, scfCfg.StatusCodes)
	case RateLimiting:
		rlfCfg := cfg.RateLimitingCfg
		return sampling.NewRateLimiting(settings, rlfCfg.SpansPerSecond), nil
	case SpanCount:
		spCfg := cfg.SpanCountCfg
		return sampling.NewSpanCount(settings, spCfg.MinSpans, spCfg.MaxSpans), nil
	case TraceState:
		tsfCfg := cfg.TraceStateCfg
		return sampling.NewTraceStateFilter(settings, tsfCfg.Key, tsfCfg.Values), nil
	case BooleanAttribute:
		bafCfg := cfg.BooleanAttributeCfg
		return sampling.NewBooleanAttributeFilter(settings, bafCfg.Key, bafCfg.Value, bafCfg.InvertMatch), nil
	case OTTLCondition:
		ottlfCfg := cfg.OTTLConditionCfg
		return sampling.NewOTTLConditionFilter(settings, ottlfCfg.SpanConditions, ottlfCfg.SpanEventConditions, ottlfCfg.ErrorMode)
	default:
		t := string(cfg.Type)
		extension, ok := policyExtensions[t]
		if ok {
			evaluator, err := extension.NewEvaluator(cfg.Name, cfg.ExtensionCfg[t])
			if err != nil {
				return nil, fmt.Errorf("unable to load extension %s: %w", t, err)
			}
			return evaluator, nil
		}

		return nil, fmt.Errorf("unknown sampling policy type %s", cfg.Type)
	}
}

type policyDecisionMetrics struct {
	tracesSampled int
	spansSampled  int64
}

type policyMetrics struct {
	idNotFoundOnMapCount, evaluateErrorCount, decisionSampled, decisionNotSampled, decisionDropped int64
	tracesSampledByPolicyDecision                                                                  []map[samplingpolicy.Decision]policyDecisionMetrics
}

func newPolicyMetrics(numPolicies int) *policyMetrics {
	tracesSampledByPolicyDecision := make([]map[samplingpolicy.Decision]policyDecisionMetrics, numPolicies)
	for i := range tracesSampledByPolicyDecision {
		tracesSampledByPolicyDecision[i] = make(map[samplingpolicy.Decision]policyDecisionMetrics)
	}
	return &policyMetrics{
		tracesSampledByPolicyDecision: tracesSampledByPolicyDecision,
	}
}

func (m *policyMetrics) addDecision(policyIndex int, decision samplingpolicy.Decision, spansSampled int64) {
	stats := m.tracesSampledByPolicyDecision[policyIndex][decision]
	stats.tracesSampled++
	stats.spansSampled += spansSampled
	m.tracesSampledByPolicyDecision[policyIndex][decision] = stats
}

func (tsp *tailSamplingSpanProcessor) loadSamplingPolicy(cfgs []PolicyCfg) error {
	telemetrySettings := tsp.set.TelemetrySettings
	componentID := tsp.set.ID.Name()

	cLen := len(cfgs)
	policies := make([]*policy, 0, cLen)
	dropPolicies := make([]*policy, 0, cLen)
	policyNames := make(map[string]struct{}, cLen)

	for i := range cfgs {
		cfg := cfgs[i]
		if cfg.Name == "" {
			return errors.New("policy name cannot be empty")
		}

		if _, exists := policyNames[cfg.Name]; exists {
			return fmt.Errorf("duplicate policy name %q", cfg.Name)
		}
		policyNames[cfg.Name] = struct{}{}

		eval, err := getPolicyEvaluator(telemetrySettings, &cfg, tsp.extensions())
		if err != nil {
			return fmt.Errorf("failed to create policy evaluator for %q: %w", cfg.Name, err)
		}

		uniquePolicyName := cfg.Name
		if componentID != "" {
			uniquePolicyName = fmt.Sprintf("%s.%s", componentID, cfg.Name)
		}

		p := &policy{
			name:      cfg.Name,
			evaluator: eval,
			attribute: metric.WithAttributes(attribute.String("policy", uniquePolicyName)),
		}

		if cfg.Type == Drop {
			dropPolicies = append(dropPolicies, p)
		} else {
			policies = append(policies, p)
		}
	}
	// Dropped decision takes precedence over all others, therefore we evaluate them first.
	tsp.policies = slices.Concat(dropPolicies, policies)

	tsp.logger.Debug("Loaded sampling policy", zap.Int("policies.len", len(policies)))

	return nil
}

func (tsp *tailSamplingSpanProcessor) SetSamplingPolicy(cfgs []PolicyCfg) {
	tsp.logger.Debug("Setting pending sampling policy", zap.Int("pending.len", len(cfgs)))

	tsp.setPolicyMux.Lock()
	defer tsp.setPolicyMux.Unlock()

	tsp.pendingPolicy = cfgs
}

func (tsp *tailSamplingSpanProcessor) loadPendingSamplingPolicy() error {
	tsp.setPolicyMux.Lock()
	defer tsp.setPolicyMux.Unlock()

	// Nothing pending, do nothing.
	pLen := len(tsp.pendingPolicy)
	if pLen == 0 {
		return nil
	}

	tsp.logger.Debug("Loading pending sampling policy", zap.Int("pending.len", pLen))

	err := tsp.loadSamplingPolicy(tsp.pendingPolicy)

	// Empty pending regardless of error. If policy is invalid, it will fail on
	// every tick, no need to do extra work and flood the log with errors.
	tsp.pendingPolicy = nil
	return err
}

func (tsp *tailSamplingSpanProcessor) samplingPolicyOnTick() {
	tsp.logger.Debug("Sampling Policy Evaluation ticked")

	err := tsp.loadPendingSamplingPolicy()
	if err != nil {
		tsp.logger.Error("Failed to load pending sampling policy", zap.Error(err))
		tsp.logger.Debug("Continuing to use the previously loaded sampling policy")
	}

	ctx := context.Background()
	metrics := newPolicyMetrics(len(tsp.policies))
	startTime := time.Now()

	batch, _ := tsp.decisionBatcher.CloseCurrentAndTakeFirstBatch()
	batchLen := len(batch)

	for _, id := range batch {
		d, ok := tsp.idToTrace.Load(id)
		if !ok {
			metrics.idNotFoundOnMapCount++
			continue
		}
		trace := d.(*samplingpolicy.TraceData)
		trace.DecisionTime = time.Now()

		decision := tsp.makeDecision(id, trace, metrics)

		tsp.telemetry.ProcessorTailSamplingGlobalCountTracesSampled.Add(tsp.ctx, 1, decisionToAttributes[decision])

		// Sampled or not, remove the batches
		trace.Lock()
		allSpans := trace.ReceivedBatches
		trace.FinalDecision = decision
		trace.ReceivedBatches = ptrace.NewTraces()
		trace.Unlock()

		if decision == samplingpolicy.Sampled {
			tsp.releaseSampledTrace(ctx, id, allSpans)
		} else {
			tsp.releaseNotSampledTrace(id)
		}
	}

	tsp.telemetry.ProcessorTailSamplingSamplingDecisionTimerLatency.Record(tsp.ctx, int64(time.Since(startTime)/time.Millisecond))
	tsp.telemetry.ProcessorTailSamplingSamplingTracesOnMemory.Record(tsp.ctx, int64(tsp.numTracesOnMap.Load()))
	tsp.telemetry.ProcessorTailSamplingSamplingTraceDroppedTooEarly.Add(tsp.ctx, metrics.idNotFoundOnMapCount)
	tsp.telemetry.ProcessorTailSamplingSamplingPolicyEvaluationError.Add(tsp.ctx, metrics.evaluateErrorCount)

	for i, p := range tsp.policies {
		for decision, stats := range metrics.tracesSampledByPolicyDecision[i] {
			tsp.telemetry.ProcessorTailSamplingCountTracesSampled.Add(tsp.ctx, int64(stats.tracesSampled), p.attribute, decisionToAttributes[decision])
			if telemetry.IsMetricStatCountSpansSampledEnabled() {
				tsp.telemetry.ProcessorTailSamplingCountSpansSampled.Add(tsp.ctx, stats.spansSampled, p.attribute, decisionToAttributes[decision])
			}
		}
	}

	tsp.logger.Debug("Sampling policy evaluation completed",
		zap.Int("batch.len", batchLen),
		zap.Int64("sampled", metrics.decisionSampled),
		zap.Int64("notSampled", metrics.decisionNotSampled),
		zap.Int64("dropped", metrics.decisionDropped),
		zap.Int64("droppedPriorToEvaluation", metrics.idNotFoundOnMapCount),
		zap.Int64("policyEvaluationErrors", metrics.evaluateErrorCount),
	)
}

func (tsp *tailSamplingSpanProcessor) makeDecision(id pcommon.TraceID, trace *samplingpolicy.TraceData, metrics *policyMetrics) samplingpolicy.Decision {
	finalDecision := samplingpolicy.NotSampled
	samplingDecisions := map[samplingpolicy.Decision]*policy{
		samplingpolicy.Error:            nil,
		samplingpolicy.Sampled:          nil,
		samplingpolicy.NotSampled:       nil,
		samplingpolicy.InvertSampled:    nil,
		samplingpolicy.InvertNotSampled: nil,
		samplingpolicy.Dropped:          nil,
	}

	ctx := context.Background()
	startTime := time.Now()

	// Check all policies before making a final decision.
	for i, p := range tsp.policies {
		decision, err := p.evaluator.Evaluate(ctx, id, trace)
		latency := time.Since(startTime)
		tsp.telemetry.ProcessorTailSamplingSamplingDecisionLatency.Record(ctx, int64(latency/time.Microsecond), p.attribute)

		if err != nil {
			if samplingDecisions[samplingpolicy.Error] == nil {
				samplingDecisions[samplingpolicy.Error] = p
			}
			metrics.evaluateErrorCount++
			tsp.logger.Debug("Sampling policy error", zap.Error(err))
			continue
		}

		metrics.addDecision(i, decision, trace.SpanCount.Load())

		// We associate the first policy with the sampling decision to understand what policy sampled a span
		if samplingDecisions[decision] == nil {
			samplingDecisions[decision] = p
		}

		// Break early if dropped. This can drastically reduce tick/decision latency.
		if decision == samplingpolicy.Dropped {
			break
		}
		// If sampleOnFirstMatch is enabled, make decision as soon as a policy matches
		if tsp.sampleOnFirstMatch && decision == samplingpolicy.Sampled {
			break
		}
	}

	var sampledPolicy *policy

	switch {
	case samplingDecisions[samplingpolicy.Dropped] != nil: // Dropped takes precedence
		finalDecision = samplingpolicy.Dropped
	case samplingDecisions[samplingpolicy.InvertNotSampled] != nil: // Then InvertNotSampled
		finalDecision = samplingpolicy.NotSampled
	case samplingDecisions[samplingpolicy.Sampled] != nil:
		finalDecision = samplingpolicy.Sampled
		sampledPolicy = samplingDecisions[samplingpolicy.Sampled]
	case samplingDecisions[samplingpolicy.InvertSampled] != nil && samplingDecisions[samplingpolicy.NotSampled] == nil:
		finalDecision = samplingpolicy.Sampled
		sampledPolicy = samplingDecisions[samplingpolicy.InvertSampled]
	}

	if tsp.recordPolicy && sampledPolicy != nil {
		sampling.SetAttrOnScopeSpans(trace, "tailsampling.policy", sampledPolicy.name)
	}

	switch finalDecision {
	case samplingpolicy.Sampled:
		metrics.decisionSampled++
	case samplingpolicy.NotSampled:
		metrics.decisionNotSampled++
	case samplingpolicy.Dropped:
		metrics.decisionDropped++
	}

	return finalDecision
}

// ConsumeTraces is required by the processor.Traces interface.
func (tsp *tailSamplingSpanProcessor) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		tsp.processTraces(resourceSpans.At(i))
	}
	return nil
}

func (*tailSamplingSpanProcessor) groupSpansByTraceKey(resourceSpans ptrace.ResourceSpans) map[pcommon.TraceID][]spanAndScope {
	idToSpans := make(map[pcommon.TraceID][]spanAndScope)
	ilss := resourceSpans.ScopeSpans()
	for j := 0; j < ilss.Len(); j++ {
		scope := ilss.At(j)
		spans := scope.Spans()
		is := scope.Scope()
		spansLen := spans.Len()
		for k := 0; k < spansLen; k++ {
			span := spans.At(k)
			key := span.TraceID()
			idToSpans[key] = append(idToSpans[key], spanAndScope{
				span:                 &span,
				instrumentationScope: &is,
			})
		}
	}
	return idToSpans
}

func (tsp *tailSamplingSpanProcessor) processTraces(resourceSpans ptrace.ResourceSpans) {
	currTime := time.Now()

	// Group spans per their traceId to minimize contention on idToTrace
	idToSpansAndScope := tsp.groupSpansByTraceKey(resourceSpans)
	var newTraceIDs int64
	for id, spans := range idToSpansAndScope {
		// If the trace ID is in the sampled cache, short circuit the decision
		if _, ok := tsp.sampledIDCache.Get(id); ok {
			tsp.logger.Debug("Trace ID is in the sampled cache", zap.Stringer("id", id))
			traceTd := ptrace.NewTraces()
			appendToTraces(traceTd, resourceSpans, spans)
			if tsp.recordPolicy {
				sampling.SetBoolAttrOnScopeSpans(traceTd, "tailsampling.cached_decision", true)
			}
			tsp.forwardSpans(tsp.ctx, traceTd)
			tsp.telemetry.ProcessorTailSamplingEarlyReleasesFromCacheDecision.
				Add(tsp.ctx, int64(len(spans)), attrSampledTrue)
			continue
		}
		// If the trace ID is in the non-sampled cache, short circuit the decision
		if _, ok := tsp.nonSampledIDCache.Get(id); ok {
			tsp.logger.Debug("Trace ID is in the non-sampled cache", zap.Stringer("id", id))
			tsp.telemetry.ProcessorTailSamplingEarlyReleasesFromCacheDecision.
				Add(tsp.ctx, int64(len(spans)), attrSampledFalse)
			continue
		}

		lenSpans := int64(len(spans))

		d, loaded := tsp.idToTrace.Load(id)
		if !loaded {
			spanCount := &atomic.Int64{}
			spanCount.Store(lenSpans)

			td := &samplingpolicy.TraceData{
				ArrivalTime:     currTime,
				SpanCount:       spanCount,
				ReceivedBatches: ptrace.NewTraces(),
			}

			if d, loaded = tsp.idToTrace.LoadOrStore(id, td); !loaded {
				newTraceIDs++
				tsp.decisionBatcher.AddToCurrentBatch(id)
				tsp.numTracesOnMap.Add(1)
				tsp.traceLimiter.AcceptTrace(tsp.ctx, id, currTime)
			}
		}

		actualData := d.(*samplingpolicy.TraceData)
		if loaded {
			actualData.SpanCount.Add(lenSpans)
		}

		actualData.Lock()
		finalDecision := actualData.FinalDecision

		if finalDecision == samplingpolicy.Unspecified {
			// If the final decision hasn't been made, add the new spans under the lock.
			appendToTraces(actualData.ReceivedBatches, resourceSpans, spans)
			actualData.Unlock()
			continue
		}

		actualData.Unlock()

		switch finalDecision {
		case samplingpolicy.Sampled:
			traceTd := ptrace.NewTraces()
			appendToTraces(traceTd, resourceSpans, spans)
			tsp.forwardSpans(tsp.ctx, traceTd)
		case samplingpolicy.NotSampled:
			tsp.releaseNotSampledTrace(id)
		default:
			tsp.logger.Warn("Unexpected sampling decision", zap.Int("decision", int(finalDecision)))
		}

		if !actualData.DecisionTime.IsZero() {
			tsp.telemetry.ProcessorTailSamplingSamplingLateSpanAge.Record(tsp.ctx, int64(time.Since(actualData.DecisionTime)/time.Second))
		}
	}

	tsp.telemetry.ProcessorTailSamplingNewTraceIDReceived.Add(tsp.ctx, newTraceIDs)
}

func (*tailSamplingSpanProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// Start is invoked during service startup.
func (tsp *tailSamplingSpanProcessor) Start(_ context.Context, host component.Host) error {
	// We need to store the host before loading sampling policies in order to load any extensions.
	tsp.host = host
	if tsp.policies == nil {
		err := tsp.loadPendingSamplingPolicy()
		if err != nil {
			tsp.logger.Error("Failed to load initial sampling policy", zap.Error(err))
			return err
		}
	}
	tsp.policyTicker.Start(tsp.tickerFrequency)
	return nil
}

func (tsp *tailSamplingSpanProcessor) extensions() map[string]samplingpolicy.Extension {
	if tsp.host == nil {
		return nil
	}
	extensions := tsp.host.GetExtensions()
	scoped := map[string]samplingpolicy.Extension{}
	for id, ext := range extensions {
		evaluator, ok := ext.(samplingpolicy.Extension)
		if ok {
			scoped[id.String()] = evaluator
		}
	}
	return scoped
}

// Shutdown is invoked during service shutdown.
func (tsp *tailSamplingSpanProcessor) Shutdown(context.Context) error {
	tsp.decisionBatcher.Stop()
	tsp.policyTicker.Stop()
	return nil
}

func (tsp *tailSamplingSpanProcessor) dropTrace(traceID pcommon.TraceID, deletionTime time.Time) {
	var trace *samplingpolicy.TraceData
	if d, ok := tsp.idToTrace.Load(traceID); ok {
		trace = d.(*samplingpolicy.TraceData)
		tsp.idToTrace.Delete(traceID)
		// Subtract one from numTracesOnMap per https://godoc.org/sync/atomic#AddUint64
		tsp.numTracesOnMap.Add(^uint64(0))
		tsp.traceLimiter.OnDeleteTrace()
	}
	if trace == nil {
		tsp.logger.Debug("Attempt to delete trace ID not on table", zap.Stringer("id", traceID))
		return
	}

	tsp.telemetry.ProcessorTailSamplingSamplingTraceRemovalAge.Record(tsp.ctx, int64(deletionTime.Sub(trace.ArrivalTime)/time.Second))
}

// forwardSpans sends the trace data to the next consumer. it is different from
// releaseSampledTrace in that it does not modify any tsp state.
func (tsp *tailSamplingSpanProcessor) forwardSpans(ctx context.Context, td ptrace.Traces) {
	if err := tsp.nextConsumer.ConsumeTraces(ctx, td); err != nil {
		tsp.logger.Warn(
			"Error sending spans to destination",
			zap.Error(err))
	}
}

// releaseSampledTrace sends the trace data to the next consumer. It
// additionally adds the trace ID to the cache of sampled trace IDs. If the
// trace ID is cached, it deletes the spans from the internal map.
func (tsp *tailSamplingSpanProcessor) releaseSampledTrace(ctx context.Context, id pcommon.TraceID, td ptrace.Traces) {
	tsp.sampledIDCache.Put(id, true)
	tsp.forwardSpans(ctx, td)
	_, ok := tsp.sampledIDCache.Get(id)
	if ok {
		tsp.dropTrace(id, time.Now())
	}
}

// releaseNotSampledTrace adds the trace ID to the cache of not sampled trace
// IDs. If the trace ID is cached, it deletes the spans from the internal map.
func (tsp *tailSamplingSpanProcessor) releaseNotSampledTrace(id pcommon.TraceID) {
	tsp.nonSampledIDCache.Put(id, true)
	_, ok := tsp.nonSampledIDCache.Get(id)
	if ok {
		tsp.dropTrace(id, time.Now())
	}
}

func appendToTraces(dest ptrace.Traces, rss ptrace.ResourceSpans, spanAndScopes []spanAndScope) {
	rs := dest.ResourceSpans().AppendEmpty()
	rss.Resource().CopyTo(rs.Resource())

	scopePointerToNewScope := make(map[*pcommon.InstrumentationScope]*ptrace.ScopeSpans)
	for _, spanAndScope := range spanAndScopes {
		// If the scope of the spanAndScope is not in the map, add it to the map and the destination.
		if scope, ok := scopePointerToNewScope[spanAndScope.instrumentationScope]; !ok {
			is := rs.ScopeSpans().AppendEmpty()
			spanAndScope.instrumentationScope.CopyTo(is.Scope())
			scopePointerToNewScope[spanAndScope.instrumentationScope] = &is

			sp := is.Spans().AppendEmpty()
			spanAndScope.span.CopyTo(sp)
		} else {
			sp := scope.Spans().AppendEmpty()
			spanAndScope.span.CopyTo(sp)
		}
	}
}
