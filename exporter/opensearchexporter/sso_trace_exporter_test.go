// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"bytes"
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/antchfx/jsonquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func Test_span_sample(t *testing.T) {
	testCase := &singleSpanTestCase{
		name:          "sample span",
		configMod:     func(config *Config) {},
		givenResource: sampleResource(),
		givenSpan:     sampleSpan(),
		givenScope:    sampleScope(),
		assertNode: func(t *testing.T, doc *jsonquery.Node) {
			assertJSONValueEqual(t, "ip_tcp", doc, "/attributes/net.transport")
			assertJSONValueEqual(t, float64(3), doc, "/droppedAttributesCount")
			assertJSONValueEqual(t, float64(5), doc, "/droppedEventsCount")
			assertJSONValueEqual(t, float64(8), doc, "/droppedLinksCount")
			assertJSONValueEqual(t, sampleTimeB.Format(time.RFC3339Nano), doc, "/endTime")
			assertJSONValueEqual(t, "Internal", doc, "/kind")
			assertJSONValueEqual(t, "sample-span", doc, "/name")
			assertJSONValueEqual(t, sampleSpanIDB.String(), doc, "/parentSpanId")
			assertJSONValueEqual(t, sampleSpanIDA.String(), doc, "/spanId")
			assertJSONValueEqual(t, "Error", doc, "/status/code")
			assertJSONValueEqual(t, "sample status message", doc, "/status/message")
			assertJSONValueEqual(t, sampleTimeA.Format(time.RFC3339Nano), doc, "/startTime")
			assertJSONValueEqual(t, rawSampleTraceState, doc, "/traceState")
			assertJSONValueEqual(t, sampleTraceIDA.String(), doc, "/traceId")
		},
	}

	executeTestCase(t, testCase)
}

func Test_instrumentation_scope(t *testing.T) {
	testCases := []*singleSpanTestCase{
		{
			name:          "sample scope",
			configMod:     func(config *Config) {},
			givenResource: sampleResource(),
			givenSpan:     sampleSpan(),
			givenScope:    sampleScope(),
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				scope := jsonquery.FindOne(doc, "/instrumentationScope")
				assert.NotNil(t, scope)
				assert.NotNil(t, jsonquery.FindOne(scope, "name"))
				assert.NotNil(t, jsonquery.FindOne(scope, "version"))
				assertJSONValueEqual(t, sampleSchemaURL, scope, "/schemaUrl")
				assertJSONValueEqual(t, "com.example.some-sdk", scope, "/attributes/scope.short_name")
			},
		},
	}

	for _, testCase := range testCases {
		executeTestCase(t, testCase)
	}
}

func Test_links(t *testing.T) {
	testCases := []*singleSpanTestCase{
		{
			name:          "no links",
			configMod:     func(config *Config) {},
			givenResource: sampleResource(),
			givenSpan:     sampleSpan(),
			givenScope:    sampleScope(),
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				links := jsonquery.FindOne(doc, "/links")
				assert.Nil(t, links)
			},
		},

		{
			name:          "sample link",
			configMod:     func(config *Config) {},
			givenResource: sampleResource(),
			givenSpan:     withSampleLink(sampleSpan()),
			givenScope:    sampleScope(),
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				links := jsonquery.FindOne(doc, "/links/*")

				assertJSONValueEqual(t, sampleTraceIDB.String(), links, "/traceId")
				assertJSONValueEqual(t, sampleSpanIDB.String(), links, "/spanId")
				assertJSONValueEqual(t, float64(3), links, "droppedAttributesCount")
				assertJSONValueEqual(t, rawSampleTraceState, links, "traceState")
				assertJSONValueEqual(t, "remote-link", links, "/attributes/link_attr")
			},
		},
	}

	for _, testCase := range testCases {
		executeTestCase(t, testCase)
	}
}

// Test_events verifies that events are mapped as expected.
func Test_events(t *testing.T) {
	testCases := []*singleSpanTestCase{
		{
			name:          "no events",
			configMod:     func(config *Config) {},
			givenResource: sampleResource(),
			givenScope:    sampleScope(),
			givenSpan:     sampleSpan(),
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				eventsNode := jsonquery.FindOne(doc, "/events")
				assert.Nil(t, eventsNode)
			},
		},
		{
			name:          "event with timestamp",
			configMod:     func(config *Config) {},
			givenResource: sampleResource(),
			givenScope:    sampleScope(),
			givenSpan:     withEventWithTimestamp(sampleSpan()),
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				eventsNode := jsonquery.FindOne(doc, "/events/*")
				assertJSONValueEqual(t, "cloudevent", eventsNode, "name")
				// JSON Go encodes all numbers as floats, see https://pkg.go.dev/encoding/json#Marshal
				assertJSONValueEqual(t, float64(42), eventsNode, "droppedAttributesCount")
				assert.NotNil(t, eventsNode.SelectElement("@timestamp"))
				assert.Nil(t, eventsNode.SelectElement("observedTimestamp"))
			},
		},
		{
			name:          "event, no timestamp, observedTimestamp added",
			configMod:     func(config *Config) {},
			givenResource: sampleResource(),
			givenScope:    sampleScope(),
			givenSpan:     withEventNoTimestamp(sampleSpan()),
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				eventsNode := jsonquery.FindOne(doc, "/events/*")
				assertJSONValueEqual(t, "exception", eventsNode, "name")
				assert.NotNil(t, eventsNode.SelectElement("observedTimestamp"))
				assert.Nil(t, eventsNode.SelectElement("@timestamp"))
			},
		},
	}

	for _, testCase := range testCases {
		executeTestCase(t, testCase)
	}
}

// Test_attributes_data_stream verifies that Dataset and Namespace configuration options are
// recorded in the `.attributes.data_stream` object of an exported span
func Test_attributes_data_stream(t *testing.T) {
	// validateDataStreamType asserts that /attributes/data_stream/type exists and is equal to "span"
	validateDataStreamType := func(t *testing.T, node *jsonquery.Node) {
		assert.Equal(t, "span", jsonquery.FindOne(node, "/attributes/data_stream/type").Value())
	}
	testCases := []struct {
		name       string
		assertNode func(*testing.T, *jsonquery.Node)
		configMod  func(*Config)
	}{
		{
			name:      "no data_stream attribute expected",
			configMod: func(config *Config) {},
			assertNode: func(t *testing.T, node *jsonquery.Node) {
				// no data_stream attribute expected
				luckyNode := jsonquery.FindOne(node, "/attributes/data_stream")
				assert.Nil(t, luckyNode)
			},
		},
		{
			name:      "datatset is ngnix",
			configMod: func(config *Config) { config.Dataset = "ngnix" },
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				assertJSONValueEqual(t, "ngnix", doc, "/attributes/data_stream/dataset")
				validateDataStreamType(t, doc)
			},
		},
		{
			name:      "namespace is exceptions",
			configMod: func(config *Config) { config.Namespace = "exceptions" },
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				// no attributes property expected
				assertJSONValueEqual(t, "exceptions", doc, "/attributes/data_stream/namespace")
				validateDataStreamType(t, doc)
			},
		},
		{
			name: "dataset is mysql, namespace is warnings",
			configMod: func(config *Config) {
				config.Namespace = "warnings"
				config.Dataset = "mysql"
			},
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				// no attributes property expected
				assertJSONValueEqual(t, "warnings", doc, "/attributes/data_stream/namespace")
				assertJSONValueEqual(t, "mysql", doc, "/attributes/data_stream/dataset")

				validateDataStreamType(t, doc)
			},
		},
	}

	for _, sampleTestCase := range testCases {
		testCase := &singleSpanTestCase{
			name:          sampleTestCase.name,
			configMod:     sampleTestCase.configMod,
			givenResource: sampleResource(),
			givenScope:    sampleScope(),
			givenSpan:     sampleSpan(),
			assertNode:    sampleTestCase.assertNode,
		}
		executeTestCase(t, testCase)
	}

}

// Test_sso_routing verifies that Dataset and Namespace configuration options determine how export data is routed
func Test_sso_routing(t *testing.T) {
	testCases := []struct {
		name          string
		expectedIndex string
		configMod     func(config *Config)
	}{
		{
			name:          "default routing",
			expectedIndex: "sso_traces-default-namespace",
			configMod:     func(config *Config) {},
		},
		{
			name: "dataset is webapp",
			configMod: func(config *Config) {
				config.Dataset = "webapp"
			},
			expectedIndex: "sso_traces-webapp-namespace",
		},
		{
			name: "namespace is exceptions",
			configMod: func(config *Config) {
				config.Namespace = "exceptions"
			},
			expectedIndex: "sso_traces-default-exceptions",
		},
		{
			name: "namespace is warnings, dataset is mysql",
			configMod: func(config *Config) {
				config.Namespace = "warnings"
				config.Dataset = "mysql"
			},
			expectedIndex: "sso_traces-mysql-warnings",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			rec := newBulkRecorder()
			server := newRecTestServer(t, rec)
			exporter := newSSOTestTracesExporter(t, logger, server.URL, testCase.configMod)
			err := exporter.pushTraceRecord(context.TODO(), *sampleResource(), *sampleScope(), sampleSchemaURL, *sampleSpan())
			require.NoError(t, err)
			rec.WaitItems(1)

			// Sanity test
			require.Equal(t, 1, rec.NumItems())

			// Sanity test
			require.Len(t, rec.Requests()[0], 1)

			doc, err := jsonquery.Parse(bytes.NewReader(rec.Requests()[0][0].Action))

			// Sanity test
			require.NoError(t, err)

			inx := jsonquery.FindOne(doc, "/create/_index")
			assert.Equal(t, testCase.expectedIndex, inx.Value())
		})
	}
}

// newSSOTestTracesExporter creates a traces exporter that sends data in Simple Schema for Observability schema.
// See https://github.com/opensearch-project/observability for details
func newSSOTestTracesExporter(t *testing.T, logger *zap.Logger, url string, fns ...func(*Config)) *SSOTracesExporter {
	exporter, err := newSSOTracesExporter(logger, withTestTracesExporterConfig(fns...)(url))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, exporter.Shutdown(context.TODO()))
	})
	return exporter
}

// newRecTestServer creates a http webserver that records all the requests in the provided bulkRecorder
func newRecTestServer(t *testing.T, rec *bulkRecorder) *httptest.Server {
	server := newTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
		rec.Record(docs)
		return itemsAllOK(docs)
	})
	t.Cleanup(func() {
		server.Close()
	})
	return server
}

func assertJSONValueEqual(t *testing.T, expected interface{}, node *jsonquery.Node, jq string) {
	luckyNode := jsonquery.FindOne(node, jq)
	assert.NotNil(t, luckyNode)
	assert.Equal(t, expected, luckyNode.Value())
}

// singleSpanTestCase represents a test case that configures sso exporter with configMod,
// pushes givenResource and givenSpan and verifies the bulk request that sso exporter generates
// using assertNode
type singleSpanTestCase struct {
	name          string
	configMod     func(*Config)
	givenResource *pcommon.Resource
	givenSpan     *ptrace.Span
	assertNode    func(*testing.T, *jsonquery.Node)
	givenScope    *pcommon.InstrumentationScope
}

func executeTestCase(t *testing.T, testCase *singleSpanTestCase) {
	t.Run(testCase.name, func(t *testing.T) {
		logger := zaptest.NewLogger(t)

		ctx := context.TODO()
		rec := newBulkRecorder()
		server := newRecTestServer(t, rec)
		exporter := newSSOTestTracesExporter(t, logger, server.URL, testCase.configMod)
		err := exporter.pushTraceRecord(ctx, *testCase.givenResource, *testCase.givenScope, sampleSchemaURL, *testCase.givenSpan)

		// Verify that push completed successfully
		require.NoError(t, err)

		// Wait for HTTP server to capture the bulk request
		rec.WaitItems(1)

		// Confirm there was only one bulk request
		require.Equal(t, 1, rec.NumItems())
		require.Len(t, rec.Requests()[0], 1)

		// Read the bulk request document
		doc, err := jsonquery.Parse(bytes.NewReader(rec.Requests()[0][0].Document))

		require.NoError(t, err)

		// Verify document matches expectations
		testCase.assertNode(t, doc)
	})
}

func sampleResource() *pcommon.Resource {
	r := pcommon.NewResource()
	// An attribute of each supported type
	r.Attributes().PutStr("service.name", "opensearchexporter")
	r.Attributes().PutBool("browser.mobile", false)
	r.Attributes().PutInt("process.pid", 300)
	return &r
}

func sampleScope() *pcommon.InstrumentationScope {
	sample := pcommon.NewInstrumentationScope()
	sample.Attributes().PutStr("scope.short_name", "com.example.some-sdk")
	sample.SetDroppedAttributesCount(7)
	sample.SetName("some-sdk by Example.com")
	sample.SetVersion("semver:2.5.0-SNAPSHOT")
	return &sample
}

func sampleSpan() *ptrace.Span {
	sp := ptrace.NewSpan()

	sp.SetDroppedAttributesCount(3)
	sp.SetDroppedEventsCount(5)
	sp.SetDroppedLinksCount(8)
	sp.SetSpanID(sampleSpanIDA)
	sp.SetParentSpanID(sampleSpanIDB)
	sp.SetTraceID(sampleTraceIDA)
	sp.SetKind(ptrace.SpanKindInternal)
	sp.TraceState().FromRaw(rawSampleTraceState)
	sp.Attributes().PutStr("net.transport", "ip_tcp")
	sp.SetStartTimestamp(pcommon.NewTimestampFromTime(sampleTimeA))
	sp.SetEndTimestamp(pcommon.NewTimestampFromTime(sampleTimeB))
	sp.SetName("sample-span")
	sp.Status().SetCode(ptrace.StatusCodeError)
	sp.Status().SetMessage("sample status message")
	return &sp
}

func withEventNoTimestamp(span *ptrace.Span) *ptrace.Span {
	event := span.Events().AppendEmpty()
	// This event lacks timestamp
	event.Attributes().PutStr("exception.message", "Invalid user input")
	event.Attributes().PutStr("exception.type", "ApplicationException")
	event.SetDroppedAttributesCount(42)
	event.SetName("exception")
	return span
}

func withEventWithTimestamp(span *ptrace.Span) *ptrace.Span {
	event := span.Events().AppendEmpty()
	event.Attributes().PutStr("cloudevents.event_id", "001")
	event.Attributes().PutStr("cloudevents.event_source", "example.com/example-service")
	event.Attributes().PutStr("cloudevents.event_type", "com.example.example-service.panic")
	event.SetName("cloudevent")
	event.SetDroppedAttributesCount(42)
	event.SetTimestamp(
		pcommon.NewTimestampFromTime(sampleTimeC))
	return span
}

func withSampleLink(span *ptrace.Span) *ptrace.Span {
	link := span.Links().AppendEmpty()
	link.SetDroppedAttributesCount(3)
	link.SetTraceID(sampleTraceIDB)
	link.SetSpanID(sampleSpanIDB)
	link.TraceState().FromRaw(rawSampleTraceState)
	link.Attributes().PutStr("link_attr", "remote-link")
	return span
}

const sampleSchemaURL string = "https://example.com/schema/1.0"

var sampleTraceIDA = pcommon.TraceID{1, 0, 0, 0}
var sampleTraceIDB = pcommon.TraceID{1, 1, 0, 0}
var sampleSpanIDA = pcommon.SpanID{1, 0, 0, 0}
var sampleSpanIDB = pcommon.SpanID{1, 1, 0, 0}
var sampleTimeA = time.Date(1990, 1, 1, 1, 1, 1, 1, time.UTC)
var sampleTimeB = time.Date(1995, 1, 1, 1, 1, 1, 1, time.UTC)
var sampleTimeC = time.Date(1998, 1, 1, 1, 1, 1, 1, time.UTC)

// TraceState is defined by a W3C spec. Sample value take from https://www.w3.org/TR/trace-context/#examples-of-tracestate-http-headers
const rawSampleTraceState = "rojo=00f067aa0ba902b7,congo=t61rcWkgMzE"

func withTestTracesExporterConfig(fns ...func(*Config)) func(string) *Config {
	return func(url string) *Config {
		var configMods []func(*Config)
		configMods = append(configMods, func(cfg *Config) {
			cfg.Endpoints = []string{url}
			cfg.NumWorkers = 1
			cfg.Flush.Interval = 10 * time.Millisecond
		})
		configMods = append(configMods, fns...)
		return withDefaultConfig(configMods...)
	}
}
