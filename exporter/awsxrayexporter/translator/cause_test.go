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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
	semconventions "go.opentelemetry.io/collector/translator/conventions"
)

func TestCauseWithStatusMessage(t *testing.T) {
	errorMsg := "this is a test"
	attributes := make(map[string]interface{})
	attributes[semconventions.AttributeHTTPMethod] = "POST"
	attributes[semconventions.AttributeHTTPURL] = "https://api.example.com/widgets"
	attributes[semconventions.AttributeHTTPStatusCode] = 500
	span := constructExceptionServerSpan(attributes)
	span.Status().SetMessage(errorMsg)
	filtered, _ := makeHTTP(span)

	isError, isFault, filtered, cause := makeCause(span.Status(), filtered)

	assert.False(t, isError)
	assert.True(t, isFault)
	assert.NotNil(t, filtered)
	assert.NotNil(t, cause)
	w := testWriters.borrow()
	if err := w.Encode(cause); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	testWriters.release(w)
	assert.True(t, strings.Contains(jsonStr, errorMsg))
}

func TestCauseWithHttpStatusMessage(t *testing.T) {
	errorMsg := "this is a test"
	attributes := make(map[string]interface{})
	attributes[semconventions.AttributeHTTPMethod] = "POST"
	attributes[semconventions.AttributeHTTPURL] = "https://api.example.com/widgets"
	attributes[semconventions.AttributeHTTPStatusCode] = 500
	attributes[semconventions.AttributeHTTPStatusText] = errorMsg
	span := constructExceptionServerSpan(attributes)
	filtered, _ := makeHTTP(span)

	isError, isFault, filtered, cause := makeCause(span.Status(), filtered)

	assert.False(t, isError)
	assert.True(t, isFault)
	assert.NotNil(t, filtered)
	assert.NotNil(t, cause)
	w := testWriters.borrow()
	if err := w.Encode(cause); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	testWriters.release(w)
	assert.True(t, strings.Contains(jsonStr, errorMsg))
}

func constructExceptionServerSpan(attributes map[string]interface{}) pdata.Span {
	endTime := time.Now().Round(time.Second)
	startTime := endTime.Add(-90 * time.Second)
	spanAttributes := constructSpanAttributes(attributes)

	span := pdata.NewSpan()
	span.InitEmpty()
	span.SetTraceID(newTraceID())
	span.SetSpanID(newSegmentID())
	span.SetParentSpanID(newSegmentID())
	span.SetName("/widgets")
	span.SetKind(pdata.SpanKindSERVER)
	span.SetStartTime(pdata.TimestampUnixNano(startTime.UnixNano()))
	span.SetEndTime(pdata.TimestampUnixNano(endTime.UnixNano()))

	status := pdata.NewSpanStatus()
	status.InitEmpty()
	status.SetCode(pdata.StatusCode(13))
	status.CopyTo(span.Status())

	spanAttributes.CopyTo(span.Attributes())
	return span
}
