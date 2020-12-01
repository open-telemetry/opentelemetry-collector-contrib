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

func TestCauseWithExceptions(t *testing.T) {
	errorMsg := "this is a test"
	attributeMap := make(map[string]interface{})
	attributeMap[semconventions.AttributeHTTPMethod] = "POST"
	attributeMap[semconventions.AttributeHTTPURL] = "https://api.example.com/widgets"
	attributeMap[semconventions.AttributeHTTPStatusCode] = 500

	event1 := pdata.NewSpanEvent()
	event1.SetName(semconventions.AttributeExceptionEventName)
	attributes := pdata.NewAttributeMap()
	attributes.InsertString(semconventions.AttributeExceptionType, "java.lang.IllegalStateException")
	attributes.InsertString(semconventions.AttributeExceptionMessage, "bad state")
	attributes.InsertString(semconventions.AttributeExceptionStacktrace, `java.lang.IllegalStateException: state is not legal
	at io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException(RecordEventsReadableSpanTest.java:626)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke0(Native Method)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62)
Caused by: java.lang.IllegalArgumentException: bad argument`)
	attributes.CopyTo(event1.Attributes())

	event2 := pdata.NewSpanEvent()
	event2.SetName(semconventions.AttributeExceptionEventName)
	attributes = pdata.NewAttributeMap()
	attributes.InsertString(semconventions.AttributeExceptionType, "EmptyError")
	attributes.CopyTo(event2.Attributes())

	span := constructExceptionServerSpan(attributeMap, pdata.StatusCodeError)
	span.Status().SetMessage(errorMsg)
	span.Events().Append(event1)
	span.Events().Append(event2)
	filtered, _ := makeHTTP(span)

	res := pdata.NewResource()
	res.Attributes().InsertString(semconventions.AttributeTelemetrySDKLanguage, "java")
	isError, isFault, filteredResult, cause := makeCause(span, filtered, res)

	assert.True(t, isError)
	assert.False(t, isFault)
	assert.Equal(t, filtered, filteredResult)
	assert.NotNil(t, cause)
	assert.Len(t, cause.Exceptions, 3)
	assert.NotEmpty(t, cause.Exceptions[0].ID)
	assert.Equal(t, "java.lang.IllegalStateException", *cause.Exceptions[0].Type)
	assert.Equal(t, "bad state", *cause.Exceptions[0].Message)
	assert.Len(t, cause.Exceptions[0].Stack, 3)
	assert.Equal(t, cause.Exceptions[1].ID, cause.Exceptions[0].Cause)
	assert.Equal(t, "java.lang.IllegalArgumentException", *cause.Exceptions[1].Type)
	assert.NotEmpty(t, cause.Exceptions[2].ID)
	assert.Equal(t, "EmptyError", *cause.Exceptions[2].Type)
	assert.Empty(t, cause.Exceptions[2].Message)
}

func TestCauseWithStatusMessage(t *testing.T) {
	errorMsg := "this is a test"
	attributes := make(map[string]interface{})
	attributes[semconventions.AttributeHTTPMethod] = "POST"
	attributes[semconventions.AttributeHTTPURL] = "https://api.example.com/widgets"
	attributes[semconventions.AttributeHTTPStatusCode] = 500
	span := constructExceptionServerSpan(attributes, pdata.StatusCodeError)
	span.Status().SetMessage(errorMsg)
	filtered, _ := makeHTTP(span)

	res := pdata.NewResource()
	isError, isFault, filtered, cause := makeCause(span, filtered, res)

	assert.True(t, isError)
	assert.False(t, isFault)
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
	span := constructExceptionServerSpan(attributes, pdata.StatusCodeError)
	filtered, _ := makeHTTP(span)

	res := pdata.NewResource()
	isError, isFault, filtered, cause := makeCause(span, filtered, res)

	assert.True(t, isError)
	assert.False(t, isFault)
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

func TestCauseWithZeroStatusMessage(t *testing.T) {
	errorMsg := "this is a test"
	attributes := make(map[string]interface{})
	attributes[semconventions.AttributeHTTPMethod] = "POST"
	attributes[semconventions.AttributeHTTPURL] = "https://api.example.com/widgets"
	attributes[semconventions.AttributeHTTPStatusCode] = 500
	attributes[semconventions.AttributeHTTPStatusText] = errorMsg

	span := constructExceptionServerSpan(attributes, pdata.StatusCodeUnset)
	filtered, _ := makeHTTP(span)
	// Status is used to determine whether an error or not.
	// This span illustrates incorrect instrumentation,
	// marking a success status with an error http status code, and status wins.
	// We do not expect to see such spans in practice.
	res := pdata.NewResource()
	isError, isFault, filtered, cause := makeCause(span, filtered, res)

	assert.False(t, isError)
	assert.False(t, isFault)
	assert.NotNil(t, filtered)
	assert.Nil(t, cause)
}

func TestCauseWithClientErrorMessage(t *testing.T) {
	errorMsg := "this is a test"
	attributes := make(map[string]interface{})
	attributes[semconventions.AttributeHTTPMethod] = "POST"
	attributes[semconventions.AttributeHTTPURL] = "https://api.example.com/widgets"
	attributes[semconventions.AttributeHTTPStatusCode] = 499
	attributes[semconventions.AttributeHTTPStatusText] = errorMsg

	span := constructExceptionServerSpan(attributes, pdata.StatusCodeError)
	filtered, _ := makeHTTP(span)

	res := pdata.NewResource()
	isError, isFault, filtered, cause := makeCause(span, filtered, res)

	assert.True(t, isError)
	assert.False(t, isFault)
	assert.NotNil(t, filtered)
	assert.NotNil(t, cause)
}

func constructExceptionServerSpan(attributes map[string]interface{}, statuscode pdata.StatusCode) pdata.Span {
	endTime := time.Now().Round(time.Second)
	startTime := endTime.Add(-90 * time.Second)
	spanAttributes := constructSpanAttributes(attributes)

	span := pdata.NewSpan()
	span.SetTraceID(newTraceID())
	span.SetSpanID(newSegmentID())
	span.SetParentSpanID(newSegmentID())
	span.SetName("/widgets")
	span.SetKind(pdata.SpanKindSERVER)
	span.SetStartTime(pdata.TimestampUnixNano(startTime.UnixNano()))
	span.SetEndTime(pdata.TimestampUnixNano(endTime.UnixNano()))

	status := pdata.NewSpanStatus()
	status.InitEmpty()
	status.SetCode(statuscode)
	status.CopyTo(span.Status())

	spanAttributes.CopyTo(span.Attributes())
	return span
}

func TestParseExceptionWithoutStacktrace(t *testing.T) {
	exceptionType := "com.foo.Exception"
	message := "Error happened"
	stacktrace := ""

	exceptions := parseException(exceptionType, message, stacktrace, "")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "com.foo.Exception", *exceptions[0].Type)
	assert.Equal(t, "Error happened", *exceptions[0].Message)
	assert.Nil(t, exceptions[0].Stack)
}

func TestParseExceptionWithoutMessage(t *testing.T) {
	exceptionType := "com.foo.Exception"
	message := ""
	stacktrace := ""

	exceptions := parseException(exceptionType, message, stacktrace, "")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "com.foo.Exception", *exceptions[0].Type)
	assert.Empty(t, exceptions[0].Message)
	assert.Nil(t, exceptions[0].Stack)
}

func TestParseExceptionWithJavaStacktraceNoCause(t *testing.T) {
	exceptionType := "com.foo.Exception"
	message := "Error happened"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `java.lang.IllegalStateException: state is not legal
	at io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException(RecordEventsReadableSpanTest.java:626)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke0(Native Method)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62)`

	exceptions := parseException(exceptionType, message, stacktrace, "java")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "com.foo.Exception", *exceptions[0].Type)
	assert.Equal(t, "Error happened", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 3)
	assert.Equal(t, "io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "RecordEventsReadableSpanTest.java", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 626, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke0", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "Native Method", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[1].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke", *exceptions[0].Stack[2].Label)
	assert.Equal(t, "NativeMethodAccessorImpl.java", *exceptions[0].Stack[2].Path)
	assert.Equal(t, 62, *exceptions[0].Stack[2].Line)
}

func TestParseExceptionWithStacktraceNotJava(t *testing.T) {
	exceptionType := "com.foo.Exception"
	message := "Error happened"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `java.lang.IllegalStateException: state is not legal
	at io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException(RecordEventsReadableSpanTest.java:626)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke0(Native Method)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62)`

	exceptions := parseException(exceptionType, message, stacktrace, "")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "com.foo.Exception", *exceptions[0].Type)
	assert.Equal(t, "Error happened", *exceptions[0].Message)
	assert.Empty(t, exceptions[0].Stack)
}

func TestParseExceptionWithJavaStacktraceAndCauseWithoutStacktrace(t *testing.T) {
	exceptionType := "com.foo.Exception"
	message := "Error happened"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `java.lang.IllegalStateException: state is not legal
	at io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException(RecordEventsReadableSpanTest.java:626)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke0(Native Method)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62)
Caused by: java.lang.IllegalArgumentException: bad argument`

	exceptions := parseException(exceptionType, message, stacktrace, "java")
	assert.Len(t, exceptions, 2)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "com.foo.Exception", *exceptions[0].Type)
	assert.Equal(t, "Error happened", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 3)
	assert.Equal(t, "io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "RecordEventsReadableSpanTest.java", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 626, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke0", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "Native Method", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[1].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke", *exceptions[0].Stack[2].Label)
	assert.Equal(t, "NativeMethodAccessorImpl.java", *exceptions[0].Stack[2].Path)
	assert.Equal(t, 62, *exceptions[0].Stack[2].Line)
	assert.NotNil(t, exceptions[1].ID)
	assert.Equal(t, exceptions[1].ID, exceptions[0].Cause)
	assert.Equal(t, "java.lang.IllegalArgumentException", *exceptions[1].Type)
	assert.Equal(t, "bad argument", *exceptions[1].Message)
	assert.Empty(t, exceptions[1].Stack)
}

func TestParseExceptionWithJavaStacktraceAndCauseWithoutMessageOrStacktrace(t *testing.T) {
	exceptionType := "com.foo.Exception"
	message := "Error happened"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `java.lang.IllegalStateException: state is not legal
	at io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException(RecordEventsReadableSpanTest.java:626)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke0(Native Method)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62)
Caused by: java.lang.IllegalArgumentException`

	exceptions := parseException(exceptionType, message, stacktrace, "java")
	assert.Len(t, exceptions, 2)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "com.foo.Exception", *exceptions[0].Type)
	assert.Equal(t, "Error happened", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 3)
	assert.Equal(t, "io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "RecordEventsReadableSpanTest.java", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 626, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke0", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "Native Method", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[1].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke", *exceptions[0].Stack[2].Label)
	assert.Equal(t, "NativeMethodAccessorImpl.java", *exceptions[0].Stack[2].Path)
	assert.Equal(t, 62, *exceptions[0].Stack[2].Line)
	assert.NotNil(t, exceptions[1].ID)
	assert.Equal(t, exceptions[1].ID, exceptions[0].Cause)
	assert.Equal(t, "java.lang.IllegalArgumentException", *exceptions[1].Type)
	assert.Empty(t, *exceptions[1].Message)
	assert.Empty(t, exceptions[1].Stack)
}

func TestParseExceptionWithJavaStacktraceAndCauseWithStacktrace(t *testing.T) {
	exceptionType := "com.foo.Exception"
	message := "Error happened"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `java.lang.IllegalStateException: state is not legal
	at io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException(RecordEventsReadableSpanTest.java:626)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke0(Native Method)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62)
Caused by: java.lang.IllegalArgumentException: bad argument
	at org.junit.platform.engine.support.hierarchical.ThrowableCollector.execute(ThrowableCollector.java:73)
	at org.junit.platform.engine.support.hierarchical.NodeTestTask.executeRecursively(NodeTestTask.java)`

	exceptions := parseException(exceptionType, message, stacktrace, "java")
	assert.Len(t, exceptions, 2)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "com.foo.Exception", *exceptions[0].Type)
	assert.Equal(t, "Error happened", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 3)
	assert.Equal(t, "io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "RecordEventsReadableSpanTest.java", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 626, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke0", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "Native Method", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[1].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke", *exceptions[0].Stack[2].Label)
	assert.Equal(t, "NativeMethodAccessorImpl.java", *exceptions[0].Stack[2].Path)
	assert.Equal(t, 62, *exceptions[0].Stack[2].Line)
	assert.NotNil(t, exceptions[1].ID)
	assert.Equal(t, exceptions[1].ID, exceptions[0].Cause)
	assert.Equal(t, "java.lang.IllegalArgumentException", *exceptions[1].Type)
	assert.Equal(t, "bad argument", *exceptions[1].Message)
	assert.Len(t, exceptions[1].Stack, 2)
	assert.Equal(t, "org.junit.platform.engine.support.hierarchical.ThrowableCollector.execute", *exceptions[1].Stack[0].Label)
	assert.Equal(t, "ThrowableCollector.java", *exceptions[1].Stack[0].Path)
	assert.Equal(t, 73, *exceptions[1].Stack[0].Line)
	assert.Equal(t, "org.junit.platform.engine.support.hierarchical.NodeTestTask.executeRecursively", *exceptions[1].Stack[1].Label)
	assert.Equal(t, "NodeTestTask.java", *exceptions[1].Stack[1].Path)
	assert.Equal(t, 0, *exceptions[1].Stack[1].Line)
}

func TestParseExceptionWithJavaStacktraceAndCauseWithStacktraceSkipCommonAndSuppressedAndMalformed(t *testing.T) {
	exceptionType := "com.foo.Exception"
	message := "Error happened"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `java.lang.IllegalStateException: state is not legal
	at io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException(RecordEventsReadableSpanTest.java:626)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke0(Native Method)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62)afaefaef
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62
	at java.base/java.util.ArrayList.forEach(ArrayList.java:)
	Suppressed: Resource$CloseFailException: Resource ID = 2
		at Resource.close(Resource.java:26)	
		at Foo3.main(Foo3.java:5)
	Suppressed: Resource$CloseFailException: Resource ID = 1
		at Resource.close(Resource.java:26)
		at Foo3.main(Foo3.java:5)
Caused by: java.lang.IllegalArgumentException: bad argument
	at org.junit.platform.engine.support.hierarchical.ThrowableCollector.execute(ThrowableCollector.java:73)
	at org.junit.platform.engine.support.hierarchical.NodeTestTask.executeRecursively(NodeTestTask.java)
	... 99 more`

	exceptions := parseException(exceptionType, message, stacktrace, "java")
	assert.Len(t, exceptions, 2)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "com.foo.Exception", *exceptions[0].Type)
	assert.Equal(t, "Error happened", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 4)
	assert.Equal(t, "io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "RecordEventsReadableSpanTest.java", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 626, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke0", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "Native Method", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[1].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke", *exceptions[0].Stack[2].Label)
	assert.Equal(t, "NativeMethodAccessorImpl.java", *exceptions[0].Stack[2].Path)
	assert.Equal(t, 62, *exceptions[0].Stack[2].Line)
	// This is technically malformed but close enough to our format we may as well accept it.
	assert.Equal(t, "java.util.ArrayList.forEach", *exceptions[0].Stack[3].Label)
	assert.Equal(t, "ArrayList.java", *exceptions[0].Stack[3].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[3].Line)
	assert.NotNil(t, exceptions[1].ID)
	assert.Equal(t, exceptions[1].ID, exceptions[0].Cause)
	assert.Equal(t, "java.lang.IllegalArgumentException", *exceptions[1].Type)
	assert.Equal(t, "bad argument", *exceptions[1].Message)
	assert.Len(t, exceptions[1].Stack, 2)
	assert.Equal(t, "org.junit.platform.engine.support.hierarchical.ThrowableCollector.execute", *exceptions[1].Stack[0].Label)
	assert.Equal(t, "ThrowableCollector.java", *exceptions[1].Stack[0].Path)
	assert.Equal(t, 73, *exceptions[1].Stack[0].Line)
	assert.Equal(t, "org.junit.platform.engine.support.hierarchical.NodeTestTask.executeRecursively", *exceptions[1].Stack[1].Label)
	assert.Equal(t, "NodeTestTask.java", *exceptions[1].Stack[1].Path)
	assert.Equal(t, 0, *exceptions[1].Stack[1].Line)
}

func TestParseExceptionWithPythonStacktraceNoCause(t *testing.T) {
	exceptionType := "TypeError"
	message := "must be str, not int"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `Traceback (most recent call last):
  File "main.py", line 14, in <module>
    greet_many(['Chad', 'Dan', 1])
  File "greetings.py", line 12, in greet_many
    print('hi, ' + person)
TypeError: must be str, not int`

	exceptions := parseException(exceptionType, message, stacktrace, "python")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "TypeError", *exceptions[0].Type)
	assert.Equal(t, "must be str, not int", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 2)
	assert.Equal(t, "greet_many", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "greetings.py", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 12, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "<module>", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "main.py", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 14, *exceptions[0].Stack[1].Line)
}

func TestParseExceptionWithPythonStacktraceAndCause(t *testing.T) {
	exceptionType := "TypeError"
	message := "must be str, not int"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `Traceback (most recent call last):
  File "bar.py", line 10, in greet_many
    greet(person)
  File "foo.py", line 5, in greet
    print(greeting + ', ' + who_to_greet(someone))
ValueError: bad value

During handling of the above exception, another exception occurred:

Traceback (most recent call last):
  File "main.py", line 14, in <module>
    greet_many(['Chad', 'Dan', 1])
  File "greetings.py", line 12, in greet_many
    print('hi, ' + person)
TypeError: must be str, not int`

	exceptions := parseException(exceptionType, message, stacktrace, "python")
	assert.Len(t, exceptions, 2)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "TypeError", *exceptions[0].Type)
	assert.Equal(t, "must be str, not int", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 2)
	assert.Equal(t, "greet_many", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "greetings.py", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 12, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "<module>", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "main.py", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 14, *exceptions[0].Stack[1].Line)

	assert.NotEmpty(t, exceptions[1].ID)
	assert.Equal(t, "ValueError", *exceptions[1].Type)
	assert.Equal(t, "bad value", *exceptions[1].Message)
	assert.Len(t, exceptions[1].Stack, 2)
	assert.Equal(t, "greet", *exceptions[1].Stack[0].Label)
	assert.Equal(t, "foo.py", *exceptions[1].Stack[0].Path)
	assert.Equal(t, 5, *exceptions[1].Stack[0].Line)
	assert.Equal(t, "greet_many", *exceptions[1].Stack[1].Label)
	assert.Equal(t, "bar.py", *exceptions[1].Stack[1].Path)
	assert.Equal(t, 10, *exceptions[1].Stack[1].Line)
}

func TestParseExceptionWithPythonStacktraceAndMultiLineCause(t *testing.T) {
	exceptionType := "TypeError"
	message := "must be str, not int"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `Traceback (most recent call last):
  File "bar.py", line 10, in greet_many
    greet(person)
  File "foo.py", line 5, in greet
    print(greeting + ', ' + who_to_greet(someone))
ValueError: bad value
with more on
new lines

During handling of the above exception, another exception occurred:

Traceback (most recent call last):
  File "main.py", line 14, in <module>
    greet_many(['Chad', 'Dan', 1])
  File "greetings.py", line 12, in greet_many
    print('hi, ' + person)
TypeError: must be str, not int`

	exceptions := parseException(exceptionType, message, stacktrace, "python")
	assert.Len(t, exceptions, 2)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "TypeError", *exceptions[0].Type)
	assert.Equal(t, "must be str, not int", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 2)
	assert.Equal(t, "greet_many", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "greetings.py", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 12, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "<module>", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "main.py", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 14, *exceptions[0].Stack[1].Line)

	assert.NotEmpty(t, exceptions[1].ID)
	assert.Equal(t, "ValueError", *exceptions[1].Type)
	assert.Equal(t, "bad value\nwith more on\nnew lines", *exceptions[1].Message)
	assert.Len(t, exceptions[1].Stack, 2)
	assert.Equal(t, "greet", *exceptions[1].Stack[0].Label)
	assert.Equal(t, "foo.py", *exceptions[1].Stack[0].Path)
	assert.Equal(t, 5, *exceptions[1].Stack[0].Line)
	assert.Equal(t, "greet_many", *exceptions[1].Stack[1].Label)
	assert.Equal(t, "bar.py", *exceptions[1].Stack[1].Path)
	assert.Equal(t, 10, *exceptions[1].Stack[1].Line)
}

func TestParseExceptionWithPythonStacktraceMalformedLines(t *testing.T) {
	exceptionType := "TypeError"
	message := "must be str, not int"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `Traceback (most recent call last):
  File "main.py", line 14 in <module>
    greet_many(['Chad', 'Dan', 1])
  File "main.py", lin 14, in <module>
    greet_many(['Chad', 'Dan', 1])
  File "main.py", line 14, fin <module>
    greet_many(['Chad', 'Dan', 1])
  File "greetings.py", line 12, in greet_many
    print('hi, ' + person)
TypeError: must be str, not int`

	exceptions := parseException(exceptionType, message, stacktrace, "python")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "TypeError", *exceptions[0].Type)
	assert.Equal(t, "must be str, not int", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 3)
	assert.Equal(t, "greet_many", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "greetings.py", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 12, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "main.py", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 14, *exceptions[0].Stack[1].Line)
	assert.Equal(t, "<module>", *exceptions[0].Stack[2].Label)
	assert.Equal(t, "main.py", *exceptions[0].Stack[2].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[2].Line)
}

func TestParseExceptionWithPythonStacktraceAndMalformedCause(t *testing.T) {
	exceptionType := "TypeError"
	message := "must be str, not int"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `Traceback (most recent call last):
ValueError: bad value

During handling of the above exception, another exception occurred:

Traceback (most recent call last):
  File "main.py", line 14, in <module>
    greet_many(['Chad', 'Dan', 1])
  File "greetings.py", line 12, in greet_many
    print('hi, ' + person)
TypeError: must be str, not int`

	exceptions := parseException(exceptionType, message, stacktrace, "python")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "TypeError", *exceptions[0].Type)
	assert.Equal(t, "must be str, not int", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 2)
	assert.Equal(t, "greet_many", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "greetings.py", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 12, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "<module>", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "main.py", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 14, *exceptions[0].Stack[1].Line)
}

func TestParseExceptionWithPythonStacktraceAndMalformedCauseMessage(t *testing.T) {
	exceptionType := "TypeError"
	message := "must be str, not int"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `Traceback (most recent call last):
  File "bar.py", line 10, in greet_many
    greet(person)
  File "foo.py", line 5, in greet
    print(greeting + ', ' + who_to_greet(someone))
ValueError bad value

During handling of the above exception, another exception occurred:

Traceback (most recent call last):
  File "main.py", line 14, in <module>
    greet_many(['Chad', 'Dan', 1])
  File "greetings.py", line 12, in greet_many
    print('hi, ' + person)
TypeError: must be str, not int`

	exceptions := parseException(exceptionType, message, stacktrace, "python")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "TypeError", *exceptions[0].Type)
	assert.Equal(t, "must be str, not int", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 2)
	assert.Equal(t, "greet_many", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "greetings.py", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 12, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "<module>", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "main.py", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 14, *exceptions[0].Stack[1].Line)
}
