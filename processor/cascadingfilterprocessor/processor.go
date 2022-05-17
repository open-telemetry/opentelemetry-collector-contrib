// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cascadingfilterprocessor

import (
	"context"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cascadingfilterprocessor/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cascadingfilterprocessor/idbatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cascadingfilterprocessor/sampling"
)

// TraceAcceptEvaluator combines a sampling policy evaluator with the destinations to be
// used for that policy.
type TraceAcceptEvaluator struct {
	// Name used to identify this policy instance.
	Name string
	// Evaluator that decides if a trace is sampled or not by this policy instance.
	Evaluator sampling.PolicyEvaluator
	// ctx used to carry metric tags of each policy.
	ctx context.Context
	// probabilisticFilter determines whether `sampling.probability` field must be calculated and added
	probabilisticFilter bool
}

// TraceRejectEvaluator holds checking if trace should be dropped completely before further processing
type TraceRejectEvaluator struct {
	// Name used to identify this policy instance.
	Name string
	// Evaluator that decides if a trace is sampled or not by this policy instance.
	Evaluator sampling.DropTraceEvaluator
	// ctx used to carry metric tags of each policy.
	ctx context.Context
}

// traceKey is defined since sync.Map requires a comparable type, isolating it on its own
// type to help track usage.
type traceKey [16]byte

// cascadingFilterSpanProcessor handles the incoming trace data and uses the given sampling
// policy to sample traces.
type cascadingFilterSpanProcessor struct {
	ctx              context.Context
	nextConsumer     consumer.Traces
	start            sync.Once
	maxNumTraces     uint64
	traceAcceptRules []*TraceAcceptEvaluator
	traceRejectRules []*TraceRejectEvaluator
	logger           *zap.Logger
	idToTrace        sync.Map
	policyTicker     tTicker
	decisionBatcher  idbatcher.Batcher
	deleteChan       chan traceKey
	numTracesOnMap   uint64

	filteringEnabled bool

	currentSecond        int64
	maxSpansPerSecond    int32
	spansInCurrentSecond int32
}

const (
	probabilisticFilterPolicyName = "probabilistic_filter"
	probabilisticRuleVale         = "probabilistic"
	filteredRuleValue             = "filtered"
	AttributeSamplingRule         = "sampling.rule"

	AttributeSamplingProbability = "sampling.probability"
)

// newTraceProcessor returns a processor.TraceProcessor that will perform Cascading Filter according to the given
// configuration.
func newTraceProcessor(logger *zap.Logger, nextConsumer consumer.Traces, cfg config.Config) (component.TracesProcessor, error) {
	if nextConsumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}

	return newCascadingFilterSpanProcessor(logger, nextConsumer, cfg)
}

func newCascadingFilterSpanProcessor(logger *zap.Logger, nextConsumer consumer.Traces, cfg config.Config) (*cascadingFilterSpanProcessor, error) {
	numDecisionBatches := uint64(cfg.DecisionWait.Seconds())
	inBatcher, err := idbatcher.New(numDecisionBatches, cfg.ExpectedNewTracesPerSec, uint64(2*runtime.NumCPU()))
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	var policies []*TraceAcceptEvaluator
	var dropTraceEvals []*TraceRejectEvaluator

	// Prepare Trace Reject config

	for _, dropCfg := range cfg.TraceRejectCfgs {
		dropCtx, err := tag.New(ctx, tag.Upsert(tagPolicyKey, dropCfg.Name), tag.Upsert(tagPolicyDecisionKey, statusDropped))
		if err != nil {
			return nil, err
		}
		evaluator, err := sampling.NewDropTraceEvaluator(logger, dropCfg)
		if err != nil {
			return nil, err
		}
		dropEval := &TraceRejectEvaluator{
			Name:      dropCfg.Name,
			Evaluator: evaluator,
			ctx:       dropCtx,
		}
		logger.Info("Adding trace reject rule", zap.String("name", dropCfg.Name))
		dropTraceEvals = append(dropTraceEvals, dropEval)
	}

	// Prepare Trace Accept config

	var policyCfgs []config.TraceAcceptCfg
	totalRate := int32(0)

	if len(cfg.TraceAcceptCfgs) > 0 {
		policyCfgs = append(policyCfgs, cfg.TraceAcceptCfgs...)
	}

	if len(cfg.PolicyCfgs) > 0 {
		logger.Warn("'traceAcceptRules' is deprecated and will be removed in future versions, please use 'trace_accept_filters' instead")
		policyCfgs = append(policyCfgs, cfg.PolicyCfgs...)
	}

	for i := range policyCfgs {
		policyCfg := policyCfgs[i]
		policyCtx, err := tag.New(ctx, tag.Upsert(tagPolicyKey, policyCfg.Name))
		if err != nil {
			return nil, err
		}
		eval, err := buildPolicyEvaluator(logger, &policyCfg)
		if err != nil {
			return nil, err
		}
		policy := &TraceAcceptEvaluator{
			Name:                policyCfg.Name,
			Evaluator:           eval,
			ctx:                 policyCtx,
			probabilisticFilter: false,
		}
		if policyCfg.SpansPerSecond > 0 {
			totalRate += policyCfg.SpansPerSecond
		}
		logger.Info("Adding trace accept rule",
			zap.String("name", policyCfg.Name),
			zap.Int32("spans_per_second", policyCfg.SpansPerSecond))
		policies = append(policies, policy)
	}

	// Recalculate the total spans per second rate if needed
	spansPerSecond := cfg.SpansPerSecond
	if spansPerSecond == 0 {
		spansPerSecond = totalRate
		if cfg.ProbabilisticFilteringRate != nil && *cfg.ProbabilisticFilteringRate > 0 {
			spansPerSecond += *cfg.ProbabilisticFilteringRate
		}
	}

	if spansPerSecond != 0 {
		logger.Info("Setting total spans per second limit", zap.Int32("spans_per_second", spansPerSecond))
	} else {
		logger.Info("Not setting total spans per second limit (only selected traces will be filtered out)")
	}

	// Setup probabilistic filtering - using either ratio or rate.
	// This must be always evaluated first as it must select traces independently of other traceAcceptRules

	probabilisticFilteringRate := int32(-1)

	if cfg.ProbabilisticFilteringRatio != nil && *cfg.ProbabilisticFilteringRatio > 0.0 && spansPerSecond > 0 {
		probabilisticFilteringRate = int32(float32(spansPerSecond) * *cfg.ProbabilisticFilteringRatio)
	} else if cfg.ProbabilisticFilteringRate != nil && *cfg.ProbabilisticFilteringRate > 0 {
		probabilisticFilteringRate = *cfg.ProbabilisticFilteringRate
	}

	if probabilisticFilteringRate > 0 {
		logger.Info("Setting probabilistic filtering rate", zap.Int32("probabilistic_filtering_rate", probabilisticFilteringRate))

		policyCtx, err := tag.New(ctx, tag.Upsert(tagPolicyKey, probabilisticFilterPolicyName))
		if err != nil {
			return nil, err
		}
		eval, err := buildProbabilisticFilterEvaluator(logger, probabilisticFilteringRate)
		if err != nil {
			return nil, err
		}
		policy := &TraceAcceptEvaluator{
			Name:                probabilisticFilterPolicyName,
			Evaluator:           eval,
			ctx:                 policyCtx,
			probabilisticFilter: true,
		}
		policies = append([]*TraceAcceptEvaluator{policy}, policies...)
	} else {
		logger.Info("Not setting probabilistic filtering rate")
	}

	if len(policies) == 0 && len(dropTraceEvals) == 0 {
		logger.Info("No rules set for cascading_filter processor. Processor wil output all incoming spans without filtering.")
	}

	// Build the span procesor

	cfsp := &cascadingFilterSpanProcessor{
		ctx:               ctx,
		nextConsumer:      nextConsumer,
		maxNumTraces:      cfg.NumTraces,
		maxSpansPerSecond: spansPerSecond,
		logger:            logger,
		decisionBatcher:   inBatcher,
		traceAcceptRules:  policies,
		traceRejectRules:  dropTraceEvals,
		filteringEnabled:  len(policies) > 0 || len(dropTraceEvals) > 0,
	}

	cfsp.policyTicker = &policyTicker{onTick: cfsp.samplingPolicyOnTick}
	cfsp.deleteChan = make(chan traceKey, cfg.NumTraces)

	return cfsp, nil
}

func buildPolicyEvaluator(logger *zap.Logger, cfg *config.TraceAcceptCfg) (sampling.PolicyEvaluator, error) {
	return sampling.NewFilter(logger, cfg)
}

func buildProbabilisticFilterEvaluator(logger *zap.Logger, maxSpanRate int32) (sampling.PolicyEvaluator, error) {
	return sampling.NewProbabilisticFilter(logger, maxSpanRate)
}

type policyMetrics struct {
	idNotFoundOnMapCount, evaluateErrorCount, decisionSampled, decisionNotSampled int64
}

func (cfsp *cascadingFilterSpanProcessor) updateRate(currSecond int64, numSpans int32) sampling.Decision {
	if cfsp.maxSpansPerSecond <= 0 {
		return sampling.Sampled
	}

	if cfsp.currentSecond != currSecond {
		cfsp.currentSecond = currSecond
		cfsp.spansInCurrentSecond = 0
	}

	spansInSecondIfSampled := cfsp.spansInCurrentSecond + numSpans
	if spansInSecondIfSampled <= cfsp.maxSpansPerSecond {
		cfsp.spansInCurrentSecond = spansInSecondIfSampled
		return sampling.Sampled
	}

	return sampling.NotSampled
}

func (cfsp *cascadingFilterSpanProcessor) samplingPolicyOnTick() {
	metrics := policyMetrics{}

	startTime := time.Now()
	batch, _ := cfsp.decisionBatcher.CloseCurrentAndTakeFirstBatch()
	batchLen := len(batch)
	cfsp.logger.Debug("Sampling Policy Evaluation ticked")

	currSecond := time.Now().Unix()

	totalSpans := int64(0)
	selectedByProbabilisticFilterSpans := int64(0)

	// The first run applies decisions to batches, executing each policy separately
	for _, id := range batch {
		d, ok := cfsp.idToTrace.Load(traceKey(id.Bytes()))
		if !ok {
			metrics.idNotFoundOnMapCount++
			continue
		}
		trace := d.(*sampling.TraceData)
		trace.DecisionTime = time.Now()

		var provisionalDecision sampling.Decision

		// Dropped traces are not included in probabilistic filtering calculations
		if cfsp.shouldBeDropped(id, trace) {
			provisionalDecision = sampling.Dropped
		} else {
			totalSpans += int64(trace.SpanCount)
			provisionalDecision, _ = cfsp.makeProvisionalDecision(id, trace)
		}

		if provisionalDecision == sampling.Sampled {
			trace.FinalDecision = cfsp.updateRate(currSecond, trace.SpanCount)
			if trace.FinalDecision == sampling.Sampled {
				if trace.SelectedByProbabilisticFilter {
					selectedByProbabilisticFilterSpans += int64(trace.SpanCount)
				}
				err := stats.RecordWithTags(
					cfsp.ctx,
					[]tag.Mutator{tag.Insert(tagCascadingFilterDecisionKey, statusSampled)},
					statCascadingFilterDecision.M(int64(1)),
				)
				if err != nil {
					cfsp.logger.Error("Sampling Policy Evaluation error on first run tick", zap.Error(err))
				}
			} else {
				err := stats.RecordWithTags(
					cfsp.ctx,
					[]tag.Mutator{tag.Insert(tagCascadingFilterDecisionKey, statusExceededKey)},
					statCascadingFilterDecision.M(int64(1)),
				)
				if err != nil {
					cfsp.logger.Error("Sampling Policy Evaluation error on first run tick", zap.Error(err))
				}
			}
		} else if provisionalDecision == sampling.SecondChance {
			trace.FinalDecision = sampling.SecondChance
		} else {
			trace.FinalDecision = provisionalDecision
			err := stats.RecordWithTags(
				cfsp.ctx,
				[]tag.Mutator{tag.Insert(tagCascadingFilterDecisionKey, statusNotSampled)},
				statCascadingFilterDecision.M(int64(1)),
			)
			if err != nil {
				cfsp.logger.Error("Sampling Policy Evaluation error on first run tick", zap.Error(err))
			}
		}
	}

	// The second run executes the decisions and makes "SecondChance" decisions in the meantime
	for _, id := range batch {
		d, ok := cfsp.idToTrace.Load(traceKey(id.Bytes()))
		if !ok {
			continue
		}
		trace := d.(*sampling.TraceData)
		if trace.FinalDecision == sampling.SecondChance {
			trace.FinalDecision = cfsp.updateRate(currSecond, trace.SpanCount)
			if trace.FinalDecision == sampling.Sampled {
				err := stats.RecordWithTags(
					cfsp.ctx,
					[]tag.Mutator{tag.Insert(tagCascadingFilterDecisionKey, statusSecondChanceSampled)},
					statCascadingFilterDecision.M(int64(1)),
				)
				if err != nil {
					cfsp.logger.Error("Sampling Policy Evaluation error on second run tick", zap.Error(err))
				}
			} else {
				err := stats.RecordWithTags(
					cfsp.ctx,
					[]tag.Mutator{tag.Insert(tagCascadingFilterDecisionKey, statusSecondChanceExceeded)},
					statCascadingFilterDecision.M(int64(1)),
				)
				if err != nil {
					cfsp.logger.Error("Sampling Policy Evaluation error on second run tick", zap.Error(err))
				}
			}
		}

		// Sampled or not, remove the batches
		trace.Lock()
		traceBatches := trace.ReceivedBatches
		trace.ReceivedBatches = nil
		trace.Unlock()

		if trace.FinalDecision == sampling.Sampled {
			metrics.decisionSampled++

			// Combine all individual batches into a single batch so
			// consumers may operate on the entire trace
			allSpans := pdata.NewTraces()
			for j := 0; j < len(traceBatches); j++ {
				batch := traceBatches[j]
				batch.ResourceSpans().MoveAndAppendTo(allSpans.ResourceSpans())
			}

			if trace.SelectedByProbabilisticFilter {
				updateProbabilisticRateTag(allSpans, selectedByProbabilisticFilterSpans, totalSpans)
			} else if len(cfsp.traceAcceptRules) > 0 {
				// Set filtering tag only if there were actually any accept rules set otherwise
				updateFilteringTag(allSpans)
			}

			err := cfsp.nextConsumer.ConsumeTraces(cfsp.ctx, allSpans)
			if err != nil {
				cfsp.logger.Error("Sampling Policy Evaluation error on consuming traces", zap.Error(err))
			}
		} else {
			metrics.decisionNotSampled++
		}
	}

	stats.Record(cfsp.ctx,
		statOverallDecisionLatencyus.M(int64(time.Since(startTime)/time.Microsecond)),
		statDroppedTooEarlyCount.M(metrics.idNotFoundOnMapCount),
		statPolicyEvaluationErrorCount.M(metrics.evaluateErrorCount),
		statTracesOnMemoryGauge.M(int64(atomic.LoadUint64(&cfsp.numTracesOnMap))))

	cfsp.logger.Debug("Sampling policy evaluation completed",
		zap.Int("batch.len", batchLen),
		zap.Int64("sampled", metrics.decisionSampled),
		zap.Int64("notSampled", metrics.decisionNotSampled),
		zap.Int64("droppedPriorToEvaluation", metrics.idNotFoundOnMapCount),
		zap.Int64("policyEvaluationErrors", metrics.evaluateErrorCount),
	)
}

func updateProbabilisticRateTag(traces pdata.Traces, probabilisticSpans int64, allSpans int64) {
	ratio := float64(probabilisticSpans) / float64(allSpans)

	rs := traces.ResourceSpans()

	for i := 0; i < rs.Len(); i++ {
		ils := rs.At(i).InstrumentationLibrarySpans()
		for j := 0; j < ils.Len(); j++ {
			spans := ils.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				attrs := spans.At(k).Attributes()
				av, found := attrs.Get(AttributeSamplingProbability)
				if found && av.Type() == pdata.AttributeValueTypeDouble && !math.IsNaN(av.DoubleVal()) && av.DoubleVal() > 0.0 {
					av.SetDoubleVal(av.DoubleVal() * ratio)
				} else {
					attrs.UpsertDouble(AttributeSamplingProbability, ratio)
				}
				attrs.UpsertString(AttributeSamplingRule, probabilisticRuleVale)
			}
		}
	}
}

func updateFilteringTag(traces pdata.Traces) {
	rs := traces.ResourceSpans()

	for i := 0; i < rs.Len(); i++ {
		ils := rs.At(i).InstrumentationLibrarySpans()
		for j := 0; j < ils.Len(); j++ {
			spans := ils.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				attrs := spans.At(k).Attributes()
				attrs.UpsertString(AttributeSamplingRule, filteredRuleValue)
			}
		}
	}
}

func (cfsp *cascadingFilterSpanProcessor) shouldBeDropped(id pdata.TraceID, trace *sampling.TraceData) bool {
	for _, dropRule := range cfsp.traceRejectRules {
		if dropRule.Evaluator.ShouldDrop(id, trace) {
			stats.Record(dropRule.ctx, statPolicyDecision.M(int64(1)))
			return true
		}
	}
	return false
}

func (cfsp *cascadingFilterSpanProcessor) makeProvisionalDecision(id pdata.TraceID, trace *sampling.TraceData) (sampling.Decision, *TraceAcceptEvaluator) {
	// When no rules are defined, always sample
	if len(cfsp.traceAcceptRules) == 0 {
		return sampling.Sampled, nil
	}

	provisionalDecision := sampling.Unspecified

	for i, policy := range cfsp.traceAcceptRules {
		policyEvaluateStartTime := time.Now()
		decision := policy.Evaluator.Evaluate(id, trace)
		stats.Record(
			policy.ctx,
			statDecisionLatencyMicroSec.M(int64(time.Since(policyEvaluateStartTime)/time.Microsecond)))

		trace.Decisions[i] = decision

		switch decision {
		case sampling.Sampled:
			// any single policy that decides to sample will cause the decision to be sampled
			// the nextConsumer will get the context from the first matching policy
			provisionalDecision = sampling.Sampled

			if policy.probabilisticFilter {
				trace.SelectedByProbabilisticFilter = true
			}

			err := stats.RecordWithTags(
				policy.ctx,
				[]tag.Mutator{tag.Insert(tagPolicyDecisionKey, statusSampled)},
				statPolicyDecision.M(int64(1)),
			)
			if err != nil {
				cfsp.logger.Error("Making provisional decision error", zap.Error(err))
			}

			// No need to continue
			return provisionalDecision, policy
		case sampling.NotSampled:
			if provisionalDecision == sampling.Unspecified {
				provisionalDecision = sampling.NotSampled
			}
			err := stats.RecordWithTags(
				policy.ctx,
				[]tag.Mutator{tag.Insert(tagPolicyDecisionKey, statusNotSampled)},
				statPolicyDecision.M(int64(1)),
			)
			if err != nil {
				cfsp.logger.Error("Making provisional decision error", zap.Error(err))
			}
		case sampling.SecondChance:
			if provisionalDecision != sampling.Sampled {
				provisionalDecision = sampling.SecondChance
			}

			err := stats.RecordWithTags(
				policy.ctx,
				[]tag.Mutator{tag.Insert(tagPolicyDecisionKey, statusSecondChance)},
				statPolicyDecision.M(int64(1)),
			)
			if err != nil {
				cfsp.logger.Error("Making provisional decision error", zap.Error(err))
			}
		}
	}

	return provisionalDecision, nil
}

// ConsumeTraces is required by the SpanProcessor interface.
func (cfsp *cascadingFilterSpanProcessor) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	if !cfsp.filteringEnabled {
		return cfsp.nextConsumer.ConsumeTraces(ctx, td)
	}

	cfsp.start.Do(func() {
		cfsp.logger.Info("First trace data arrived, starting cascading_filter timers")
		cfsp.policyTicker.Start(1 * time.Second)
	})
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		resourceSpan := resourceSpans.At(i)
		cfsp.processTraces(ctx, resourceSpan)
	}
	return nil
}

func (cfsp *cascadingFilterSpanProcessor) groupSpansByTraceKey(resourceSpans pdata.ResourceSpans) map[traceKey][]*pdata.Span {
	idToSpans := make(map[traceKey][]*pdata.Span)
	ilss := resourceSpans.InstrumentationLibrarySpans()
	for j := 0; j < ilss.Len(); j++ {
		ils := ilss.At(j)
		spansLen := ils.Spans().Len()
		for k := 0; k < spansLen; k++ {
			span := ils.Spans().At(k)
			tk := traceKey(span.TraceID().Bytes())
			if len(tk) != 16 {
				cfsp.logger.Warn("Span without valid TraceId")
			}
			idToSpans[tk] = append(idToSpans[tk], &span)
		}
	}
	return idToSpans
}

func (cfsp *cascadingFilterSpanProcessor) processTraces(ctx context.Context, resourceSpans pdata.ResourceSpans) {
	// Group spans per their traceId to minimize contention on idToTrace
	idToSpans := cfsp.groupSpansByTraceKey(resourceSpans)
	var newTraceIDs int64
	for id, spans := range idToSpans {
		lenSpans := int32(len(spans))
		lenPolicies := len(cfsp.traceAcceptRules)
		initialDecisions := make([]sampling.Decision, lenPolicies)

		for i := 0; i < lenPolicies; i++ {
			initialDecisions[i] = sampling.Pending
		}
		initialTraceData := &sampling.TraceData{
			Decisions:   initialDecisions,
			ArrivalTime: time.Now(),
			SpanCount:   lenSpans,
		}
		d, loaded := cfsp.idToTrace.LoadOrStore(id, initialTraceData)

		actualData := d.(*sampling.TraceData)
		if loaded {
			// PMM: why actualData is not updated with new trace?
			atomic.AddInt32(&actualData.SpanCount, lenSpans)
		} else {
			newTraceIDs++
			cfsp.decisionBatcher.AddToCurrentBatch(pdata.NewTraceID(id))
			atomic.AddUint64(&cfsp.numTracesOnMap, 1)
			postDeletion := false
			currTime := time.Now()

			for !postDeletion {
				select {
				case cfsp.deleteChan <- id:
					postDeletion = true
				default:
					// Note this is a buffered channel, so this will only delete excessive traces (if they exist)
					traceKeyToDrop := <-cfsp.deleteChan
					cfsp.dropTrace(traceKeyToDrop, currTime)
				}
			}
		}

		// Add the spans to the trace, but only once for all policy, otherwise same spans will
		// be duplicated in the final trace.
		actualData.Lock()
		traceTd := prepareTraceBatch(resourceSpans, spans)
		actualData.ReceivedBatches = append(actualData.ReceivedBatches, traceTd)
		finalDecision := actualData.FinalDecision
		actualData.Unlock()

		// This section is run in case the decision was already applied earlier
		switch finalDecision {
		case sampling.Unspecified:
			// This has not been determined yet
		case sampling.Pending:
			// All process for pending done above, keep the case so it doesn't go to default.
		case sampling.SecondChance:
			// It shouldn't normally get here, keep the case so it doesn't go to default, like above.
		case sampling.Sampled:
			// Forward the spans to the policy destinations
			traceTd := prepareTraceBatch(resourceSpans, spans)
			if err := cfsp.nextConsumer.ConsumeTraces(ctx, traceTd); err != nil {
				cfsp.logger.Warn("Error sending late arrived spans to destination",
					zap.Error(err))
			}
			stats.Record(cfsp.ctx, statLateSpanArrivalAfterDecision.M(int64(time.Since(actualData.DecisionTime)/time.Second)))
		case sampling.NotSampled:
			stats.Record(cfsp.ctx, statLateSpanArrivalAfterDecision.M(int64(time.Since(actualData.DecisionTime)/time.Second)))
		case sampling.Dropped:
			stats.Record(cfsp.ctx, statLateSpanArrivalAfterDecision.M(int64(time.Since(actualData.DecisionTime)/time.Second)))
		default:
			cfsp.logger.Warn("Encountered unexpected sampling decision",
				zap.Int("decision", int(finalDecision)))
		}
	}

	stats.Record(cfsp.ctx, statNewTraceIDReceivedCount.M(newTraceIDs))
}

// func (cfsp *cascadingFilterSpanProcessor) GetCapabilities() component.ProcessorCapabilities {
// 	return component.ProcessorCapabilities{MutatesConsumedData: false}
// }

func (cfsp *cascadingFilterSpanProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// Start is invoked during service startup.
func (cfsp *cascadingFilterSpanProcessor) Start(context.Context, component.Host) error {
	return nil
}

// Shutdown is invoked during service shutdown.
func (cfsp *cascadingFilterSpanProcessor) Shutdown(context.Context) error {
	return nil
}

func (cfsp *cascadingFilterSpanProcessor) dropTrace(traceID traceKey, deletionTime time.Time) {
	var trace *sampling.TraceData
	if d, ok := cfsp.idToTrace.Load(traceID); ok {
		trace = d.(*sampling.TraceData)
		cfsp.idToTrace.Delete(traceID)
		// Subtract one from numTracesOnMap per https://godoc.org/sync/atomic#AddUint64
		atomic.AddUint64(&cfsp.numTracesOnMap, ^uint64(0))
	}
	if trace == nil {
		cfsp.logger.Error("Attempt to delete traceID not on table")
		return
	}

	stats.Record(cfsp.ctx, statTraceRemovalAgeSec.M(int64(deletionTime.Sub(trace.ArrivalTime)/time.Second)))
}

func prepareTraceBatch(rss pdata.ResourceSpans, spans []*pdata.Span) pdata.Traces {
	traceTd := pdata.NewTraces()
	rs := traceTd.ResourceSpans().AppendEmpty()
	rss.Resource().CopyTo(rs.Resource())
	ils := rs.InstrumentationLibrarySpans().AppendEmpty()
	ilsSpans := ils.Spans()
	for _, span := range spans {
		span.CopyTo(ilsSpans.AppendEmpty())
	}
	return traceTd
}

// tTicker interface allows easier testing of ticker related functionality used by cascadingfilterprocessor
type tTicker interface {
	// Start sets the frequency of the ticker and starts the periodic calls to OnTick.
	Start(d time.Duration)
	// OnTick is called when the ticker fires.
	OnTick()
	// Stops firing the ticker.
	Stop()
}

type policyTicker struct {
	ticker *time.Ticker
	onTick func()
}

func (pt *policyTicker) Start(d time.Duration) {
	pt.ticker = time.NewTicker(d)
	go func() {
		for range pt.ticker.C {
			pt.OnTick()
		}
	}()
}
func (pt *policyTicker) OnTick() {
	pt.onTick()
}
func (pt *policyTicker) Stop() {
	pt.ticker.Stop()
}

var _ tTicker = (*policyTicker)(nil)
