// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudlogserviceexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter"

import (
	"encoding/json"
	"strconv"
	"time"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/tracetranslator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

const (
	traceIDField       = "traceID"
	spanIDField        = "spanID"
	parentSpanIDField  = "parentSpanID"
	nameField          = "name"
	kindField          = "kind"
	linksField         = "links"
	timeField          = "time"
	startTimeField     = "start"
	endTimeField       = "end"
	traceStateField    = "traceState"
	durationField      = "duration"
	attributeField     = "attribute"
	statusCodeField    = "statusCode"
	statusMessageField = "statusMessage"
	logsField          = "logs"
)

// traceDataToLogService translates trace data into the LogService format.
func traceDataToLogServiceData(td ptrace.Traces) []*sls.Log {
	var slsLogs []*sls.Log
	resourceSpansSlice := td.ResourceSpans()
	for i := 0; i < resourceSpansSlice.Len(); i++ {
		logs := resourceSpansToLogServiceData(resourceSpansSlice.At(i))
		slsLogs = append(slsLogs, logs...)
	}
	return slsLogs
}

func resourceSpansToLogServiceData(resourceSpans ptrace.ResourceSpans) []*sls.Log {
	resourceContents := resourceToLogContents(resourceSpans.Resource())
	scopeSpansSlice := resourceSpans.ScopeSpans()
	var slsLogs []*sls.Log
	for i := 0; i < scopeSpansSlice.Len(); i++ {
		insLibSpans := scopeSpansSlice.At(i)
		instrumentationLibraryContents := instrumentationScopeToLogContents(insLibSpans.Scope())
		spans := insLibSpans.Spans()
		for j := 0; j < spans.Len(); j++ {
			if slsLog := spanToLogServiceData(spans.At(j), resourceContents, instrumentationLibraryContents); slsLog != nil {
				slsLogs = append(slsLogs, slsLog)
			}
		}
	}
	return slsLogs
}

func spanToLogServiceData(span ptrace.Span, resourceContents, instrumentationLibraryContents []*sls.LogContent) *sls.Log {
	timeNano := int64(span.EndTimestamp())
	if timeNano == 0 {
		timeNano = time.Now().UnixNano()
	}
	slsLog := sls.Log{
		Time: new(uint32(timeNano / 1000 / 1000 / 1000)),
	}
	// pre alloc, refine if logContent's len > 16
	preAllocCount := 16
	slsLog.Contents = make([]*sls.LogContent, 0, preAllocCount+len(resourceContents)+len(instrumentationLibraryContents))
	contentsBuffer := make([]sls.LogContent, 0, preAllocCount)

	slsLog.Contents = append(slsLog.Contents, resourceContents...)
	slsLog.Contents = append(slsLog.Contents, instrumentationLibraryContents...)

	contentsBuffer = append(contentsBuffer,
		sls.LogContent{
			Key:   new(traceIDField),
			Value: new(traceutil.TraceIDToHexOrEmptyString(span.TraceID())),
		},
		sls.LogContent{
			Key:   new(spanIDField),
			Value: new(traceutil.SpanIDToHexOrEmptyString(span.SpanID())),
		},
		// if ParentSpanID is not valid, the return "", it is compatible for log service
		sls.LogContent{
			Key:   new(parentSpanIDField),
			Value: new(traceutil.SpanIDToHexOrEmptyString(span.ParentSpanID())),
		},
		sls.LogContent{
			Key:   new(kindField),
			Value: new(spanKindToShortString(span.Kind())),
		},
		sls.LogContent{
			Key:   new(nameField),
			Value: new(span.Name()),
		},
		sls.LogContent{
			Key:   new(linksField),
			Value: new(spanLinksToString(span.Links())),
		},
		sls.LogContent{
			Key:   new(logsField),
			Value: new(eventsToString(span.Events())),
		},
		sls.LogContent{
			Key:   new(traceStateField),
			Value: new(span.TraceState().AsRaw()),
		},
		sls.LogContent{
			Key:   new(startTimeField),
			Value: new(strconv.FormatUint(uint64(span.StartTimestamp()/1000), 10)),
		},
		sls.LogContent{
			Key:   new(endTimeField),
			Value: new(strconv.FormatUint(uint64(span.EndTimestamp()/1000), 10)),
		},
		sls.LogContent{
			Key:   new(durationField),
			Value: new(strconv.FormatUint(uint64((span.EndTimestamp()-span.StartTimestamp())/1000), 10)),
		})
	attributeMap := span.Attributes().AsRaw()
	attributeJSONBytes, _ := json.Marshal(attributeMap)
	contentsBuffer = append(contentsBuffer,
		sls.LogContent{
			Key:   new(attributeField),
			Value: new(string(attributeJSONBytes)),
		},
		sls.LogContent{
			Key:   new(statusCodeField),
			Value: new(statusCodeToShortString(span.Status().Code())),
		},
		sls.LogContent{
			Key:   new(statusMessageField),
			Value: new(span.Status().Message()),
		})

	for i := range contentsBuffer {
		slsLog.Contents = append(slsLog.Contents, &contentsBuffer[i])
	}
	return &slsLog
}

func spanKindToShortString(kind ptrace.SpanKind) string {
	switch kind {
	case ptrace.SpanKindInternal:
		return string(tracetranslator.OpenTracingSpanKindInternal)
	case ptrace.SpanKindClient:
		return string(tracetranslator.OpenTracingSpanKindClient)
	case ptrace.SpanKindServer:
		return string(tracetranslator.OpenTracingSpanKindServer)
	case ptrace.SpanKindProducer:
		return string(tracetranslator.OpenTracingSpanKindProducer)
	case ptrace.SpanKindConsumer:
		return string(tracetranslator.OpenTracingSpanKindConsumer)
	default:
		return string(tracetranslator.OpenTracingSpanKindUnspecified)
	}
}

func statusCodeToShortString(code ptrace.StatusCode) string {
	switch code {
	case ptrace.StatusCodeError:
		return "ERROR"
	case ptrace.StatusCodeOk:
		return "OK"
	default:
		return "UNSET"
	}
}

func eventsToString(events ptrace.SpanEventSlice) string {
	eventArray := make([]map[string]any, 0, events.Len())
	for i := 0; i < events.Len(); i++ {
		spanEvent := events.At(i)
		event := map[string]any{}
		event[nameField] = spanEvent.Name()
		event[timeField] = spanEvent.Timestamp()
		event[attributeField] = spanEvent.Attributes().AsRaw()
		eventArray = append(eventArray, event)
	}
	eventArrayBytes, _ := json.Marshal(&eventArray)
	return string(eventArrayBytes)
}

func spanLinksToString(spanLinkSlice ptrace.SpanLinkSlice) string {
	linkArray := make([]map[string]any, 0, spanLinkSlice.Len())
	for i := 0; i < spanLinkSlice.Len(); i++ {
		spanLink := spanLinkSlice.At(i)
		link := map[string]any{}
		link[spanIDField] = traceutil.SpanIDToHexOrEmptyString(spanLink.SpanID())
		link[traceIDField] = traceutil.TraceIDToHexOrEmptyString(spanLink.TraceID())
		link[attributeField] = spanLink.Attributes().AsRaw()
		linkArray = append(linkArray, link)
	}
	linkArrayBytes, _ := json.Marshal(&linkArray)
	return string(linkArrayBytes)
}
