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
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter/internal/translator/elastic"
)

func TestEncodeSpanEventsNonExceptions(t *testing.T) {
	nonExceptionEvent := pdata.NewSpanEvent()
	nonExceptionEvent.SetName("not_exception")

	incompleteExceptionEvent := pdata.NewSpanEvent()
	incompleteExceptionEvent.SetName("exception")
	incompleteExceptionEvent.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		// At least one of exception.message and exception.type is required.
		conventions.AttributeExceptionStacktrace: pdata.NewAttributeValueString("stacktrace"),
	})

	_, errors := encodeSpanEvents(t, "java", nonExceptionEvent, incompleteExceptionEvent)
	require.Empty(t, errors)
}

func TestEncodeSpanEventsJavaExceptions(t *testing.T) {
	timestamp := time.Unix(123, 0).UTC()

	exceptionEvent1 := pdata.NewSpanEvent()
	exceptionEvent1.SetTimestamp(pdata.NewTimestampFromTime(timestamp))
	exceptionEvent1.SetName("exception")
	exceptionEvent1.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"exception.type":    pdata.NewAttributeValueString("java.net.ConnectException.OSError"),
		"exception.message": pdata.NewAttributeValueString("Division by zero"),
		"exception.escaped": pdata.NewAttributeValueBool(true),
		"exception.stacktrace": pdata.NewAttributeValueString(`
Exception in thread "main" java.lang.RuntimeException: Test exception
	at com.example.GenerateTrace.methodB(GenerateTrace.java:13)
	at com.example.GenerateTrace.methodA(GenerateTrace.java:9)
	at com.example.GenerateTrace.main(GenerateTrace.java:5)
	at com.foo.loader/foo@9.0/com.foo.Main.run(Main.java)
	at com.foo.loader//com.foo.bar.App.run(App.java:12)
	at java.base/java.lang.Thread.run(Unknown Source)
`[1:],
		),
	})
	exceptionEvent2 := pdata.NewSpanEvent()
	exceptionEvent2.SetTimestamp(pdata.NewTimestampFromTime(timestamp))
	exceptionEvent2.SetName("exception")
	exceptionEvent2.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"exception.type":    pdata.NewAttributeValueString("HighLevelException"),
		"exception.message": pdata.NewAttributeValueString("MidLevelException: LowLevelException"),
		"exception.stacktrace": pdata.NewAttributeValueString(`
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
	... 3 more`[1:],
		),
	})

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

	var events []pdata.SpanEvent
	for _, stacktrace := range stacktraces {
		event := pdata.NewSpanEvent()
		event.SetName("exception")
		event.Attributes().InitFromMap(map[string]pdata.AttributeValue{
			"exception.type":       pdata.NewAttributeValueString("ExceptionType"),
			"exception.stacktrace": pdata.NewAttributeValueString(stacktrace),
		})
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

	exceptionEvent := pdata.NewSpanEvent()
	exceptionEvent.SetTimestamp(pdata.NewTimestampFromTime(timestamp))
	exceptionEvent.SetName("exception")
	exceptionEvent.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"exception.type":       pdata.NewAttributeValueString("the_type"),
		"exception.message":    pdata.NewAttributeValueString("the_message"),
		"exception.stacktrace": pdata.NewAttributeValueString("the_stacktrace"),
	})

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

func encodeSpanEvents(t *testing.T, language string, events ...pdata.SpanEvent) (model.Transaction, []model.Error) {
	traceID := model.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	transactionID := model.SpanID{1, 1, 1, 1, 1, 1, 1, 1}

	span := pdata.NewSpan()
	span.SetTraceID(pdata.NewTraceID(traceID))
	span.SetSpanID(pdata.NewSpanID(transactionID))
	for _, event := range events {
		tgt := span.Events().AppendEmpty()
		event.CopyTo(tgt)
	}

	var w fastjson.Writer
	var recorder transporttest.RecorderTransport
	resource := pdata.NewResource()
	resource.Attributes().InsertString(conventions.AttributeTelemetrySDKLanguage, language)
	elastic.EncodeResourceMetadata(resource, &w)
	err := elastic.EncodeSpan(span, pdata.NewInstrumentationLibrary(), resource, &w)
	assert.NoError(t, err)
	sendStream(t, &w, &recorder)

	payloads := recorder.Payloads()
	require.Len(t, payloads.Transactions, 1)
	return payloads.Transactions[0], payloads.Errors
}
