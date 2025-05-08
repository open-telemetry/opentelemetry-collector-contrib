// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"

import (
	"context"
	"errors"
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/cache"
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

	set       processor.Settings
	telemetry *metadata.TelemetryBuilder
	logger    *zap.Logger

	nextConsumer      consumer.Traces
	maxNumTraces      uint64
	policies          []*policy
	idToTrace         sync.Map
	policyTicker      timeutils.TTicker
	tickerFrequency   time.Duration
	decisionBatcher   idbatcher.Batcher
	sampledIDCache    cache.Cache[bool]
	nonSampledIDCache cache.Cache[bool]
	deleteChan        chan pcommon.TraceID
	numTracesOnMap    *atomic.Uint64
	recordPolicy      bool
	setPolicyMux      sync.Mutex
	pendingPolicy     []PolicyCfg
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
		sampling.Dropped:          attrSampledFalse,
	}
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
		ctx:               ctx,
		set:               set,
		telemetry:         telemetry,
		nextConsumer:      nextConsumer,
		maxNumTraces:      cfg.NumTraces,
		sampledIDCache:    sampledDecisions,
		nonSampledIDCache: nonSampledDecisions,
		logger:            telemetrySettings.Logger,
		numTracesOnMap:    &atomic.Uint64{},
		deleteChan:        make(chan pcommon.TraceID, cfg.NumTraces),
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

func getPolicyEvaluator(settings component.TelemetrySettings, cfg *PolicyCfg) (sampling.PolicyEvaluator, error) {
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
		minValue := nafCfg.MinValue
		maxValue := nafCfg.MaxValue
		return sampling.NewNumericAttributeFilter(settings, nafCfg.Key, &minValue, &maxValue, nafCfg.InvertMatch), nil
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

func (tsp *tailSamplingSpanProcessor) loadSamplingPolicy(cfgs []PolicyCfg) error {
	telemetrySettings := tsp.set.TelemetrySettings
	componentID := tsp.set.ID.Name()

	cLen := len(cfgs)
	policies := make([]*policy, 0, cLen)
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

		policies = append(policies, &policy{
			name:      cfg.Name,
			evaluator: eval,
			attribute: metric.WithAttributes(attribute.String("policy", uniquePolicyName)),
		})
	}

	tsp.policies = policies

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
		trace := d.(*sampling.TraceData)
		trace.DecisionTime = time.Now()

		decision := tsp.makeDecision(id, trace, &metrics)

		tsp.telemetry.ProcessorTailSamplingGlobalCountTracesSampled.Add(tsp.ctx, 1, decisionToAttribute[decision])

		// Sampled or not, remove the batches
		trace.Lock()
		allSpans := trace.ReceivedBatches
		trace.FinalDecision = decision
		trace.ReceivedBatches = ptrace.NewTraces()
		trace.Unlock()

		switch decision {
		case sampling.Sampled:
			tsp.releaseSampledTrace(ctx, id, allSpans)
		case sampling.NotSampled:
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

func (tsp *tailSamplingSpanProcessor) makeDecision(id pcommon.TraceID, trace *sampling.TraceData, metrics *policyMetrics) sampling.Decision {
	finalDecision := sampling.NotSampled
	samplingDecisions := map[sampling.Decision]*policy{
		sampling.Error:            nil,
		sampling.Sampled:          nil,
		sampling.NotSampled:       nil,
		sampling.InvertSampled:    nil,
		sampling.InvertNotSampled: nil,
		sampling.Dropped:          nil,
	}

	ctx := context.Background()
	startTime := time.Now()

	// Check all policies before making a final decision.
	for _, p := range tsp.policies {
		decision, err := p.evaluator.Evaluate(ctx, id, trace)
		latency := time.Since(startTime)
		tsp.telemetry.ProcessorTailSamplingSamplingDecisionLatency.Record(ctx, int64(latency/time.Microsecond), p.attribute)

		if err != nil {
			if samplingDecisions[sampling.Error] == nil {
				samplingDecisions[sampling.Error] = p
			}
			metrics.evaluateErrorCount++
			tsp.logger.Debug("Sampling policy error", zap.Error(err))
			continue
		}

		tsp.telemetry.ProcessorTailSamplingCountTracesSampled.Add(ctx, 1, p.attribute, decisionToAttribute[decision])

		if telemetry.IsMetricStatCountSpansSampledEnabled() {
			tsp.telemetry.ProcessorTailSamplingCountSpansSampled.Add(ctx, trace.SpanCount.Load(), p.attribute, decisionToAttribute[decision])
		}

		// We associate the first policy with the sampling decision to understand what policy sampled a span
		if samplingDecisions[decision] == nil {
			samplingDecisions[decision] = p
		}

		// Break early if dropped. This can drastically reduce tick/decision latency.
		if decision == sampling.Dropped {
			break
		}
	}

	var sampledPolicy *policy

	switch {
	case samplingDecisions[sampling.Dropped] != nil: // Dropped takes precedence
		finalDecision = sampling.NotSampled
	case samplingDecisions[sampling.InvertNotSampled] != nil: // Then InvertNotSampled
		finalDecision = sampling.NotSampled
	case samplingDecisions[sampling.Sampled] != nil:
		finalDecision = sampling.Sampled
		sampledPolicy = samplingDecisions[sampling.Sampled]
	case samplingDecisions[sampling.InvertSampled] != nil && samplingDecisions[sampling.NotSampled] == nil:
		finalDecision = sampling.Sampled
		sampledPolicy = samplingDecisions[sampling.InvertSampled]
	}

	if tsp.recordPolicy && sampledPolicy != nil {
		sampling.SetAttrOnScopeSpans(trace, "tailsampling.policy", sampledPolicy.name)
	}

	switch finalDecision {
	case sampling.Sampled:
		metrics.decisionSampled++
	case sampling.NotSampled:
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
			tsp.releaseSampledTrace(tsp.ctx, id, traceTd)
			metric.WithAttributeSet(attribute.NewSet())
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

			td := &sampling.TraceData{
				ArrivalTime:     currTime,
				SpanCount:       spanCount,
				ReceivedBatches: ptrace.NewTraces(),
			}

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
						tsp.dropTrace(traceKeyToDrop, currTime)
					}
				}
			}
		}

		actualData := d.(*sampling.TraceData)
		if loaded {
			actualData.SpanCount.Add(lenSpans)
		}

		actualData.Lock()
		finalDecision := actualData.FinalDecision

		if finalDecision == sampling.Unspecified {
			// If the final decision hasn't been made, add the new spans under the lock.
			appendToTraces(actualData.ReceivedBatches, resourceSpans, spans)
			actualData.Unlock()
			continue
		}

		actualData.Unlock()

		switch finalDecision {
		case sampling.Sampled:
			traceTd := ptrace.NewTraces()
			appendToTraces(traceTd, resourceSpans, spans)
			tsp.releaseSampledTrace(tsp.ctx, id, traceTd)
		case sampling.NotSampled:
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
		tsp.logger.Debug("Attempt to delete trace ID not on table", zap.Stringer("id", traceID))
		return
	}

	tsp.telemetry.ProcessorTailSamplingSamplingTraceRemovalAge.Record(tsp.ctx, int64(deletionTime.Sub(trace.ArrivalTime)/time.Second))
}

// releaseSampledTrace sends the trace data to the next consumer. It
// additionally adds the trace ID to the cache of sampled trace IDs. If the
// trace ID is cached, it deletes the spans from the internal map.
func (tsp *tailSamplingSpanProcessor) releaseSampledTrace(ctx context.Context, id pcommon.TraceID, td ptrace.Traces) {
	tsp.sampledIDCache.Put(id, true)
	if err := tsp.nextConsumer.ConsumeTraces(ctx, td); err != nil {
		tsp.logger.Warn(
			"Error sending spans to destination",
			zap.Error(err))
	}
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
