// Copyright 2020, OpenTelemetry Authors
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

package alibabacloudlogserviceexporter

import (
	"encoding/json"
	"strconv"
	"time"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/gogo/protobuf/proto"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/tracetranslator"
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
func traceDataToLogServiceData(td pdata.Traces) []*sls.Log {
	var slsLogs []*sls.Log
	resourceSpansSlice := td.ResourceSpans()
	for i := 0; i < resourceSpansSlice.Len(); i++ {
		logs := resourceSpansToLogServiceData(resourceSpansSlice.At(i))
		slsLogs = append(slsLogs, logs...)
	}
	return slsLogs
}

func resourceSpansToLogServiceData(resourceSpans pdata.ResourceSpans) []*sls.Log {
	resourceContents := resourceToLogContents(resourceSpans.Resource())
	insLibSpansSlice := resourceSpans.InstrumentationLibrarySpans()
	var slsLogs []*sls.Log
	for i := 0; i < insLibSpansSlice.Len(); i++ {
		insLibSpans := insLibSpansSlice.At(i)
		instrumentationLibraryContents := instrumentationLibraryToLogContents(insLibSpans.InstrumentationLibrary())
		spans := insLibSpans.Spans()
		for j := 0; j < spans.Len(); j++ {
			if slsLog := spanToLogServiceData(spans.At(j), resourceContents, instrumentationLibraryContents); slsLog != nil {
				slsLogs = append(slsLogs, slsLog)
			}
		}
	}
	return slsLogs
}

func spanToLogServiceData(span pdata.Span, resourceContents, instrumentationLibraryContents []*sls.LogContent) *sls.Log {
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
		Value: proto.String(span.TraceID().HexString()),
	})
	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(spanIDField),
		Value: proto.String(span.SpanID().HexString()),
	})
	// if ParentSpanID is not valid, the return "", it is compatible for log service
	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(parentSpanIDField),
		Value: proto.String(span.ParentSpanID().HexString()),
	})

	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(kindField),
		Value: proto.String(spanKindToShortString(span.Kind())),
	})
	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(nameField),
		Value: proto.String(span.Name()),
	})

	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(linksField),
		Value: proto.String(spanLinksToString(span.Links())),
	})
	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(logsField),
		Value: proto.String(eventsToString(span.Events())),
	})
	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(traceStateField),
		Value: proto.String(string(span.TraceState())),
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
	attributeMap := pdata.AttributeMapToMap(span.Attributes())
	attributeJSONBytes, _ := json.Marshal(attributeMap)
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
	return &slsLog
}

func spanKindToShortString(kind pdata.SpanKind) string {
	switch kind {
	case pdata.SpanKindInternal:
		return string(tracetranslator.OpenTracingSpanKindInternal)
	case pdata.SpanKindClient:
		return string(tracetranslator.OpenTracingSpanKindClient)
	case pdata.SpanKindServer:
		return string(tracetranslator.OpenTracingSpanKindServer)
	case pdata.SpanKindProducer:
		return string(tracetranslator.OpenTracingSpanKindProducer)
	case pdata.SpanKindConsumer:
		return string(tracetranslator.OpenTracingSpanKindConsumer)
	default:
		return string(tracetranslator.OpenTracingSpanKindUnspecified)
	}
}

func statusCodeToShortString(code pdata.StatusCode) string {
	switch code {
	case pdata.StatusCodeError:
		return "ERROR"
	case pdata.StatusCodeOk:
		return "OK"
	default:
		return "UNSET"
	}
}

func eventsToString(events pdata.SpanEventSlice) string {
	eventArray := make([]map[string]interface{}, 0, events.Len())
	for i := 0; i < events.Len(); i++ {
		spanEvent := events.At(i)
		event := map[string]interface{}{}
		event[nameField] = spanEvent.Name()
		event[timeField] = spanEvent.Timestamp()
		event[attributeField] = pdata.AttributeMapToMap(spanEvent.Attributes())
		eventArray = append(eventArray, event)
	}
	eventArrayBytes, _ := json.Marshal(&eventArray)
	return string(eventArrayBytes)

}

func spanLinksToString(spanLinkSlice pdata.SpanLinkSlice) string {
	linkArray := make([]map[string]interface{}, 0, spanLinkSlice.Len())
	for i := 0; i < spanLinkSlice.Len(); i++ {
		spanLink := spanLinkSlice.At(i)
		link := map[string]interface{}{}
		link[spanIDField] = spanLink.SpanID().HexString()
		link[traceIDField] = spanLink.TraceID().HexString()
		link[attributeField] = pdata.AttributeMapToMap(spanLink.Attributes())
		linkArray = append(linkArray, link)
	}
	linkArrayBytes, _ := json.Marshal(&linkArray)
	return string(linkArrayBytes)
}
