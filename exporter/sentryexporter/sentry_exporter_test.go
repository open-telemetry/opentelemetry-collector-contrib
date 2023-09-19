// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sentryexporter

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

/*
	for trace d6c4f03650bd47699ec65c84352b6208:
	rootSpan1 <- childSpan1 <- childChildSpan1
	rootSpan1 <- childSpan2
	rootSpan2 <- root2childSpan
	orphanSpan
*/

var (
	rootSpan1 = &sentry.Span{
		TraceID:     TraceIDFromHex("d6c4f03650bd47699ec65c84352b6208"),
		SpanID:      SpanIDFromHex("1cc4b26ab9094ef0"),
		Description: "/api/users/{user_id}",
		Op:          "http.server",
		Tags: map[string]string{
			"organization":   "12345",
			"status_message": "HTTP OK",
			"span_kind":      "server",
		},
		StartTime: unixNanoToTime(5),
		EndTime:   unixNanoToTime(10),
		Status:    sentry.SpanStatusOK,
	}

	childSpan1 = &sentry.Span{
		TraceID:      TraceIDFromHex("d6c4f03650bd47699ec65c84352b6208"),
		SpanID:       SpanIDFromHex("93ba92db3fa24752"),
		ParentSpanID: SpanIDFromHex("1cc4b26ab9094ef0"),
		Description:  `SELECT * FROM user WHERE "user"."id" = {id}`,
		Op:           "db",
		Tags: map[string]string{
			"function_name":  "get_users",
			"status_message": "MYSQL OK",
			"span_kind":      "server",
		},
		StartTime: unixNanoToTime(5),
		EndTime:   unixNanoToTime(7),
		Status:    sentry.SpanStatusOK,
	}

	childChildSpan1 = &sentry.Span{
		TraceID:      TraceIDFromHex("d6c4f03650bd47699ec65c84352b6208"),
		SpanID:       SpanIDFromHex("1fa8913ec3814d34"),
		ParentSpanID: SpanIDFromHex("93ba92db3fa24752"),
		Description:  `DB locked`,
		Op:           "db",
		Tags: map[string]string{
			"db_status":      "oh no im locked rn",
			"status_message": "MYSQL OK",
			"span_kind":      "server",
		},
		StartTime: unixNanoToTime(6),
		EndTime:   unixNanoToTime(7),
		Status:    sentry.SpanStatusOK,
	}

	childSpan2 = &sentry.Span{
		TraceID:      TraceIDFromHex("d6c4f03650bd47699ec65c84352b6208"),
		SpanID:       SpanIDFromHex("34efcde268684bb0"),
		ParentSpanID: SpanIDFromHex("1cc4b26ab9094ef0"),
		Description:  "Serialize stuff",
		Op:           "",
		Tags: map[string]string{
			"span_kind": "server",
		},
		StartTime: unixNanoToTime(7),
		EndTime:   unixNanoToTime(10),
		Status:    sentry.SpanStatusOK,
	}

	orphanSpan1 = &sentry.Span{
		TraceID:      TraceIDFromHex("d6c4f03650bd47699ec65c84352b6208"),
		SpanID:       SpanIDFromHex("6241111811384fae"),
		ParentSpanID: SpanIDFromHex("1930bb5cc45c4003"),
		Description:  "A random span",
		Op:           "",
		StartTime:    unixNanoToTime(3),
		EndTime:      unixNanoToTime(6),
		Status:       sentry.SpanStatusOK,
	}

	rootSpan2 = &sentry.Span{
		TraceID:     TraceIDFromHex("d6c4f03650bd47699ec65c84352b6208"),
		SpanID:      SpanIDFromHex("4c7f56556ffe4e4a"),
		Description: "Navigating to fancy website",
		Op:          "pageload",
		Tags: map[string]string{
			"status_message": "HTTP OK",
			"span_kind":      "client",
		},
		StartTime: unixNanoToTime(0),
		EndTime:   unixNanoToTime(5),
		Status:    sentry.SpanStatusOK,
	}

	root2childSpan = &sentry.Span{
		TraceID:      TraceIDFromHex("d6c4f03650bd47699ec65c84352b6208"),
		SpanID:       SpanIDFromHex("7ff3c8daf8184fee"),
		ParentSpanID: SpanIDFromHex("4c7f56556ffe4e4a"),
		Description:  "<FancyReactComponent />",
		Op:           "react",
		Tags: map[string]string{
			"span_kind": "server",
		},
		StartTime: unixNanoToTime(4),
		EndTime:   unixNanoToTime(5),
		Status:    sentry.SpanStatusOK,
	}
)

func TraceIDFromHex(s string) sentry.TraceID {
	var id sentry.TraceID
	_, err := hex.Decode(id[:], []byte(s))
	if err != nil {
		panic(err)
	}
	return id
}

func SpanIDFromHex(s string) sentry.SpanID {
	var id sentry.SpanID
	_, err := hex.Decode(id[:], []byte(s))
	if err != nil {
		panic(err)
	}
	return id
}

func generateEmptyTransactionMap(spans ...*sentry.Span) map[sentry.SpanID]*sentry.Event {
	transactionMap := make(map[sentry.SpanID]*sentry.Event)
	environment := "development"
	for _, span := range spans {
		transactionMap[span.SpanID] = transactionFromSpan(span, environment)
	}
	return transactionMap
}

func generateOrphanSpansFromSpans(spans ...*sentry.Span) []*sentry.Span {
	orphanSpans := make([]*sentry.Span, 0, len(spans))
	orphanSpans = append(orphanSpans, spans...)

	return orphanSpans
}

type SpanEventToSentryEventCases struct {
	testName            string
	errorMessage        string
	errorType           string
	sampleSentrySpan    *sentry.Span
	expectedSentryEvent *sentry.Event
	expectedError       error
}

func TestSpanEventToSentryEvent(t *testing.T) {
	// Creating expected Sentry Events and Sample Sentry Spans before hand to avoid redundancy
	sampleSentrySpanForEvent := &sentry.Span{
		TraceID:      TraceIDFromHex("01020304050607080807060504030201"),
		SpanID:       SpanIDFromHex("0102030405060708"),
		ParentSpanID: SpanIDFromHex("0807060504030201"),
		Description:  "span_name",
		Op:           "",
		Tags: map[string]string{
			"key":             "value",
			"library_name":    "otel-go",
			"library_version": "1.4.3",
			"aws_instance":    "ap-south-1",
			"unique_id":       "abcd1234",
			"span_kind":       traceutil.SpanKindStr(ptrace.SpanKindClient),
			"status_message":  "message",
		},
		StartTime: unixNanoToTime(123),
		EndTime:   unixNanoToTime(1234567890),
		Status:    sentry.SpanStatusOK,
	}
	sentryEventBase := sentry.Event{
		Level: "error",
		Sdk: sentry.SdkInfo{
			Name:    "sentry.opentelemetry",
			Version: "0.0.2",
		},
		Tags:        sampleSentrySpanForEvent.Tags,
		Timestamp:   sampleSentrySpanForEvent.EndTime,
		Transaction: sampleSentrySpanForEvent.Description,
		Contexts:    map[string]sentry.Context{},
		Extra:       map[string]interface{}{},
		Modules:     map[string]string{},
		StartTime:   unixNanoToTime(123),
	}
	sentryEventBase.Contexts["trace"] = sentry.TraceContext{
		TraceID:      sampleSentrySpanForEvent.TraceID,
		SpanID:       sampleSentrySpanForEvent.SpanID,
		ParentSpanID: sampleSentrySpanForEvent.ParentSpanID,
		Op:           sampleSentrySpanForEvent.Op,
		Description:  sampleSentrySpanForEvent.Description,
		Status:       sampleSentrySpanForEvent.Status,
	}.Map()

	errorType := "mySampleType"
	errorMessage := "Kernel Panic"
	testCases := []SpanEventToSentryEventCases{
		{
			testName:         "Exception Event with both exception type and message",
			errorMessage:     errorMessage,
			errorType:        errorType,
			sampleSentrySpan: sampleSentrySpanForEvent,
			expectedSentryEvent: func() *sentry.Event {
				expectedSentryEventWithTypeAndMessage := sentryEventBase
				expectedSentryEventWithTypeAndMessage.Type = errorType
				expectedSentryEventWithTypeAndMessage.Message = errorMessage
				expectedSentryEventWithTypeAndMessage.Exception = []sentry.Exception{{
					Value: errorMessage,
					Type:  errorType,
				}}
				return &expectedSentryEventWithTypeAndMessage
			}(),
			expectedError: nil,
		},
		{
			testName:         "Exception Event with only exception type",
			errorMessage:     "",
			errorType:        errorType,
			sampleSentrySpan: sampleSentrySpanForEvent,
			expectedSentryEvent: func() *sentry.Event {
				expectedSentryEventWithType := sentryEventBase
				expectedSentryEventWithType.Type = errorType
				expectedSentryEventWithType.Exception = []sentry.Exception{{
					Value: "",
					Type:  errorType,
				}}
				return &expectedSentryEventWithType
			}(),
			expectedError: nil,
		},
		{
			testName:            "Exception Event with neither exception type nor exception message",
			errorMessage:        "",
			errorType:           "",
			sampleSentrySpan:    sampleSentrySpanForEvent,
			expectedSentryEvent: nil,
			expectedError:       errors.New("error type and error message were both empty"),
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.testName, func(t *testing.T) {
			sentryEvent, err := sentryEventFromError(test.errorMessage, test.errorType, test.sampleSentrySpan)
			if sentryEvent != nil {
				sentryEvent.EventID = test.expectedSentryEvent.EventID
			}
			assert.Equal(t, test.expectedError, err)
			assert.Equal(t, test.expectedSentryEvent, sentryEvent)
		})
	}
}

func TestSpanToSentrySpan(t *testing.T) {
	t.Run("with root span and invalid parent span_id", func(t *testing.T) {
		testSpan := ptrace.NewSpan()
		testSpan.SetParentSpanID(pcommon.NewSpanIDEmpty())

		sentrySpan := convertToSentrySpan(testSpan, pcommon.NewInstrumentationScope(), map[string]string{})
		assert.NotNil(t, sentrySpan)
		assert.True(t, spanIsTransaction(testSpan))
	})

	t.Run("with full span", func(t *testing.T) {
		testSpan := ptrace.NewSpan()

		traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 8, 7, 6, 5, 4, 3, 2, 1})
		spanID := pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
		parentSpanID := pcommon.SpanID([8]byte{8, 7, 6, 5, 4, 3, 2, 1})
		name := "span_name"
		var startTime pcommon.Timestamp = 123
		var endTime pcommon.Timestamp = 1234567890
		kind := ptrace.SpanKindClient
		statusMessage := "message"

		testSpan.Attributes().PutStr("key", "value")

		testSpan.SetTraceID(traceID)
		testSpan.SetSpanID(spanID)
		testSpan.SetParentSpanID(parentSpanID)
		testSpan.SetName(name)
		testSpan.SetStartTimestamp(startTime)
		testSpan.SetEndTimestamp(endTime)
		testSpan.SetKind(kind)

		testSpan.Status().SetMessage(statusMessage)
		testSpan.Status().SetCode(ptrace.StatusCodeOk)

		library := pcommon.NewInstrumentationScope()
		library.SetName("otel-python")
		library.SetVersion("1.4.3")

		resourceTags := map[string]string{
			"aws_instance": "ca-central-1",
			"unique_id":    "abcd1234",
		}

		actual := convertToSentrySpan(testSpan, library, resourceTags)

		assert.NotNil(t, actual)
		assert.False(t, spanIsTransaction(testSpan))

		expected := &sentry.Span{
			TraceID:      TraceIDFromHex("01020304050607080807060504030201"),
			SpanID:       SpanIDFromHex("0102030405060708"),
			ParentSpanID: SpanIDFromHex("0807060504030201"),
			Description:  name,
			Op:           "",
			Tags: map[string]string{
				"key":             "value",
				"library_name":    "otel-python",
				"library_version": "1.4.3",
				"aws_instance":    "ca-central-1",
				"unique_id":       "abcd1234",
				"span_kind":       traceutil.SpanKindStr(ptrace.SpanKindClient),
				"status_message":  statusMessage,
			},
			StartTime: unixNanoToTime(startTime),
			EndTime:   unixNanoToTime(endTime),
			Status:    sentry.SpanStatusOK,
		}

		opts := cmpopts.IgnoreUnexported(sentry.Span{})
		if diff := cmp.Diff(expected, actual, opts); diff != "" {
			t.Errorf("Span mismatch (-expected +actual):\n%s", diff)
		}
	})
}

type SpanDescriptorsCase struct {
	testName string
	// input
	name     string
	attrs    map[string]interface{}
	spanKind ptrace.SpanKind
	// output
	op          string
	description string
}

func TestGenerateSpanDescriptors(t *testing.T) {
	testCases := []SpanDescriptorsCase{
		{
			testName: "http-client",
			name:     "/api/users/{user_id}",
			attrs: map[string]interface{}{
				conventions.AttributeHTTPMethod: "GET",
			},
			spanKind:    ptrace.SpanKindClient,
			op:          "http.client",
			description: "GET /api/users/{user_id}",
		},
		{
			testName: "http-server",
			name:     "/api/users/{user_id}",
			attrs: map[string]interface{}{
				conventions.AttributeHTTPMethod: "POST",
			},
			spanKind:    ptrace.SpanKindServer,
			op:          "http.server",
			description: "POST /api/users/{user_id}",
		},
		{
			testName: "db-call-without-statement",
			name:     "SET mykey 'Val'",
			attrs: map[string]interface{}{
				conventions.AttributeDBSystem: "redis",
			},
			spanKind:    ptrace.SpanKindClient,
			op:          "db",
			description: "SET mykey 'Val'",
		},
		{
			testName: "db-call-with-statement",
			name:     "mysql call",
			attrs: map[string]interface{}{
				conventions.AttributeDBSystem:    "sqlite",
				conventions.AttributeDBStatement: "SELECT * FROM table",
			},
			spanKind:    ptrace.SpanKindClient,
			op:          "db",
			description: "SELECT * FROM table",
		},
		{
			testName: "rpc",
			name:     "grpc.test.EchoService/Echo",
			attrs: map[string]interface{}{
				conventions.AttributeRPCService: "EchoService",
			},
			spanKind:    ptrace.SpanKindClient,
			op:          "rpc",
			description: "grpc.test.EchoService/Echo",
		},
		{
			testName: "message-system",
			name:     "message-destination",
			attrs: map[string]interface{}{
				"messaging.system": "kafka",
			},
			spanKind:    ptrace.SpanKindProducer,
			op:          "message",
			description: "message-destination",
		},
		{
			testName: "faas",
			name:     "message-destination",
			attrs: map[string]interface{}{
				"faas.trigger": "pubsub",
			},
			spanKind:    ptrace.SpanKindServer,
			op:          "pubsub",
			description: "message-destination",
		},
	}

	for _, test := range testCases {
		t.Run(test.testName, func(t *testing.T) {
			attrs := pcommon.NewMap()
			assert.NoError(t, attrs.FromRaw(test.attrs))
			op, description := generateSpanDescriptors(test.name, attrs, test.spanKind)
			assert.Equal(t, test.op, op)
			assert.Equal(t, test.description, description)
		})
	}
}

func TestGenerateTagsFromAttributes(t *testing.T) {
	attrs := pcommon.NewMap()

	attrs.PutStr("string-key", "string-value")
	attrs.PutBool("bool-key", true)
	attrs.PutDouble("double-key", 123.123)
	attrs.PutInt("int-key", 321)

	tags := generateTagsFromAttributes(attrs)

	stringVal := tags["string-key"]
	assert.Equal(t, stringVal, "string-value")
	boolVal := tags["bool-key"]
	assert.Equal(t, boolVal, "true")
	doubleVal := tags["double-key"]
	assert.Equal(t, doubleVal, "123.123")
	intVal := tags["int-key"]
	assert.Equal(t, intVal, "321")
}

type SpanStatusCase struct {
	testName string
	// input
	spanStatus ptrace.Status
	// output
	status  sentry.SpanStatus
	message string
	tags    map[string]string
}

func TestStatusFromSpanStatus(t *testing.T) {
	testCases := []SpanStatusCase{
		{
			testName:   "with empty status",
			spanStatus: ptrace.NewStatus(),
			status:     sentry.SpanStatusOK,
			message:    "",
			tags:       map[string]string{},
		},
		{
			testName: "with status code",
			spanStatus: func() ptrace.Status {
				spanStatus := ptrace.NewStatus()
				spanStatus.SetMessage("message")
				spanStatus.SetCode(ptrace.StatusCodeError)

				return spanStatus
			}(),
			status:  sentry.SpanStatusUnknown,
			message: "message",
			tags:    map[string]string{},
		},
		{
			testName: "with unimplemented status code",
			spanStatus: func() ptrace.Status {
				spanStatus := ptrace.NewStatus()
				spanStatus.SetMessage("message")
				spanStatus.SetCode(ptrace.StatusCode(1337))

				return spanStatus
			}(),
			status:  sentry.SpanStatusUnknown,
			message: "error code 1337",
			tags:    map[string]string{},
		},
		{
			testName: "with ok status code",
			spanStatus: func() ptrace.Status {
				spanStatus := ptrace.NewStatus()
				spanStatus.SetMessage("message")
				spanStatus.SetCode(ptrace.StatusCodeOk)

				return spanStatus
			}(),
			status:  sentry.SpanStatusOK,
			message: "message",
			tags:    map[string]string{},
		},
		{
			testName: "with 400 http status code",
			spanStatus: func() ptrace.Status {
				spanStatus := ptrace.NewStatus()
				spanStatus.SetMessage("message")
				spanStatus.SetCode(ptrace.StatusCodeError)

				return spanStatus
			}(),
			status:  sentry.SpanStatusUnauthenticated,
			message: "message",
			tags: map[string]string{
				"http.status_code": "401",
			},
		},
		{
			testName: "with canceled grpc status code",
			spanStatus: func() ptrace.Status {
				spanStatus := ptrace.NewStatus()
				spanStatus.SetMessage("message")
				spanStatus.SetCode(ptrace.StatusCodeError)

				return spanStatus
			}(),
			status:  sentry.SpanStatusCanceled,
			message: "message",
			tags: map[string]string{
				"rpc.grpc.status_code": "1",
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.testName, func(t *testing.T) {
			status, message := statusFromSpanStatus(test.spanStatus, test.tags)
			assert.Equal(t, test.status, status)
			assert.Equal(t, test.message, message)
		})
	}
}

type ClassifyOrphanSpanTestCase struct {
	testName string
	// input
	idMap          map[sentry.SpanID]sentry.SpanID
	transactionMap map[sentry.SpanID]*sentry.Event
	spans          []*sentry.Span
	// output
	assertion func(t *testing.T, orphanSpans []*sentry.Span)
}

func TestClassifyOrphanSpans(t *testing.T) {
	testCases := []ClassifyOrphanSpanTestCase{
		{
			testName:       "with no root spans",
			idMap:          make(map[sentry.SpanID]sentry.SpanID),
			transactionMap: generateEmptyTransactionMap(),
			spans:          generateOrphanSpansFromSpans(childSpan1, childSpan2),
			assertion: func(t *testing.T, orphanSpans []*sentry.Span) {
				assert.Len(t, orphanSpans, 2)
			},
		},
		{
			testName: "with no remaining orphans",
			idMap: func() map[sentry.SpanID]sentry.SpanID {
				idMap := make(map[sentry.SpanID]sentry.SpanID)
				idMap[rootSpan1.SpanID] = rootSpan1.SpanID
				return idMap
			}(),
			transactionMap: generateEmptyTransactionMap(rootSpan1),
			spans:          generateOrphanSpansFromSpans(childChildSpan1, childSpan1, childSpan2),
			assertion: func(t *testing.T, orphanSpans []*sentry.Span) {
				assert.Len(t, orphanSpans, 0)
			},
		},
		{
			testName: "with some remaining orphans",
			idMap: func() map[sentry.SpanID]sentry.SpanID {
				idMap := make(map[sentry.SpanID]sentry.SpanID)
				idMap[rootSpan1.SpanID] = rootSpan1.SpanID
				return idMap
			}(),
			transactionMap: generateEmptyTransactionMap(rootSpan1),
			spans:          generateOrphanSpansFromSpans(childChildSpan1, childSpan1, childSpan2, orphanSpan1),
			assertion: func(t *testing.T, orphanSpans []*sentry.Span) {
				assert.Len(t, orphanSpans, 1)
				assert.Equal(t, orphanSpan1, orphanSpans[0])
			},
		},
		{
			testName: "with multiple roots",
			idMap: func() map[sentry.SpanID]sentry.SpanID {
				idMap := make(map[sentry.SpanID]sentry.SpanID)
				idMap[rootSpan1.SpanID] = rootSpan1.SpanID
				idMap[rootSpan2.SpanID] = rootSpan2.SpanID
				return idMap
			}(),
			transactionMap: generateEmptyTransactionMap(rootSpan1, rootSpan2),
			spans:          generateOrphanSpansFromSpans(childChildSpan1, childSpan1, root2childSpan, childSpan2),
			assertion: func(t *testing.T, orphanSpans []*sentry.Span) {
				assert.Len(t, orphanSpans, 0)
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.testName, func(t *testing.T) {
			orphanSpans := classifyAsOrphanSpans(test.spans, len(test.spans)+1, test.idMap, test.transactionMap)
			test.assertion(t, orphanSpans)
		})
	}
}

func TestGenerateTransactions(t *testing.T) {
	transactionMap := generateEmptyTransactionMap(rootSpan1, rootSpan2)
	orphanSpans := generateOrphanSpansFromSpans(orphanSpan1, childSpan1)
	environment := "staging"

	transactions := generateTransactions(transactionMap, orphanSpans, environment)

	assert.Len(t, transactions, 4)
}

type mockTransport struct {
	called       bool
	transactions []*sentry.Event
}

func (t *mockTransport) SendEvents(transactions []*sentry.Event) {
	t.transactions = transactions
	t.called = true
}

func (t *mockTransport) Configure(_ sentry.ClientOptions) {}
func (t *mockTransport) Flush(_ context.Context) bool {
	return true
}

type PushTraceDataTestCase struct {
	testName string
	// input
	td ptrace.Traces
	// output
	called bool
}

func TestPushTraceData(t *testing.T) {
	testCases := []PushTraceDataTestCase{
		{
			testName: "with no resources",
			td:       ptrace.NewTraces(),
			called:   false,
		},
		{
			testName: "with no libraries",
			td: func() ptrace.Traces {
				traces := ptrace.NewTraces()
				resourceSpans := ptrace.NewResourceSpans()
				tgt := traces.ResourceSpans().AppendEmpty()
				resourceSpans.CopyTo(tgt)
				return traces
			}(),
			called: false,
		},
		{
			testName: "with no spans",
			td: func() ptrace.Traces {
				traces := ptrace.NewTraces()
				resourceSpans := traces.ResourceSpans()
				resourceSpans.AppendEmpty().ScopeSpans().AppendEmpty()
				return traces
			}(),
			called: false,
		},
		{
			testName: "with full trace",
			td: func() ptrace.Traces {
				traces := ptrace.NewTraces()
				resourceSpans := traces.ResourceSpans()
				resourceSpans.AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				return traces
			}(),
			called: true,
		},
	}

	for _, test := range testCases {
		t.Run(test.testName, func(t *testing.T) {
			transport := &mockTransport{
				called: false,
			}
			s := &SentryExporter{
				transport: transport,
			}

			err := s.pushTraceData(context.Background(), test.td)
			assert.Nil(t, err)
			assert.Equal(t, test.called, transport.called)
		})
	}
}

type TransactionFromSpanMarshalEventTestCase struct {
	testName string
	// input
	span *sentry.Span
	// output
	wantContains string
}

// This is a regression test for https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/13415
// to make sure that `parent_span_id` is not included in the serialized context if it is not defined
func TestTransactionContextFromSpanMarshalEvent(t *testing.T) {
	testCases := []TransactionFromSpanMarshalEventTestCase{
		{
			testName: "with parent span id",
			span: &sentry.Span{
				TraceID:      TraceIDFromHex("1915f8aa35ff8fbebbfeedb9d7e07216"),
				SpanID:       SpanIDFromHex("ea4864700408805c"),
				ParentSpanID: SpanIDFromHex("4c577fe4aec9523b"),
			},
			wantContains: `{"trace":{"parent_span_id":"4c577fe4aec9523b","span_id":"ea4864700408805c","trace_id":"1915f8aa35ff8fbebbfeedb9d7e07216"}}`,
		},
		{
			testName: "without parent span id",
			span: &sentry.Span{
				TraceID: TraceIDFromHex("11ab4adc8ac6ed96f245cd96b5b6d141"),
				SpanID:  SpanIDFromHex("cc55ac735f0170ac"),
			},
			wantContains: `{"trace":{"span_id":"cc55ac735f0170ac","trace_id":"11ab4adc8ac6ed96f245cd96b5b6d141"}}`,
		},
	}

	environment := "production"
	for _, test := range testCases {
		t.Run(test.testName, func(t *testing.T) {
			event := transactionFromSpan(test.span, environment)
			// mimic what sentry is doing internally
			// see: https://github.com/getsentry/sentry-go/blob/v0.13.0/transport.go#L66-L70
			d, err := json.Marshal(event.Contexts)
			assert.NoError(t, err)
			assert.Contains(t, string(d), test.wantContains)
		})
	}
}
