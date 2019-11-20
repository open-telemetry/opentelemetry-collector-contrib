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
	filtered, _ := makeHttp(span)

	isError, isFault, filtered, cause := makeCause(span.Status, filtered)

	assert.False(t, isError)
	assert.True(t, isFault)
	assert.NotNil(t, filtered)
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
	filtered, _ := makeHttp(span)

	isError, isFault, filtered, cause := makeCause(span.Status, filtered)

	assert.False(t, isError)
	assert.True(t, isFault)
	assert.NotNil(t, filtered)
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
	span := constructExceptionServerSpan(attributes)
	filtered, _ := makeHttp(span)

	isError, isFault, filtered, cause := makeCause(span.Status, filtered)

	assert.False(t, isError)
	assert.True(t, isFault)
	assert.NotNil(t, filtered)
	assert.NotNil(t, cause)
	w := borrow()
	if err := w.Encode(cause); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, errorMsg))
}

func constructExceptionServerSpan(attributes map[string]interface{}) *tracepb.Span {
	endTime := time.Now().Round(time.Second)
	startTime := endTime.Add(-90 * time.Second)
	spanAttributes := constructSpanAttributes(attributes)

	return &tracepb.Span{
		TraceId:      NewTraceID(),
		SpanId:       NewSegmentID(),
		ParentSpanId: NewSegmentID(),
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
			Labels: constructDefaultResourceLabels(),
		},
	}
}
