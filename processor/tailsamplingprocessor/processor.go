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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/cache"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/idbatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	internalsampling "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/telemetry"
)

// policy combines a sampling policy evaluator with the destinations to be
// used for that policy.
type policy struct {
	// name used to identify this policy instance.
	name string
	// evaluator that decides if a trace is sampled or not by this policy instance.
	evaluator internalsampling.PolicyEvaluator
	// attribute to use in the telemetry to denote the policy.
	attribute metric.MeasurementOption
}

// tailSamplingSpanProcessor handles the incoming trace data and forward the appropriate
// sampling decision to the next stage of the pipeline.
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
	sampledIDCache     cache.Cache[sampling.Threshold]
	nonSampledIDCache  cache.Cache[bool]
	deleteChan         chan pcommon.TraceID
	numTracesOnMap     *atomic.Uint64
	recordPolicy       bool
	setPolicyMux       sync.Mutex
	pendingPolicy      []PolicyCfg
	sampleOnFirstMatch bool

	// OTEP 235 TraceState management for consistent probability sampling
	traceStateManager *internalsampling.TraceStateManager
}

// spanAndScope a structure for holding information about span and its instrumentation scope.
// required for preserving the instrumentation library information while sampling.
// We use pointers there to fast find the span in the map.
type spanAndScope struct {
	span                 *ptrace.Span
	instrumentationScope *pcommon.InstrumentationScope
}

var (
	attrSampledTrue  = metric.WithAttributes(attribute.String("sampled", "true"))
	attrSampledFalse = metric.WithAttributes(attribute.String("sampled", "false"))
)

// decisionToMetricAttribute converts a Decision to a metric attribute for telemetry.
func decisionToMetricAttribute(decision internalsampling.Decision) metric.MeasurementOption {
	if decision.IsSampled() {
		return attrSampledTrue
	}
	return attrSampledFalse
}

type Option func(*tailSamplingSpanProcessor)

// newTracesProcessor returns a processor.TracesProcessor that will perform tail sampling according to the given
// configuration.
func newTracesProcessor(ctx context.Context, set processor.Settings, nextConsumer consumer.Traces, cfg Config) (processor.Traces, error) {
	telemetrySettings := set.TelemetrySettings
	telemetry, err := metadata.NewTelemetryBuilder(telemetrySettings)
	if err != nil {
		return nil, err
	}
	nopCache := cache.NewNopDecisionCache[sampling.Threshold]()
	sampledDecisions := nopCache
	nonSampledDecisions := cache.NewNopDecisionCache[bool]()
	if cfg.DecisionCache.SampledCacheSize > 0 {
		sampledDecisions, err = cache.NewLRUDecisionCache[sampling.Threshold](cfg.DecisionCache.SampledCacheSize)
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
		deleteChan:         make(chan pcommon.TraceID, cfg.NumTraces),
		sampleOnFirstMatch: cfg.SampleOnFirstMatch,
		traceStateManager:  internalsampling.NewTraceStateManager(),
	}
	tsp.policyTicker = &timeutils.PolicyTicker{OnTickFunc: tsp.samplingPolicyOnTick}

	for _, opt := range cfg.Options {
		opt(tsp)
	}

	if tsp.tickerFrequency == 0 {
		tsp.tickerFrequency = time.Second
	}

	if tsp.policies == nil {
		err := tsp.loadSamplingPolicy(cfg.PolicyCfgs)
		if err != nil {
			return nil, err
		}
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
func WithSampledDecisionCache(c cache.Cache[sampling.Threshold]) Option {
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

func getPolicyEvaluator(settings component.TelemetrySettings, cfg *PolicyCfg) (internalsampling.PolicyEvaluator, error) {
	switch cfg.Type {
	case Composite:
		return getNewCompositePolicy(settings, &cfg.CompositeCfg)
	case And:
		return getNewAndPolicy(settings, &cfg.AndCfg)
	case Drop:
		return getNewDropPolicy(settings, &cfg.DropCfg)
	default:
		return getSharedPolicyEvaluator(settings, &cfg.sharedPolicyCfg)
	}
}

func getSharedPolicyEvaluator(settings component.TelemetrySettings, cfg *sharedPolicyCfg) (internalsampling.PolicyEvaluator, error) {
	settings.Logger = settings.Logger.With(zap.Any("policy", cfg.Type))

	switch cfg.Type {
	case AlwaysSample:
		return internalsampling.NewAlwaysSample(settings), nil
	case Latency:
		lfCfg := cfg.LatencyCfg
		return internalsampling.NewLatency(settings, lfCfg.ThresholdMs, lfCfg.UpperThresholdmsMs), nil
	case NumericAttribute:
		nafCfg := cfg.NumericAttributeCfg
		minValue := nafCfg.MinValue
		maxValue := nafCfg.MaxValue
		return internalsampling.NewNumericAttributeFilter(settings, nafCfg.Key, &minValue, &maxValue, nafCfg.InvertMatch), nil
	case Probabilistic:
		pCfg := cfg.ProbabilisticCfg
		return internalsampling.NewProbabilisticSampler(settings, pCfg.HashSalt, pCfg.SamplingPercentage), nil
	case StringAttribute:
		safCfg := cfg.StringAttributeCfg
		return internalsampling.NewStringAttributeFilter(settings, safCfg.Key, safCfg.Values, safCfg.EnabledRegexMatching, safCfg.CacheMaxSize, safCfg.InvertMatch), nil
	case StatusCode:
		scfCfg := cfg.StatusCodeCfg
		return internalsampling.NewStatusCodeFilter(settings, scfCfg.StatusCodes)
	case RateLimiting:
		rlfCfg := cfg.RateLimitingCfg
		return internalsampling.NewRateLimiting(settings, rlfCfg.SpansPerSecond), nil
	case SpanCount:
		spCfg := cfg.SpanCountCfg
		return internalsampling.NewSpanCount(settings, spCfg.MinSpans, spCfg.MaxSpans), nil
	case TraceState:
		tsfCfg := cfg.TraceStateCfg
		return internalsampling.NewTraceStateFilter(settings, tsfCfg.Key, tsfCfg.Values), nil
	case BooleanAttribute:
		bafCfg := cfg.BooleanAttributeCfg
		return internalsampling.NewBooleanAttributeFilter(settings, bafCfg.Key, bafCfg.Value, bafCfg.InvertMatch), nil
	case OTTLCondition:
		ottlfCfg := cfg.OTTLConditionCfg
		return internalsampling.NewOTTLConditionFilter(settings, ottlfCfg.SpanConditions, ottlfCfg.SpanEventConditions, ottlfCfg.ErrorMode)

	default:
		return nil, fmt.Errorf("unknown sampling policy type %s", cfg.Type)
	}
}

type policyMetrics struct {
	idNotFoundOnMapCount, evaluateErrorCount, decisionSampled, decisionNotSampled int64
}

func (tsp *tailSamplingSpanProcessor) loadSamplingPolicy(cfgs []PolicyCfg) error {
	telemetrySettings := tsp.set.TelemetrySettings
	componentID := tsp.set.ID.Name()

	cLen := len(cfgs)
	policies := make([]*policy, 0, cLen)
	dropPolicies := make([]*policy, 0, cLen)
	policyNames := make(map[string]struct{}, cLen)

	for _, cfg := range cfgs {
		if cfg.Name == "" {
			return errors.New("policy name cannot be empty")
		}

		if _, exists := policyNames[cfg.Name]; exists {
			return fmt.Errorf("duplicate policy name %q", cfg.Name)
		}
		policyNames[cfg.Name] = struct{}{}

		eval, err := getPolicyEvaluator(telemetrySettings, &cfg)
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

func (tsp *tailSamplingSpanProcessor) loadPendingSamplingPolicy() {
	tsp.setPolicyMux.Lock()
	defer tsp.setPolicyMux.Unlock()

	// Nothing pending, do nothing.
	pLen := len(tsp.pendingPolicy)
	if pLen == 0 {
		return
	}

	tsp.logger.Debug("Loading pending sampling policy", zap.Int("pending.len", pLen))

	err := tsp.loadSamplingPolicy(tsp.pendingPolicy)

	// Empty pending regardless of error. If policy is invalid, it will fail on
	// every tick, no need to do extra work and flood the log with errors.
	tsp.pendingPolicy = nil

	if err != nil {
		tsp.logger.Error("Failed to load pending sampling policy", zap.Error(err))
		tsp.logger.Debug("Continuing to use the previously loaded sampling policy")
	}
}

func (tsp *tailSamplingSpanProcessor) samplingPolicyOnTick() {
	tsp.logger.Debug("Sampling Policy Evaluation ticked")

	tsp.loadPendingSamplingPolicy()

	ctx := context.Background()
	metrics := policyMetrics{}
	startTime := time.Now()

	batch, _ := tsp.decisionBatcher.CloseCurrentAndTakeFirstBatch()
	batchLen := len(batch)

	for _, id := range batch {
		d, ok := tsp.idToTrace.Load(id)
		if !ok {
			metrics.idNotFoundOnMapCount++
			continue
		}
		trace := d.(*internalsampling.TraceData)

		decision := tsp.makeDecision(id, trace, &metrics)

		tsp.telemetry.ProcessorTailSamplingGlobalCountTracesSampled.Add(tsp.ctx, 1, decisionToMetricAttribute(decision))

		// Sampled or not, remove the batches
		trace.Lock()
		allSpans := trace.ReceivedBatches
		trace.FinalDecision = decision
		trace.ReceivedBatches = ptrace.NewTraces()
		trace.Unlock()

		switch {
		case decision.IsSampled():
			tsp.releaseSampledTrace(ctx, id, allSpans)
		default:
			tsp.releaseNotSampledTrace(id)
		}
	}

	tsp.telemetry.ProcessorTailSamplingSamplingDecisionTimerLatency.Record(tsp.ctx, int64(time.Since(startTime)/time.Millisecond))
	tsp.telemetry.ProcessorTailSamplingSamplingTracesOnMemory.Record(tsp.ctx, int64(tsp.numTracesOnMap.Load()))
	tsp.telemetry.ProcessorTailSamplingSamplingTraceDroppedTooEarly.Add(tsp.ctx, metrics.idNotFoundOnMapCount)
	tsp.telemetry.ProcessorTailSamplingSamplingPolicyEvaluationError.Add(tsp.ctx, metrics.evaluateErrorCount)

	tsp.logger.Debug("Sampling policy evaluation completed",
		zap.Int("batch.len", batchLen),
		zap.Int64("sampled", metrics.decisionSampled),
		zap.Int64("notSampled", metrics.decisionNotSampled),
		zap.Int64("droppedPriorToEvaluation", metrics.idNotFoundOnMapCount),
		zap.Int64("policyEvaluationErrors", metrics.evaluateErrorCount),
	)
}

func (tsp *tailSamplingSpanProcessor) makeDecision(id pcommon.TraceID, trace *internalsampling.TraceData, metrics *policyMetrics) internalsampling.Decision {
	finalDecision := internalsampling.NotSampled

	// Track policy decisions without using Decision as map key
	var errorPolicy *policy
	var sampledPolicy *policy
	var policyDecisions []internalsampling.Decision
	var droppedPolicy *policy

	ctx := context.Background()
	startTime := time.Now()

	// Check all policies and collect their threshold decisions.
	for _, p := range tsp.policies {
		decision, err := p.evaluator.Evaluate(ctx, id, trace)
		latency := time.Since(startTime)
		tsp.telemetry.ProcessorTailSamplingSamplingDecisionLatency.Record(ctx, int64(latency/time.Microsecond), p.attribute)

		if err != nil {
			if errorPolicy == nil {
				errorPolicy = p
			}
			metrics.evaluateErrorCount++
			tsp.logger.Debug("Sampling policy error", zap.Error(err))
			continue
		}

		tsp.telemetry.ProcessorTailSamplingCountTracesSampled.Add(ctx, 1, p.attribute, decisionToMetricAttribute(decision))

		if telemetry.IsMetricStatCountSpansSampledEnabled() {
			tsp.telemetry.ProcessorTailSamplingCountSpansSampled.Add(ctx, trace.SpanCount.Load(), p.attribute, decisionToMetricAttribute(decision))
		}

		// Collect all policy decisions for OTEP 250 threshold combination
		policyDecisions = append(policyDecisions, decision)

		// Track policies for metrics and debugging (first occurrence only)
		if decision.IsDropped() && droppedPolicy == nil {
			droppedPolicy = p
		} else if decision.IsSampled() && sampledPolicy == nil {
			sampledPolicy = p
		}

		// Break early if dropped. This can drastically reduce tick/decision latency.
		if decision.IsDropped() {
			break
		}
		// If sampleOnFirstMatch is enabled, make decision as soon as a policy produces a sampling decision
		if tsp.sampleOnFirstMatch && (decision.IsSampled() || decision.IsInvertSampled()) {
			break
		}
	}

	// OTEP 250 Decision Logic with precedence for backward compatibility
	switch {
	case droppedPolicy != nil:
		// Dropped takes precedence over all other decisions
		finalDecision = internalsampling.NotSampled
	default:
		// For SampleOnFirstMatch, use the last collected decision (which was the matching one)
		if tsp.sampleOnFirstMatch && len(policyDecisions) > 0 {
			lastDecision := policyDecisions[len(policyDecisions)-1]
			if lastDecision.IsSampled() || lastDecision.IsInvertSampled() {
				finalDecision = internalsampling.Sampled
				finalThreshold := sampling.AlwaysSampleThreshold
				trace.FinalThreshold = &finalThreshold
			} else {
				finalDecision = internalsampling.NotSampled
				finalThreshold := sampling.NeverSampleThreshold
				trace.FinalThreshold = &finalThreshold
			}
		} else {
			// Normal precedence-based logic when not using SampleOnFirstMatch
			// Check for explicit non-sampling decisions first
			hasExplicitNotSampled := false
			hasRegularSampled := false
			hasInvertSampled := false
			hasInvertNotSampled := false
			minThreshold := sampling.NeverSampleThreshold

			for _, decision := range policyDecisions {
				if decision.IsDropped() || decision.IsError() {
					continue
				}

				// Track decision types for precedence logic
				if decision.IsNotSampled() {
					hasExplicitNotSampled = true
				} else if decision.IsSampled() && !decision.IsInverted() {
					hasRegularSampled = true
				} else if decision.IsInvertSampled() {
					hasInvertSampled = true
				} else if decision.IsInvertNotSampled() {
					hasInvertNotSampled = true
				}

				// Track minimum threshold for pure threshold-based policies
				if sampling.ThresholdLessThan(decision.Threshold, minThreshold) {
					minThreshold = decision.Threshold
				}
			}

			// Apply precedence rules for backward compatibility:
			// 1. Explicit NotSampled and InvertNotSampled override everything else (they both mean "don't sample")
			// 2. Regular Sampled takes precedence over InvertSampled
			// 3. InvertSampled only works if no explicit NotSampled or InvertNotSampled
			if hasExplicitNotSampled || hasInvertNotSampled {
				finalDecision = internalsampling.NotSampled
				finalThreshold := sampling.NeverSampleThreshold
				trace.FinalThreshold = &finalThreshold
			} else if hasRegularSampled {
				finalDecision = internalsampling.Sampled
				finalThreshold := sampling.AlwaysSampleThreshold
				trace.FinalThreshold = &finalThreshold
			} else if hasInvertSampled {
				finalDecision = internalsampling.Sampled
				finalThreshold := sampling.AlwaysSampleThreshold
				trace.FinalThreshold = &finalThreshold
			} else {
				// No explicit sampling decisions, use pure threshold logic
				if minThreshold == sampling.AlwaysSampleThreshold {
					finalDecision = internalsampling.Sampled
				} else if minThreshold == sampling.NeverSampleThreshold {
					finalDecision = internalsampling.NotSampled
				} else {
					finalDecision = internalsampling.NewDecisionWithThreshold(minThreshold)
				}
				trace.FinalThreshold = &minThreshold
			}
		}
	}

	// OTEP 235 Phase 5: Update TraceState if threshold changed and TraceState is present
	if trace.TraceStatePresent && trace.FinalThreshold != nil {
		if err := tsp.traceStateManager.UpdateTraceState(trace, *trace.FinalThreshold); err != nil {
			tsp.logger.Warn("Failed to update TraceState with final threshold",
				zap.Stringer("traceID", id), zap.Error(err))
		}
	}

	if tsp.recordPolicy && sampledPolicy != nil {
		internalsampling.SetAttrOnScopeSpans(trace, "tailsampling.policy", sampledPolicy.name)
	}

	// OTEP 250: Apply deferred attribute inserters from the final decision
	// This implements the deferred attribute pattern for rule-based sampling outcomes
	if finalDecision.IsSampled() {
		finalDecision.ApplyAttributeInserters(trace)
	}

	if finalDecision.IsSampled() {
		metrics.decisionSampled++
	} else {
		metrics.decisionNotSampled++
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

func (tsp *tailSamplingSpanProcessor) groupSpansByTraceKey(resourceSpans ptrace.ResourceSpans) map[pcommon.TraceID][]spanAndScope {
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
	// Group spans per their traceId to minimize contention on idToTrace
	idToSpansAndScope := tsp.groupSpansByTraceKey(resourceSpans)
	var newTraceIDs int64
	for id, spans := range idToSpansAndScope {
		// If the trace ID is in the sampled cache, apply threshold-based evaluation
		if finalThreshold, ok := tsp.sampledIDCache.Get(id); ok {
			tsp.logger.Debug("Trace ID is in the sampled cache", zap.Stringer("id", id))

			// For OTEP 235 correctness, evaluate each span against the cached final threshold
			traceTd := ptrace.NewTraces()
			sampledSpans := []spanAndScope{}
			for _, spanAndScope := range spans {
				// Extract span's current threshold and compare with final threshold
				if tsp.shouldIncludeSpanBasedOnThreshold(spanAndScope.span, finalThreshold) {
					sampledSpans = append(sampledSpans, spanAndScope)
				}
			}

			if len(sampledSpans) > 0 {
				appendToTraces(traceTd, resourceSpans, sampledSpans)
				tsp.releaseSampledTrace(tsp.ctx, id, traceTd)
			}

			tsp.telemetry.ProcessorTailSamplingEarlyReleasesFromCacheDecision.
				Add(tsp.ctx, int64(len(sampledSpans)), attrSampledTrue)
			continue
		}
		// If the trace ID is in the non-sampled cache, short circuit the decision
		if notSampled, ok := tsp.nonSampledIDCache.Get(id); ok && !notSampled {
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

			td := &internalsampling.TraceData{
				ArrivalTime:     time.Now(),
				SpanCount:       spanCount,
				ReceivedBatches: ptrace.NewTraces(),
			}

			tsp.traceStateManager.InitializeTraceData(tsp.ctx, id, td)

			if d, loaded = tsp.idToTrace.LoadOrStore(id, td); !loaded {
				newTraceIDs++
				tsp.decisionBatcher.AddToCurrentBatch(id)
				tsp.numTracesOnMap.Add(1)
				postDeletion := false
				for !postDeletion {
					select {
					case tsp.deleteChan <- id:
						postDeletion = true
					default:
						traceKeyToDrop := <-tsp.deleteChan
						tsp.dropTrace(traceKeyToDrop, time.Now())
					}
				}
			}
		}

		actualData := d.(*internalsampling.TraceData)
		if loaded {
			actualData.SpanCount.Add(lenSpans)
		}

		actualData.Lock()
		finalDecision := actualData.FinalDecision

		if finalDecision.IsUnspecified() {
			// If the final decision hasn't been made, add the new spans under the lock.
			appendToTraces(actualData.ReceivedBatches, resourceSpans, spans)
			actualData.Unlock()
			continue
		}

		actualData.Unlock()

		if finalDecision.IsSampled() {
			traceTd := ptrace.NewTraces()
			appendToTraces(traceTd, resourceSpans, spans)
			tsp.releaseSampledTrace(tsp.ctx, id, traceTd)
		} else if !finalDecision.IsError() {
			tsp.releaseNotSampledTrace(id)
		} else {
			tsp.logger.Warn("Unexpected sampling decision", zap.String("decision", "error"))
		}
	}

	tsp.telemetry.ProcessorTailSamplingNewTraceIDReceived.Add(tsp.ctx, newTraceIDs)
}

func (tsp *tailSamplingSpanProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// Start is invoked during service startup.
func (tsp *tailSamplingSpanProcessor) Start(context.Context, component.Host) error {
	tsp.policyTicker.Start(tsp.tickerFrequency)
	return nil
}

// Shutdown is invoked during service shutdown.
func (tsp *tailSamplingSpanProcessor) Shutdown(context.Context) error {
	tsp.decisionBatcher.Stop()
	tsp.policyTicker.Stop()
	return nil
}

func (tsp *tailSamplingSpanProcessor) dropTrace(traceID pcommon.TraceID, deletionTime time.Time) {
	var trace *internalsampling.TraceData
	if d, ok := tsp.idToTrace.Load(traceID); ok {
		trace = d.(*internalsampling.TraceData)
		tsp.idToTrace.Delete(traceID)
		// Subtract one from numTracesOnMap per https://godoc.org/sync/atomic#AddUint64
		tsp.numTracesOnMap.Add(^uint64(0))
	}
	if trace == nil {
		tsp.logger.Debug("Attempt to delete trace ID not on table", zap.Stringer("id", traceID))
		return
	}

	tsp.telemetry.ProcessorTailSamplingSamplingTraceRemovalAge.Record(tsp.ctx, int64(deletionTime.Sub(trace.ArrivalTime)/time.Second))
}

// shouldIncludeSpanBasedOnThreshold evaluates if a span should be included
// based on the final threshold decision for the trace
func (tsp *tailSamplingSpanProcessor) shouldIncludeSpanBasedOnThreshold(span *ptrace.Span, finalThreshold sampling.Threshold) bool {
	// Extract current threshold from span's TraceState (if present)
	traceState := span.TraceState()
	if traceState.AsRaw() == "" {
		// If no TraceState, include the span (backward compatibility)
		return true
	}

	// Parse the W3C TraceState to extract threshold
	w3cTS, err := sampling.NewW3CTraceState(traceState.AsRaw())
	if err != nil {
		// If parsing fails, include the span (backward compatibility)
		return true
	}

	otelTS := w3cTS.OTelValue()
	if otelTS == nil {
		// No OpenTelemetry TraceState, include the span
		return true
	}

	spanThreshold, err := tsp.traceStateManager.ExtractThreshold(otelTS)
	if err != nil {
		// If no threshold in TraceState, include the span (backward compatibility)
		return true
	}

	// Include span if the final threshold is more permissive than or equal to the span's threshold
	return !sampling.ThresholdGreater(finalThreshold, *spanThreshold)
}

// additionally adds the trace ID to the cache of sampled trace IDs. If the
// trace ID is cached, it deletes the spans from the internal map.
func (tsp *tailSamplingSpanProcessor) releaseSampledTrace(ctx context.Context, id pcommon.TraceID, td ptrace.Traces) {
	// OTEP 235 Phase 5: Ensure TraceState is properly propagated in outgoing spans
	// This is particularly important for span batches that might not have gone through
	// the full policy evaluation pipeline (e.g., cached decisions)
	tsp.propagateTraceStateIfNeeded(ctx, id, td)

	// Store the final threshold in cache for OTEP 235 correctness
	finalThreshold := sampling.AlwaysSampleThreshold // Default for backward compatibility
	if d, ok := tsp.idToTrace.Load(id); ok {
		trace := d.(*internalsampling.TraceData)
		if trace.FinalThreshold != nil {
			finalThreshold = *trace.FinalThreshold
		}
	}

	tsp.sampledIDCache.Put(id, finalThreshold)
	if err := tsp.nextConsumer.ConsumeTraces(ctx, td); err != nil {
		tsp.logger.Warn(
			"Error sending spans to destination",
			zap.Error(err))
	}
	if _, ok := tsp.sampledIDCache.Get(id); ok {
		tsp.dropTrace(id, time.Now())
	}
}

// releaseNotSampledTrace adds the trace ID to the non-sampled cache and drops the trace.
// This ensures that future spans with the same trace ID are quickly rejected.
func (tsp *tailSamplingSpanProcessor) releaseNotSampledTrace(id pcommon.TraceID) {
	tsp.nonSampledIDCache.Put(id, false)
	tsp.dropTrace(id, time.Now())
}

// propagateTraceStateIfNeeded ensures TraceState consistency for outgoing spans.
// This handles cases where spans arrive after sampling decisions are made.
func (tsp *tailSamplingSpanProcessor) propagateTraceStateIfNeeded(ctx context.Context, id pcommon.TraceID, td ptrace.Traces) {
	// Check if we have trace data with OTEP 235 information
	if d, ok := tsp.idToTrace.Load(id); ok {
		trace := d.(*internalsampling.TraceData)

		// If trace has TraceState and a final threshold, ensure consistency
		if trace.TraceStatePresent && trace.FinalThreshold != nil {
			// Create a temporary TraceData for the outgoing spans
			tempTrace := &internalsampling.TraceData{
				ReceivedBatches:   td,
				TraceStatePresent: true,
				FinalThreshold:    trace.FinalThreshold,
			}

			// Update TraceState in outgoing spans
			if err := tsp.traceStateManager.UpdateTraceState(tempTrace, *trace.FinalThreshold); err != nil {
				tsp.logger.Debug("Failed to propagate TraceState to outgoing spans",
					zap.Stringer("traceID", id), zap.Error(err))
			}
		}
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
