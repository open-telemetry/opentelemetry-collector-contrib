package intracesampler

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

const (
	// The constants help translate user friendly percentages to numbers direct used in sampling.
	numHashBuckets        = 0x4000 // Using a power of 2 to avoid division.
	bitMaskHashBuckets    = numHashBuckets - 1
	percentageScaleFactor = numHashBuckets / 100.0
)

type inTraceSamplerProcessor struct {
	logger             *zap.Logger
	config             Config
	scaledSamplingRate uint32
}

func newInTraceSamplerSpansProcessor(ctx context.Context, set component.ProcessorCreateSettings, cfg *Config, nextConsumer consumer.Traces) (component.TracesProcessor, error) {

	its := &inTraceSamplerProcessor{
		logger:             set.Logger,
		config:             *cfg,
		scaledSamplingRate: uint32(cfg.SamplingPercentage * percentageScaleFactor),
	}

	return processorhelper.NewTracesProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		its.processTraces,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}

type FullSpan struct {
	resource pcommon.Resource
	scope    pcommon.InstrumentationScope
	span     ptrace.Span
}

type TraceTreeData struct {

	// map each span id to a full span object with scope and resource
	fullSpans map[pcommon.SpanID]FullSpan

	// map each span id to its children ids
	// this enables fast leaf detection and traversal of the tree from roots
	children map[pcommon.SpanID][]pcommon.SpanID

	// spans that have no parent or their parent is not in the trace
	roots []pcommon.SpanID
}

// this map enable us to find the parent span of a span in O(1), and all the children of a givin span
// it also generates a span object that contains the resource and scope all at once
// this is useful for the sampler to be able to make decisions on spans
func spansToTraceTree(td ptrace.Traces) TraceTreeData {
	fullSpans := make(map[pcommon.SpanID]FullSpan)
	spanChildren := make(map[pcommon.SpanID][]pcommon.SpanID)

	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		resource := rs.Resource()
		scopespans := rs.ScopeSpans()
		for j := 0; j < scopespans.Len(); j++ {
			ss := scopespans.At(j)
			spans := ss.Spans()
			scope := ss.Scope()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)

				fullspan := FullSpan{
					resource: resource,
					scope:    scope,
					span:     span,
				}

				fullSpans[span.SpanID()] = fullspan
				spanChildren[span.ParentSpanID()] = append(spanChildren[span.ParentSpanID()], span.SpanID())
			}
		}
	}

	// find roots
	roots := make([]pcommon.SpanID, 0)
	for _, fullspan := range fullSpans {
		parentSpanId := fullspan.span.ParentSpanID()
		if _, ok := fullSpans[parentSpanId]; !ok {
			currentSpanId := fullspan.span.SpanID()
			roots = append(roots, currentSpanId)
		}
	}

	traceTreeData := TraceTreeData{
		fullSpans: fullSpans,
		children:  spanChildren,
		roots:     roots,
	}

	return traceTreeData
}

// check if all spans in td are from the span trace id.
// this indicates that the processor is run after another processor
// that emits completed traces after timeout
func isAllSameTraceID(td ptrace.Traces) bool {
	firstTraceID := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID()
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		scopespans := rs.ScopeSpans()
		for j := 0; j < scopespans.Len(); j++ {
			ss := scopespans.At(j)
			spans := ss.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if span.TraceID() != firstTraceID {
					return false
				}
			}
		}
	}

	return true
}

func (its *inTraceSamplerProcessor) getScopeBranchesToUnsampleRec(traceTreeData TraceTreeData, currentSpanId pcommon.SpanID, unsampledScopes map[pcommon.SpanID]bool) bool {
	currentFullSpan := traceTreeData.fullSpans[currentSpanId]
	currentScopeName := currentFullSpan.scope.Name()

	// currrent span should be unsampled if it's in the unsampledScopes map
	// and all it's children are also in the unsampled.
	currentUnsampled := slices.Contains(its.config.ScopeLeaves, currentScopeName)
	for _, childSpanId := range traceTreeData.children[currentSpanId] {
		childUnsampled := its.getScopeBranchesToUnsampleRec(traceTreeData, childSpanId, unsampledScopes)
		currentUnsampled = currentUnsampled && childUnsampled
	}

	if currentUnsampled {
		unsampledScopes[currentFullSpan.span.SpanID()] = true
	}
	return currentUnsampled
}

func (its *inTraceSamplerProcessor) getScopeBranchesToUnsample(traceTreeData TraceTreeData) map[pcommon.SpanID]bool {
	unsampledScopes := make(map[pcommon.SpanID]bool, 0)
	for _, rootSpanId := range traceTreeData.roots {
		its.getScopeBranchesToUnsampleRec(traceTreeData, rootSpanId, unsampledScopes)
	}

	return unsampledScopes
}

func removeSpansByIds(td ptrace.Traces, idsToRemove map[pcommon.SpanID]bool) {
	td.ResourceSpans().RemoveIf(func(rs ptrace.ResourceSpans) bool {
		rs.ScopeSpans().RemoveIf(func(ss ptrace.ScopeSpans) bool {
			ss.Spans().RemoveIf(func(span ptrace.Span) bool {
				remove := idsToRemove[span.SpanID()]
				return remove
			})
			return ss.Spans().Len() == 0
		})
		return rs.ScopeSpans().Len() == 0
	})
}

func (its *inTraceSamplerProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {

	// some of the traces will be sampled in trace, but some will still be allowed to pass through as is
	tidBytes := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID()
	sampled := hash(tidBytes[:], its.config.HashSeed)&bitMaskHashBuckets < its.scaledSamplingRate
	// sampled means we keep all spans (not dropping anything), thus forwarding td as is
	if sampled {
		return td, nil
	}

	// the ssampler assumes it receives full "completed" traces
	if !isAllSameTraceID(td) {
		its.logger.Warn("in trace sampler received spans from different traces. it should run after tailsampler or groupby processor")
		return td, nil
	}

	traceTreeData := spansToTraceTree(td)
	unsampledSpanIds := its.getScopeBranchesToUnsample(traceTreeData)
	if len(unsampledSpanIds) == 0 {
		return td, nil
	}

	its.logger.Info("unsampling spans in a trace", zap.Int("num spans", len(unsampledSpanIds)))
	removeSpansByIds(td, unsampledSpanIds)
	return td, nil
}
