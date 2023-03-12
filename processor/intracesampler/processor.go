// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package intracesampler // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/intracesamplerprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
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
	scopeLeavesMap     map[string]struct{}
	scaledSamplingRate uint32
	hashSeedBytes      []byte
}

func newInTraceSamplerSpansProcessor(ctx context.Context, set processor.CreateSettings, cfg *Config, nextConsumer consumer.Traces) (processor.Traces, error) {

	scopeLeavesMap := make(map[string]struct{})
	for i := 0; i < len(cfg.ScopeLeaves); i++ {
		scopeLeavesMap[cfg.ScopeLeaves[i]] = struct{}{}
	}

	its := &inTraceSamplerProcessor{
		logger:             set.Logger,
		config:             *cfg,
		scopeLeavesMap:     scopeLeavesMap,
		scaledSamplingRate: uint32(cfg.SamplingPercentage * percentageScaleFactor),
		hashSeedBytes:      i32tob(cfg.HashSeed),
	}

	return processorhelper.NewTracesProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		its.processTraces,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}

type fullSpan struct {
	resource pcommon.Resource
	scope    pcommon.InstrumentationScope
	span     ptrace.Span
}

type traceTreeData struct {

	// map each span id to a full span object with scope and resource
	fullSpans map[pcommon.SpanID]fullSpan

	// map each span id to its children ids
	// this enables fast leaf detection and traversal of the tree from roots
	children map[pcommon.SpanID][]pcommon.SpanID

	// spans that have no parent or their parent is not in the trace
	roots []pcommon.SpanID
}

// this map enable us to find the parent span of a span in O(1), and all the children of a givin span
// it also generates a span object that contains the resource and scope all at once
// this is useful for the sampler to be able to make decisions on spans
func spansToTraceTree(td ptrace.Traces) traceTreeData {
	fullSpans := make(map[pcommon.SpanID]fullSpan)
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

				fullspan := fullSpan{
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
		parentSpanID := fullspan.span.ParentSpanID()
		if _, ok := fullSpans[parentSpanID]; !ok {
			currentSpanID := fullspan.span.SpanID()
			roots = append(roots, currentSpanID)
		}
	}

	traceTreeData := traceTreeData{
		fullSpans: fullSpans,
		children:  spanChildren,
		roots:     roots,
	}

	return traceTreeData
}

// check if all spans in td are from the span trace id.
// this indicates that the processor is run after another processor
// that emits completed traces after timeout
// if a single trace id is found, it is returend, otherwise nil is returned
func getSingleTraceID(td ptrace.Traces) *pcommon.TraceID {
	var traceID *pcommon.TraceID
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		scopespans := rs.ScopeSpans()
		for j := 0; j < scopespans.Len(); j++ {
			ss := scopespans.At(j)
			spans := ss.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				currentTraceID := span.TraceID()
				if traceID == nil {
					traceID = &currentTraceID
				} else if currentTraceID != *traceID {
					return nil
				}
			}
		}
	}

	// will be nil it the batch is empty
	return traceID
}

func (its *inTraceSamplerProcessor) getScopeBranchesToUnsampleRec(traceTreeData traceTreeData, currentSpanID pcommon.SpanID, unsampledScopes map[pcommon.SpanID]bool) bool {
	currentFullSpan := traceTreeData.fullSpans[currentSpanID]
	currentScopeName := currentFullSpan.scope.Name()

	// currrent span should be unsampled if it's in the unsampledScopes map
	// and all its children are also unsampled.
	_, currentUnsampled := its.scopeLeavesMap[currentScopeName]
	for _, childSpanID := range traceTreeData.children[currentSpanID] {
		childUnsampled := its.getScopeBranchesToUnsampleRec(traceTreeData, childSpanID, unsampledScopes)
		currentUnsampled = currentUnsampled && childUnsampled
	}

	if currentUnsampled {
		unsampledScopes[currentFullSpan.span.SpanID()] = true
	}
	return currentUnsampled
}

func (its *inTraceSamplerProcessor) getScopeBranchesToUnsample(traceTreeData traceTreeData) map[pcommon.SpanID]bool {
	unsampledScopes := make(map[pcommon.SpanID]bool, 0)
	for _, rootSpanID := range traceTreeData.roots {
		its.getScopeBranchesToUnsampleRec(traceTreeData, rootSpanID, unsampledScopes)
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

	// the sampler assumes it receives full "completed" traces
	singleTraceID := getSingleTraceID(td)
	if singleTraceID == nil {
		its.logger.Warn("in trace sampler received spans from different traces. it should run after tailsampler or groupby processor")
		return td, nil
	}

	// some of the traces will be sampled in trace, but some will still be allowed to pass through as is
	sampled := computeHash((*singleTraceID)[:], its.hashSeedBytes)&bitMaskHashBuckets < its.scaledSamplingRate
	// sampled means we keep all spans (not dropping anything), thus forwarding td as is
	if sampled {
		return td, nil
	}

	traceTreeData := spansToTraceTree(td)
	unsampledSpanIds := its.getScopeBranchesToUnsample(traceTreeData)
	if len(unsampledSpanIds) == 0 {
		return td, nil
	}

	its.logger.Debug("unsampling spans in a trace", zap.Int("num spans", len(unsampledSpanIds)))
	removeSpansByIds(td, unsampledSpanIds)
	return td, nil
}
