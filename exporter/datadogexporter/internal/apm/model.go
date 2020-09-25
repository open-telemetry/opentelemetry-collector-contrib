/*
 * Unless explicitly stated otherwise all files in this repository are licensed
 * under the Apache License Version 2.0.
 *
 * This product includes software developed at Datadog (https://www.datadoghq.com/).
 * Copyright 2020 Datadog, Inc.
 */
package datadogexporter

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/DataDog/datadog-agent/pkg/trace/event"
	"github.com/DataDog/datadog-agent/pkg/trace/obfuscate"
	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	"github.com/DataDog/datadog-agent/pkg/trace/sampler"
	"github.com/DataDog/datadog-agent/pkg/trace/stats"
	"github.com/DataDog/datadog-agent/pkg/trace/traceutil"
)

type (
	// TraceList is an incoming trace payload
	traceList struct {
		Traces [][]span `json:"traces"`
	}
	// Span contains span metadata
	span struct {
		Service  string             `json:"service"`
		Name     string             `json:"name"`
		Resource string             `json:"resource"`
		TraceID  string             `json:"trace_id"`
		SpanID   string             `json:"span_id"`
		ParentID string             `json:"parent_id"`
		Start    int64              `json:"start"`
		Duration int64              `json:"duration"`
		Error    int32              `json:"error"`
		Meta     map[string]string  `json:"meta"`
		Metrics  map[string]float64 `json:"metrics"`
		Type     string             `json:"type"`
	}
)

const (
	originMetadataKey       = "_dd.origin"
	parentSourceMetadataKey = "_dd.parent_source"
	sourceXray              = "xray"
)

// ProcessTrace parses, applies tags, and obfuscates a trace
func ProcessTrace(content string, obfuscator *obfuscate.Obfuscator, tags string) ([]*pb.TracePayload, error) {
	tracePayloads, err := ParseTrace(content)
	if err != nil {
		fmt.Printf("Couldn't forward trace: %v", err)
		return tracePayloads, err
	}

	AddTagsToTracePayloads(tracePayloads, tags)
	ObfuscatePayload(obfuscator, tracePayloads)
	return tracePayloads, nil
}

// ParseTrace reads a trace using the standard log format
func ParseTrace(content string) ([]*pb.TracePayload, error) {
	var tl traceList
	err := json.Unmarshal([]byte(content), &tl)
	traces := []*pb.TracePayload{}

	if err != nil {
		return traces, fmt.Errorf("couldn't parse trace, %v", err)
	}
	removedSpans := map[uint64]*pb.Span{}

	for _, trace := range tl.Traces {
		apiTraces := map[uint64]*pb.APITrace{}

		for _, sp := range trace {
			// Make sure the span is marked as comming from lambda,
			// this will help the backend make sampling decisions
			if sp.Meta == nil {
				sp.Meta = map[string]string{}
			}
			sp.Meta[originMetadataKey] = "lambda"
			pbSpan := convertSpanToPB(sp)
			// We skip root dd-trace spans that are parented to X-Ray,
			// since those root spans are placeholders for the X-Ray
			// parent span.
			if IsParentedToXray(pbSpan) {
				removedSpans[pbSpan.SpanID] = pbSpan
				continue
			}
			var apiTrace *pb.APITrace
			var ok bool

			// Reparent any spans that have been removed
			if removedSpans[pbSpan.ParentID] != nil {
				pbSpan.ParentID = removedSpans[pbSpan.ParentID].ParentID
			}

			if apiTrace, ok = apiTraces[pbSpan.TraceID]; !ok {
				apiTrace = &pb.APITrace{
					TraceID:   pbSpan.TraceID,
					Spans:     []*pb.Span{},
					StartTime: 0,
					EndTime:   0,
				}
				apiTraces[apiTrace.TraceID] = apiTrace
			}
			addToAPITrace(apiTrace, pbSpan)
		}

		payload := pb.TracePayload{
			HostName:     "",
			Env:          "none",
			Traces:       []*pb.APITrace{},
			Transactions: []*pb.Span{},
		}
		for _, apiTrace := range apiTraces {
			top := GetAnalyzedSpans(apiTrace.Spans)
			ComputeSublayerMetrics(apiTrace.Spans)
			payload.Transactions = append(payload.Transactions, top...)
			payload.Traces = append(payload.Traces, apiTrace)
		}

		traces = append(traces, &payload)
	}

	return traces, nil
}

// IsParentedToXray detects whether the parent of a trace is an X-Ray span
func IsParentedToXray(span *pb.Span) bool {
	if span.Meta == nil {
		return false
	}
	source := span.Meta[parentSourceMetadataKey]
	return source == sourceXray
}

// AddTagsToTracePayloads takes a single string of tags, eg 'a:b,c:d', and applies them
// to each span in a trace
func AddTagsToTracePayloads(tracePayloads []*pb.TracePayload, tags string) {
	tagMap := map[string]string{}
	tl := strings.Split(tags, ",")
	service := ""
	env := ""
	for _, tag := range tl {
		values := strings.SplitN(tag, ":", 2)
		if len(values) < 2 {
			continue
		}
		tag := values[0]
		value := values[1]
		if strings.ToLower(tag) == "service" {
			service = value
		} else if strings.ToLower(tag) == "env" {
			env = value
		} else {
			tagMap[tag] = value
		}
	}
	serviceLookup := map[string]string{}
	if service != "" {
		serviceLookup = buildServiceLookup(tracePayloads, service)
	}

	for _, tracePayload := range tracePayloads {
		if env != "" {
			tracePayload.Env = env
		}
		for _, trace := range tracePayload.Traces {

			for _, span := range trace.Spans {
				if serviceLookup[span.Service] != "" {
					span.Service = serviceLookup[span.Service]
				}
				for tag, value := range tagMap {
					span.Meta[tag] = value
				}
				if serviceLookup[span.Service] != "" && span.Meta["service"] != "" {
					span.Meta["service"] = serviceLookup[span.Service]
				}
			}
		}
	}
}

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

func buildServiceLookup(tracePayloads []*pb.TracePayload, service string) map[string]string {
	remappedServices := map[string]string{}
	for _, tracePayload := range tracePayloads {
		for _, trace := range tracePayload.Traces {
			for _, span := range trace.Spans {
				if span.Name == "aws.lambda" || span.Service == "aws.lambda" {
					remappedServices[span.Service] = service
				}
			}
		}
	}
	for _, tracePayload := range tracePayloads {
		for _, trace := range tracePayload.Traces {
			for _, span := range trace.Spans {
				for k := range remappedServices {
					if strings.HasPrefix(span.Service, k) && span.Service != k && span.Service != "" {
						remappedServices[span.Service] = strings.Replace(span.Service, k, service, 1)
					}
				}
			}
		}
	}
	return remappedServices
}

func convertSpanToPB(sp span) *pb.Span {
	return &pb.Span{
		Service:  sp.Service,
		Name:     sp.Name,
		Resource: sp.Resource,
		TraceID:  decodeAPMId(sp.TraceID),
		SpanID:   decodeAPMId(sp.SpanID),
		ParentID: decodeAPMId(sp.ParentID),
		Start:    sp.Start,
		Duration: sp.Duration,
		Error:    sp.Error,
		Meta:     sp.Meta,
		Metrics:  sp.Metrics,
		Type:     sp.Type,
	}
}

func addToAPITrace(apiTrace *pb.APITrace, sp *pb.Span) {
	apiTrace.Spans = append(apiTrace.Spans, sp)
	endTime := sp.Start + sp.Duration
	if apiTrace.EndTime > endTime {
		apiTrace.EndTime = endTime
	}
	if apiTrace.StartTime == 0 || apiTrace.StartTime > sp.Start {
		apiTrace.StartTime = sp.Start
	}

}

func decodeAPMId(id string) uint64 {
	if len(id) > 16 {
		id = id[len(id)-16:]
	}
	val, err := strconv.ParseUint(id, 16, 64)
	if err != nil {
		return 0
	}
	return val
}

// GetAnalyzedSpans finds all the analyzed spans in a trace, including top level spans
// and spans marked as anaylzed by the tracer.
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
		// The tracer can mark a span to be analysed, with a value 0-1, where 1 is always keep, and 0 is always reject.
		// Values between 0-1 are used by the agent to prioritise which spans are sampled or not. Since we can't
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
