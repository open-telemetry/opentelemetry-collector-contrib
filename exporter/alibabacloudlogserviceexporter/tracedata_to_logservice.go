// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudlogserviceexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter"

import (
	"encoding/json"
	"strconv"
	"time"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/gogo/protobuf/proto"
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
func traceDataToLogServiceData(td ptrace.Traces) ([]*sls.Log, error) {
	var slsLogs []*sls.Log
	resourceSpansSlice := td.ResourceSpans()
	for i := 0; i < resourceSpansSlice.Len(); i++ {
		logs, err := resourceSpansToLogServiceData(resourceSpansSlice.At(i))
		if err != nil {
			return nil, err
		}
		slsLogs = append(slsLogs, logs...)
	}
	return slsLogs, nil
}

func resourceSpansToLogServiceData(resourceSpans ptrace.ResourceSpans) ([]*sls.Log, error) {
	resourceContents, err := resourceToLogContents(resourceSpans.Resource())
	if err != nil {
		return nil, err
	}
	scopeSpansSlice := resourceSpans.ScopeSpans()
	var slsLogs []*sls.Log
	for i := 0; i < scopeSpansSlice.Len(); i++ {
		insLibSpans := scopeSpansSlice.At(i)
		instrumentationLibraryContents := instrumentationScopeToLogContents(insLibSpans.Scope())
		spans := insLibSpans.Spans()
		for j := 0; j < spans.Len(); j++ {
			if slsLog, err2 := spanToLogServiceData(spans.At(j), resourceContents, instrumentationLibraryContents); slsLog != nil {
				if err2 != nil {
					return nil, err2
				}
				slsLogs = append(slsLogs, slsLog)
			}
		}
	}
	return slsLogs, nil
}

func spanToLogServiceData(span ptrace.Span, resourceContents, instrumentationLibraryContents []*sls.LogContent) (*sls.Log, error) {
	timeNano := int64(span.EndTimestamp())
	if timeNano == 0 {
		timeNano = time.Now().UnixNano()
	}
	slsLog := sls.Log{
		Time: proto.Uint32(uint32(timeNano / 1000 / 1000 / 1000)),
	}
	// pre alloc, refine if logContent's len > 16
	preAllocCount := 16
	slsLog.Contents = make([]*sls.LogContent, 0, preAllocCount+len(resourceContents)+len(instrumentationLibraryContents))
	contentsBuffer := make([]sls.LogContent, 0, preAllocCount)

	slsLog.Contents = append(slsLog.Contents, resourceContents...)
	slsLog.Contents = append(slsLog.Contents, instrumentationLibraryContents...)

	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(traceIDField),
		Value: proto.String(traceutil.TraceIDToHexOrEmptyString(span.TraceID())),
	})
	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(spanIDField),
		Value: proto.String(traceutil.SpanIDToHexOrEmptyString(span.SpanID())),
	})
	// if ParentSpanID is not valid, the return "", it is compatible for log service
	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(parentSpanIDField),
		Value: proto.String(traceutil.SpanIDToHexOrEmptyString(span.ParentSpanID())),
	})

	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(kindField),
		Value: proto.String(spanKindToShortString(span.Kind())),
	})
	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(nameField),
		Value: proto.String(span.Name()),
	})

	links, err := spanLinksToString(span.Links())
	if err != nil {
		return nil, err
	}
	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(linksField),
		Value: proto.String(links),
	})
	events, err := eventsToString(span.Events())
	if err != nil {
		return nil, err
	}
	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(logsField),
		Value: proto.String(events),
	})
	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(traceStateField),
		Value: proto.String(span.TraceState().AsRaw()),
	})
	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(startTimeField),
		Value: proto.String(strconv.FormatUint(uint64(span.StartTimestamp()/1000), 10)),
	})
	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(endTimeField),
		Value: proto.String(strconv.FormatUint(uint64(span.EndTimestamp()/1000), 10)),
	})
	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(durationField),
		Value: proto.String(strconv.FormatUint(uint64((span.EndTimestamp()-span.StartTimestamp())/1000), 10)),
	})
	attributeMap := span.Attributes().AsRaw()
	attributeJSONBytes, err := json.Marshal(attributeMap)
	if err != nil {
		return nil, err
	}
	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(attributeField),
		Value: proto.String(string(attributeJSONBytes)),
	})

	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(statusCodeField),
		Value: proto.String(statusCodeToShortString(span.Status().Code())),
	})

	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(statusMessageField),
		Value: proto.String(span.Status().Message()),
	})

	for i := range contentsBuffer {
		slsLog.Contents = append(slsLog.Contents, &contentsBuffer[i])
	}
	return &slsLog, nil
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

func eventsToString(events ptrace.SpanEventSlice) (string, error) {
	eventArray := make([]map[string]interface{}, 0, events.Len())
	for i := 0; i < events.Len(); i++ {
		spanEvent := events.At(i)
		event := map[string]interface{}{}
		event[nameField] = spanEvent.Name()
		event[timeField] = spanEvent.Timestamp()
		event[attributeField] = spanEvent.Attributes().AsRaw()
		eventArray = append(eventArray, event)
	}
	eventArrayBytes, err := json.Marshal(&eventArray)
	return string(eventArrayBytes), err

}

func spanLinksToString(spanLinkSlice ptrace.SpanLinkSlice) (string, error) {
	linkArray := make([]map[string]interface{}, 0, spanLinkSlice.Len())
	for i := 0; i < spanLinkSlice.Len(); i++ {
		spanLink := spanLinkSlice.At(i)
		link := map[string]interface{}{}
		link[spanIDField] = traceutil.SpanIDToHexOrEmptyString(spanLink.SpanID())
		link[traceIDField] = traceutil.TraceIDToHexOrEmptyString(spanLink.TraceID())
		link[attributeField] = spanLink.Attributes().AsRaw()
		linkArray = append(linkArray, link)
	}
	linkArrayBytes, err := json.Marshal(&linkArray)
	return string(linkArrayBytes), err
}
