// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sentryexporter

import (
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector/consumer/pdata"
	"github.com/open-telemetry/opentelemetry-collector/translator/conventions"
	otlptrace "github.com/open-telemetry/opentelemetry-proto/gen/go/trace/v1"
	"github.com/stretchr/testify/assert"
)

// func TestGenerateSentryTraceID(t *testing.T) {
// 	td := testdata.GenerateTraceDataOneSpan()
// 	ilss := td.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0)
// 	assert.EqualValues(t, 2, ilss.Spans().Len())
// }

func TestGenerateTagsFromAttributes(t *testing.T) {
	attrs := pdata.NewAttributeMap()

	attrs.InsertString("string-key", "string-value")
	attrs.InsertBool("bool-key", true)
	attrs.InsertDouble("double-key", 123.123)
	attrs.InsertInt("int-key", 321)

	tags := generateTagsFromAttributes(attrs)

	stringVal, _ := tags["string-key"]
	assert.Equal(t, stringVal, "string-value")
	boolVal, _ := tags["bool-key"]
	assert.Equal(t, boolVal, "true")
	doubleVal, _ := tags["double-key"]
	assert.Equal(t, doubleVal, "123.123")
	intVal, _ := tags["int-key"]
	assert.Equal(t, intVal, "321")
}

func TestGenerateStatusFromSpanStatus(t *testing.T) {
	spanStatus := pdata.NewSpanStatus()

	status1, message1 := generateStatusFromSpanStatus(spanStatus)

	assert.Equal(t, "unknown", status1)
	assert.Equal(t, "", message1)

	spanStatus.InitEmpty()
	spanStatus.SetMessage("message")
	spanStatus.SetCode(pdata.StatusCode(otlptrace.Status_ResourceExhausted))

	status2, message2 := generateStatusFromSpanStatus(spanStatus)

	assert.Equal(t, "resource_exhausted", status2)
	assert.Equal(t, "message", message2)
}

// TODO: Finish this test plz
func TestSpanToSentrySpan(t *testing.T) {
	t.Skip("Finish this test")
	testSpan := pdata.NewSpan()

	// testSpan.InitEmpty()

	sentrySpan1, isRootSpan1 := spanToSentrySpan(testSpan)
	assert.Nil(t, sentrySpan1)
	assert.False(t, isRootSpan1)

	testSpan.InitEmpty()

	traceID := []byte{1, 2, 3, 4, 5, 6, 7, 8, 8, 7, 6, 5, 4, 3, 2, 1}
	spanID := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	parentSpanID := []byte{8, 7, 6, 5, 4, 3, 2, 1}
	name := "test-name"
	attributes := pdata.NewAttributeMap()
	attributes.InsertString("key", "value")
	var startTime pdata.TimestampUnixNano = 123
	var endTime pdata.TimestampUnixNano = 1234567890

	testSpan.SetTraceID(traceID)
	testSpan.SetSpanID(spanID)
	testSpan.SetParentSpanID(parentSpanID)
	testSpan.SetName(name)
	testSpan.SetStartTime(startTime)
	testSpan.SetEndTime(endTime)

	sentrySpan2, isRootSpan2 := spanToSentrySpan(testSpan)
	fmt.Println(sentrySpan2)
	assert.Nil(t, sentrySpan2)
	assert.False(t, isRootSpan2)
}

type SpanDescriptorsCase struct {
	testName    string
	name        string
	attrs       pdata.AttributeMap
	spanKind    pdata.SpanKind
	op          string
	description string
}

func generateSpanDescriptorsTestCases() []SpanDescriptorsCase {
	return []SpanDescriptorsCase{
		{
			testName: "http-client",
			name:     "/api/users/{user_id}",
			attrs: pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{
				conventions.AttributeHTTPMethod: pdata.NewAttributeValueString("GET"),
			}),
			spanKind:    pdata.SpanKindCLIENT,
			op:          "http.client",
			description: "GET /api/users/{user_id}",
		},
		{
			testName: "http-server",
			name:     "/api/users/{user_id}",
			attrs: pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{
				conventions.AttributeHTTPMethod: pdata.NewAttributeValueString("POST"),
			}),
			spanKind:    pdata.SpanKindSERVER,
			op:          "http.server",
			description: "POST /api/users/{user_id}",
		},
		{
			testName: "db-call-without-statement",
			name:     "SET mykey 'Val'",
			attrs: pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{
				conventions.AttributeDBType: pdata.NewAttributeValueString("redis"),
			}),
			spanKind:    pdata.SpanKindCLIENT,
			op:          "db",
			description: "SET mykey 'Val'",
		},
		{
			testName: "db-call-with-statement",
			name:     "mysql call",
			attrs: pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{
				conventions.AttributeDBType:      pdata.NewAttributeValueString("sql"),
				conventions.AttributeDBStatement: pdata.NewAttributeValueString("SELECT * FROM table"),
			}),
			spanKind:    pdata.SpanKindCLIENT,
			op:          "db",
			description: "SELECT * FROM table",
		},
		{
			testName: "rpc",
			name:     "grpc.test.EchoService/Echo",
			attrs: pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{
				conventions.AttributeRPCService: pdata.NewAttributeValueString("EchoService"),
			}),
			spanKind:    pdata.SpanKindCLIENT,
			op:          "rpc",
			description: "grpc.test.EchoService/Echo",
		},
		{
			testName: "message-system",
			name:     "message-destination",
			attrs: pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{
				"messaging.system": pdata.NewAttributeValueString("kafka"),
			}),
			spanKind:    pdata.SpanKindPRODUCER,
			op:          "message",
			description: "message-destination",
		},
		{
			testName: "faas",
			name:     "message-destination",
			attrs: pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{
				"faas.trigger": pdata.NewAttributeValueString("pubsub"),
			}),
			spanKind:    pdata.SpanKindSERVER,
			op:          "pubsub",
			description: "message-destination",
		},
	}
}

func TestGenerateSpanDescriptors(t *testing.T) {
	allTestCases := generateSpanDescriptorsTestCases()

	for _, test := range allTestCases {
		t.Run(test.testName, func(t *testing.T) {
			op, description := generateSpanDescriptors(test.name, test.attrs, test.spanKind)
			assert.Equal(t, test.op, op)
			assert.Equal(t, test.description, description)
		})
	}
}

/*

&sentryexporter.SentrySpan{
	TraceID:"01020304050607080807060504030201",
	SpanID:"0102030405060708",
	ParentSpanID:"0807060504030201",
	Description:"test-name",
	Op:"",
	Tags:sentryexporter.Tags{"span_kind":"SPAN_KIND_UNSPECIFIED"},
	EndTimestamp:"1969-12-31 19:00:01.23456789 -0500 EST",
	Timestamp:"1969-12-31 19:00:00.000000123 -0500 EST",
	Status:"unknown"
}
*/
