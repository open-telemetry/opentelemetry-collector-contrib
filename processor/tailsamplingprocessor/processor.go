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
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/cache"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/idbatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/tailstorageextension"
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
	// isDrop indicates this is a drop policy.
	isDrop bool
}

// TraceData is a wrapper around the publically used samplingpolicy.TraceData
// that tracks information related to the decision making process but not
// needed by any sampler implementations.
type TraceData struct {
	samplingpolicy.TraceData
	FinalDecision samplingpolicy.Decision
	PolicyName    string

	arrivalTime   time.Time
	decisionTime  time.Time
	deleteElement *list.Element
	batchID       uint64
}

type DecisionHook func(ctx context.Context, id pcommon.TraceID, td *TraceData)

type tailSamplingSpanProcessor struct {
	ctx context.Context

	set       processor.Settings
	telemetry *metadata.TelemetryBuilder
	logger    *zap.Logger
	tracer    trace.Tracer

	deleteTraceQueue   *list.List
	nextConsumer       consumer.Traces
	policies           []*policy
	idToTrace          map[pcommon.TraceID]*TraceData
	tailStorage        tailstorageextension.TailStorage
	tickerFrequency    time.Duration
	decisionBatcher    idbatcher.Batcher
	sampledIDCache     cache.Cache
	nonSampledIDCache  cache.Cache
	recordPolicy       bool
	sampleOnFirstMatch bool
	blockOnOverflow    bool
	maxTraceSizeBytes  uint64

	cfg  Config
	host component.Host

	sampledHooks    []DecisionHook
	nonSampledHooks []DecisionHook

	newPolicyChan    chan newPolicyCmd
	newTraceSizeChan chan uint64
	workChan         chan []traceBatch
	doneChan         chan struct{}

	// tickChan triggers ticks and responds on the provided channel when the tick is complete.
	// this is used in tests to produce deterministic ticks.
	tickChan chan chan struct{}
}

func newTracesProcessor(ctx context.Context, set processor.Settings, nextConsumer consumer.Traces, cfg Config) (processor.Traces, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	telemetrySettings := set.TelemetrySettings
	telemetry, err := metadata.NewTelemetryBuilder(telemetrySettings)
	if err != nil {
		return nil, err
	}
	tracer := metadata.Tracer(telemetrySettings)
	nopCache := cache.NewNopDecisionCache()
	sampledDecisions := nopCache
	nonSampledDecisions := nopCache
	if cfg.DecisionCache.SampledCacheSize > 0 {
		sampledDecisions, err = cache.NewLRUDecisionCache(cfg.DecisionCache.SampledCacheSize)
		if err != nil {
			return nil, err
		}
	}
	if cfg.DecisionCache.NonSampledCacheSize > 0 {
		nonSampledDecisions, err = cache.NewLRUDecisionCache(cfg.DecisionCache.NonSampledCacheSize)
		if err != nil {
			return nil, err
		}
	}

	tsp := &tailSamplingSpanProcessor{
		ctx:                ctx,
		cfg:                cfg,
		set:                set,
		telemetry:          telemetry,
		tracer:             tracer,
		nextConsumer:       nextConsumer,
		sampledIDCache:     sampledDecisions,
		nonSampledIDCache:  nonSampledDecisions,
		logger:             set.Logger,
		idToTrace:          make(map[pcommon.TraceID]*TraceData),
		deleteTraceQueue:   list.New(),
		sampleOnFirstMatch: cfg.SampleOnFirstMatch,
		blockOnOverflow:    cfg.BlockOnOverflow,
		maxTraceSizeBytes:  cfg.MaximumTraceSizeBytes,
		// Similar to the id batcher, allow a batch per CPU to be buffered before blocking ConsumeTraces.
		workChan: make(chan []traceBatch, runtime.NumCPU()),
		// We need to buffer one new policy command/size update so that external callers can
		// queue up new policies without blocking on the ticker.
		newPolicyChan:    make(chan newPolicyCmd, 1),
		newTraceSizeChan: make(chan uint64, 1),
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
	if tsp.cfg.TailStorageID != nil {
		tailStorageExt, err := tailStorageExtension(host, *tsp.cfg.TailStorageID)
		if err != nil {
			return err
		}
		tsp.tailStorage = tailStorageExt
	} else {
		tsp.tailStorage = tailstorageextension.NewInMemoryTailStorage()
	}
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
		idBatcher, err := idbatcher.New(uint64(numDecisionBatches), tsp.cfg.ExpectedNewTracesPerSec)
		if err != nil {
			return err
		}
		tsp.decisionBatcher = idBatcher
	}

	tsp.doneChan = make(chan struct{})
	go tsp.loop()
	return nil
}

// ConsumeTraces is required by the processor.Traces interface.
func (tsp *tailSamplingSpanProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	_, span := tsp.tracer.Start(ctx, "tailsampling.ConsumeTraces")
	defer span.End()

	var totalSpans, totalTraces, totalResourceSpans int64

	for _, rss := range td.ResourceSpans().All() {
		totalResourceSpans++
		// First group all spans by trace.
		idToSpansAndScope := groupSpansByTraceKey(rss)

		// Then, create a new ptrace.ResourceSpans for each trace copying all
		// data. Paying this cost upfront allows the inner loop of the TSP to
		// be more efficient on its single goroutine.
		batch := []traceBatch{}
		for traceID, spans := range idToSpansAndScope {
			newRSS, rootSpan := newResourceSpanFromSpanAndScopes(rss, spans)
			batch = append(batch, traceBatch{
				id:        traceID,
				rootSpan:  rootSpan,
				rss:       newRSS,
				spanCount: int64(len(spans)),
			})
			totalSpans += int64(len(spans))
			totalTraces++
		}
		if len(batch) > 0 {
			tsp.workChan <- batch
		}
	}

	if span.IsRecording() {
		span.SetAttributes(
			attribute.Int64("traces.count", totalTraces),
			attribute.Int64("spans.count", totalSpans),
			attribute.Int64("resource_spans.count", totalResourceSpans),
		)
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
		if tsp.cfg.SamplingStrategy == samplingStrategySpanIngest && eval.IsStateful() {
			return nil, fmt.Errorf("sampling_strategy %q requires all policies to be stateless, but policy %q is stateful", tsp.cfg.SamplingStrategy, cfg.Name)
		}

		uniquePolicyName := cfg.Name
		if componentID != "" {
			uniquePolicyName = fmt.Sprintf("%s.%s", componentID, cfg.Name)
		}

		p := &policy{
			name:      cfg.Name,
			evaluator: eval,
			attribute: metric.WithAttributes(attribute.String("policy", uniquePolicyName)),
			isDrop:    cfg.Type == Drop,
		}

		if p.isDrop {
			dropPolicies = append(dropPolicies, p)
		} else {
			policies = append(policies, p)
		}
	}
	// Dropped decision takes precedence over all others, therefore we evaluate them first.
	return slices.Concat(dropPolicies, policies), nil
}

func (tsp *tailSamplingSpanProcessor) SetMaximumTraceSizeBytes(size uint64) {
	tsp.newTraceSizeChan <- size
}

// traceBatch contains all spans from a single batch for a single trace.
type traceBatch struct {
	id        pcommon.TraceID
	rootSpan  *ptrace.Span
	rss       ptrace.ResourceSpans
	spanCount int64
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
		samplingpolicy.Sampled:    attrDecisionSampled,
		samplingpolicy.NotSampled: attrDecisionNotSampled,
		//nolint:staticcheck // SA1019: Use of inverted decisions until they are fully removed.
		samplingpolicy.InvertNotSampled: attrDecisionNotSampled,
		//nolint:staticcheck // SA1019: Use of inverted decisions until they are fully removed.
		samplingpolicy.InvertSampled: attrDecisionSampled,
		samplingpolicy.Dropped:       attrDecisionDropped,
	}

	attrSampledTrue  = metric.WithAttributes(attribute.String("sampled", "true"))
	attrSampledFalse = metric.WithAttributes(attribute.String("sampled", "false"))
)

type Option func(*tailSamplingSpanProcessor)

// WithSampledDecisionCache sets the cache which the processor uses to store recently sampled trace IDs.
func WithSampledDecisionCache(c cache.Cache) Option {
	return func(tsp *tailSamplingSpanProcessor) {
		tsp.sampledIDCache = c
	}
}

// WithNonSampledDecisionCache sets the cache which the processor uses to store recently non-sampled trace IDs.
func WithNonSampledDecisionCache(c cache.Cache) Option {
	return func(tsp *tailSamplingSpanProcessor) {
		tsp.nonSampledIDCache = c
	}
}

// WithSampledHooks sets hooks to be called when a trace is sampled.
func WithSampledHooks(hooks ...DecisionHook) Option {
	return func(tsp *tailSamplingSpanProcessor) {
		tsp.sampledHooks = hooks
	}
}

// WithNonSampledHooks sets hooks to be called when a trace is not sampled.
func WithNonSampledHooks(hooks ...DecisionHook) Option {
	return func(tsp *tailSamplingSpanProcessor) {
		tsp.nonSampledHooks = hooks
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
	case Not:
		return getNewNotPolicy(settings, &cfg.NotCfg, policyExtensions)
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
		if rlfCfg.BurstCapacity > 0 {
			return sampling.NewRateLimitingWithBurstCapacity(settings, rlfCfg.SpansPerSecond, rlfCfg.BurstCapacity), nil
		}
		return sampling.NewRateLimiting(settings, rlfCfg.SpansPerSecond), nil
	case BytesLimiting:
		blfCfg := cfg.BytesLimitingCfg
		if blfCfg.BurstCapacity > 0 {
			return sampling.NewBytesLimitingWithBurstCapacity(settings, blfCfg.BytesPerSecond, blfCfg.BurstCapacity), nil
		}
		return sampling.NewBytesLimiting(settings, blfCfg.BytesPerSecond), nil
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
	case TraceFlags:
		return sampling.NewTraceFlags(settings), nil
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

type policyEvaluationMetrics struct {
	idNotFoundOnMapCount, evaluateErrorCount, decisionSampled, decisionNotSampled, decisionDropped int64
	tracesSampledByPolicyDecision                                                                  []map[samplingpolicy.Decision]policyDecisionMetrics
	cumulativeExecutionTime                                                                        []perPolicyExecutionTime
}

// perPolicyExecutionTime is a struct for holding the cumulative execution time
// and number of executions of a policy. This is an optimization to avoid
// instrumentation overhead in the decision making loop.
type perPolicyExecutionTime struct {
	executionTime  time.Duration
	executionCount int64
}

func newPolicyEvaluationMetrics(numPolicies int) *policyEvaluationMetrics {
	tracesSampledByPolicyDecision := make([]map[samplingpolicy.Decision]policyDecisionMetrics, numPolicies)
	for i := range tracesSampledByPolicyDecision {
		tracesSampledByPolicyDecision[i] = make(map[samplingpolicy.Decision]policyDecisionMetrics)
	}
	return &policyEvaluationMetrics{
		tracesSampledByPolicyDecision: tracesSampledByPolicyDecision,
		cumulativeExecutionTime:       make([]perPolicyExecutionTime, numPolicies),
	}
}

func (m *policyEvaluationMetrics) addDecision(policyIndex int, decision samplingpolicy.Decision, spansSampled int64) {
	stats := m.tracesSampledByPolicyDecision[policyIndex][decision]
	stats.tracesSampled++
	stats.spansSampled += spansSampled
	m.tracesSampledByPolicyDecision[policyIndex][decision] = stats
}

func (m *policyEvaluationMetrics) addDecisionTime(policyIndex int, decisionTime time.Duration) {
	perPolicyExecutionTime := m.cumulativeExecutionTime[policyIndex]
	perPolicyExecutionTime.executionTime += decisionTime
	perPolicyExecutionTime.executionCount++
	m.cumulativeExecutionTime[policyIndex] = perPolicyExecutionTime
}

func (tsp *tailSamplingSpanProcessor) recordPerPolicyEvaluationMetrics(metrics *policyEvaluationMetrics) {
	for i, p := range tsp.policies {
		for decision, stats := range metrics.tracesSampledByPolicyDecision[i] {
			tsp.telemetry.ProcessorTailSamplingCountTracesSampled.Add(tsp.ctx, int64(stats.tracesSampled), p.attribute, decisionToAttributes[decision])
			if telemetry.IsMetricStatCountSpansSampledEnabled() {
				tsp.telemetry.ProcessorTailSamplingCountSpansSampled.Add(tsp.ctx, stats.spansSampled, p.attribute, decisionToAttributes[decision])
			}
		}
		tsp.telemetry.ProcessorTailSamplingSamplingPolicyExecutionTimeSum.Add(tsp.ctx, metrics.cumulativeExecutionTime[i].executionTime.Microseconds(), p.attribute)
		tsp.telemetry.ProcessorTailSamplingSamplingPolicyExecutionCount.Add(tsp.ctx, metrics.cumulativeExecutionTime[i].executionCount, p.attribute)
	}
}

func (tsp *tailSamplingSpanProcessor) recordImmediateDecisionMetrics(decision samplingpolicy.Decision, metrics *policyEvaluationMetrics, evaluationLatency time.Duration) {
	tsp.telemetry.ProcessorTailSamplingSamplingDecisionTimerLatency.Record(tsp.ctx, evaluationLatency.Milliseconds())
	tsp.telemetry.ProcessorTailSamplingSamplingPolicyEvaluationError.Add(tsp.ctx, metrics.evaluateErrorCount)

	if attrs, ok := decisionToAttributes[decision]; ok {
		tsp.telemetry.ProcessorTailSamplingGlobalCountTracesSampled.Add(tsp.ctx, 1, attrs)
	}

	tsp.recordPerPolicyEvaluationMetrics(metrics)
}

func (tsp *tailSamplingSpanProcessor) loop() {
	ticker := time.NewTicker(tsp.tickerFrequency)
	defer ticker.Stop()
	defer close(tsp.doneChan)
	for tsp.iter(ticker.C, tsp.workChan) {
	}
}

func (tsp *tailSamplingSpanProcessor) iter(tickChan <-chan time.Time, workChan <-chan []traceBatch) bool {
	select {
	case <-tsp.ctx.Done():
		// If the context is done then we can't send anything anymore so just exit.
		return false
	case batch, ok := <-workChan:
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

		for _, trace := range batch {
			// Short circuit if the trace has already been sampled or dropped.
			if tsp.processCachedTrace(trace.id, trace.rss, trace.spanCount) {
				continue
			}

			_, ok = tsp.idToTrace[trace.id]
			if !ok && uint64(len(tsp.idToTrace)) >= tsp.cfg.NumTraces {
				tsp.waitForSpace(tickChan)
			}

			tsp.processTrace(trace.id, trace.rss, trace.spanCount, trace.rootSpan != nil)
		}
	case cmd := <-tsp.newPolicyChan:
		tsp.policies = cmd.policies
		tsp.logger.Debug("New policies loaded", zap.Int("policies.len", len(tsp.policies)))
	case maxTraceSize := <-tsp.newTraceSizeChan:
		tsp.maxTraceSizeBytes = maxTraceSize
		tsp.logger.Debug("New maximum trace size loaded", zap.Uint64("size", maxTraceSize))
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
func (tsp *tailSamplingSpanProcessor) processCachedTrace(traceID pcommon.TraceID, rss ptrace.ResourceSpans, spanCount int64) bool {
	if metadata, ok := tsp.sampledIDCache.Get(traceID); ok {
		tsp.logger.Debug("Trace ID is in the sampled cache", zap.Stringer("id", traceID))
		traceTd := ptrace.NewTraces()
		appendToTraces(traceTd, rss)
		if tsp.recordPolicy {
			if metadata.PolicyName != "" {
				sampling.SetAttrOnScopeSpans(traceTd, "tailsampling.policy", metadata.PolicyName)
			}
			sampling.SetBoolAttrOnScopeSpans(traceTd, "tailsampling.cached_decision", true)
		}
		tsp.forwardSpans(tsp.ctx, traceTd)
		tsp.telemetry.ProcessorTailSamplingEarlyReleasesFromCacheDecision.
			Add(tsp.ctx, spanCount, attrSampledTrue)
		return true
	}

	if _, ok := tsp.nonSampledIDCache.Get(traceID); ok {
		tsp.logger.Debug("Trace ID is in the non-sampled cache", zap.Stringer("id", traceID))
		tsp.telemetry.ProcessorTailSamplingEarlyReleasesFromCacheDecision.
			Add(tsp.ctx, spanCount, attrSampledFalse)
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
		for uint64(len(tsp.idToTrace)) >= tsp.cfg.NumTraces {
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
	if !tsp.dropTrace(front.Value.(pcommon.TraceID), time.Now()) {
		// Somehow the trace was already removed from idToTrace, but not the
		// queue. Drop the element to avoid an infinite loop.
		tsp.deleteTraceQueue.Remove(front)
	}
}

// samplingPolicyOnTick takes the next batch and process all traces in that batch. Returns if there are more batches in the batcher.
func (tsp *tailSamplingSpanProcessor) samplingPolicyOnTick() bool {
	tsp.logger.Debug("Sampling Policy Evaluation ticked")

	metrics := newPolicyEvaluationMetrics(len(tsp.policies))
	startTime := time.Now()
	globalTracesSampledByDecision := make(map[samplingpolicy.Decision]int64)

	batch, hasMore := tsp.decisionBatcher.CloseCurrentAndTakeFirstBatch()
	batchLen := len(batch)
	var span trace.Span
	ctx := context.Background()
	if batchLen > 0 {
		ctx, span = tsp.tracer.Start(ctx, "tailsampling.samplingPolicyOnTick")
		defer span.End()
	}

	for id := range batch {
		trace, ok := tsp.idToTrace[id]
		if !ok {
			metrics.idNotFoundOnMapCount++
			continue
		}
		// A decision was already made, no need to do it again. This happens
		// when no decision cache is used and a trace was processed both due to
		// a root span trigger and after decision_wait.
		if trace.FinalDecision != samplingpolicy.Unspecified {
			continue
		}

		// In span-ingest mode, tick is a cleanup path only. Finalize any
		// still-pending trace as implicit not sampled without policy evaluation.
		if tsp.cfg.SamplingStrategy == samplingStrategySpanIngest {
			trace.decisionTime = time.Now()
			trace.FinalDecision = samplingpolicy.NotSampled
			globalTracesSampledByDecision[samplingpolicy.NotSampled]++
			metrics.decisionNotSampled++
			tsp.releaseNotSampledTrace(id, trace)
			trace.ReceivedBatches = ptrace.NewTraces()
			continue
		}

		allSpans, ok := tsp.tailStorage.Take(id)
		if !ok {
			metrics.idNotFoundOnMapCount++
			continue
		}

		trace.decisionTime = time.Now()
		traceForDecision := samplingpolicy.TraceData{
			SpanCount:       trace.SpanCount,
			ReceivedBatches: allSpans,
		}
		decision, policyName := tsp.makeDecision(ctx, id, &traceForDecision, metrics)
		globalTracesSampledByDecision[decision]++
		// Keep release paths working with tail storage by attaching the
		// retrieved batches back to trace state for this decision.
		trace.ReceivedBatches = traceForDecision.ReceivedBatches
		trace.FinalDecision = decision
		trace.PolicyName = policyName

		if decision == samplingpolicy.Sampled {
			tsp.releaseSampledTrace(ctx, id, trace)
		} else {
			tsp.releaseNotSampledTrace(id, trace)
		}

		// Sampled or not, remove the batches
		trace.ReceivedBatches = ptrace.NewTraces()
	}

	tsp.telemetry.ProcessorTailSamplingSamplingDecisionTimerLatency.Record(tsp.ctx, time.Since(startTime).Milliseconds())
	tsp.telemetry.ProcessorTailSamplingSamplingTracesOnMemory.Record(tsp.ctx, int64(len(tsp.idToTrace)))
	tsp.telemetry.ProcessorTailSamplingSamplingTraceDroppedTooEarly.Add(tsp.ctx, metrics.idNotFoundOnMapCount)
	tsp.telemetry.ProcessorTailSamplingSamplingPolicyEvaluationError.Add(tsp.ctx, metrics.evaluateErrorCount)

	for decision, count := range globalTracesSampledByDecision {
		tsp.telemetry.ProcessorTailSamplingGlobalCountTracesSampled.Add(tsp.ctx, count, decisionToAttributes[decision])
	}
	tsp.recordPerPolicyEvaluationMetrics(metrics)

	if span != nil && span.IsRecording() {
		span.SetAttributes(
			attribute.Int("batch.size", batchLen),
			attribute.Int("policy.count", len(tsp.policies)),
			attribute.Int64("policy.errors", metrics.evaluateErrorCount),
			attribute.Int64("decisions.sampled", metrics.decisionSampled),
			attribute.Int64("decisions.not_sampled", metrics.decisionNotSampled),
			attribute.Int64("decisions.dropped", metrics.decisionDropped),
		)
	}

	if metrics.evaluateErrorCount > 0 && span != nil && span.IsRecording() {
		span.SetStatus(codes.Error, fmt.Sprintf("%d policy evaluation errors", metrics.evaluateErrorCount))
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

func (tsp *tailSamplingSpanProcessor) makeDecision(ctx context.Context, id pcommon.TraceID, traceData *samplingpolicy.TraceData, metrics *policyEvaluationMetrics) (samplingpolicy.Decision, string) {
	finalDecision := samplingpolicy.NotSampled
	samplingDecisions := map[samplingpolicy.Decision]*policy{
		samplingpolicy.Error:      nil,
		samplingpolicy.Sampled:    nil,
		samplingpolicy.NotSampled: nil,
		//nolint:staticcheck // SA1019: Use of inverted decisions until they are fully removed.
		samplingpolicy.InvertSampled: nil,
		//nolint:staticcheck // SA1019: Use of inverted decisions until they are fully removed.
		samplingpolicy.InvertNotSampled: nil,
		samplingpolicy.Dropped:          nil,
	}

	// Check all policies before making a final decision.
	for i, p := range tsp.policies {
		startTime := time.Now()
		decision, err := p.evaluator.Evaluate(ctx, id, traceData)
		metrics.addDecisionTime(i, time.Since(startTime))

		if err != nil {
			if samplingDecisions[samplingpolicy.Error] == nil {
				samplingDecisions[samplingpolicy.Error] = p
			}
			metrics.evaluateErrorCount++
			tsp.logger.Debug("Sampling policy error", zap.Error(err))
			continue
		}

		metrics.addDecision(i, decision, traceData.SpanCount)

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
		sampledPolicy = samplingDecisions[samplingpolicy.Dropped]
	//nolint:staticcheck // SA1019: Use of inverted decisions until they are fully removed.
	case samplingDecisions[samplingpolicy.InvertNotSampled] != nil: // Then InvertNotSampled
		finalDecision = samplingpolicy.NotSampled
	case samplingDecisions[samplingpolicy.Sampled] != nil:
		finalDecision = samplingpolicy.Sampled
		sampledPolicy = samplingDecisions[samplingpolicy.Sampled]
	//nolint:staticcheck // SA1019: Use of inverted decisions until they are fully removed.
	case samplingDecisions[samplingpolicy.InvertSampled] != nil && samplingDecisions[samplingpolicy.NotSampled] == nil:
		finalDecision = samplingpolicy.Sampled
		//nolint:staticcheck // SA1019: Use of inverted decisions until they are fully removed.
		sampledPolicy = samplingDecisions[samplingpolicy.InvertSampled]
	}

	if tsp.recordPolicy && sampledPolicy != nil && finalDecision == samplingpolicy.Sampled {
		sampling.SetAttrOnScopeSpans(traceData.ReceivedBatches, "tailsampling.policy", sampledPolicy.name)
	}

	switch finalDecision {
	case samplingpolicy.Sampled:
		metrics.decisionSampled++
	case samplingpolicy.NotSampled:
		metrics.decisionNotSampled++
	case samplingpolicy.Dropped:
		metrics.decisionDropped++
	}

	return finalDecision, getPolicyName(sampledPolicy)
}

// makeDecisionOnSpanIngest is used by span-ingest mode. It only returns
// terminal decisions at ingest time. All other outcomes remain pending.
func (tsp *tailSamplingSpanProcessor) makeDecisionOnSpanIngest(id pcommon.TraceID, trace *samplingpolicy.TraceData, metrics *policyEvaluationMetrics) (samplingpolicy.Decision, string) {
	ctx := context.Background()
	// Track whether a drop policy didn't match this batch. A drop policy
	// returning NotSampled means it didn't match these spans but could still
	// match future spans for this trace, so we must defer a Sampled decision
	// to avoid forwarding spans that should ultimately be dropped.
	dropPolicyPending := false
	for i, p := range tsp.policies {
		// If we have moved past all drop policies and one is pending there is
		// no use wasting work on evaluation when we can't sample the trace.
		if !p.isDrop && dropPolicyPending {
			break
		}

		startTime := time.Now()
		decision, err := p.evaluator.Evaluate(ctx, id, trace)
		metrics.addDecisionTime(i, time.Since(startTime))

		if err != nil {
			metrics.evaluateErrorCount++
			tsp.logger.Debug("Sampling policy error", zap.Error(err))
			continue
		}

		metrics.addDecision(i, decision, trace.SpanCount)

		if decision == samplingpolicy.Dropped {
			metrics.decisionDropped++
			return samplingpolicy.Dropped, p.name
		}

		if p.isDrop {
			// Treat all other results as pending as the result could change.
			dropPolicyPending = true
			continue
		}

		// Since drop policies are always sorted to the top we know dropPolicyPending is accurate.
		if decision == samplingpolicy.Sampled && !dropPolicyPending {
			metrics.decisionSampled++
			if tsp.recordPolicy {
				sampling.SetAttrOnScopeSpans(trace.ReceivedBatches, "tailsampling.policy", p.name)
			}
			return samplingpolicy.Sampled, p.name
		}
	}
	return samplingpolicy.Pending, ""
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

func (tsp *tailSamplingSpanProcessor) processTrace(id pcommon.TraceID, rss ptrace.ResourceSpans, spanCount int64, containsRootSpan bool) {
	currTime := time.Now()

	var newTraceIDs int64
	defer func() {
		// Need a closure here to delay evaluation of newTraceIDs.
		tsp.telemetry.ProcessorTailSamplingNewTraceIDReceived.Add(tsp.ctx, newTraceIDs)
	}()

	actualData, ok := tsp.idToTrace[id]
	if !ok {
		actualData = &TraceData{
			arrivalTime: currTime,
			TraceData: samplingpolicy.TraceData{
				SpanCount:       spanCount,
				ReceivedBatches: ptrace.NewTraces(),
			},
		}

		tsp.idToTrace[id] = actualData

		newTraceIDs++
		actualData.batchID = tsp.decisionBatcher.AddToCurrentBatch(id)

		if !tsp.blockOnOverflow {
			actualData.deleteElement = tsp.deleteTraceQueue.PushBack(id)
		}
	} else {
		actualData.SpanCount += spanCount
	}
	if containsRootSpan && tsp.cfg.DecisionWaitAfterRootReceived > 0 {
		actualData.batchID = tsp.decisionBatcher.MoveToEarlierBatch(id, actualData.batchID, uint64(tsp.cfg.DecisionWaitAfterRootReceived.Seconds()))
	}

	finalDecision := actualData.FinalDecision

	marshaler := &ptrace.ProtoMarshaler{}
	actualData.SizeBytes += uint64(marshaler.ResourceSpansSize(rss))

	if finalDecision == samplingpolicy.Unspecified &&
		tsp.maxTraceSizeBytes > 0 &&
		actualData.SizeBytes > tsp.maxTraceSizeBytes {
		tsp.telemetry.ProcessorTailSamplingTracesDroppedTooLarge.Add(tsp.ctx, 1)
		actualData.FinalDecision = samplingpolicy.NotSampled
		// Since we are not in a normal decision flow when dropping large traces, also be sure to remove it from the batcher.
		tsp.decisionBatcher.RemoveFromBatch(id, actualData.batchID)
		tsp.releaseNotSampledTrace(id, actualData)
		return
	}

	if finalDecision == samplingpolicy.Unspecified {
		if tsp.cfg.SamplingStrategy == samplingStrategySpanIngest {
			metrics := newPolicyEvaluationMetrics(len(tsp.policies))
			evaluationStart := time.Now()
			actualData.decisionTime = evaluationStart
			// Build an isolated one-batch trace view for ingest-time evaluation
			// using moves to avoid deep-copying span data.
			spanIngestTraceData := samplingpolicy.TraceData{
				SpanCount:       actualData.SpanCount,
				ReceivedBatches: ptrace.NewTraces(),
			}
			appendToTraces(spanIngestTraceData.ReceivedBatches, rss)
			decision, policyName := tsp.makeDecisionOnSpanIngest(id, &spanIngestTraceData, metrics)
			tsp.recordImmediateDecisionMetrics(decision, metrics, time.Since(evaluationStart))

			if decision == samplingpolicy.Sampled || decision == samplingpolicy.Dropped {
				// Release all accumulated spans (prior pending batches + current batch)
				// without writing the current batch to storage first.
				merged := ptrace.NewTraces()
				if allSpans, ok := tsp.tailStorage.Take(id); ok {
					appendAllTraces(merged, allSpans)
				}
				appendAllTraces(merged, spanIngestTraceData.ReceivedBatches)
				actualData.ReceivedBatches = merged

				actualData.FinalDecision = decision
				actualData.PolicyName = policyName
				if decision == samplingpolicy.Sampled {
					tsp.releaseSampledTrace(tsp.ctx, id, actualData)
				} else {
					tsp.releaseNotSampledTrace(id, actualData)
				}
			} else {
				// Persist current batch for pending traces.
				// Use the moved batch from spanIngestTraceData (rss has been moved).
				for _, rs := range spanIngestTraceData.ReceivedBatches.ResourceSpans().All() {
					tsp.tailStorage.Append(id, rs)
				}
			}
			return
		}

		// If the final decision hasn't been made, add the new spans to the
		// existing trace.
		tsp.tailStorage.Append(id, rss)
		return
	}

	switch finalDecision {
	case samplingpolicy.Sampled:
		traceTd := ptrace.NewTraces()
		appendToTraces(traceTd, rss)
		tsp.forwardSpans(tsp.ctx, traceTd)
	case samplingpolicy.NotSampled:
		// TODO: I don't think this is correct? If it isn't sampled shouldn't we just do nothing?
		tsp.releaseNotSampledTrace(id, actualData)
	default:
		tsp.logger.Warn("Unexpected sampling decision", zap.Int("decision", int(finalDecision)))
	}

	if !actualData.decisionTime.IsZero() {
		tsp.telemetry.ProcessorTailSamplingSamplingLateSpanAge.Record(tsp.ctx, int64(time.Since(actualData.decisionTime)/time.Second))
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

func tailStorageExtension(host component.Host, storageID component.ID) (tailstorageextension.TailStorage, error) {
	if host == nil {
		return nil, errors.New("tail storage extension configured but host is nil")
	}
	extension, ok := host.GetExtensions()[storageID]
	if !ok {
		return nil, fmt.Errorf("tail storage extension '%s' not found", storageID)
	}
	storageExtension, ok := extension.(tailstorageextension.TailStorage)
	if !ok {
		return nil, fmt.Errorf("non-tail-storage extension '%s' found", storageID)
	}
	return storageExtension, nil
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

// dropTrace removes the trace from all memory locations. Returns true if it was removed and false if not found.
func (tsp *tailSamplingSpanProcessor) dropTrace(traceID pcommon.TraceID, deletionTime time.Time) bool {
	trace, ok := tsp.idToTrace[traceID]
	if !ok {
		tsp.logger.Debug("Attempt to delete trace ID not on table", zap.Stringer("id", traceID))
		return false
	}

	delete(tsp.idToTrace, traceID)
	tsp.tailStorage.Delete(traceID)
	if trace.deleteElement != nil {
		tsp.deleteTraceQueue.Remove(trace.deleteElement)
	}

	tsp.telemetry.ProcessorTailSamplingSamplingTraceRemovalAge.Record(tsp.ctx, int64(deletionTime.Sub(trace.arrivalTime)/time.Second))
	return true
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
func (tsp *tailSamplingSpanProcessor) releaseSampledTrace(ctx context.Context, id pcommon.TraceID, td *TraceData) {
	for _, hook := range tsp.sampledHooks {
		hook(ctx, id, td)
	}
	tsp.sampledIDCache.Put(id, cache.DecisionMetadata{PolicyName: td.PolicyName})
	tsp.forwardSpans(ctx, td.ReceivedBatches)
	_, ok := tsp.sampledIDCache.Get(id)
	if ok {
		tsp.dropTrace(id, time.Now())
	}
}

// releaseNotSampledTrace adds the trace ID to the cache of not sampled trace
// IDs. If the trace ID is cached, it deletes the spans from the internal map.
func (tsp *tailSamplingSpanProcessor) releaseNotSampledTrace(id pcommon.TraceID, td *TraceData) {
	for _, hook := range tsp.nonSampledHooks {
		hook(context.Background(), id, td)
	}
	tsp.nonSampledIDCache.Put(id, cache.DecisionMetadata{PolicyName: td.PolicyName})
	_, ok := tsp.nonSampledIDCache.Get(id)
	if ok {
		tsp.dropTrace(id, time.Now())
	}
}

func getPolicyName(policy *policy) string {
	if policy == nil {
		return ""
	}
	return policy.name
}

func appendToTraces(dest ptrace.Traces, rss ptrace.ResourceSpans) {
	rs := dest.ResourceSpans().AppendEmpty()
	rss.MoveTo(rs)
}

func appendAllTraces(dest, src ptrace.Traces) {
	rs := src.ResourceSpans()
	for i := 0; i < rs.Len(); i++ {
		appendToTraces(dest, rs.At(i))
	}
}

func newResourceSpanFromSpanAndScopes(rss ptrace.ResourceSpans, spanAndScopes []spanAndScope) (ptrace.ResourceSpans, *ptrace.Span) {
	rs := ptrace.NewResourceSpans()
	rss.Resource().CopyTo(rs.Resource())
	var rootSpan *ptrace.Span

	scopePointerToNewScope := make(map[*pcommon.InstrumentationScope]*ptrace.ScopeSpans)
	for _, spanAndScope := range spanAndScopes {
		// If the scope of the spanAndScope is not in the map, add it to the map and the destination.
		var sp ptrace.Span
		if scope, ok := scopePointerToNewScope[spanAndScope.instrumentationScope]; !ok {
			is := rs.ScopeSpans().AppendEmpty()
			spanAndScope.instrumentationScope.CopyTo(is.Scope())
			scopePointerToNewScope[spanAndScope.instrumentationScope] = &is

			sp = is.Spans().AppendEmpty()
		} else {
			sp = scope.Spans().AppendEmpty()
		}

		spanAndScope.span.CopyTo(sp)
		if sp.ParentSpanID().IsEmpty() {
			rootSpan = &sp
		}
	}
	return rs, rootSpan
}
