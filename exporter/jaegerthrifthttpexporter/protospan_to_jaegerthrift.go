// Copyright 2019, OpenTelemetry Authors
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

package jaegerthrifthttpexporter

import (
	"fmt"
	"time"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/jaegertracing/jaeger/thrift-gen/jaeger"
	"go.opentelemetry.io/collector/consumer/pdata"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	unknownProcess = &jaeger.Process{ServiceName: "unknown-service-name"}
)

const (
	annotationDescriptionKey = "description"

	messageEventIDKey               = "message.id"
	messageEventTypeKey             = "message.type"
	messageEventCompressedSizeKey   = "message.compressed_size"
	messageEventUncompressedSizeKey = "message.uncompressed_size"
)

// traceData helper struct for conversion.
// TODO: Remove this when exporter translates directly to pdata.
type traceData struct {
	Node     *commonpb.Node
	Resource *resourcepb.Resource
	Spans    []*tracepb.Span
}

// oCProtoToJaegerThrift translates OpenCensus trace data into the Jaeger Thrift format.
func oCProtoToJaegerThrift(td traceData) (*jaeger.Batch, error) {
	jSpans, err := ocSpansToJaegerSpans(td.Spans)
	if err != nil {
		return nil, err
	}

	jb := &jaeger.Batch{
		Process: ocNodeAndResourceToJaegerProcess(td.Node, td.Resource),
		Spans:   jSpans,
	}

	return jb, nil
}

func ocNodeAndResourceToJaegerProcess(node *commonpb.Node, resource *resourcepb.Resource) *jaeger.Process {
	if node == nil {
		// Jaeger requires a non-nil Process
		return unknownProcess
	}

	var jTags []*jaeger.Tag
	nodeAttribsLen := len(node.Attributes)
	if nodeAttribsLen > 0 {
		jTags = make([]*jaeger.Tag, 0, nodeAttribsLen)
		for k, v := range node.Attributes {
			str := v
			jTag := &jaeger.Tag{
				Key:   k,
				VType: jaeger.TagType_STRING,
				VStr:  &str,
			}
			jTags = append(jTags, jTag)
		}
	}

	if node.Identifier != nil {
		if node.Identifier.HostName != "" {
			hostTag := &jaeger.Tag{
				Key:   "hostname",
				VType: jaeger.TagType_STRING,
				VStr:  &node.Identifier.HostName,
			}
			jTags = append(jTags, hostTag)
		}
		if node.Identifier.Pid != 0 {
			pid := int64(node.Identifier.Pid)
			hostTag := &jaeger.Tag{
				Key:   "pid",
				VType: jaeger.TagType_LONG,
				VLong: &pid,
			}
			jTags = append(jTags, hostTag)
		}
		if node.Identifier.StartTimestamp != nil && node.Identifier.StartTimestamp.Seconds != 0 {
			startTimeStr := node.Identifier.StartTimestamp.AsTime().Format(time.RFC3339Nano)
			hostTag := &jaeger.Tag{
				Key:   "start.time",
				VType: jaeger.TagType_STRING,
				VStr:  &startTimeStr,
			}
			jTags = append(jTags, hostTag)
		}
	}

	// Add OpenCensus library information as tags if available
	ocLib := node.LibraryInfo
	if ocLib != nil {
		// Only add language if specified
		if ocLib.Language != commonpb.LibraryInfo_LANGUAGE_UNSPECIFIED {
			languageStr := ocLib.Language.String()
			languageTag := &jaeger.Tag{
				Key:   opencensusLanguage,
				VType: jaeger.TagType_STRING,
				VStr:  &languageStr,
			}
			jTags = append(jTags, languageTag)
		}
		if ocLib.ExporterVersion != "" {
			exporterTag := &jaeger.Tag{
				Key:   opencensusExporterVersion,
				VType: jaeger.TagType_STRING,
				VStr:  &ocLib.ExporterVersion,
			}
			jTags = append(jTags, exporterTag)
		}
		if ocLib.CoreLibraryVersion != "" {
			exporterTag := &jaeger.Tag{
				Key:   opencensusCoreLibVersion,
				VType: jaeger.TagType_STRING,
				VStr:  &ocLib.CoreLibraryVersion,
			}
			jTags = append(jTags, exporterTag)
		}
	}

	var serviceName string
	if node.ServiceInfo != nil && node.ServiceInfo.Name != "" {
		serviceName = node.ServiceInfo.Name
	}

	if resource != nil {
		resourceType := resource.GetType()
		if resourceType != "" {
			resourceTypeTag := &jaeger.Tag{
				Key:   opencensusResourceType,
				VType: jaeger.TagType_STRING,
				VStr:  &resourceType,
			}
			jTags = append(jTags, resourceTypeTag)
		}
		for k, v := range resource.GetLabels() {
			str := v
			resourceTag := &jaeger.Tag{
				Key:   k,
				VType: jaeger.TagType_STRING,
				VStr:  &str,
			}
			jTags = append(jTags, resourceTag)
		}
	}

	if serviceName == "" && len(jTags) == 0 {
		// No info to put in the process...
		return nil
	}

	jProc := &jaeger.Process{
		ServiceName: serviceName,
		Tags:        jTags,
	}

	return jProc
}

func ocSpansToJaegerSpans(ocSpans []*tracepb.Span) ([]*jaeger.Span, error) {
	if ocSpans == nil {
		return nil, nil
	}

	// Pre-allocate assuming that few, if any spans, are nil.
	jSpans := make([]*jaeger.Span, 0, len(ocSpans))
	for _, ocSpan := range ocSpans {
		traceIDHigh, traceIDLow, err := traceIDToInt64(ocSpan.TraceId)
		if err != nil {
			return nil, fmt.Errorf("OC span has invalid trace ID: %v", err)
		}
		if traceIDLow == 0 && traceIDHigh == 0 {
			return nil, errZeroTraceID
		}
		jReferences, err := ocLinksToJaegerReferences(ocSpan.Links)
		if err != nil {
			return nil, fmt.Errorf("error converting OC links to Jaeger references: %v", err)
		}
		spanID, err := spanIDToInt64(ocSpan.SpanId)
		if err != nil {
			return nil, fmt.Errorf("OC span has invalid span ID: %v", err)
		}
		if spanID == 0 {
			return nil, errZeroSpanID
		}
		// OC ParentSpanId can be nil/empty: only attempt conversion if not nil/empty.
		var parentSpanID int64
		if len(ocSpan.ParentSpanId) != 0 {
			parentSpanID, err = spanIDToInt64(ocSpan.ParentSpanId)
			if err != nil {
				return nil, fmt.Errorf("OC span has invalid parent span ID: %v", err)
			}
		}
		startTime := timestampToEpochMicroseconds(ocSpan.StartTime)
		jSpan := &jaeger.Span{
			TraceIdLow:    traceIDLow,
			TraceIdHigh:   traceIDHigh,
			SpanId:        spanID,
			ParentSpanId:  parentSpanID,
			OperationName: truncableStringToStr(ocSpan.Name),
			References:    jReferences,
			// Flags: TODO (@pjanotti) Nothing from OC-Proto seems to match the values for Flags see https://www.jaegertracing.io/docs/1.8/client-libraries/
			StartTime: startTime,
			Duration:  timestampToEpochMicroseconds(ocSpan.EndTime) - startTime,
			Tags:      ocSpanAttributesToJaegerTags(ocSpan.Attributes),
			Logs:      ocTimeEventsToJaegerLogs(ocSpan.TimeEvents),
		}

		// Only add the "span.kind" tag if not set in the OC span attributes.
		if !ocAttributeKeyExist(ocSpan.Attributes, tracetranslator.TagSpanKind) {
			jSpan.Tags = appendJaegerTagFromOCSpanKind(jSpan.Tags, ocSpan.Kind)
		}
		// Only add status tags if neither status.code and status.message are set in the OC span attributes.
		if !ocAttributeKeyExist(ocSpan.Attributes, tracetranslator.TagStatusCode) &&
			!ocAttributeKeyExist(ocSpan.Attributes, tracetranslator.TagStatusMsg) {
			jSpan.Tags = appendJaegerThriftTagFromOCStatus(jSpan.Tags, ocSpan.Status)
		}
		jSpans = append(jSpans, jSpan)
	}

	return jSpans, nil
}

func ocLinksToJaegerReferences(ocSpanLinks *tracepb.Span_Links) ([]*jaeger.SpanRef, error) {
	if ocSpanLinks == nil || ocSpanLinks.Link == nil {
		return nil, nil
	}

	ocLinks := ocSpanLinks.Link
	jRefs := make([]*jaeger.SpanRef, 0, len(ocLinks))
	for _, ocLink := range ocLinks {
		traceIDHigh, traceIDLow, err := traceIDToInt64(ocLink.TraceId)
		if err != nil {
			return nil, fmt.Errorf("OC link has invalid trace ID: %v", err)
		}

		var jRefType jaeger.SpanRefType
		switch ocLink.Type {
		case tracepb.Span_Link_PARENT_LINKED_SPAN:
			jRefType = jaeger.SpanRefType_CHILD_OF
		default:
			// TODO: (@pjanotti) Jaeger doesn't have a unknown SpanRefType, it has FOLLOWS_FROM or CHILD_OF
			// at first mapping all others to FOLLOWS_FROM.
			jRefType = jaeger.SpanRefType_FOLLOWS_FROM
		}

		spanID, err := spanIDToInt64(ocLink.SpanId)
		if err != nil {
			return nil, fmt.Errorf("OC link has invalid span ID: %v", err)
		}

		jRef := &jaeger.SpanRef{
			TraceIdLow:  traceIDLow,
			TraceIdHigh: traceIDHigh,
			RefType:     jRefType,
			SpanId:      spanID,
		}
		jRefs = append(jRefs, jRef)
	}

	return jRefs, nil
}

func appendJaegerThriftTagFromOCStatus(jTags []*jaeger.Tag, ocStatus *tracepb.Status) []*jaeger.Tag {
	if ocStatus == nil {
		return jTags
	}

	code := int64(ocStatus.Code)
	jTags = append(jTags, &jaeger.Tag{
		Key:   tracetranslator.TagStatusCode,
		VLong: &code,
		VType: jaeger.TagType_LONG,
	})

	if ocStatus.Message != "" {
		jTags = append(jTags, &jaeger.Tag{
			Key:   tracetranslator.TagStatusMsg,
			VStr:  &ocStatus.Message,
			VType: jaeger.TagType_STRING,
		})
	}

	return jTags
}

func appendJaegerTagFromOCSpanKind(jTags []*jaeger.Tag, ocSpanKind tracepb.Span_SpanKind) []*jaeger.Tag {
	// Follow OpenTracing conventions to set span kind value as a tag.

	var tagValue string
	switch ocSpanKind {
	case tracepb.Span_CLIENT:
		tagValue = string(tracetranslator.OpenTracingSpanKindClient)
	case tracepb.Span_SERVER:
		tagValue = string(tracetranslator.OpenTracingSpanKindServer)
	}

	if tagValue != "" {
		jTag := &jaeger.Tag{
			Key:   tracetranslator.TagSpanKind,
			VStr:  &tagValue,
			VType: jaeger.TagType_STRING,
		}
		jTags = append(jTags, jTag)
	}

	return jTags
}

func ocTimeEventsToJaegerLogs(ocSpanTimeEvents *tracepb.Span_TimeEvents) []*jaeger.Log {
	if ocSpanTimeEvents == nil || ocSpanTimeEvents.TimeEvent == nil {
		return nil
	}

	ocTimeEvents := ocSpanTimeEvents.TimeEvent

	// Assume that in general no time events are going to produce nil Jaeger logs.
	jLogs := make([]*jaeger.Log, 0, len(ocTimeEvents))
	for _, ocTimeEvent := range ocTimeEvents {
		jLog := &jaeger.Log{
			Timestamp: timestampToEpochMicroseconds(ocTimeEvent.Time),
		}
		switch teValue := ocTimeEvent.Value.(type) {
		case *tracepb.Span_TimeEvent_Annotation_:
			jLog.Fields = ocAnnotationToJagerTags(teValue.Annotation)
		case *tracepb.Span_TimeEvent_MessageEvent_:
			jLog.Fields = ocMessageEventToJaegerTags(teValue.MessageEvent)
		default:
			msg := "An unknown OpenCensus TimeEvent type was detected when translating to Jaeger"
			jTag := &jaeger.Tag{
				Key:  ocTimeEventUnknownType,
				VStr: &msg,
			}
			jLog.Fields = append(jLog.Fields, jTag)
		}

		jLogs = append(jLogs, jLog)
	}

	return jLogs
}

func ocAnnotationToJagerTags(annotation *tracepb.Span_TimeEvent_Annotation) []*jaeger.Tag {
	if annotation == nil {
		return nil
	}

	jTags := ocSpanAttributesToJaegerTags(annotation.Attributes)

	desc := truncableStringToStr(annotation.Description)
	if desc != "" {
		jDescTag := &jaeger.Tag{
			Key:   annotationDescriptionKey,
			VStr:  &desc,
			VType: jaeger.TagType_STRING,
		}
		jTags = append(jTags, jDescTag)
	}

	return jTags
}

func ocMessageEventToJaegerTags(msgEvent *tracepb.Span_TimeEvent_MessageEvent) []*jaeger.Tag {
	if msgEvent == nil {
		return nil
	}

	jID := int64(msgEvent.Id)
	idTag := &jaeger.Tag{
		Key:   messageEventIDKey,
		VLong: &jID,
		VType: jaeger.TagType_LONG,
	}

	msgTypeStr := msgEvent.Type.String()
	msgType := &jaeger.Tag{
		Key:   messageEventTypeKey,
		VStr:  &msgTypeStr,
		VType: jaeger.TagType_STRING,
	}

	// Some implementations always have these two fields as zeros.
	if msgEvent.CompressedSize == 0 && msgEvent.UncompressedSize == 0 {
		return []*jaeger.Tag{
			idTag, msgType,
		}
	}

	// There is a risk in this cast since we are converting from uint64, but
	// seems a good compromise since the risk of such large values are small.
	compSize := int64(msgEvent.CompressedSize)
	compressedSize := &jaeger.Tag{
		Key:   messageEventCompressedSizeKey,
		VLong: &compSize,
		VType: jaeger.TagType_LONG,
	}

	uncompSize := int64(msgEvent.UncompressedSize)
	uncompressedSize := &jaeger.Tag{
		Key:   messageEventUncompressedSizeKey,
		VLong: &uncompSize,
		VType: jaeger.TagType_LONG,
	}

	return []*jaeger.Tag{
		idTag, msgType, compressedSize, uncompressedSize,
	}
}

func truncableStringToStr(ts *tracepb.TruncatableString) string {
	if ts == nil {
		return ""
	}
	return ts.Value
}

func timestampToEpochMicroseconds(ts *timestamppb.Timestamp) int64 {
	if ts == nil {
		return 0
	}
	return ts.GetSeconds()*1e6 + int64(ts.GetNanos()/1e3)
}

func ocSpanAttributesToJaegerTags(ocAttribs *tracepb.Span_Attributes) []*jaeger.Tag {
	if ocAttribs == nil {
		return nil
	}

	// Pre-allocate assuming that few attributes, if any at all, are nil.
	jTags := make([]*jaeger.Tag, 0, len(ocAttribs.AttributeMap))
	for key, attrib := range ocAttribs.AttributeMap {
		if attrib == nil || attrib.Value == nil {
			continue
		}

		jTag := &jaeger.Tag{Key: key}
		switch attribValue := attrib.Value.(type) {
		case *tracepb.AttributeValue_StringValue:
			// Jaeger-to-OC maps binary tags to string attributes and encodes them as
			// base64 strings. Blindingly attempting to decode base64 seems too much.
			str := truncableStringToStr(attribValue.StringValue)
			jTag.VStr = &str
			jTag.VType = jaeger.TagType_STRING
		case *tracepb.AttributeValue_IntValue:
			i := attribValue.IntValue
			jTag.VLong = &i
			jTag.VType = jaeger.TagType_LONG
		case *tracepb.AttributeValue_BoolValue:
			b := attribValue.BoolValue
			jTag.VBool = &b
			jTag.VType = jaeger.TagType_BOOL
		case *tracepb.AttributeValue_DoubleValue:
			d := attribValue.DoubleValue
			jTag.VDouble = &d
			jTag.VType = jaeger.TagType_DOUBLE
		default:
			str := "<Unknown OpenCensus Attribute for key \"" + key + "\">"
			jTag.VStr = &str
			jTag.VType = jaeger.TagType_STRING
		}
		jTags = append(jTags, jTag)
	}

	return jTags
}

func traceIDToInt64(traceID []byte) (int64, int64, error) {
	if len(traceID) != 16 {
		return 0, 0, errInvalidTraceID
	}
	tid := [16]byte{}
	copy(tid[:], traceID)
	hi, lo := tracetranslator.TraceIDToUInt64Pair(pdata.NewTraceID(tid))
	return int64(hi), int64(lo), nil
}

func spanIDToInt64(spanID []byte) (int64, error) {
	if len(spanID) != 8 {
		return 0, errInvalidSpanID
	}
	sid := [8]byte{}
	copy(sid[:], spanID)
	return int64(tracetranslator.SpanIDToUInt64(pdata.NewSpanID(sid))), nil
}

// ocAttributeKeyExist returns true if a key in attribute of an OC Span exists.
// It returns false, if attributes is nil, the map itself is nil or the key wasn't found.
func ocAttributeKeyExist(ocAttributes *tracepb.Span_Attributes, key string) bool {
	if ocAttributes == nil || ocAttributes.AttributeMap == nil {
		return false
	}
	_, foundKey := ocAttributes.AttributeMap[key]
	return foundKey
}
