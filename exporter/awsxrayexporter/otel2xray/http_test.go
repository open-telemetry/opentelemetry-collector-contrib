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

package otel2xray

import (
	"fmt"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/stretchr/testify/assert"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestClientSpanWithUrlAttribute(t *testing.T) {
	attributes := make(map[string]interface{})
	attributes[ComponentAttribute] = HttpComponentType
	attributes[MethodAttribute] = "GET"
	attributes[URLAttribute] = "https://api.example.com/users/junit"
	attributes[StatusCodeAttribute] = 200
	span := constructHttpClientSpan(attributes)

	filtered, httpData := makeHttp(span.Kind, span.Status.Code, span.Attributes.AttributeMap)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := borrow()
	if err := w.Encode(httpData); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, "https://api.example.com/users/junit"))
}

func TestClientSpanWithSchemeHostTargetAttributes(t *testing.T) {
	attributes := make(map[string]interface{})
	attributes[ComponentAttribute] = HttpComponentType
	attributes[MethodAttribute] = "GET"
	attributes[SchemeAttribute] = "https"
	attributes[HostAttribute] = "api.example.com"
	attributes[TargetAttribute] = "/users/junit"
	attributes[StatusCodeAttribute] = 200
	attributes["user.id"] = "junit"
	span := constructHttpClientSpan(attributes)

	filtered, httpData := makeHttp(span.Kind, span.Status.Code, span.Attributes.AttributeMap)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := borrow()
	if err := w.Encode(httpData); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, "https://api.example.com/users/junit"))
}

func TestClientSpanWithPeerAttributes(t *testing.T) {
	attributes := make(map[string]interface{})
	attributes[ComponentAttribute] = HttpComponentType
	attributes[MethodAttribute] = "GET"
	attributes[SchemeAttribute] = "http"
	attributes[PeerHostAttribute] = "kb234.example.com"
	attributes[PeerPortAttribute] = 8080
	attributes[PeerIpv4Attribute] = "10.8.17.36"
	attributes[TargetAttribute] = "/users/junit"
	attributes[StatusCodeAttribute] = 200
	span := constructHttpClientSpan(attributes)

	filtered, httpData := makeHttp(span.Kind, span.Status.Code, span.Attributes.AttributeMap)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := borrow()
	if err := w.Encode(httpData); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, "http://kb234.example.com:8080/users/junit"))
}

func TestClientSpanWithPeerIp4Attributes(t *testing.T) {
	attributes := make(map[string]interface{})
	attributes[ComponentAttribute] = HttpComponentType
	attributes[MethodAttribute] = "GET"
	attributes[SchemeAttribute] = "http"
	attributes[PeerIpv4Attribute] = "10.8.17.36"
	attributes[PeerPortAttribute] = "8080"
	attributes[TargetAttribute] = "/users/junit"
	span := constructHttpClientSpan(attributes)

	filtered, httpData := makeHttp(span.Kind, span.Status.Code, span.Attributes.AttributeMap)
	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := borrow()
	if err := w.Encode(httpData); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, "http://10.8.17.36:8080/users/junit"))
}

func TestClientSpanWithPeerIp6Attributes(t *testing.T) {
	attributes := make(map[string]interface{})
	attributes[ComponentAttribute] = HttpComponentType
	attributes[MethodAttribute] = "GET"
	attributes[SchemeAttribute] = "https"
	attributes[PeerIpv6Attribute] = "2001:db8:85a3::8a2e:370:7334"
	attributes[PeerPortAttribute] = "443"
	attributes[TargetAttribute] = "/users/junit"
	span := constructHttpClientSpan(attributes)

	filtered, httpData := makeHttp(span.Kind, span.Status.Code, span.Attributes.AttributeMap)
	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := borrow()
	if err := w.Encode(httpData); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, "https://2001:db8:85a3::8a2e:370:7334/users/junit"))
}

func TestServerSpanWithUrlAttribute(t *testing.T) {
	attributes := make(map[string]interface{})
	attributes[ComponentAttribute] = HttpComponentType
	attributes[MethodAttribute] = "GET"
	attributes[URLAttribute] = "https://api.example.com/users/junit"
	attributes[UserAgentAttribute] = "PostmanRuntime/7.16.3"
	attributes[ClientIpAttribute] = "192.168.15.32"
	attributes[StatusCodeAttribute] = 200
	span := constructHttpServerSpan(attributes)

	filtered, httpData := makeHttp(span.Kind, span.Status.Code, span.Attributes.AttributeMap)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := borrow()
	if err := w.Encode(httpData); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, "https://api.example.com/users/junit"))
}

func TestServerSpanWithSchemeHostTargetAttributes(t *testing.T) {
	attributes := make(map[string]interface{})
	attributes[ComponentAttribute] = HttpComponentType
	attributes[MethodAttribute] = "GET"
	attributes[SchemeAttribute] = "https"
	attributes[HostAttribute] = "api.example.com"
	attributes[TargetAttribute] = "/users/junit"
	attributes[UserAgentAttribute] = "PostmanRuntime/7.16.3"
	attributes[ClientIpAttribute] = "192.168.15.32"
	attributes[StatusCodeAttribute] = 200
	span := constructHttpServerSpan(attributes)

	filtered, httpData := makeHttp(span.Kind, span.Status.Code, span.Attributes.AttributeMap)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := borrow()
	if err := w.Encode(httpData); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, "https://api.example.com/users/junit"))
}

func TestServerSpanWithSchemeServernamePortTargetAttributes(t *testing.T) {
	attributes := make(map[string]interface{})
	attributes[ComponentAttribute] = HttpComponentType
	attributes[MethodAttribute] = "GET"
	attributes[SchemeAttribute] = "https"
	attributes[ServerNameAttribute] = "api.example.com"
	attributes[PortAttribute] = 443
	attributes[TargetAttribute] = "/users/junit"
	attributes[UserAgentAttribute] = "PostmanRuntime/7.16.3"
	attributes[ClientIpAttribute] = "192.168.15.32"
	attributes[StatusCodeAttribute] = 200
	span := constructHttpServerSpan(attributes)

	filtered, httpData := makeHttp(span.Kind, span.Status.Code, span.Attributes.AttributeMap)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := borrow()
	if err := w.Encode(httpData); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, "https://api.example.com/users/junit"))
}

func TestServerSpanWithSchemeNamePortTargetAttributes(t *testing.T) {
	attributes := make(map[string]interface{})
	attributes[ComponentAttribute] = HttpComponentType
	attributes[MethodAttribute] = "GET"
	attributes[SchemeAttribute] = "http"
	attributes[HostNameAttribute] = "kb234.example.com"
	attributes[PortAttribute] = 8080
	attributes[TargetAttribute] = "/users/junit"
	attributes[UserAgentAttribute] = "PostmanRuntime/7.16.3"
	attributes[ClientIpAttribute] = "192.168.15.32"
	attributes[StatusCodeAttribute] = 200
	attributes[ContentLenAttribute] = 21378
	span := constructHttpServerSpan(attributes)

	filtered, httpData := makeHttp(span.Kind, span.Status.Code, span.Attributes.AttributeMap)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := borrow()
	if err := w.Encode(httpData); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, "http://kb234.example.com:8080/users/junit"))
}

func TestHttpStatusFromSpanStatus(t *testing.T) {
	attributes := make(map[string]interface{})
	attributes[ComponentAttribute] = HttpComponentType
	attributes[MethodAttribute] = "GET"
	attributes[URLAttribute] = "https://api.example.com/users/junit"
	span := constructHttpClientSpan(attributes)

	filtered, httpData := makeHttp(span.Kind, span.Status.Code, span.Attributes.AttributeMap)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := borrow()
	if err := w.Encode(httpData); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, "200"))
}

func constructHttpClientSpan(attributes map[string]interface{}) *tracepb.Span {
	endTime := time.Now().Round(time.Second)
	startTime := endTime.Add(-90 * time.Second)
	spanAttributes := constructSpanAttributes(attributes)

	return &tracepb.Span{
		TraceId:      []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F},
		SpanId:       []byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8},
		ParentSpanId: []byte{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8},
		Name:         &tracepb.TruncatableString{Value: "/users/junit"},
		Kind:         tracepb.Span_CLIENT,
		StartTime:    convertTimeToTimestamp(startTime),
		EndTime:      convertTimeToTimestamp(endTime),
		Status: &tracepb.Status{
			Code:    0,
			Message: "OK",
		},
		SameProcessAsParentSpan: &wrappers.BoolValue{Value: false},
		Tracestate: &tracepb.Span_Tracestate{
			Entries: []*tracepb.Span_Tracestate_Entry{
				{Key: "foo", Value: "bar"},
				{Key: "a", Value: "b"},
			},
		},
		Attributes: &tracepb.Span_Attributes{
			AttributeMap: spanAttributes,
		},
		Resource: &resourcepb.Resource{
			Type:   "container",
			Labels: constructResourceLabels(),
		},
	}
}

func constructHttpServerSpan(attributes map[string]interface{}) *tracepb.Span {
	endTime := time.Now().Round(time.Second)
	startTime := endTime.Add(-90 * time.Second)
	spanAttributes := constructSpanAttributes(attributes)

	return &tracepb.Span{
		TraceId:      []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F},
		SpanId:       []byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8},
		ParentSpanId: []byte{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8},
		Name:         &tracepb.TruncatableString{Value: "/users/junit"},
		Kind:         tracepb.Span_SERVER,
		StartTime:    convertTimeToTimestamp(startTime),
		EndTime:      convertTimeToTimestamp(endTime),
		Status: &tracepb.Status{
			Code:    0,
			Message: "OK",
		},
		SameProcessAsParentSpan: &wrappers.BoolValue{Value: false},
		Tracestate: &tracepb.Span_Tracestate{
			Entries: []*tracepb.Span_Tracestate_Entry{
				{Key: "foo", Value: "bar"},
				{Key: "a", Value: "b"},
			},
		},
		Attributes: &tracepb.Span_Attributes{
			AttributeMap: spanAttributes,
		},
		Resource: &resourcepb.Resource{
			Type:   "container",
			Labels: constructResourceLabels(),
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

func constructResourceLabels() map[string]string {
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
