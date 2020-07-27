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
	"encoding/hex"
	"encoding/json"
	"strconv"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
)

const (
	traceIDField       = "traceID"
	spanIDField        = "spanID"
	parentSpanIDField  = "parentSpanID"
	operationNameField = "operationName"
	referenceField     = "reference"
	startTimeField     = "startTime"
	durationField      = "duration"
	tagsPrefix         = "tags."
	logsField          = "logs"
	serviceNameField   = "process.serviceName"
	processTagsPrefix  = "process.tags."
)

const (
	// Tags
	opencensusLanguage        = "language"
	opencensusExporterVersion = "exporter_version"
	opencensusCoreLibVersion  = "core_lib_version"
	opencensusResourceType    = "resource_type"
)

// traceDataToLogService translates trace data into the LogService format.
func traceDataToLogServiceData(td consumerdata.TraceData) []*sls.Log {
	logs := spansToLogServiceData(td.Spans)
	tagContents := nodeAndResourceToLogContent(td.Node, td.Resource)

	for _, log := range logs {
		log.Contents = append(log.Contents, tagContents...)
	}
	return logs
}

func nodeAndResourceToLogContent(node *commonpb.Node, resource *resourcepb.Resource) []*sls.LogContent {
	if node == nil {
		return []*sls.LogContent{}
	}

	var contents []*sls.LogContent

	for key, val := range node.Attributes {
		contents = append(contents, &sls.LogContent{
			Key:   proto.String(processTagsPrefix + key),
			Value: proto.String(val),
		})
	}
	if node.Identifier != nil {
		if node.Identifier.HostName != "" {
			contents = append(contents, &sls.LogContent{
				Key:   proto.String(processTagsPrefix + "hostname"),
				Value: proto.String(node.Identifier.HostName),
			})
		}
		if node.Identifier.Pid != 0 {
			contents = append(contents, &sls.LogContent{
				Key:   proto.String(processTagsPrefix + "pid"),
				Value: proto.String(strconv.Itoa(int(node.Identifier.Pid))),
			})
		}
		if node.Identifier.StartTimestamp != nil && node.Identifier.StartTimestamp.Seconds != 0 {
			startTimeStr := ptypes.TimestampString(node.Identifier.StartTimestamp)
			contents = append(contents, &sls.LogContent{
				Key:   proto.String(processTagsPrefix + "start.time"),
				Value: proto.String(startTimeStr),
			})
		}
	}

	// Add OpenCensus library information as tags if available
	ocLib := node.LibraryInfo
	if ocLib != nil {
		// Only add language if specified
		if ocLib.Language != commonpb.LibraryInfo_LANGUAGE_UNSPECIFIED {
			languageStr := ocLib.Language.String()
			contents = append(contents, &sls.LogContent{
				Key:   proto.String(processTagsPrefix + opencensusLanguage),
				Value: proto.String(languageStr),
			})
		}
		if ocLib.ExporterVersion != "" {
			contents = append(contents, &sls.LogContent{
				Key:   proto.String(processTagsPrefix + opencensusExporterVersion),
				Value: proto.String(ocLib.ExporterVersion),
			})
		}
		if ocLib.CoreLibraryVersion != "" {
			contents = append(contents, &sls.LogContent{
				Key:   proto.String(processTagsPrefix + opencensusCoreLibVersion),
				Value: proto.String(ocLib.CoreLibraryVersion),
			})
		}
	}

	var serviceName string
	if node.ServiceInfo != nil && node.ServiceInfo.Name != "" {
		serviceName = node.ServiceInfo.Name
	}

	if resource != nil {
		resourceType := resource.GetType()
		if resourceType != "" {
			contents = append(contents, &sls.LogContent{
				Key:   proto.String(processTagsPrefix + opencensusResourceType),
				Value: proto.String(resourceType),
			})
		}
		for k, v := range resource.GetLabels() {
			contents = append(contents, &sls.LogContent{
				Key:   proto.String(processTagsPrefix + k),
				Value: proto.String(v),
			})
		}
	}

	if serviceName == "" && len(contents) == 0 {
		// No info to put in the process...
		return nil
	}

	contents = append(contents, &sls.LogContent{
		Key:   proto.String(serviceNameField),
		Value: proto.String(serviceName),
	})
	return contents
}

func spansToLogServiceData(spans []*tracepb.Span) []*sls.Log {
	if spans == nil {
		return nil
	}

	// Pre-allocate assuming that few, if any spans, are nil.
	logs := make([]*sls.Log, 0, len(spans))
	for _, span := range spans {
		contents := make([]*sls.LogContent, 0)
		traceID := hex.EncodeToString(span.TraceId[:])
		spanID := hex.EncodeToString(span.SpanId[:])
		contents = append(contents,
			&sls.LogContent{
				Key:   proto.String(traceIDField),
				Value: proto.String(traceID),
			})
		contents = append(contents,
			&sls.LogContent{
				Key:   proto.String(spanIDField),
				Value: proto.String(spanID),
			})
		linksContent := linksToLogContents(span.Links)
		if linksContent != nil {
			contents = append(contents, linksContent)
		}
		if len(span.ParentSpanId) != 0 {
			parentSpanID := hex.EncodeToString(span.ParentSpanId[:])
			contents = append(contents,
				&sls.LogContent{
					Key:   proto.String(parentSpanIDField),
					Value: proto.String(parentSpanID),
				})
		} else {
			// set "0" if no ParentSpanId
			contents = append(contents,
				&sls.LogContent{
					Key:   proto.String(parentSpanIDField),
					Value: proto.String("0"),
				})
		}
		startTime := timestampToEpochMicroseconds(span.StartTime)
		contents = append(contents,
			&sls.LogContent{
				Key:   proto.String(startTimeField),
				Value: proto.String(strconv.FormatInt(startTime, 10)),
			})

		contents = append(contents,
			&sls.LogContent{
				Key:   proto.String(operationNameField),
				Value: proto.String(truncableStringToStr(span.Name)),
			})

		contents = append(contents,
			&sls.LogContent{
				Key:   proto.String(durationField),
				Value: proto.String(strconv.FormatInt(timestampToEpochMicroseconds(span.EndTime)-startTime, 10)),
			})

		for k, v := range span.GetAttributes().GetAttributeMap() {
			contents = append(contents, &sls.LogContent{
				Key:   proto.String(tagsPrefix + string(k)),
				Value: proto.String(attributeValueToString(v)),
			})
		}

		if logContent := timeEventToLogContent(span.TimeEvents); logContent != nil {
			contents = append(contents, logContent)
		}

		// Only add the "span.kind" tag if not set in the OC span attributes.
		if !tracetranslator.OCAttributeKeyExist(span.Attributes, tracetranslator.TagSpanKind) {
			contents = append(contents,
				&sls.LogContent{
					Key:   proto.String(tagsPrefix + tracetranslator.TagSpanKind),
					Value: proto.String(spanKindToStr(span.Kind)),
				})
		}
		// Only add status tags if neither status.code and status.message are set in the OC span attributes.
		if !tracetranslator.OCAttributeKeyExist(span.Attributes, tracetranslator.TagStatusCode) &&
			!tracetranslator.OCAttributeKeyExist(span.Attributes, tracetranslator.TagStatusMsg) {
			contents = append(contents,
				&sls.LogContent{
					Key:   proto.String(tagsPrefix + tracetranslator.TagStatusCode),
					Value: proto.String(strconv.Itoa(int(span.GetStatus().GetCode()))),
				})
			contents = append(contents,
				&sls.LogContent{
					Key:   proto.String(tagsPrefix + tracetranslator.TagStatusMsg),
					Value: proto.String(span.GetStatus().GetMessage()),
				})
		}

		logs = append(logs, &sls.Log{
			Time:     proto.Uint32(uint32(span.GetEndTime().GetSeconds())),
			Contents: contents,
		})
	}

	return logs
}

func linksToLogContents(ocSpanLinks *tracepb.Span_Links) *sls.LogContent {
	if ocSpanLinks == nil || ocSpanLinks.Link == nil {
		return nil
	}

	ocLinks := ocSpanLinks.Link

	type linkSpanRef struct {
		TraceID string
		SpanID  string
		RefType string
	}
	spanRefs := make([]linkSpanRef, 0, len(ocLinks))

	for _, ocLink := range ocLinks {
		spanRefs = append(spanRefs, linkSpanRef{
			TraceID: hex.EncodeToString(ocLink.TraceId[:]),
			SpanID:  hex.EncodeToString(ocLink.SpanId[:]),
			RefType: ocLink.GetType().String(),
		})
	}

	spanRefsStr, _ := json.Marshal(spanRefs)
	return &sls.LogContent{
		Key:   proto.String(referenceField),
		Value: proto.String(string(spanRefsStr)),
	}
}

func attributeValueToString(v *tracepb.AttributeValue) string {
	switch attribValue := v.Value.(type) {
	case *tracepb.AttributeValue_StringValue:
		return truncableStringToStr(attribValue.StringValue)
	case *tracepb.AttributeValue_IntValue:
		return strconv.FormatInt(attribValue.IntValue, 10)
	case *tracepb.AttributeValue_BoolValue:
		if attribValue.BoolValue {
			return "true"
		}
		return "false"
	case *tracepb.AttributeValue_DoubleValue:
		return strconv.FormatFloat(attribValue.DoubleValue, 'g', -1, 64)
	default:
	}
	return "<Unknown OpenCensus Attribute>"
}

func spanKindToStr(spanKind tracepb.Span_SpanKind) string {

	switch spanKind {
	case tracepb.Span_CLIENT:
		return string(tracetranslator.OpenTracingSpanKindClient)
	case tracepb.Span_SERVER:
		return string(tracetranslator.OpenTracingSpanKindServer)
	}
	return ""
}

func timeEventToLogContent(ocSpanTimeEvents *tracepb.Span_TimeEvents) *sls.LogContent {
	if ocSpanTimeEvents == nil || ocSpanTimeEvents.TimeEvent == nil {
		return nil
	}

	ocTimeEvents := ocSpanTimeEvents.TimeEvent

	type timeEvent struct {
		TimeUs int64
		Fields map[string]string
	}

	// Assume that in general no time events are going to produce nil Jaeger logs.
	timeEvents := make([]timeEvent, 0, len(ocTimeEvents))
	for _, ocTimeEvent := range ocTimeEvents {
		e := timeEvent{
			TimeUs: timestampToEpochMicroseconds(ocTimeEvent.Time),
		}
		switch teValue := ocTimeEvent.Value.(type) {
		case *tracepb.Span_TimeEvent_Annotation_:
			e.Fields = ocAnnotationToMap(teValue.Annotation)
		case *tracepb.Span_TimeEvent_MessageEvent_:
			e.Fields = ocMessageEventToMap(teValue.MessageEvent)
		default:
			msg := "An unknown OpenCensus TimeEvent type was detected when translating"
			e.Fields = make(map[string]string)
			e.Fields["error"] = msg
		}
		timeEvents = append(timeEvents, e)
	}

	// ignore marshal error
	timeEventsStr, _ := json.Marshal(timeEvents)
	return &sls.LogContent{
		Key:   proto.String(logsField),
		Value: proto.String(string(timeEventsStr)),
	}
}

func ocAnnotationToMap(annotation *tracepb.Span_TimeEvent_Annotation) map[string]string {
	if annotation == nil {
		return nil
	}

	keyVals := make(map[string]string)
	for k, v := range annotation.GetAttributes().GetAttributeMap() {
		keyVals[k] = attributeValueToString(v)
	}

	desc := truncableStringToStr(annotation.Description)
	if desc != "" {
		keyVals[tracetranslator.AnnotationDescriptionKey] = desc
	}
	return keyVals
}

func ocMessageEventToMap(msgEvent *tracepb.Span_TimeEvent_MessageEvent) map[string]string {
	if msgEvent == nil {
		return nil
	}

	keyVals := make(map[string]string)
	keyVals[tracetranslator.MessageEventIDKey] = strconv.FormatUint(msgEvent.Id, 10)
	keyVals[tracetranslator.MessageEventTypeKey] = msgEvent.Type.String()

	// Some implementations always have these two fields as zeros.
	if msgEvent.CompressedSize == 0 && msgEvent.UncompressedSize == 0 {
		return keyVals
	}

	keyVals[tracetranslator.MessageEventCompressedSizeKey] = strconv.FormatUint(msgEvent.CompressedSize, 10)
	keyVals[tracetranslator.MessageEventUncompressedSizeKey] = strconv.FormatUint(msgEvent.UncompressedSize, 10)

	return keyVals
}

func truncableStringToStr(ts *tracepb.TruncatableString) string {
	if ts == nil {
		return ""
	}
	return ts.Value
}

func timestampToEpochMicroseconds(ts *timestamp.Timestamp) int64 {
	if ts == nil {
		return 0
	}
	return ts.GetSeconds()*1e6 + int64(ts.GetNanos()/1e3)
}
