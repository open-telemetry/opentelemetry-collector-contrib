// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"

import (
	"context"
	"fmt"
	"math"
	"runtime"
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/cache"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/idbatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/telemetry"
)

// policy combines a sampling policy evaluator with the destinations to be
// used for that policy.
type policy struct {
	// name used to identify this policy instance.
	name string
	// evaluator that decides if a trace is sampled or not by this policy instance.
	evaluator sampling.PolicyEvaluator
	// attribute to use in the telemetry to denote the policy.
	attribute metric.MeasurementOption
}

// tailSamplingSpanProcessor handles the incoming trace data and uses the given sampling
// policy to sample traces.
type tailSamplingSpanProcessor struct {
	ctx context.Context

	telemetry *metadata.TelemetryBuilder
	logger    *zap.Logger

	nextConsumer    consumer.Traces
	maxNumTraces    uint64
	policies        []*policy
	idToTrace       sync.Map
	policyTicker    timeutils.TTicker
	tickerFrequency time.Duration
	decisionBatcher idbatcher.Batcher
	sampledIDCache  cache.Cache[bool]
	deleteChan      chan pcommon.TraceID
	numTracesOnMap  *atomic.Uint64
}

// spanAndScope a structure for holding information about span and its instrumentation scope.
// required for preserving the instrumentation library information while sampling.
// We use pointers there to fast find the span in the map.
type spanAndScope struct {
	span                 *ptrace.Span
	instrumentationScope *pcommon.InstrumentationScope
}

var (
	attrSampledTrue     = metric.WithAttributes(attribute.String("sampled", "true"))
	attrSampledFalse    = metric.WithAttributes(attribute.String("sampled", "false"))
	decisionToAttribute = map[sampling.Decision]metric.MeasurementOption{
		sampling.Sampled:          attrSampledTrue,
		sampling.NotSampled:       attrSampledFalse,
		sampling.InvertNotSampled: attrSampledFalse,
		sampling.InvertSampled:    attrSampledTrue,
	}
)

type Option func(*tailSamplingSpanProcessor)

// newTracesProcessor returns a processor.TracesProcessor that will perform tail sampling according to the given
// configuration.
func newTracesProcessor(ctx context.Context, set processor.Settings, nextConsumer consumer.Traces, cfg Config, opts ...Option) (processor.Traces, error) {
	telemetrySettings := set.TelemetrySettings
	telemetry, err := metadata.NewTelemetryBuilder(telemetrySettings)
	if err != nil {
		return nil, err
	}
	sampledDecisions := cache.NewNopDecisionCache[bool]()
	if cfg.DecisionCache.SampledCacheSize > 0 {
		sampledDecisions, err = cache.NewLRUDecisionCache[bool](cfg.DecisionCache.SampledCacheSize)
		if err != nil {
			return nil, err
		}
	}

	tsp := &tailSamplingSpanProcessor{
		ctx:            ctx,
		telemetry:      telemetry,
		nextConsumer:   nextConsumer,
		maxNumTraces:   cfg.NumTraces,
		sampledIDCache: sampledDecisions,
		logger:         telemetrySettings.Logger,
		numTracesOnMap: &atomic.Uint64{},
		deleteChan:     make(chan pcommon.TraceID, cfg.NumTraces),
	}
	tsp.policyTicker = &timeutils.PolicyTicker{OnTickFunc: tsp.samplingPolicyOnTick}

	for _, opt := range opts {
		opt(tsp)
	}

	if tsp.tickerFrequency == 0 {
		tsp.tickerFrequency = time.Second
	}

	if tsp.policies == nil {
		policyNames := map[string]bool{}
		tsp.policies = make([]*policy, len(cfg.PolicyCfgs))
		componentID := set.ID.Name()
		for i := range cfg.PolicyCfgs {
			policyCfg := &cfg.PolicyCfgs[i]

			if policyNames[policyCfg.Name] {
				return nil, fmt.Errorf("duplicate policy name %q", policyCfg.Name)
			}
			policyNames[policyCfg.Name] = true

			eval, err := getPolicyEvaluator(telemetrySettings, policyCfg)
			if err != nil {
				return nil, err
			}
			uniquePolicyName := policyCfg.Name
			if componentID != "" {
				uniquePolicyName = fmt.Sprintf("%s.%s", componentID, policyCfg.Name)
			}
			p := &policy{
				name:      policyCfg.Name,
				evaluator: eval,
				attribute: metric.WithAttributes(attribute.String("policy", uniquePolicyName)),
			}
			tsp.policies[i] = p
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

// withSampledDecisionCache sets the cache which the processor uses to store recently sampled trace IDs.
func withSampledDecisionCache(c cache.Cache[bool]) Option {
	return func(tsp *tailSamplingSpanProcessor) {
		tsp.sampledIDCache = c
	}
}

func getPolicyEvaluator(settings component.TelemetrySettings, cfg *PolicyCfg) (sampling.PolicyEvaluator, error) {
	switch cfg.Type {
	case Composite:
		return getNewCompositePolicy(settings, &cfg.CompositeCfg)
	case And:
		return getNewAndPolicy(settings, &cfg.AndCfg)
	default:
		return getSharedPolicyEvaluator(settings, &cfg.sharedPolicyCfg)
	}
}

func getSharedPolicyEvaluator(settings component.TelemetrySettings, cfg *sharedPolicyCfg) (sampling.PolicyEvaluator, error) {
	settings.Logger = settings.Logger.With(zap.Any("policy", cfg.Type))

	switch cfg.Type {
	case AlwaysSample:
		return sampling.NewAlwaysSample(settings), nil
	case Latency:
		lfCfg := cfg.LatencyCfg
		return sampling.NewLatency(settings, lfCfg.ThresholdMs, lfCfg.UpperThresholdmsMs), nil
	case NumericAttribute:
		nafCfg := cfg.NumericAttributeCfg
		return sampling.NewNumericAttributeFilter(settings, nafCfg.Key, nafCfg.MinValue, nafCfg.MaxValue, nafCfg.InvertMatch), nil
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
		return nil, fmt.Errorf("unknown sampling policy type %s", cfg.Type)
	}
}

type policyMetrics struct {
	idNotFoundOnMapCount, evaluateErrorCount, decisionSampled, decisionNotSampled int64
}

func (tsp *tailSamplingSpanProcessor) samplingPolicyOnTick() {
	metrics := policyMetrics{}

	startTime := time.Now()
	batch, _ := tsp.decisionBatcher.CloseCurrentAndTakeFirstBatch()
	batchLen := len(batch)
	tsp.logger.Debug("Sampling Policy Evaluation ticked")
	for _, id := range batch {
		d, ok := tsp.idToTrace.Load(id)
		if !ok {
			metrics.idNotFoundOnMapCount++
			continue
		}
		trace := d.(*sampling.TraceData)
		trace.DecisionTime = time.Now()

		decision := tsp.makeDecision(id, trace, &metrics)
		tsp.telemetry.ProcessorTailSamplingSamplingDecisionTimerLatency.Record(tsp.ctx, int64(time.Since(startTime)/time.Microsecond))
		tsp.telemetry.ProcessorTailSamplingSamplingTraceDroppedTooEarly.Add(tsp.ctx, metrics.idNotFoundOnMapCount)
		tsp.telemetry.ProcessorTailSamplingSamplingPolicyEvaluationError.Add(tsp.ctx, metrics.evaluateErrorCount)
		tsp.telemetry.ProcessorTailSamplingSamplingTracesOnMemory.Record(tsp.ctx, int64(tsp.numTracesOnMap.Load()))
		tsp.telemetry.ProcessorTailSamplingGlobalCountTracesSampled.Add(tsp.ctx, 1, decisionToAttribute[decision])

		// Sampled or not, remove the batches
		trace.Lock()
		allSpans := trace.ReceivedBatches
		trace.FinalDecision = decision
		trace.ReceivedBatches = ptrace.NewTraces()
		trace.Unlock()

		if decision == sampling.Sampled {
			tsp.releaseSampledTrace(context.Background(), id, allSpans)
		}
	}

	tsp.logger.Debug("Sampling policy evaluation completed",
		zap.Int("batch.len", batchLen),
		zap.Int64("sampled", metrics.decisionSampled),
		zap.Int64("notSampled", metrics.decisionNotSampled),
		zap.Int64("droppedPriorToEvaluation", metrics.idNotFoundOnMapCount),
		zap.Int64("policyEvaluationErrors", metrics.evaluateErrorCount),
	)
}

func (tsp *tailSamplingSpanProcessor) makeDecision(id pcommon.TraceID, trace *sampling.TraceData, metrics *policyMetrics) sampling.Decision {
	finalDecision := sampling.NotSampled
	samplingDecision := map[sampling.Decision]bool{
		sampling.Error:            false,
		sampling.Sampled:          false,
		sampling.NotSampled:       false,
		sampling.InvertSampled:    false,
		sampling.InvertNotSampled: false,
	}

	ctx := context.Background()
	// Check all policies before making a final decision
	for _, p := range tsp.policies {
		policyEvaluateStartTime := time.Now()
		decision, err := p.evaluator.Evaluate(ctx, id, trace)
		tsp.telemetry.ProcessorTailSamplingSamplingDecisionLatency.Record(ctx, int64(time.Since(policyEvaluateStartTime)/time.Microsecond), p.attribute)
		if err != nil {
			samplingDecision[sampling.Error] = true
			metrics.evaluateErrorCount++
			tsp.logger.Debug("Sampling policy error", zap.Error(err))
		} else {
			tsp.telemetry.ProcessorTailSamplingCountTracesSampled.Add(ctx, 1, p.attribute, decisionToAttribute[decision])
			if telemetry.IsMetricStatCountSpansSampledEnabled() {
				tsp.telemetry.ProcessorTailSamplingCountSpansSampled.Add(ctx, trace.SpanCount.Load(), p.attribute, decisionToAttribute[decision])
			}

			samplingDecision[decision] = true
		}
	}

	// InvertNotSampled takes precedence over any other decision
	switch {
	case samplingDecision[sampling.InvertNotSampled]:
		finalDecision = sampling.NotSampled
	case samplingDecision[sampling.Sampled]:
		finalDecision = sampling.Sampled
	case samplingDecision[sampling.InvertSampled] && !samplingDecision[sampling.NotSampled]:
		finalDecision = sampling.Sampled
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
		// If the trace ID is in the sampled cache, short circuit the decision
		if _, ok := tsp.sampledIDCache.Get(id); ok {
			traceTd := ptrace.NewTraces()
			appendToTraces(traceTd, resourceSpans, spans)
			tsp.releaseSampledTrace(tsp.ctx, id, traceTd)
			tsp.telemetry.ProcessorTailSamplingEarlyReleasesFromCacheDecision.Add(tsp.ctx, int64(len(spans)))
			continue
		}

		lenSpans := int64(len(spans))
		lenPolicies := len(tsp.policies)
		initialDecisions := make([]sampling.Decision, lenPolicies)
		for i := 0; i < lenPolicies; i++ {
			initialDecisions[i] = sampling.Pending
		}
		d, loaded := tsp.idToTrace.Load(id)
		if !loaded {
			spanCount := &atomic.Int64{}
			spanCount.Store(lenSpans)
			d, loaded = tsp.idToTrace.LoadOrStore(id, &sampling.TraceData{
				ArrivalTime:     time.Now(),
				SpanCount:       spanCount,
				ReceivedBatches: ptrace.NewTraces(),
			})
		}
		actualData := d.(*sampling.TraceData)
		if loaded {
			actualData.SpanCount.Add(lenSpans)
		} else {
			newTraceIDs++
			tsp.decisionBatcher.AddToCurrentBatch(id)
			tsp.numTracesOnMap.Add(1)
			postDeletion := false
			currTime := time.Now()
			for !postDeletion {
				select {
				case tsp.deleteChan <- id:
					postDeletion = true
				default:
					traceKeyToDrop := <-tsp.deleteChan
					tsp.dropTrace(traceKeyToDrop, currTime)
				}
			}
		}

		// The only thing we really care about here is the final decision.
		actualData.Lock()
		finalDecision := actualData.FinalDecision

		if finalDecision == sampling.Unspecified {
			// If the final decision hasn't been made, add the new spans under the lock.
			appendToTraces(actualData.ReceivedBatches, resourceSpans, spans)
			actualData.Unlock()
		} else {
			actualData.Unlock()

			switch finalDecision {
			case sampling.Sampled:
				// Forward the spans to the policy destinations
				traceTd := ptrace.NewTraces()
				appendToTraces(traceTd, resourceSpans, spans)
				tsp.releaseSampledTrace(tsp.ctx, id, traceTd)
			case sampling.NotSampled:
				tsp.telemetry.ProcessorTailSamplingSamplingLateSpanAge.Record(tsp.ctx, int64(time.Since(actualData.DecisionTime)/time.Second))
			default:
				tsp.logger.Warn("Encountered unexpected sampling decision",
					zap.Int("decision", int(finalDecision)))
			}
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
	var trace *sampling.TraceData
	if d, ok := tsp.idToTrace.Load(traceID); ok {
		trace = d.(*sampling.TraceData)
		tsp.idToTrace.Delete(traceID)
		// Subtract one from numTracesOnMap per https://godoc.org/sync/atomic#AddUint64
		tsp.numTracesOnMap.Add(^uint64(0))
	}
	if trace == nil {
		tsp.logger.Debug("Attempt to delete traceID not on table")
		return
	}

	tsp.telemetry.ProcessorTailSamplingSamplingTraceRemovalAge.Record(tsp.ctx, int64(deletionTime.Sub(trace.ArrivalTime)/time.Second))
}

// releaseSampledTrace sends the trace data to the next consumer.
// It additionally adds the trace ID to the cache of sampled trace IDs.
// It does not (yet) delete the spans from the internal map.
func (tsp *tailSamplingSpanProcessor) releaseSampledTrace(ctx context.Context, id pcommon.TraceID, td ptrace.Traces) {
	tsp.sampledIDCache.Put(id, true)
	if err := tsp.nextConsumer.ConsumeTraces(ctx, td); err != nil {
		tsp.logger.Warn(
			"Error sending spans to destination",
			zap.Error(err))
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
