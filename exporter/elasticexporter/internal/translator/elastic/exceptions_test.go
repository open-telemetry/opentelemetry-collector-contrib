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

package elastic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.elastic.co/apm/model"
	"go.elastic.co/apm/transport/transporttest"
	"go.elastic.co/fastjson"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter/internal/translator/elastic"
)

func TestEncodeSpanEventsNonExceptions(t *testing.T) {
	nonExceptionEvent := ptrace.NewSpanEvent()
	nonExceptionEvent.SetName("not_exception")

	incompleteExceptionEvent := ptrace.NewSpanEvent()
	incompleteExceptionEvent.SetName("exception")
	// At least one of exception.message and exception.type is required.
	incompleteExceptionEvent.Attributes().InsertString(conventions.AttributeExceptionStacktrace, "stacktrace")

	_, errors := encodeSpanEvents(t, "java", nonExceptionEvent, incompleteExceptionEvent)
	require.Empty(t, errors)
}

func TestEncodeSpanEventsJavaExceptions(t *testing.T) {
	timestamp := time.Unix(123, 0).UTC()

	exceptionEvent1 := ptrace.NewSpanEvent()
	exceptionEvent1.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	exceptionEvent1.SetName("exception")
	exceptionEvent1.Attributes().InsertString("exception.type", "java.net.ConnectException.OSError")
	exceptionEvent1.Attributes().InsertString("exception.message", "Division by zero")
	exceptionEvent1.Attributes().InsertBool("exception.escaped", true)
	exceptionEvent1.Attributes().InsertString("exception.stacktrace", `
	Exception in thread "main" java.lang.RuntimeException: Test exception
		at com.example.GenerateTrace.methodB(GenerateTrace.java:13)
		at com.example.GenerateTrace.methodA(GenerateTrace.java:9)
		at com.example.GenerateTrace.main(GenerateTrace.java:5)
		at com.foo.loader/foo@9.0/com.foo.Main.run(Main.java)
		at com.foo.loader//com.foo.bar.App.run(App.java:12)
		at java.base/java.lang.Thread.run(Unknown Source)
`[1:])
	exceptionEvent2 := ptrace.NewSpanEvent()
	exceptionEvent2.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	exceptionEvent2.SetName("exception")
	exceptionEvent2.Attributes().InsertString("exception.type", "HighLevelException")
	exceptionEvent2.Attributes().InsertString("exception.message", "MidLevelException: LowLevelException")
	exceptionEvent2.Attributes().InsertString("exception.stacktrace", `
	HighLevelException: MidLevelException: LowLevelException
		at Junk.a(Junk.java:13)
		at Junk.main(Junk.java:4)
	Caused by: MidLevelException: LowLevelException
		at Junk.c(Junk.java:23)
		at Junk.b(Junk.java:17)
		at Junk.a(Junk.java:11)
		... 1 more
		Suppressed: java.lang.ArithmeticException: / by zero
			at Junk.c(Junk.java:25)
			... 3 more
	Caused by: LowLevelException
		at Junk.e(Junk.java:37)
		at Junk.d(Junk.java:34)
		at Junk.c(Junk.java:21)
		... 3 more`[1:])

	transaction, errors := encodeSpanEvents(t, "java", exceptionEvent1, exceptionEvent2)
	assert.Equal(t, []model.Error{{
		TraceID:   transaction.TraceID,
		ParentID:  transaction.ID,
		Timestamp: model.Time(timestamp),
		Exception: model.Exception{
			Type:    "java.net.ConnectException.OSError",
			Message: "Division by zero",
			Handled: false,
			Stacktrace: []model.StacktraceFrame{{
				Classname: "com.example.GenerateTrace",
				Function:  "methodB",
				File:      "GenerateTrace.java",
				Line:      13,
			}, {
				Classname: "com.example.GenerateTrace",
				Function:  "methodA",
				File:      "GenerateTrace.java",
				Line:      9,
			}, {
				Classname: "com.example.GenerateTrace",
				Function:  "main",
				File:      "GenerateTrace.java",
				Line:      5,
			}, {
				Module:    "foo@9.0",
				Classname: "com.foo.Main",
				Function:  "run",
				File:      "Main.java",
			}, {
				Classname: "com.foo.bar.App",
				Function:  "run",
				File:      "App.java",
				Line:      12,
			}, {
				Module:    "java.base",
				Classname: "java.lang.Thread",
				Function:  "run",
				File:      "Unknown Source",
			}},
		},
	}, {
		TraceID:   transaction.TraceID,
		ParentID:  transaction.ID,
		Timestamp: model.Time(timestamp),
		Exception: model.Exception{
			Type:    "HighLevelException",
			Message: "MidLevelException: LowLevelException",
			Handled: true,
			Stacktrace: []model.StacktraceFrame{{
				Classname: "Junk",
				Function:  "a",
				File:      "Junk.java",
				Line:      13,
			}, {
				Classname: "Junk",
				Function:  "main",
				File:      "Junk.java",
				Line:      4,
			}},
			Cause: []model.Exception{{
				Message: "MidLevelException: LowLevelException",
				Handled: true,
				Stacktrace: []model.StacktraceFrame{{
					Classname: "Junk",
					Function:  "c",
					File:      "Junk.java",
					Line:      23,
				}, {
					Classname: "Junk",
					Function:  "b",
					File:      "Junk.java",
					Line:      17,
				}, {
					Classname: "Junk",
					Function:  "a",
					File:      "Junk.java",
					Line:      11,
				}, {
					Classname: "Junk",
					Function:  "main",
					File:      "Junk.java",
					Line:      4,
				}},
				Cause: []model.Exception{{
					Message: "LowLevelException",
					Handled: true,
					Stacktrace: []model.StacktraceFrame{{
						Classname: "Junk",
						Function:  "e",
						File:      "Junk.java",
						Line:      37,
					}, {
						Classname: "Junk",
						Function:  "d",
						File:      "Junk.java",
						Line:      34,
					}, {
						Classname: "Junk",
						Function:  "c",
						File:      "Junk.java",
						Line:      21,
					}, {
						Classname: "Junk",
						Function:  "b",
						File:      "Junk.java",
						Line:      17,
					}, {
						Classname: "Junk",
						Function:  "a",
						File:      "Junk.java",
						Line:      11,
					}, {
						Classname: "Junk",
						Function:  "main",
						File:      "Junk.java",
						Line:      4,
					}},
				}},
			}},
		},
	}}, errors)
}

func TestEncodeSpanEventsJavaExceptionsUnparsedStacktrace(t *testing.T) {
	stacktraces := []string{
		// Unexpected prefix.
		"abc\ndef",

		// "... N more" with no preceding exception.
		"abc\n... 1 more",

		// "... N more" where N is greater than the number of stack
		// frames in the enclosing exception.
		`ignored message
	at Class.method(Class.java:1)
Caused by: something else
	at Class.method(Class.java:2)
	... 2 more`,

		// "... N more" where N is not a sequence of digits.
		`abc
	at Class.method(Class.java:1)
Caused by: whatever
	at Class.method(Class.java:2)
	... lots more`,

		// "at <location>" where <location> is invalid.
		`abc
	at the movies`,
	}

	var events []ptrace.SpanEvent
	for _, stacktrace := range stacktraces {
		event := ptrace.NewSpanEvent()
		event.SetName("exception")
		event.Attributes().InsertString("exception.type", "ExceptionType")
		event.Attributes().InsertString("exception.stacktrace", stacktrace)
		events = append(events, event)
	}

	_, errors := encodeSpanEvents(t, "java", events...)
	require.Len(t, errors, len(stacktraces))

	for i, e := range errors {
		assert.Empty(t, e.Exception.Stacktrace)
		assert.Equal(t, map[string]interface{}{"stacktrace": stacktraces[i]}, e.Exception.Attributes)
	}
}

func TestEncodeSpanEventsNonJavaExceptions(t *testing.T) {
	timestamp := time.Unix(123, 0).UTC()

	exceptionEvent := ptrace.NewSpanEvent()
	exceptionEvent.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	exceptionEvent.SetName("exception")
	exceptionEvent.Attributes().InsertString("exception.type", "the_type")
	exceptionEvent.Attributes().InsertString("exception.message", "the_message")
	exceptionEvent.Attributes().InsertString("exception.stacktrace", "the_stacktrace")

	// For languages where we do not explicitly parse the stacktrace,
	// the raw stacktrace is stored as an attribute on the exception.
	transaction, errors := encodeSpanEvents(t, "COBOL", exceptionEvent)
	require.Len(t, errors, 1)

	assert.Equal(t, model.Error{
		TraceID:   transaction.TraceID,
		ParentID:  transaction.ID,
		Timestamp: model.Time(timestamp),
		Exception: model.Exception{
			Type:    "the_type",
			Message: "the_message",
			Handled: true,
			Attributes: map[string]interface{}{
				"stacktrace": "the_stacktrace",
			},
		},
	}, errors[0])
}

func encodeSpanEvents(t *testing.T, language string, events ...ptrace.SpanEvent) (model.Transaction, []model.Error) {
	traceID := model.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	transactionID := model.SpanID{1, 1, 1, 1, 1, 1, 1, 1}

	span := ptrace.NewSpan()
	span.SetTraceID(pcommon.NewTraceID(traceID))
	span.SetSpanID(pcommon.NewSpanID(transactionID))
	for _, event := range events {
		tgt := span.Events().AppendEmpty()
		event.CopyTo(tgt)
	}

	var w fastjson.Writer
	var recorder transporttest.RecorderTransport
	resource := pcommon.NewResource()
	resource.Attributes().InsertString(conventions.AttributeTelemetrySDKLanguage, language)
	assert.NoError(t, elastic.EncodeResourceMetadata(resource, &w))
	assert.NoError(t, elastic.EncodeSpan(span, pcommon.NewInstrumentationScope(), resource, &w))
	sendStream(t, &w, &recorder)

	payloads := recorder.Payloads()
	require.Len(t, payloads.Transactions, 1)
	return payloads.Transactions[0], payloads.Errors
}
