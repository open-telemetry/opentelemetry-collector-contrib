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

package translator

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/assert"
	semconventions "go.opentelemetry.io/collector/translator/conventions"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
)

var (
	testWriters = newWriterPool(2048)
)

func TestClientSpanWithAwsSdkClient(t *testing.T) {
	spanName := "AmazonDynamoDB.getItem"
	parentSpanID := newSegmentID()
	user := "testingT"
	attributes := make(map[string]interface{})
	attributes[semconventions.AttributeComponent] = semconventions.ComponentTypeHTTP
	attributes[semconventions.AttributeHTTPMethod] = "POST"
	attributes[semconventions.AttributeHTTPScheme] = "https"
	attributes[semconventions.AttributeHTTPHost] = "dynamodb.us-east-1.amazonaws.com"
	attributes[semconventions.AttributeHTTPTarget] = "/"
	attributes[AWSServiceAttribute] = "DynamoDB"
	attributes[AWSOperationAttribute] = "GetItem"
	attributes[AWSRequestIDAttribute] = "18BO1FEPJSSAOGNJEDPTPCMIU7VV4KQNSO5AEMVJF66Q9ASUAAJG"
	attributes[AWSTableNameAttribute] = "otel-dev-Testing"
	labels := constructDefaultResourceLabels()
	span := constructClientSpan(parentSpanID, spanName, 0, "OK", attributes, labels)

	segment := MakeSegment(spanName, span)
	assert.Equal(t, "DynamoDB", segment.Name)
	assert.Equal(t, "aws", segment.Namespace)
	assert.Equal(t, "subsegment", segment.Type)

	jsonStr, err := MakeSegmentDocumentString(spanName, span)

	assert.NotNil(t, jsonStr)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(jsonStr, "DynamoDB"))
	assert.False(t, strings.Contains(jsonStr, user))
	assert.False(t, strings.Contains(jsonStr, semconventions.AttributeComponent))
}

func TestServerSpanWithInternalServerError(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	errorMessage := "java.lang.NullPointerException"
	userAgent := "PostmanRuntime/7.21.0"
	enduser := "go.tester@example.com"
	attributes := make(map[string]interface{})
	attributes[semconventions.AttributeComponent] = semconventions.ComponentTypeHTTP
	attributes[semconventions.AttributeHTTPMethod] = "POST"
	attributes[semconventions.AttributeHTTPURL] = "https://api.example.org/api/locations"
	attributes[semconventions.AttributeHTTPTarget] = "/api/locations"
	attributes[semconventions.AttributeHTTPStatusCode] = 500
	attributes[semconventions.AttributeHTTPStatusText] = "java.lang.NullPointerException"
	attributes[semconventions.AttributeHTTPUserAgent] = userAgent
	attributes[semconventions.AttributeEnduserID] = enduser
	labels := constructDefaultResourceLabels()
	span := constructServerSpan(parentSpanID, spanName, tracetranslator.OCInternal, errorMessage, attributes, labels)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTime)
	span.TimeEvents = &timeEvents

	segment := MakeSegment(spanName, span)

	assert.NotNil(t, segment)
	assert.NotNil(t, segment.Cause)
	assert.Equal(t, spanName, segment.Name)
	assert.True(t, segment.Fault)
	w := testWriters.borrow()
	if err := w.Encode(segment); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	testWriters.release(w)
	assert.True(t, strings.Contains(jsonStr, spanName))
	assert.True(t, strings.Contains(jsonStr, errorMessage))
	assert.True(t, strings.Contains(jsonStr, userAgent))
	assert.True(t, strings.Contains(jsonStr, enduser))
}

func TestServerSpanNoParentId(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	labels := constructDefaultResourceLabels()
	span := constructServerSpan(parentSpanID, spanName, 0, "OK", nil, labels)

	segment := MakeSegment(spanName, span)

	assert.Empty(t, segment.ParentID)
}

func TestSpanWithNoStatus(t *testing.T) {
	span := &tracepb.Span{
		TraceId:      newTraceID(),
		SpanId:       newSegmentID(),
		ParentSpanId: newSegmentID(),
		Name:         &tracepb.TruncatableString{Value: "nostatus"},
		Kind:         tracepb.Span_SERVER,
		StartTime:    convertTimeToTimestamp(time.Now()),
		EndTime:      convertTimeToTimestamp(time.Now().Add(10)),
	}
	segment := MakeSegment("nostatus", span)
	assert.NotNil(t, segment)
}

func TestClientSpanWithDbComponent(t *testing.T) {
	spanName := "call update_user_preference( ?, ?, ? )"
	parentSpanID := newSegmentID()
	enterpriseAppID := "25F2E73B-4769-4C79-9DF3-7EBE85D571EA"
	attributes := make(map[string]interface{})
	attributes[semconventions.AttributeDBType] = "sql"
	attributes[semconventions.AttributeDBInstance] = "customers"
	attributes[semconventions.AttributeDBStatement] = spanName
	attributes[semconventions.AttributeDBUser] = "userprefsvc"
	attributes[semconventions.AttributeDBURL] = "mysql://db.dev.example.com:3306"
	attributes[semconventions.AttributeNetPeerName] = "db.dev.example.com"
	attributes[semconventions.AttributeNetPeerPort] = "3306"
	attributes["enterprise.app.id"] = enterpriseAppID
	labels := constructDefaultResourceLabels()
	span := constructClientSpan(parentSpanID, spanName, 0, "OK", attributes, labels)

	segment := MakeSegment(spanName, span)

	assert.NotNil(t, segment)
	assert.NotNil(t, segment.SQL)
	assert.NotNil(t, segment.Service)
	assert.NotNil(t, segment.AWS)
	assert.NotNil(t, segment.Annotations)
	assert.Nil(t, segment.Cause)
	assert.Nil(t, segment.HTTP)
	assert.Equal(t, spanName, segment.Name)
	assert.False(t, segment.Fault)
	assert.False(t, segment.Error)
	assert.Equal(t, "remote", segment.Namespace)

	w := testWriters.borrow()
	if err := w.Encode(segment); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	testWriters.release(w)
	assert.True(t, strings.Contains(jsonStr, spanName))
	assert.True(t, strings.Contains(jsonStr, enterpriseAppID))
}

func TestSpanWithInvalidTraceId(t *testing.T) {
	spanName := "platformapi.widgets.searchWidgets"
	attributes := make(map[string]interface{})
	attributes[semconventions.AttributeComponent] = semconventions.ComponentTypeGRPC
	attributes[semconventions.AttributeHTTPMethod] = "GET"
	attributes[semconventions.AttributeHTTPScheme] = "ipv6"
	attributes[semconventions.AttributeNetPeerIP] = "2607:f8b0:4000:80c::2004"
	attributes[semconventions.AttributeNetPeerPort] = "9443"
	attributes[semconventions.AttributeHTTPTarget] = spanName
	labels := constructDefaultResourceLabels()
	span := constructClientSpan(nil, spanName, 0, "OK", attributes, labels)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTime)
	span.TimeEvents = &timeEvents
	span.TraceId[0] = 0x11

	jsonStr, err := MakeSegmentDocumentString(spanName, span)

	assert.NotNil(t, jsonStr)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(jsonStr, spanName))
	assert.False(t, strings.Contains(jsonStr, "1-11"))
}

func TestFixSegmentName(t *testing.T) {
	validName := "EP @ test_15.testing-d\u00F6main.org#GO"
	fixedName := fixSegmentName(validName)
	assert.Equal(t, validName, fixedName)
	invalidName := "<subDomain>.example.com"
	fixedName = fixSegmentName(invalidName)
	assert.Equal(t, "subDomain.example.com", fixedName)
	fullyInvalidName := "<>"
	fixedName = fixSegmentName(fullyInvalidName)
	assert.Equal(t, defaultSegmentName, fixedName)
}

func TestFixAnnotationKey(t *testing.T) {
	validKey := "Key_1"
	fixedKey := fixAnnotationKey(validKey)
	assert.Equal(t, validKey, fixedKey)
	invalidKey := "Key@1"
	fixedKey = fixAnnotationKey(invalidKey)
	assert.Equal(t, "Key_1", fixedKey)
}

func TestServerSpanWithNilAttributes(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := newSegmentID()
	attributes := make(map[string]interface{})
	labels := constructDefaultResourceLabels()
	span := constructServerSpan(parentSpanID, spanName, tracetranslator.OCInternal, "OK", attributes, labels)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTime)
	span.TimeEvents = &timeEvents
	span.Attributes = nil

	segment := MakeSegment(spanName, span)

	assert.NotNil(t, segment)
	assert.NotNil(t, segment.Cause)
	assert.Equal(t, spanName, segment.Name)
	assert.True(t, segment.Fault)
	w := testWriters.borrow()
	if err := w.Encode(segment); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	testWriters.release(w)
	assert.True(t, strings.Contains(jsonStr, spanName))
}

func constructClientSpan(parentSpanID []byte, name string, code int32, message string,
	attributes map[string]interface{}, rscLabels map[string]string) *tracepb.Span {
	var (
		traceID        = newTraceID()
		spanID         = newSegmentID()
		endTime        = time.Now()
		startTime      = endTime.Add(-215 * time.Millisecond)
		spanAttributes = constructSpanAttributes(attributes)
	)

	return &tracepb.Span{
		TraceId:      traceID,
		SpanId:       spanID,
		ParentSpanId: parentSpanID,
		Name:         &tracepb.TruncatableString{Value: name},
		Kind:         tracepb.Span_CLIENT,
		StartTime:    convertTimeToTimestamp(startTime),
		EndTime:      convertTimeToTimestamp(endTime),
		Status: &tracepb.Status{
			Code:    code,
			Message: message,
		},
		Attributes: &tracepb.Span_Attributes{
			AttributeMap: spanAttributes,
		},
		Resource: &resourcepb.Resource{
			Type:   "container",
			Labels: rscLabels,
		},
	}
}

func constructServerSpan(parentSpanID []byte, name string, code int32, message string,
	attributes map[string]interface{}, rscLabels map[string]string) *tracepb.Span {
	var (
		traceID        = newTraceID()
		spanID         = newSegmentID()
		endTime        = time.Now()
		startTime      = endTime.Add(-215 * time.Millisecond)
		spanAttributes = constructSpanAttributes(attributes)
	)

	return &tracepb.Span{
		TraceId:      traceID,
		SpanId:       spanID,
		ParentSpanId: parentSpanID,
		Name:         &tracepb.TruncatableString{Value: name},
		Kind:         tracepb.Span_SERVER,
		StartTime:    convertTimeToTimestamp(startTime),
		EndTime:      convertTimeToTimestamp(endTime),
		Status: &tracepb.Status{
			Code:    code,
			Message: message,
		},
		Attributes: &tracepb.Span_Attributes{
			AttributeMap: spanAttributes,
		},
		Resource: &resourcepb.Resource{
			Type:   "container",
			Labels: rscLabels,
		},
	}
}

func constructSpanAttributes(attributes map[string]interface{}) map[string]*tracepb.AttributeValue {
	attrs := make(map[string]*tracepb.AttributeValue)
	for key, value := range attributes {
		valType := reflect.TypeOf(value)
		var attrVal tracepb.AttributeValue
		if valType.Kind() == reflect.Int {
			attrVal = tracepb.AttributeValue{Value: &tracepb.AttributeValue_IntValue{
				IntValue: int64(value.(int)),
			}}
		} else if valType.Kind() == reflect.Int64 {
			attrVal = tracepb.AttributeValue{Value: &tracepb.AttributeValue_IntValue{
				IntValue: value.(int64),
			}}
		} else {
			attrVal = tracepb.AttributeValue{Value: &tracepb.AttributeValue_StringValue{
				StringValue: &tracepb.TruncatableString{Value: fmt.Sprintf("%v", value)},
			}}
		}
		attrs[key] = &attrVal
	}
	return attrs
}

func constructDefaultResourceLabels() map[string]string {
	labels := make(map[string]string)
	labels[semconventions.AttributeServiceName] = "signup_aggregator"
	labels[semconventions.AttributeServiceVersion] = "semver:1.1.4"
	labels[semconventions.AttributeContainerName] = "signup_aggregator"
	labels[semconventions.AttributeContainerImage] = "otel/signupaggregator"
	labels[semconventions.AttributeContainerTag] = "v1"
	labels[semconventions.AttributeK8sCluster] = "production"
	labels[semconventions.AttributeK8sNamespace] = "default"
	labels[semconventions.AttributeK8sDeployment] = "signup_aggregator"
	labels[semconventions.AttributeK8sPod] = "signup_aggregator-x82ufje83"
	labels[semconventions.AttributeCloudProvider] = "aws"
	labels[semconventions.AttributeCloudAccount] = "123456789"
	labels[semconventions.AttributeCloudRegion] = "us-east-1"
	labels[semconventions.AttributeCloudZone] = "us-east-1c"
	return labels
}

func convertTimeToTimestamp(t time.Time) *timestamp.Timestamp {
	if t.IsZero() {
		return nil
	}
	nanoTime := t.UnixNano()
	return &timestamp.Timestamp{
		Seconds: nanoTime / 1e9,
		Nanos:   int32(nanoTime % 1e9),
	}
}

func constructTimedEventsWithReceivedMessageEvent(tm *timestamp.Timestamp) tracepb.Span_TimeEvents {
	eventAttrMap := make(map[string]*tracepb.AttributeValue)
	eventAttrMap[semconventions.AttributeMessageType] =
		&tracepb.AttributeValue{Value: &tracepb.AttributeValue_StringValue{
			StringValue: &tracepb.TruncatableString{Value: "RECEIVED"},
		}}
	eventAttrMap[semconventions.AttributeMessageID] = &tracepb.AttributeValue{Value: &tracepb.AttributeValue_IntValue{
		IntValue: 1,
	}}
	eventAttrMap[semconventions.AttributeMessageCompressedSize] =
		&tracepb.AttributeValue{Value: &tracepb.AttributeValue_IntValue{
			IntValue: 6478,
		}}
	eventAttrMap[semconventions.AttributeMessageUncompressedSize] =
		&tracepb.AttributeValue{Value: &tracepb.AttributeValue_IntValue{
			IntValue: 12452,
		}}
	eventAttrbutes := tracepb.Span_Attributes{
		AttributeMap:           eventAttrMap,
		DroppedAttributesCount: 0,
	}
	annotation := tracepb.Span_TimeEvent_Annotation{
		Attributes: &eventAttrbutes,
	}
	event := tracepb.Span_TimeEvent{
		Time: tm,
		Value: &tracepb.Span_TimeEvent_Annotation_{
			Annotation: &annotation,
		},
	}
	events := make([]*tracepb.Span_TimeEvent, 1)
	events[0] = &event
	timeEvents := tracepb.Span_TimeEvents{
		TimeEvent:                 events,
		DroppedAnnotationsCount:   0,
		DroppedMessageEventsCount: 0,
	}
	return timeEvents
}

func constructTimedEventsWithSentMessageEvent(tm *timestamp.Timestamp) tracepb.Span_TimeEvents {
	eventAttrMap := make(map[string]*tracepb.AttributeValue)
	eventAttrMap[semconventions.AttributeMessageType] =
		&tracepb.AttributeValue{Value: &tracepb.AttributeValue_StringValue{
			StringValue: &tracepb.TruncatableString{Value: "SENT"},
		}}
	eventAttrMap[semconventions.AttributeMessageID] = &tracepb.AttributeValue{Value: &tracepb.AttributeValue_IntValue{
		IntValue: 1,
	}}
	eventAttrMap[semconventions.AttributeMessageUncompressedSize] =
		&tracepb.AttributeValue{Value: &tracepb.AttributeValue_IntValue{
			IntValue: 7480,
		}}
	eventAttrbutes := tracepb.Span_Attributes{
		AttributeMap:           eventAttrMap,
		DroppedAttributesCount: 0,
	}
	annotation := tracepb.Span_TimeEvent_Annotation{
		Attributes: &eventAttrbutes,
	}
	event := tracepb.Span_TimeEvent{
		Time: tm,
		Value: &tracepb.Span_TimeEvent_Annotation_{
			Annotation: &annotation,
		},
	}
	events := make([]*tracepb.Span_TimeEvent, 1)
	events[0] = &event
	timeEvents := tracepb.Span_TimeEvents{
		TimeEvent:                 events,
		DroppedAnnotationsCount:   0,
		DroppedMessageEventsCount: 0,
	}
	return timeEvents
}
