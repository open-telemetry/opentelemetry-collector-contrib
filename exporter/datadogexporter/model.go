// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build !windows

package datadogexporter

import (
	"github.com/DataDog/datadog-agent/pkg/trace/event"
	"github.com/DataDog/datadog-agent/pkg/trace/obfuscate"
	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	"github.com/DataDog/datadog-agent/pkg/trace/sampler"
	"github.com/DataDog/datadog-agent/pkg/trace/stats"
	"github.com/DataDog/datadog-agent/pkg/trace/traceutil"
)

// ObfuscatePayload applies obfuscator rules to the trace payloads
func ObfuscatePayload(obfuscator *obfuscate.Obfuscator, tracePayloads []*pb.TracePayload) {
	for _, tracePayload := range tracePayloads {

		// Obfuscate the traces in the payload
		for _, trace := range tracePayload.Traces {
			for _, span := range trace.Spans {
				obfuscator.Obfuscate(span)
			}
		}
	}
}

// GetAnalyzedSpans finds all the analyzed spans in a trace, including top level spans
// and spans marked as analyzed by the tracer.
// A span is considered top-level if:
// - it's a root span
// - its parent is unknown (other part of the code, distributed trace)
// - its parent belongs to another service (in that case it's a "local root"
//   being the highest ancestor of other spans belonging to this service and
//   attached to it).
func GetAnalyzedSpans(sps []*pb.Span) []*pb.Span {
	// build a lookup map
	spanIDToIdx := make(map[uint64]int, len(sps))
	for i, span := range sps {
		spanIDToIdx[span.SpanID] = i
	}

	top := []*pb.Span{}

	extractor := event.NewMetricBasedExtractor()

	// iterate on each span and mark them as top-level if relevant
	for _, span := range sps {
		// The tracer can mark a span to be analyzed, with a value 0-1, where 1 is always keep, and 0 is always reject.
		// Values between 0-1 are used by the agent to prioritize which spans are sampled or not. Since we can't
		// reliably apply sampling decisions in a serverless environment, we keep any analyzed span with a priority
		// greater than 0, and let the backend make the sampling decision instead.
		priority, extracted := extractor.Extract(span, sampler.PriorityUserKeep)
		shouldExtract := priority > 0 && extracted

		if span.ParentID != 0 {
			if parentIdx, ok := spanIDToIdx[span.ParentID]; ok && sps[parentIdx].Service == span.Service && !shouldExtract {
				continue
			}
		}

		top = append(top, span)
	}
	return top
}

// Compute Sublayers updates a spans metrics with relevant metadata so that it's duration and breakdown between different services can
// be accurately displayed in the Datadog UI
func ComputeSublayerMetrics(t pb.Trace) {
	root := traceutil.GetRoot(t)
	traceutil.ComputeTopLevel(t)

	subtraces := stats.ExtractSubtraces(t, root)
	sublayers := make(map[*pb.Span][]stats.SublayerValue)
	for _, subtrace := range subtraces {
		subtraceSublayers := stats.ComputeSublayers(subtrace.Trace)
		sublayers[subtrace.Root] = subtraceSublayers
		stats.SetSublayersOnSpan(subtrace.Root, subtraceSublayers)
	}
}
