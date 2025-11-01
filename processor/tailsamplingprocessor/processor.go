// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"slices"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/cache"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/idbatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/telemetry"
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

type tailSamplingSpanProcessor struct {
	ctx context.Context

	set       processor.Settings
	telemetry *metadata.TelemetryBuilder
	logger    *zap.Logger

	deleteTraceQueue   *list.List
	nextConsumer       consumer.Traces
	policies           []*policy
	idToTrace          map[pcommon.TraceID]*samplingpolicy.TraceData
	tickerFrequency    time.Duration
	decisionBatcher    idbatcher.Batcher
	sampledIDCache     cache.Cache[bool]
	nonSampledIDCache  cache.Cache[bool]
	recordPolicy       bool
	sampleOnFirstMatch bool
	blockOnOverflow    bool

	cfg  Config
	host component.Host

	newPolicyChan chan newPolicyCmd
	workChan      chan tracesCmd
	doneChan      chan struct{}

	// tickChan triggers ticks and responds on the provided channel when the tick is complete.
	// this is used in tests to produce deterministic ticks.
	tickChan chan chan struct{}
}

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
		cfg:                cfg,
		set:                set,
		telemetry:          telemetry,
		nextConsumer:       nextConsumer,
		sampledIDCache:     sampledDecisions,
		nonSampledIDCache:  nonSampledDecisions,
		logger:             set.Logger,
		idToTrace:          make(map[pcommon.TraceID]*samplingpolicy.TraceData),
		deleteTraceQueue:   list.New(),
		sampleOnFirstMatch: cfg.SampleOnFirstMatch,
		blockOnOverflow:    cfg.BlockOnOverflow,
		// Similar to the id batcher, allow a batch per CPU to be buffered before blocking ConsumeTraces.
		workChan: make(chan tracesCmd, runtime.NumCPU()),
		// We need to buffer one new policy command so that external callers can
		// queue up new policies without blocking on the ticker.
		newPolicyChan: make(chan newPolicyCmd, 1),
	}

	for _, opt := range cfg.Options {
		opt(tsp)
	}

	if tsp.tickerFrequency == 0 {
		tsp.tickerFrequency = time.Second
	}

	return tsp, nil
}

func (*tailSamplingSpanProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// Start is invoked during service startup.
func (tsp *tailSamplingSpanProcessor) Start(_ context.Context, host component.Host) error {
	tsp.host = host
	policies, err := tsp.loadSamplingPolicies(host, tsp.cfg.PolicyCfgs)
	if err != nil {
		return err
	}

	// If the policies are not set, set them. This is only for testing purposes,
	// so that withPolicies can inject custom policies.
	if tsp.policies == nil {
		tsp.policies = policies
	}

	if tsp.decisionBatcher == nil {
		// this will start a goroutine in the background, so we run it only if everything went
		// well in creating the policies, and only when the processor starts.
		numDecisionBatches := math.Max(1, tsp.cfg.DecisionWait.Seconds())
		inBatcher, err := idbatcher.New(uint64(numDecisionBatches), tsp.cfg.ExpectedNewTracesPerSec, uint64(2*runtime.NumCPU()))
		if err != nil {
			return err
		}
		tsp.decisionBatcher = inBatcher
	}

	tsp.doneChan = make(chan struct{})
	go tsp.loop()
	return nil
}

// ConsumeTraces is required by the processor.Traces interface.
func (tsp *tailSamplingSpanProcessor) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	for _, rss := range td.ResourceSpans().All() {
		batch := tracesCmd{
			rss: rss,
		}
		idToSpansAndScope := groupSpansByTraceKey(rss)
		for traceID, spans := range idToSpansAndScope {
			batch.traces = append(batch.traces, traceInfo{
				id:    traceID,
				spans: spans,
			})
		}
		if len(batch.traces) > 0 {
			tsp.workChan <- batch
		}
	}
	return nil
}

func (tsp *tailSamplingSpanProcessor) SetSamplingPolicy(cfgs []PolicyCfg) {
	policies, err := tsp.loadSamplingPolicies(tsp.host, cfgs)
	if err != nil {
		tsp.logger.Error("Failed to load sampling policies", zap.Error(err))
		return
	}
	tsp.newPolicyChan <- newPolicyCmd{policies: policies}
}

func (tsp *tailSamplingSpanProcessor) loadSamplingPolicies(host component.Host, cfgs []PolicyCfg) ([]*policy, error) {
	telemetrySettings := tsp.set.TelemetrySettings
	componentID := tsp.set.ID.Name()

	cLen := len(cfgs)
	policies := make([]*policy, 0, cLen)
	dropPolicies := make([]*policy, 0, cLen)
	policyNames := make(map[string]struct{}, cLen)

	for i := range cfgs {
		cfg := cfgs[i]
		if cfg.Name == "" {
			return nil, errors.New("policy name cannot be empty")
		}

		if _, exists := policyNames[cfg.Name]; exists {
			return nil, fmt.Errorf("duplicate policy name %q", cfg.Name)
		}
		policyNames[cfg.Name] = struct{}{}

		eval, err := getPolicyEvaluator(telemetrySettings, &cfg, extensions(host))
		if err != nil {
			return nil, fmt.Errorf("failed to create policy evaluator for %q: %w", cfg.Name, err)
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
	return slices.Concat(dropPolicies, policies), nil
}

type tracesCmd struct {
	rss    ptrace.ResourceSpans
	traces []traceInfo
}

type traceInfo struct {
	id    pcommon.TraceID
	spans []spanAndScope
}

type newPolicyCmd struct {
	policies []*policy
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
		return sampling.NewStringAttributeFilter(settings, safCfg.Key, safCfg.Values, safCfg.EnabledRegexMatching, safCfg.CacheMaxSize, safCfg.InvertMatch)
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

func (tsp *tailSamplingSpanProcessor) loop() {
	ticker := time.NewTicker(tsp.tickerFrequency)
	defer ticker.Stop()
	defer close(tsp.doneChan)
	for tsp.iter(ticker.C, tsp.workChan) {
	}
}

func (tsp *tailSamplingSpanProcessor) iter(tickChan <-chan time.Time, workChan <-chan tracesCmd) bool {
	select {
	case <-tsp.ctx.Done():
		// If the context is done then we can't send anything anymore so just exit.
		return false
	case td, ok := <-workChan:
		// No more traces to process as we are shutting down. Clear the queue and make decisions for all batches based on the current data.
		if !ok {
			// Stop the batcher so that we can read all batches without creating new ones.
			tsp.decisionBatcher.Stop()

			// Do the best decision we can for any traces we have already ingested unless a user wants to drop them.
			if !tsp.cfg.DropPendingTracesOnShutdown {
				for tsp.samplingPolicyOnTick() {
				}
			}
			return false
		}

		for _, trace := range td.traces {
			// Short circuit if the trace has already been sampled or dropped.
			if tsp.processCachedTrace(trace.id, td.rss, trace.spans) {
				continue
			}

			_, ok = tsp.idToTrace[trace.id]
			if !ok && len(tsp.idToTrace) >= int(tsp.cfg.NumTraces) {
				tsp.waitForSpace(tickChan)
			}

			tsp.processTrace(td.rss, trace.id, trace.spans)
		}
	case cmd := <-tsp.newPolicyChan:
		tsp.policies = cmd.policies
		tsp.logger.Debug("New policies loaded", zap.Int("policies.len", len(tsp.policies)))
	case <-tickChan:
		tsp.samplingPolicyOnTick()
	case tickChan := <-tsp.tickChan:
		tsp.samplingPolicyOnTick()
		tickChan <- struct{}{}
	}
	return true
}

// processCachedTrace checks if a given trace has already been sampled (or
// dropped) and forwards the span appropriately. It returns true if the trace
// was cached and false if regular processing must be done.
func (tsp *tailSamplingSpanProcessor) processCachedTrace(traceID pcommon.TraceID, rss ptrace.ResourceSpans, spans []spanAndScope) bool {
	if _, ok := tsp.sampledIDCache.Get(traceID); ok {
		tsp.logger.Debug("Trace ID is in the sampled cache", zap.Stringer("id", traceID))
		traceTd := ptrace.NewTraces()
		appendToTraces(traceTd, rss, spans)
		if tsp.recordPolicy {
			sampling.SetBoolAttrOnScopeSpans(traceTd, "tailsampling.cached_decision", true)
		}
		tsp.forwardSpans(tsp.ctx, traceTd)
		tsp.telemetry.ProcessorTailSamplingEarlyReleasesFromCacheDecision.
			Add(tsp.ctx, int64(len(spans)), attrSampledTrue)
		return true
	}

	if _, ok := tsp.nonSampledIDCache.Get(traceID); ok {
		tsp.logger.Debug("Trace ID is in the non-sampled cache", zap.Stringer("id", traceID))
		tsp.telemetry.ProcessorTailSamplingEarlyReleasesFromCacheDecision.
			Add(tsp.ctx, int64(len(spans)), attrSampledFalse)
		return true
	}

	return false
}

// waitForSpace blocks until space is available. Depending on configuration,
// this might immediately drop data or wait until the sampler tick frees space.
func (tsp *tailSamplingSpanProcessor) waitForSpace(tickChan <-chan time.Time) {
	if tsp.blockOnOverflow {
		// Ticks are not guaranteed to drop data, since they may process an
		// empty batch. We loop until we have space for a new trace.
		for len(tsp.idToTrace) >= int(tsp.cfg.NumTraces) {
			// Recursively iter with a nil workChan to wait for space.
			tsp.iter(tickChan, nil)
		}
		return
	}

	front := tsp.deleteTraceQueue.Front()
	if front == nil {
		// This should be impossible.
		tsp.logger.Error("deleteTraceQueue is empty, but we're waiting for space. This is a bug!")
		return
	}
	tsp.dropTrace(front.Value.(pcommon.TraceID), time.Now())
	tsp.deleteTraceQueue.Remove(front)
}

// samplingPolicyOnTick takes the next batch and process all traces in that batch. Returns if there are more batches in the batcher.
func (tsp *tailSamplingSpanProcessor) samplingPolicyOnTick() bool {
	tsp.logger.Debug("Sampling Policy Evaluation ticked")

	ctx := context.Background()
	metrics := newPolicyMetrics(len(tsp.policies))
	startTime := time.Now()

	batch, hasMore := tsp.decisionBatcher.CloseCurrentAndTakeFirstBatch()
	batchLen := len(batch)

	for _, id := range batch {
		trace, ok := tsp.idToTrace[id]
		if !ok {
			metrics.idNotFoundOnMapCount++
			continue
		}
		trace.DecisionTime = time.Now()

		decision := tsp.makeDecision(id, trace, metrics)

		tsp.telemetry.ProcessorTailSamplingGlobalCountTracesSampled.Add(tsp.ctx, 1, decisionToAttributes[decision])

		// Sampled or not, remove the batches
		allSpans := trace.ReceivedBatches
		trace.FinalDecision = decision
		trace.ReceivedBatches = ptrace.NewTraces()

		if decision == samplingpolicy.Sampled {
			tsp.releaseSampledTrace(ctx, id, allSpans)
		} else {
			tsp.releaseNotSampledTrace(id)
		}
	}

	tsp.telemetry.ProcessorTailSamplingSamplingDecisionTimerLatency.Record(tsp.ctx, int64(time.Since(startTime)/time.Millisecond))
	tsp.telemetry.ProcessorTailSamplingSamplingTracesOnMemory.Record(tsp.ctx, int64(len(tsp.idToTrace)))
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
	return hasMore
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

		metrics.addDecision(i, decision, trace.SpanCount)

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

func groupSpansByTraceKey(resourceSpans ptrace.ResourceSpans) map[pcommon.TraceID][]spanAndScope {
	idToSpans := make(map[pcommon.TraceID][]spanAndScope)
	ilss := resourceSpans.ScopeSpans()
	for j := 0; j < ilss.Len(); j++ {
		scope := ilss.At(j)
		spans := scope.Spans()
		is := scope.Scope()
		spansLen := spans.Len()
		for k := range spansLen {
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

func (tsp *tailSamplingSpanProcessor) processTrace(resourceSpans ptrace.ResourceSpans, id pcommon.TraceID, spans []spanAndScope) {
	currTime := time.Now()

	var newTraceIDs int64
	defer func() {
		// Need a closure here to delay evaluation of newTraceIDs.
		tsp.telemetry.ProcessorTailSamplingNewTraceIDReceived.Add(tsp.ctx, newTraceIDs)
	}()

	lenSpans := int64(len(spans))

	actualData, ok := tsp.idToTrace[id]
	if !ok {
		actualData = &samplingpolicy.TraceData{
			ArrivalTime:     currTime,
			SpanCount:       lenSpans,
			ReceivedBatches: ptrace.NewTraces(),
		}

		tsp.idToTrace[id] = actualData

		newTraceIDs++
		tsp.decisionBatcher.AddToCurrentBatch(id)

		if !tsp.blockOnOverflow {
			tsp.deleteTraceQueue.PushBack(id)
		}
	} else {
		actualData.SpanCount += lenSpans
	}

	finalDecision := actualData.FinalDecision

	if finalDecision == samplingpolicy.Unspecified {
		// If the final decision hasn't been made, add the new spans to the
		// existing trace.
		appendToTraces(actualData.ReceivedBatches, resourceSpans, spans)
		return
	}

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

func extensions(host component.Host) map[string]samplingpolicy.Extension {
	if host == nil {
		return nil
	}
	extensions := host.GetExtensions()
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
	// All receivers will be shutdown before processors so no sends will be done anymore.
	close(tsp.workChan)
	if tsp.doneChan != nil {
		<-tsp.doneChan
	}
	return nil
}

func (tsp *tailSamplingSpanProcessor) dropTrace(traceID pcommon.TraceID, deletionTime time.Time) {
	trace, ok := tsp.idToTrace[traceID]
	if !ok {
		tsp.logger.Debug("Attempt to delete trace ID not on table", zap.Stringer("id", traceID))
		return
	}

	delete(tsp.idToTrace, traceID)
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
