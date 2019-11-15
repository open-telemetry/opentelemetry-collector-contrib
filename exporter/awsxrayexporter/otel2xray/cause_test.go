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
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

func TestCauseWithStatusMessage(t *testing.T) {
	errorMsg := "this is a test"
	attributes := make(map[string]interface{})
	attributes[MethodAttribute] = "POST"
	attributes[URLAttribute] = "https://api.example.com/widgets"
	attributes[StatusCodeAttribute] = 500
	span := constructExceptionServerSpan(attributes)
	span.Status.Message = errorMsg
	filtered, _ := makeHttp(span.Kind, span.Status.Code, span.Attributes.AttributeMap)

	isError, isFault, cause := makeCause(span.Status, filtered)

	assert.False(t, isError)
	assert.True(t, isFault)
	assert.NotNil(t, cause)
	w := borrow()
	if err := w.Encode(cause); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, errorMsg))
}

func TestCauseWithHttpStatusMessage(t *testing.T) {
	errorMsg := "this is a test"
	attributes := make(map[string]interface{})
	attributes[MethodAttribute] = "POST"
	attributes[URLAttribute] = "https://api.example.com/widgets"
	attributes[StatusCodeAttribute] = 500
	attributes[StatusTextAttribute] = errorMsg
	span := constructExceptionServerSpan(attributes)
	filtered, _ := makeHttp(span.Kind, span.Status.Code, span.Attributes.AttributeMap)

	isError, isFault, cause := makeCause(span.Status, filtered)

	assert.False(t, isError)
	assert.True(t, isFault)
	assert.NotNil(t, cause)
	w := borrow()
	if err := w.Encode(cause); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, errorMsg))
}

func TestCauseWithErrorMessage(t *testing.T) {
	errorMsg := "this is a test"
	attributes := make(map[string]interface{})
	attributes[MethodAttribute] = "POST"
	attributes[URLAttribute] = "https://api.example.com/widgets"
	attributes[StatusCodeAttribute] = 500
	attributes[ErrorMessageAttribute] = errorMsg
	attributes[ErrorStackAttribute] = "org.springframework.beans.factory.support.ConstructorResolver.createArgumentArray(ConstructorResolver.java:749)"
	span := constructExceptionServerSpan(attributes)
	filtered, _ := makeHttp(span.Kind, span.Status.Code, span.Attributes.AttributeMap)

	isError, isFault, cause := makeCause(span.Status, filtered)

	assert.False(t, isError)
	assert.True(t, isFault)
	assert.NotNil(t, cause)
	w := borrow()
	if err := w.Encode(cause); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, errorMsg))
	assert.True(t, strings.Contains(jsonStr, "ConstructorResolver"))
}

func constructExceptionServerSpan(attributes map[string]interface{}) *tracepb.Span {
	endTime := time.Now().Round(time.Second)
	startTime := endTime.Add(-90 * time.Second)
	spanAttributes := constructSpanAttributes(attributes)

	return &tracepb.Span{
		TraceId:      []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F},
		SpanId:       []byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8},
		ParentSpanId: []byte{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8},
		Name:         &tracepb.TruncatableString{Value: "/widgets"},
		Kind:         tracepb.Span_SERVER,
		StartTime:    convertTimeToTimestamp(startTime),
		EndTime:      convertTimeToTimestamp(endTime),
		Status: &tracepb.Status{
			Code: 13,
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
