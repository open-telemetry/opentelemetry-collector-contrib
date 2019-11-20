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
	tracetranslator "github.com/open-telemetry/opentelemetry-collector/translator/trace"
	"github.com/stretchr/testify/assert"
)

func TestClientSpanWithGrpcComponent(t *testing.T) {
	spanName := "platformapi.widgets.searchWidgets"
	attributes := make(map[string]interface{})
	attributes[ComponentAttribute] = GrpcComponentType
	attributes[MethodAttribute] = "GET"
	attributes[SchemeAttribute] = "ipv6"
	attributes[PeerIpv6Attribute] = "2607:f8b0:4000:80c::2004"
	attributes[PeerPortAttribute] = "9443"
	attributes[TargetAttribute] = spanName
	labels := constructDefaultResourceLabels()
	span := constructClientSpan(nil, spanName, 0, "OK", attributes, labels)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTime)
	span.TimeEvents = &timeEvents

	jsonStr, err := MakeSegmentDocumentString(spanName, span, "user.id")

	assert.NotNil(t, jsonStr)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(jsonStr, spanName))
}

func TestClientSpanWithAwsSdkClient(t *testing.T) {
	spanName := "AmazonDynamoDB.getItem"
	parentSpanID := NewSegmentID()
	userAttribute := "originating.user"
	user := "testing"
	attributes := make(map[string]interface{})
	attributes[ComponentAttribute] = HTTPComponentType
	attributes[MethodAttribute] = "POST"
	attributes[SchemeAttribute] = "https"
	attributes[HostAttribute] = "dynamodb.us-east-1.amazonaws.com"
	attributes[TargetAttribute] = "/"
	attributes[UserAgentAttribute] = "aws-sdk-java/1.11.613 Windows_10/10.0 OpenJDK_64-Bit_server_VM/11.0.4+11-LTS"
	attributes[AwsOperationAttribute] = "GetItem"
	attributes[AwsRequestIDAttribute] = "18BO1FEPJSSAOGNJEDPTPCMIU7VV4KQNSO5AEMVJF66Q9ASUAAJG"
	attributes[AwsTableNameAttribute] = "otel-dev-Testing"
	attributes[userAttribute] = user
	labels := constructDefaultResourceLabels()
	span := constructClientSpan(parentSpanID, spanName, 0, "OK", attributes, labels)

	jsonStr, err := MakeSegmentDocumentString(spanName, span, userAttribute)

	assert.NotNil(t, jsonStr)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(jsonStr, spanName))
	assert.True(t, strings.Contains(jsonStr, user))
	assert.False(t, strings.Contains(jsonStr, ComponentAttribute))
}

func TestServerSpanWithInternalServerError(t *testing.T) {
	spanName := "/api/locations"
	parentSpanID := NewSegmentID()
	userAttribute := "originating.user"
	user := "testing"
	errorMessage := "java.lang.NullPointerException"
	attributes := make(map[string]interface{})
	attributes[ComponentAttribute] = HTTPComponentType
	attributes[MethodAttribute] = "POST"
	attributes[URLAttribute] = "https://api.example.org/api/locations"
	attributes[TargetAttribute] = "/api/locations"
	attributes[StatusCodeAttribute] = 500
	attributes[userAttribute] = user
	attributes[ErrorKindAttribute] = "java.lang.NullPointerException"
	labels := constructDefaultResourceLabels()
	span := constructServerSpan(parentSpanID, spanName, tracetranslator.OCInternal, errorMessage, attributes, labels)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTime)
	span.TimeEvents = &timeEvents

	segment := MakeSegment(spanName, span, "originating.user")

	assert.NotNil(t, segment)
	assert.NotNil(t, segment.Cause)
	assert.Equal(t, spanName, segment.Name)
	assert.True(t, segment.Fault)
	w := borrow()
	if err := w.Encode(segment); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, spanName))
	assert.True(t, strings.Contains(jsonStr, errorMessage))
	assert.False(t, strings.Contains(jsonStr, userAttribute))
}

func TestClientSpanWithDbComponent(t *testing.T) {
	spanName := "call update_user_preference( ?, ?, ? )"
	enterpriseAppID := "25F2E73B-4769-4C79-9DF3-7EBE85D571EA"
	attributes := make(map[string]interface{})
	attributes[ComponentAttribute] = DbComponentType
	attributes[DbTypeAttribute] = "sql"
	attributes[DbInstanceAttribute] = "customers"
	attributes[DbStatementAttribute] = spanName
	attributes[DbUserAttribute] = "userprefsvc"
	attributes[PeerAddressAttribute] = "mysql://db.dev.example.com:3306"
	attributes[PeerHostAttribute] = "db.dev.example.com"
	attributes[PeerPortAttribute] = "3306"
	attributes["enterprise.app.id"] = enterpriseAppID
	labels := constructDefaultResourceLabels()
	span := constructClientSpan(nil, spanName, 0, "OK", attributes, labels)

	segment := MakeSegment(spanName, span, "originating.user")

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
	w := borrow()
	if err := w.Encode(segment); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, spanName))
	assert.True(t, strings.Contains(jsonStr, enterpriseAppID))
}

func TestClientSpanWithBlankUserAttribute(t *testing.T) {
	spanName := "call update_user_preference( ?, ?, ? )"
	enterpriseAppID := "25F2E73B-4769-4C79-9DF3-7EBE85D571EA"
	attributes := make(map[string]interface{})
	attributes[ComponentAttribute] = DbComponentType
	attributes[DbTypeAttribute] = "sql"
	attributes[DbInstanceAttribute] = "customers"
	attributes[DbStatementAttribute] = spanName
	attributes[DbUserAttribute] = "userprefsvc"
	attributes[PeerAddressAttribute] = "mysql://db.dev.example.com:3306"
	attributes[PeerHostAttribute] = "db.dev.example.com"
	attributes[PeerPortAttribute] = "3306"
	attributes["enterprise.app.id"] = enterpriseAppID
	labels := constructDefaultResourceLabels()
	span := constructClientSpan(nil, spanName, 0, "OK", attributes, labels)

	segment := MakeSegment(spanName, span, "")

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
	w := borrow()
	if err := w.Encode(segment); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, spanName))
	assert.True(t, strings.Contains(jsonStr, enterpriseAppID))
}

func TestSpanWithInvalidTraceId(t *testing.T) {
	spanName := "platformapi.widgets.searchWidgets"
	attributes := make(map[string]interface{})
	attributes[ComponentAttribute] = GrpcComponentType
	attributes[MethodAttribute] = "GET"
	attributes[SchemeAttribute] = "ipv6"
	attributes[PeerIpv6Attribute] = "2607:f8b0:4000:80c::2004"
	attributes[PeerPortAttribute] = "9443"
	attributes[TargetAttribute] = spanName
	labels := constructDefaultResourceLabels()
	span := constructClientSpan(nil, spanName, 0, "OK", attributes, labels)
	timeEvents := constructTimedEventsWithSentMessageEvent(span.StartTime)
	span.TimeEvents = &timeEvents
	span.TraceId[0] = 0x11

	jsonStr, err := MakeSegmentDocumentString(spanName, span, "user.id")

	assert.NotNil(t, jsonStr)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(jsonStr, spanName))
	assert.False(t, strings.Contains(jsonStr, "1-11"))
}

func constructClientSpan(parentSpanID []byte, name string, code int32, message string,
	attributes map[string]interface{}, rscLabels map[string]string) *tracepb.Span {
	var (
		traceID        = NewTraceID()
		spanID         = NewSegmentID()
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
		traceID        = NewTraceID()
		spanID         = NewSegmentID()
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
	labels[ServiceNameAttribute] = "signup_aggregator"
	labels[ServiceVersionAttribute] = "1.1.12"
	labels[ContainerNameAttribute] = "signup_aggregator"
	labels[ContainerImageAttribute] = "otel/signupaggregator"
	labels[ContainerTagAttribute] = "v1"
	labels[K8sClusterAttribute] = "production"
	labels[K8sNamespaceAttribute] = "default"
	labels[K8sDeploymentAttribute] = "signup_aggregator"
	labels[K8sPodAttribute] = "signup_aggregator-x82ufje83"
	labels[CloudProviderAttribute] = "aws"
	labels[CloudAccountAttribute] = "123456789"
	labels[CloudRegionAttribute] = "us-east-1"
	labels[CloudZoneAttribute] = "us-east-1c"
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
	eventAttrMap[MessageTypeAttribute] = &tracepb.AttributeValue{Value: &tracepb.AttributeValue_StringValue{
		StringValue: &tracepb.TruncatableString{Value: "RECEIVED"},
	}}
	eventAttrMap[MessageIDAttribute] = &tracepb.AttributeValue{Value: &tracepb.AttributeValue_IntValue{
		IntValue: 1,
	}}
	eventAttrMap[MessageCompressedSizeAttribute] = &tracepb.AttributeValue{Value: &tracepb.AttributeValue_IntValue{
		IntValue: 6478,
	}}
	eventAttrMap[MessageUncompressedSizeAttribute] = &tracepb.AttributeValue{Value: &tracepb.AttributeValue_IntValue{
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
	events := make([]*tracepb.Span_TimeEvent, 1, 1)
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
	eventAttrMap[MessageTypeAttribute] = &tracepb.AttributeValue{Value: &tracepb.AttributeValue_StringValue{
		StringValue: &tracepb.TruncatableString{Value: "SENT"},
	}}
	eventAttrMap[MessageIDAttribute] = &tracepb.AttributeValue{Value: &tracepb.AttributeValue_IntValue{
		IntValue: 1,
	}}
	eventAttrMap[MessageUncompressedSizeAttribute] = &tracepb.AttributeValue{Value: &tracepb.AttributeValue_IntValue{
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
	events := make([]*tracepb.Span_TimeEvent, 1, 1)
	events[0] = &event
	timeEvents := tracepb.Span_TimeEvents{
		TimeEvent:                 events,
		DroppedAnnotationsCount:   0,
		DroppedMessageEventsCount: 0,
	}
	return timeEvents
}
