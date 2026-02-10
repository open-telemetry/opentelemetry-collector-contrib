// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metricgroup"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
)

const (
	expectedSpanBody               = `{"@timestamp":"2023-04-19T03:04:05.000000006Z","Attributes.service.instance.id":"23","Duration":1000000,"EndTimestamp":"2023-04-19T03:04:06.000000006Z","Events.fooEvent.eventMockBar":"bar","Events.fooEvent.eventMockFoo":"foo","Events.fooEvent.time":"2023-04-19T03:04:05.000000006Z","Kind":"SPAN_KIND_CLIENT","Link":"[{\"attribute\":{},\"spanID\":\"\",\"traceID\":\"01020304050607080807060504030200\"}]","Name":"client span","Resource.cloud.platform":"aws_elastic_beanstalk","Resource.cloud.provider":"aws","Resource.deployment.environment":"BETA","Resource.service.instance.id":"23","Resource.service.name":"some-service","Resource.service.version":"env-version-1234","Scope.lib-foo":"lib-bar","Scope.name":"io.opentelemetry.rabbitmq-2.7","Scope.version":"1.30.0-alpha","SpanId":"1920212223242526","TraceId":"01020304050607080807060504030201","TraceStatus":2,"TraceStatusDescription":"Test"}`
	expectedSpanBodyWithDataStream = `{"@timestamp":"2023-04-19T03:04:05.000000006Z","Attributes.data_stream.dataset":"two","Attributes.data_stream.namespace":"three","Attributes.data_stream.type":"one","Attributes.service.instance.id":"23","Duration":1000000,"EndTimestamp":"2023-04-19T03:04:06.000000006Z","Events.fooEvent.eventMockBar":"bar","Events.fooEvent.eventMockFoo":"foo","Events.fooEvent.time":"2023-04-19T03:04:05.000000006Z","Kind":"SPAN_KIND_CLIENT","Link":"[{\"attribute\":{},\"spanID\":\"\",\"traceID\":\"01020304050607080807060504030200\"}]","Name":"client span","Resource.cloud.platform":"aws_elastic_beanstalk","Resource.cloud.provider":"aws","Resource.deployment.environment":"BETA","Resource.service.instance.id":"23","Resource.service.name":"some-service","Resource.service.version":"env-version-1234","Scope.lib-foo":"lib-bar","Scope.name":"io.opentelemetry.rabbitmq-2.7","Scope.version":"1.30.0-alpha","SpanId":"1920212223242526","TraceId":"01020304050607080807060504030201","TraceStatus":2,"TraceStatusDescription":"Test"}`
)

const (
	expectedLogBody                   = `{"@timestamp":"2023-04-19T03:04:05.000000006Z","Attributes.log-attr1":"value1","Body":"log-body","Resource.key1":"value1","Scope.name":"","Scope.version":"","SeverityNumber":0,"TraceFlags":0}`
	expectedLogBodyWithEmptyTimestamp = `{"@timestamp":"1970-01-01T00:00:00.000000000Z","Attributes.log-attr1":"value1","Body":"log-body","Resource.key1":"value1","Scope.name":"","Scope.version":"","SeverityNumber":0,"TraceFlags":0}`
)

const expectedMetricsEncoded = `{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-hostname","name":"my-host","os":{"platform":"linux"}},"state":"idle","system":{"cpu":{"time":440.23}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-hostname","name":"my-host","os":{"platform":"linux"}},"state":"interrupt","system":{"cpu":{"time":0.0}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-hostname","name":"my-host","os":{"platform":"linux"}},"state":"nice","system":{"cpu":{"time":0.14}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-hostname","name":"my-host","os":{"platform":"linux"}},"state":"softirq","system":{"cpu":{"time":0.77}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-hostname","name":"my-host","os":{"platform":"linux"}},"state":"steal","system":{"cpu":{"time":0.0}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-hostname","name":"my-host","os":{"platform":"linux"}},"state":"system","system":{"cpu":{"time":24.8}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-hostname","name":"my-host","os":{"platform":"linux"}},"state":"user","system":{"cpu":{"time":64.78}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-hostname","name":"my-host","os":{"platform":"linux"}},"state":"wait","system":{"cpu":{"time":1.65}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-hostname","name":"my-host","os":{"platform":"linux"}},"state":"idle","system":{"cpu":{"time":475.69}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-hostname","name":"my-host","os":{"platform":"linux"}},"state":"interrupt","system":{"cpu":{"time":0.0}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-hostname","name":"my-host","os":{"platform":"linux"}},"state":"nice","system":{"cpu":{"time":0.1}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-hostname","name":"my-host","os":{"platform":"linux"}},"state":"softirq","system":{"cpu":{"time":0.57}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-hostname","name":"my-host","os":{"platform":"linux"}},"state":"steal","system":{"cpu":{"time":0.0}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-hostname","name":"my-host","os":{"platform":"linux"}},"state":"system","system":{"cpu":{"time":15.88}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-hostname","name":"my-host","os":{"platform":"linux"}},"state":"user","system":{"cpu":{"time":50.09}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-hostname","name":"my-host","os":{"platform":"linux"}},"state":"wait","system":{"cpu":{"time":0.95}}}`

func TestEncodeSpan(t *testing.T) {
	t.Run("non data stream", func(t *testing.T) {
		encoder, _ := newEncoder(MappingNone)
		td := mockResourceSpans()
		var buf bytes.Buffer
		err := encoder.encodeSpan(
			encodingContext{
				resource: td.ResourceSpans().At(0).Resource(),
				scope:    td.ResourceSpans().At(0).ScopeSpans().At(0).Scope(),
			},
			td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0),
			elasticsearch.Index{}, &buf,
		)
		assert.NoError(t, err)
		assert.Equal(t, expectedSpanBody, buf.String())
	})

	// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/42454.
	t.Run("data stream", func(t *testing.T) {
		encoder, _ := newEncoder(MappingNone)
		td := mockResourceSpans()
		var buf bytes.Buffer
		err := encoder.encodeSpan(
			encodingContext{
				resource: td.ResourceSpans().At(0).Resource(),
				scope:    td.ResourceSpans().At(0).ScopeSpans().At(0).Scope(),
			},
			td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0),
			elasticsearch.NewDataStreamIndex("one", "two", "three"),
			&buf,
		)
		assert.NoError(t, err)
		assert.Equal(t, expectedSpanBodyWithDataStream, buf.String())
	})
}

func TestEncodeLog(t *testing.T) {
	t.Run("empty timestamp with observedTimestamp override", func(t *testing.T) {
		encoder, _ := newEncoder(MappingNone)
		td := mockResourceLogs()
		td.ScopeLogs().At(0).LogRecords().At(0).SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Date(2023, 4, 19, 3, 4, 5, 6, time.UTC)))
		var buf bytes.Buffer
		err := encoder.encodeLog(
			encodingContext{
				resource:          td.Resource(),
				resourceSchemaURL: td.SchemaUrl(),
				scope:             td.ScopeLogs().At(0).Scope(),
				scopeSchemaURL:    td.ScopeLogs().At(0).SchemaUrl(),
			},
			td.ScopeLogs().At(0).LogRecords().At(0),
			elasticsearch.Index{}, &buf,
		)
		assert.NoError(t, err)
		assert.Equal(t, expectedLogBody, buf.String())
	})

	t.Run("both timestamp and observedTimestamp empty", func(t *testing.T) {
		encoder, _ := newEncoder(MappingNone)
		td := mockResourceLogs()
		var buf bytes.Buffer
		err := encoder.encodeLog(
			encodingContext{
				resource:          td.Resource(),
				resourceSchemaURL: td.SchemaUrl(),
				scope:             td.ScopeLogs().At(0).Scope(),
				scopeSchemaURL:    td.ScopeLogs().At(0).SchemaUrl(),
			},
			td.ScopeLogs().At(0).LogRecords().At(0),
			elasticsearch.Index{}, &buf,
		)
		assert.NoError(t, err)
		assert.Equal(t, expectedLogBodyWithEmptyTimestamp, buf.String())
	})
}

func TestEncodeMetric(t *testing.T) {
	// Prepare metrics to test.
	metrics := createTestMetrics(t)

	// Encode the metrics.
	encoder, _ := newEncoder(MappingECS)
	hasher := newDataPointHasher(MappingECS)

	groupedDataPoints := make(map[metricgroup.HashKey][]datapoints.DataPoint)

	var docsBytes [][]byte
	rm := metrics.ResourceMetrics().At(0)
	sm := rm.ScopeMetrics().At(0)
	m := sm.Metrics().At(0)
	dps := m.Sum().DataPoints()
	hasher.UpdateResource(rm.Resource())
	hasher.UpdateScope(sm.Scope())
	for _, dp := range dps.All() {
		dp := datapoints.NewNumber(m, dp)
		hasher.UpdateDataPoint(dp)
		dpHash := hasher.HashKey()
		dataPoints, ok := groupedDataPoints[dpHash]
		if !ok {
			groupedDataPoints[dpHash] = []datapoints.DataPoint{dp}
		} else {
			groupedDataPoints[dpHash] = append(dataPoints, dp)
		}
	}

	for _, dataPoints := range groupedDataPoints {
		var buf bytes.Buffer
		errors := make([]error, 0)
		_, err := encoder.encodeMetrics(
			encodingContext{
				resource:          rm.Resource(),
				resourceSchemaURL: rm.SchemaUrl(),
				scope:             sm.Scope(),
				scopeSchemaURL:    sm.SchemaUrl(),
			},
			dataPoints, &errors, elasticsearch.Index{}, &buf,
		)
		require.Empty(t, errors, "%+v", err)
		require.NoError(t, err)
		docsBytes = append(docsBytes, buf.Bytes())
	}

	allDocsSorted := docBytesToSortedString(docsBytes)
	assert.Equal(t, expectedMetricsEncoded, allDocsSorted)
}

func docBytesToSortedString(docsBytes [][]byte) string {
	// Convert the byte arrays to strings and sort the docs to make the test deterministic.
	docs := make([]string, len(docsBytes))
	for i, docBytes := range docsBytes {
		docs[i] = string(docBytes)
	}
	sort.Strings(docs)
	allDocsSorted := strings.Join(docs, "\n")
	return allDocsSorted
}

func createTestMetrics(t *testing.T) pmetric.Metrics {
	metricsUnmarshaler := &pmetric.JSONUnmarshaler{}
	metricBytes, err := os.ReadFile("testdata/metrics-cpu.json")
	require.NoError(t, err)
	metrics, err := metricsUnmarshaler.UnmarshalMetrics(metricBytes)
	require.NoError(t, err)
	metrics.MarkReadOnly()
	return metrics
}

func mockResourceSpans() ptrace.Traces {
	traces := ptrace.NewTraces()

	resourceSpans := traces.ResourceSpans().AppendEmpty()
	attr := resourceSpans.Resource().Attributes()
	attr.PutStr("cloud.provider", "aws")
	attr.PutStr("cloud.platform", "aws_elastic_beanstalk")
	attr.PutStr("deployment.environment", "BETA")
	attr.PutStr("service.instance.id", "23")
	attr.PutStr("service.version", "env-version-1234")

	resourceSpans.Resource().Attributes().PutStr("service.name", "some-service")

	tStart := time.Date(2023, 4, 19, 3, 4, 5, 6, time.UTC)
	tEnd := time.Date(2023, 4, 19, 3, 4, 6, 6, time.UTC)

	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	scopeSpans.Scope().SetName("io.opentelemetry.rabbitmq-2.7")
	scopeSpans.Scope().SetVersion("1.30.0-alpha")
	scopeSpans.Scope().Attributes().PutStr("lib-foo", "lib-bar")

	span := scopeSpans.Spans().AppendEmpty()
	span.SetName("client span")
	span.SetSpanID([8]byte{0x19, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26})
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 8, 7, 6, 5, 4, 3, 2, 1})
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(tStart))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(tEnd))
	span.Status().SetCode(2)
	span.Status().SetMessage("Test")
	span.Attributes().PutStr("service.instance.id", "23")
	span.Links().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 8, 7, 6, 5, 4, 3, 2, 0})

	event := span.Events().AppendEmpty()
	event.SetName("fooEvent")
	event.SetTimestamp(pcommon.NewTimestampFromTime(tStart))
	event.Attributes().PutStr("eventMockFoo", "foo")
	event.Attributes().PutStr("eventMockBar", "bar")
	traces.MarkReadOnly()
	return traces
}

func mockResourceLogs() plog.ResourceLogs {
	rl := plog.NewResourceLogs()
	rl.Resource().Attributes().PutStr("key1", "value1")
	l := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	l.Attributes().PutStr("log-attr1", "value1")
	l.Body().SetStr("log-body")
	return rl
}

func TestEncodeAttributes(t *testing.T) {
	t.Parallel()

	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().Attributes().PutStr("keyStr", "val str")
	scopeLogs.Scope().Attributes().PutInt("keyInt", 42)
	logRecord := scopeLogs.LogRecords().AppendEmpty()
	err := logRecord.Attributes().FromRaw(map[string]any{
		"s": "baz",
		"o": map[string]any{
			"sub_i": 19,
		},
	})
	require.NoError(t, err, "failed to set attributes on logRecord")

	tests := map[string]struct {
		mappingMode MappingMode
		want        string
	}{
		"raw": {
			mappingMode: MappingRaw,
			want: `
			{
			  "@timestamp": "1970-01-01T00:00:00.000000000Z",
			  "Scope.name": "",
			  "Scope.version": "",
			  "Scope.keyInt": 42,
			  "Scope.keyStr": "val str",
			  "SeverityNumber": 0,
			  "TraceFlags": 0,
			  "o.sub_i": 19,
			  "s": "baz"
			}`,
		},
		"none": {
			mappingMode: MappingNone,
			want: `
			{
			  "@timestamp": "1970-01-01T00:00:00.000000000Z",
			  "Scope.name": "",
			  "Scope.version": "",
			  "Scope.keyInt": 42,
			  "Scope.keyStr": "val str",
			  "SeverityNumber": 0,
			  "TraceFlags": 0,
			  "Attributes.o.sub_i": 19,
			  "Attributes.s": "baz"
			}`,
		},
		"ecs": {
			mappingMode: MappingECS,
			want: `
			{
			  "@timestamp": "1970-01-01T00:00:00.000000000Z",
			  "keyInt": 42,
			  "keyStr": "val str",
			  "o": {
			    "sub_i": 19
			  },
			  "s": "baz"
			}`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			encoder, err := newEncoder(test.mappingMode)
			require.NoError(t, err)

			var buf bytes.Buffer
			err = encoder.encodeLog(encodingContext{
				resource: resourceLogs.Resource(),
				scope:    scopeLogs.Scope(),
			}, logRecord, elasticsearch.Index{}, &buf)
			require.NoError(t, err)
			require.JSONEq(t, test.want, buf.String())
		})
	}
}

func TestEncodeSpan_Events(t *testing.T) {
	t.Parallel()

	span := ptrace.NewSpan()
	for i := range 4 {
		event := span.Events().AppendEmpty()
		event.SetName(fmt.Sprintf("event_%d", i))
	}

	tests := map[string]struct {
		mappingMode MappingMode
		want        string
	}{
		"raw": {
			mappingMode: MappingRaw,
			want: `
			{
			  "@timestamp": "1970-01-01T00:00:00.000000000Z",
			  "Duration": 0,
			  "EndTimestamp": "1970-01-01T00:00:00.000000000Z",
			  "Scope.name": "",
			  "Scope.version": "",
			  "Kind": "SPAN_KIND_UNSPECIFIED",
			  "Link": "[]",
			  "TraceStatus": 0,
			  "event_0.time": "1970-01-01T00:00:00.000000000Z",
			  "event_1.time": "1970-01-01T00:00:00.000000000Z",
			  "event_2.time": "1970-01-01T00:00:00.000000000Z",
			  "event_3.time": "1970-01-01T00:00:00.000000000Z"
			}`,
		},
		"none": {
			mappingMode: MappingNone,
			want: `
			{
			  "@timestamp": "1970-01-01T00:00:00.000000000Z",
			  "Duration": 0,
			  "EndTimestamp": "1970-01-01T00:00:00.000000000Z",
			  "Scope.name": "",
			  "Scope.version": "",
			  "Kind": "SPAN_KIND_UNSPECIFIED",
			  "Link": "[]",
			  "TraceStatus": 0,
			  "Events.event_0.time": "1970-01-01T00:00:00.000000000Z",
			  "Events.event_1.time": "1970-01-01T00:00:00.000000000Z",
			  "Events.event_2.time": "1970-01-01T00:00:00.000000000Z",
			  "Events.event_3.time": "1970-01-01T00:00:00.000000000Z"
			}`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			encoder, err := newEncoder(test.mappingMode)
			require.NoError(t, err)

			var buf bytes.Buffer
			err = encoder.encodeSpan(encodingContext{
				resource: pcommon.NewResource(),
				scope:    pcommon.NewInstrumentationScope(),
			}, span, elasticsearch.Index{}, &buf)
			require.NoError(t, err)
			require.JSONEq(t, test.want, buf.String())
		})
	}
}

func TestEncodeLogECSModeDuplication(t *testing.T) {
	logs := plog.NewLogs()
	resource := logs.ResourceLogs().AppendEmpty().Resource()
	err := resource.Attributes().FromRaw(map[string]any{
		"agent.name":      "custom-agent",
		"agent.version":   "1.2.3",
		"service.name":    "foo.bar",
		"host.name":       "localhost",
		"service.version": "1.1.0",
		"os.type":         "darwin",
		"os.description":  "Mac OS Mojave",
		"os.name":         "Mac OS X",
		"os.version":      "10.14.1",
	})
	require.NoError(t, err)

	want := `{"@timestamp":"2024-03-12T20:00:41.123456789Z","agent":{"name":"custom-agent","version":"1.2.3"},"container":{"image":{"tag":["v3.4.0"]}},"event":{"action":"user-password-change"},"host":{"hostname":"localhost","name":"localhost","os":{"full":"Mac OS Mojave","name":"Mac OS X","platform":"darwin","type":"macos","version":"10.14.1"}},"service":{"name":"foo.bar","version":"1.1.0"}}`

	resourceContainerImageTags := resource.Attributes().PutEmptySlice("container.image.tags")
	err = resourceContainerImageTags.FromRaw([]any{"v3.4.0"})
	require.NoError(t, err)

	scope := pcommon.NewInstrumentationScope()

	record := plog.NewLogRecord()
	err = record.Attributes().FromRaw(map[string]any{
		"event.name": "user-password-change",
	})
	require.NoError(t, err)
	observedTimestamp := pcommon.Timestamp(1710273641123456789)
	record.SetObservedTimestamp(observedTimestamp)
	logs.MarkReadOnly()

	var buf bytes.Buffer
	encoder, _ := newEncoder(MappingECS)
	err = encoder.encodeLog(
		encodingContext{resource: resource, scope: scope},
		record, elasticsearch.Index{}, &buf,
	)
	require.NoError(t, err)

	assert.Equal(t, want, buf.String())
}

func TestEncodeSpanECSMode(t *testing.T) {
	encoder, _ := newEncoder(MappingECS)

	resource := pcommon.NewResource()
	err := resource.Attributes().FromRaw(map[string]any{
		"cloud.provider":              "aws",
		"cloud.platform":              "aws_elastic_beanstalk",
		"deployment.environment":      "BETA",
		"deployment.environment.name": "BETA",
		"service.instance.id":         "23",
		"service.name":                "some-service",
		"service.version":             "env-version-1234",
		"process.parent_pid":          "42",
		"process.executable.name":     "node",
		"client.address":              "12.53.12.1",
		"source.address":              "12.53.12.1",
		"faas.instance":               "arn:aws:lambda:us-east-2:123456789012:function:custom-runtime",
		"faas.trigger":                "api-gateway",
	})
	require.NoError(t, err)

	// add slice attributes
	processCommandLineSlice := resource.Attributes().PutEmptySlice("process.command_line")
	err = processCommandLineSlice.FromRaw([]any{"node", "app.js"})
	require.NoError(t, err)

	scope := pcommon.NewInstrumentationScope()

	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans().AppendEmpty()
	resource.CopyTo(resourceSpans.Resource())
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	scope.CopyTo(scopeSpans.Scope())

	span := scopeSpans.Spans().AppendEmpty()
	err = span.Attributes().FromRaw(map[string]any{
		"db.system":               "sql",
		"db.namespace":            "users",
		"db.query.text":           "SELECT * FROM users WHERE user_id=?",
		"http.response.body.size": "http.response.encoded_body_size",
	})
	require.NoError(t, err)

	span.SetName("client span")
	span.SetSpanID([8]byte{0x19, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26})
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 8, 7, 6, 5, 4, 3, 2, 1})
	span.SetParentSpanID([8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08})
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Date(2023, 4, 19, 3, 4, 5, 6, time.UTC)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Date(2023, 4, 19, 3, 4, 6, 6, time.UTC)))
	span.Status().SetCode(ptrace.StatusCodeError)
	span.Status().SetMessage("Test")
	require.NoError(t, err)

	link1 := span.Links().AppendEmpty()
	link1.SetTraceID([16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01})
	link1.SetSpanID([8]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18})

	link2 := span.Links().AppendEmpty()
	link2.SetTraceID([16]byte{0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x28, 0x27, 0x26, 0x25, 0x24, 0x23, 0x22, 0x21})
	link2.SetSpanID([8]byte{0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38})

	var buf bytes.Buffer
	err = encoder.encodeSpan(
		encodingContext{
			resource: resource,
			scope:    scope,
		},
		span,
		elasticsearch.Index{},
		&buf,
	)
	assert.NoError(t, err)

	assert.JSONEq(t, `{
	  "@timestamp": "2023-04-19T03:04:05.000000006Z",
	  "trace": {
		"id": "01020304050607080807060504030201"
	  },
	  "span": {
		"id": "1920212223242526",
		"name": "client span",
		"kind": "CLIENT",
		"db": {
		  "instance": "users",
		  "statement": "SELECT * FROM users WHERE user_id=?",
		  "type": "sql"
		},
		"links": [
		  {
			"span"  : { "id": "1112131415161718" },
			"trace" : { "id": "01020304050607080807060504030201" }
		  },
		  {
			"span"  : { "id": "3132333435363738" },
			"trace" : { "id": "21222324252627282827262524232221" }
		  }
		]
	  },
      "parent": {
		"id": "0102030405060708"
	  },
	  "cloud": {
		"provider": "aws",
		"service": {
		  "name": "aws_elastic_beanstalk"
		}
	  },
	  "event": {
		"outcome": "failure"
	  },
	  "service": {
		"environment": "BETA",
		"name": "some-service",
		"node": {
		  "name": "23"
		},
		"version": "env-version-1234"
	  },
	  "process": {
		"parent": { "pid": "42" },
		"title": "node",
		"args" : ["node", "app.js"]
	  },
	  "client": {
		"ip": "12.53.12.1"
      },
	  "source": {
		"ip": "12.53.12.1"
      },
	  "faas": {
		"id" : "arn:aws:lambda:us-east-2:123456789012:function:custom-runtime",
		"trigger": { "type": "api-gateway" }
	  },
	  "http": {
		"response": {
		  "encoded_body_size": "http.response.encoded_body_size"
		}
	  }
	}`, buf.String())
}

func TestEncodeLogECSMode(t *testing.T) {
	logs := plog.NewLogs()
	resource := logs.ResourceLogs().AppendEmpty().Resource()
	err := resource.Attributes().FromRaw(map[string]any{
		"agent.name":                  "custom-agent",
		"agent.version":               "1.2.3",
		"service.name":                "foo.bar",
		"deployment.environment":      "BETA",
		"deployment.environment.name": "BETA",
		"service.version":             "1.1.0",
		"service.instance.id":         "i-103de39e0a",
		"telemetry.sdk.name":          "opentelemetry",
		"telemetry.sdk.version":       "7.9.12",
		"telemetry.sdk.language":      "perl",
		"cloud.provider":              "gcp",
		"cloud.account.id":            "19347013",
		"cloud.region":                "us-west-1",
		"cloud.availability_zone":     "us-west-1b",
		"cloud.platform":              "gke",
		"container.name":              "happy-seger",
		"container.id":                "e69cc5d3dda",
		"container.image.name":        "my-app",
		"container.runtime":           "docker",
		"host.name":                   "i-103de39e0a.gke.us-west-1b.cloud.google.com",
		"host.hostname":               "hostname.example.com",
		"host.id":                     "i-103de39e0a",
		"host.type":                   "t2.medium",
		"host.arch":                   "x86_64",
		"process.pid":                 9833,
		"process.command_line":        "/usr/bin/ssh -l user 10.0.0.16",
		"process.executable.path":     "/usr/bin/ssh",
		"process.runtime.name":        "OpenJDK Runtime Environment",
		"process.runtime.version":     "14.0.2",
		"os.type":                     "darwin",
		"os.description":              "Mac OS Mojave",
		"os.name":                     "Mac OS X",
		"os.version":                  "10.14.1",
		"device.id":                   "00000000-54b3-e7c7-0000-000046bffd97",
		"device.model.identifier":     "SM-G920F",
		"device.model.name":           "Samsung Galaxy S6",
		"device.manufacturer":         "Samsung",
		"k8s.namespace.name":          "default",
		"k8s.node.name":               "node-1",
		"k8s.pod.name":                "opentelemetry-pod-autoconf",
		"k8s.pod.uid":                 "275ecb36-5aa8-4c2a-9c47-d8bb681b9aff",
		"k8s.deployment.name":         "coredns",
		"k8s.job.name":                "job.name",
		"k8s.cronjob.name":            "cronjob.name",
		"k8s.statefulset.name":        "statefulset.name",
		"k8s.replicaset.name":         "replicaset.name",
		"k8s.daemonset.name":          "daemonset.name",
		"k8s.container.name":          "container.name",
		"k8s.cluster.name":            "cluster.name",
		"process.parent_pid":          "42",
		"process.executable.name":     "node",
		"client.address":              "12.53.12.1",
		"source.address":              "12.53.12.1",
		"faas.instance":               "arn:aws:lambda:us-east-2:123456789012:function:custom-runtime",
		"faas.trigger":                "api-gateway",
	})
	require.NoError(t, err)

	resourceContainerImageTags := resource.Attributes().PutEmptySlice("container.image.tags")
	err = resourceContainerImageTags.FromRaw([]any{"v3.4.0"})
	require.NoError(t, err)

	scope := pcommon.NewInstrumentationScope()

	record := plog.NewLogRecord()
	err = record.Attributes().FromRaw(map[string]any{
		"event.name":              "user-password-change",
		"http.response.body.size": 1024,
	})
	require.NoError(t, err)
	observedTimestamp := pcommon.Timestamp(1710273641123456789)
	record.SetObservedTimestamp(observedTimestamp)
	logs.MarkReadOnly()

	var buf bytes.Buffer
	encoder, _ := newEncoder(MappingECS)
	err = encoder.encodeLog(encodingContext{
		resource: resource,
		scope:    scope,
	}, record, elasticsearch.Index{}, &buf)
	require.NoError(t, err)

	require.JSONEq(t, `{
		"@timestamp": "2024-03-12T20:00:41.123456789Z",
		"agent": {
		  "name": "custom-agent",
		  "version": "1.2.3"
		},
		"cloud": {
		  "provider": "gcp",
		  "account": {"id": "19347013"},
		  "region": "us-west-1",
		  "availability_zone": "us-west-1b",
		  "service": {"name": "gke"}
		},
		"container": {
		  "name": "happy-seger",
		  "id": "e69cc5d3dda",
		  "image": {
		    "name": "my-app",
		    "tag": ["v3.4.0"]
		  },
		  "runtime": "docker"
		},
		"host": {
		  "hostname": "hostname.example.com",
		  "name": "i-103de39e0a.gke.us-west-1b.cloud.google.com",
		  "id": "i-103de39e0a",
		  "type": "t2.medium",
		  "architecture": "x86_64",
		  "os": {
		    "platform": "darwin",
		    "full": "Mac OS Mojave",
		    "name": "Mac OS X",
		    "version": "10.14.1",
		    "type": "macos"
		  }
		},
		"process": {
		  "pid": 9833,
		  "args": "/usr/bin/ssh -l user 10.0.0.16",
		  "executable": "/usr/bin/ssh",
          "parent": { "pid": "42" },
		  "title": "node"
		},
		"service": {
		  "name": "foo.bar",
		  "environment": "BETA",
		  "version": "1.1.0",
		  "node": {"name": "i-103de39e0a"},
		  "runtime": {
		    "name": "OpenJDK Runtime Environment",
		    "version": "14.0.2"
		  }
		},
		"device": {
		  "id": "00000000-54b3-e7c7-0000-000046bffd97",
		  "model": {
		    "identifier": "SM-G920F",
		    "name": "Samsung Galaxy S6"
		  },
		  "manufacturer": "Samsung"
		},
		"event": {"action": "user-password-change"},
		"http": { "response": {"encoded_body_size": 1024 }} ,
		"kubernetes": {
		  "namespace": "default",
		  "node": {"name": "node-1"},
		  "pod": {
		    "name": "opentelemetry-pod-autoconf",
		    "uid": "275ecb36-5aa8-4c2a-9c47-d8bb681b9aff"
		  },
		  "deployment": {"name": "coredns"},
		  "job": {"name": "job.name"},
		  "cronjob": {"name": "cronjob.name"},
		  "statefulset": {"name": "statefulset.name"},
		  "replicaset": {"name": "replicaset.name"},
		  "daemonset": {"name": "daemonset.name"},
		  "container": {"name": "container.name"}
		},
		"orchestrator": {"cluster": {"name": "cluster.name"}},
        "client": {
			"ip": "12.53.12.1"
		},
        "source": {
			"ip": "12.53.12.1"
		},
        "faas": {
		    "id" : "arn:aws:lambda:us-east-2:123456789012:function:custom-runtime",
		     "trigger": { "type": "api-gateway" }
	  }
	}`, buf.String())
}

func TestEncodeLogECSModeHostOSType(t *testing.T) {
	tests := map[string]struct {
		osType string
		osName string

		expectedHostOsName     string
		expectedHostOsType     string
		expectedHostOsPlatform string
	}{
		"none_set": {
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "", // should not be set
			expectedHostOsPlatform: "", // should not be set
		},
		"type_windows": {
			osType:                 "windows",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "windows",
			expectedHostOsPlatform: "windows",
		},
		"type_linux": {
			osType:                 "linux",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "linux",
			expectedHostOsPlatform: "linux",
		},
		"type_darwin": {
			osType:                 "darwin",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "macos",
			expectedHostOsPlatform: "darwin",
		},
		"type_aix": {
			osType:                 "aix",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "unix",
			expectedHostOsPlatform: "aix",
		},
		"type_hpux": {
			osType:                 "hpux",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "unix",
			expectedHostOsPlatform: "hpux",
		},
		"type_solaris": {
			osType:                 "solaris",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "unix",
			expectedHostOsPlatform: "solaris",
		},
		"type_unknown": {
			osType:                 "unknown",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "", // should not be set
			expectedHostOsPlatform: "unknown",
		},
		"name_android": {
			osName:                 "Android",
			expectedHostOsName:     "Android",
			expectedHostOsType:     "android",
			expectedHostOsPlatform: "", // should not be set
		},
		"name_ios": {
			osName:                 "iOS",
			expectedHostOsName:     "iOS",
			expectedHostOsType:     "ios",
			expectedHostOsPlatform: "", // should not be set
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logs := plog.NewLogs()
			resource := logs.ResourceLogs().AppendEmpty().Resource()
			scope := pcommon.NewInstrumentationScope()
			record := plog.NewLogRecord()

			resource.Attributes().PutStr("agent.name", "custom-agent")
			resource.Attributes().PutStr("agent.version", "1.2.3")
			if test.osType != "" {
				resource.Attributes().PutStr("os.type", test.osType)
			}
			if test.osName != "" {
				resource.Attributes().PutStr("os.name", test.osName)
			}

			timestamp := pcommon.Timestamp(1710373859123456789)
			record.SetTimestamp(timestamp)
			logs.MarkReadOnly()

			var buf bytes.Buffer
			encoder, _ := newEncoder(MappingECS)
			err := encoder.encodeLog(
				encodingContext{resource: resource, scope: scope},
				record, elasticsearch.Index{}, &buf,
			)
			require.NoError(t, err)

			expectedJSON := `{"@timestamp":"2024-03-13T23:50:59.123456789Z","agent":{"name":"custom-agent","version":"1.2.3"}`
			if test.expectedHostOsName != "" ||
				test.expectedHostOsPlatform != "" ||
				test.expectedHostOsType != "" {
				expectedJSON += `, "host":{"os":{`

				first := true
				maybeAdd := func(k, v string) {
					if v != "" {
						if first {
							first = false
						} else {
							expectedJSON += ","
						}
						expectedJSON += fmt.Sprintf("%q:%q", k, v)
					}
				}
				maybeAdd("name", test.expectedHostOsName)
				maybeAdd("type", test.expectedHostOsType)
				maybeAdd("platform", test.expectedHostOsPlatform)
				expectedJSON += "}}"
			}
			expectedJSON += "}"
			require.JSONEq(t, expectedJSON, buf.String())
		})
	}
}

func TestEncodeLogECSModeTimestamps(t *testing.T) {
	tests := map[string]struct {
		timeUnixNano         int64
		observedTimeUnixNano int64
		expectedTimestamp    string
	}{
		"only_observed_set": {
			observedTimeUnixNano: 1710273641123456789,
			expectedTimestamp:    "2024-03-12T20:00:41.123456789Z",
		},
		"both_set": {
			timeUnixNano:         1710273639345678901,
			observedTimeUnixNano: 1710273641123456789,
			expectedTimestamp:    "2024-03-12T20:00:39.345678901Z",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			resource := pcommon.NewResource()
			scope := pcommon.NewInstrumentationScope()
			record := plog.NewLogRecord()

			resource.Attributes().PutStr("agent.name", "custom-agent")
			resource.Attributes().PutStr("agent.version", "1.2.3")

			if test.timeUnixNano > 0 {
				record.SetTimestamp(pcommon.Timestamp(test.timeUnixNano))
			}
			if test.observedTimeUnixNano > 0 {
				record.SetObservedTimestamp(pcommon.Timestamp(test.observedTimeUnixNano))
			}

			var buf bytes.Buffer
			encoder, _ := newEncoder(MappingECS)
			err := encoder.encodeLog(
				encodingContext{resource: resource, scope: scope},
				record, elasticsearch.Index{}, &buf,
			)
			require.NoError(t, err)

			require.JSONEq(t, fmt.Sprintf(
				`{"@timestamp":%q,"agent":{"name":"custom-agent","version":"1.2.3"}}`, test.expectedTimestamp,
			), buf.String())
		})
	}
}

func TestMapLogAttributesToECS(t *testing.T) {
	tests := map[string]struct {
		attrs         func() pcommon.Map
		conversionMap map[string]conversionEntry
		preserveMap   map[string]bool
		expectedDoc   func() objmodel.Document
	}{
		"no_attrs": {
			attrs: pcommon.NewMap,
			conversionMap: map[string]conversionEntry{
				"foo.bar": {to: "baz"},
			},
			expectedDoc: func() objmodel.Document {
				return objmodel.Document{}
			},
		},
		"no_conversion_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				return m
			},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("foo.bar", "baz")
				return d
			},
		},
		"empty_conversion_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				return m
			},
			conversionMap: map[string]conversionEntry{},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("foo.bar", "baz")
				return d
			},
		},
		"all_attrs_in_conversion_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				m.PutInt("qux", 17)
				return m
			},
			conversionMap: map[string]conversionEntry{
				"foo.bar": {to: "bar.qux"},
				"qux":     {to: "foo"},
			},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("bar.qux", "baz")
				d.AddInt("foo", 17)
				return d
			},
		},
		"some_attrs_in_conversion_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				m.PutInt("qux", 17)
				return m
			},
			conversionMap: map[string]conversionEntry{
				"foo.bar": {to: "bar.qux"},
			},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("bar.qux", "baz")
				d.AddInt("qux", 17)
				return d
			},
		},
		"no_attrs_in_conversion_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				m.PutInt("qux", 17)
				return m
			},
			conversionMap: map[string]conversionEntry{
				"baz": {to: "qux"},
			},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("foo.bar", "baz")
				d.AddInt("qux", 17)
				return d
			},
		},
		"extra_keys_in_conversion_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				return m
			},
			conversionMap: map[string]conversionEntry{
				"foo.bar": {to: "bar.qux"},
				"qux":     {to: "foo"},
			},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("bar.qux", "baz")
				return d
			},
		},
		"preserve_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				return m
			},
			conversionMap: map[string]conversionEntry{
				"foo.bar": {to: "bar.qux", preserveOriginal: true},
				"qux":     {to: "foo"},
			},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("bar.qux", "baz")
				d.AddString("foo.bar", "baz")
				return d
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var doc objmodel.Document
			encodeAttributesECSMode(&doc, test.attrs(), test.conversionMap)

			expectedDoc := test.expectedDoc()
			require.Equal(t, expectedDoc, doc)
		})
	}
}

func TestEncodeLogECSModeKnownFieldConflict(t *testing.T) {
	t.Run("protected_field_wins_over_nested_attributes", func(t *testing.T) {
		logs := plog.NewLogs()
		resource := logs.ResourceLogs().AppendEmpty().Resource()
		err := resource.Attributes().FromRaw(map[string]any{
			"service.name":            "test-service",
			"process.executable.path": "/usr/bin/ssh",
		})
		require.NoError(t, err)

		scope := pcommon.NewInstrumentationScope()
		record := plog.NewLogRecord()
		err = record.Attributes().FromRaw(map[string]any{
			"process.executable.name":    "ssh",
			"process.executable.foo":     "bar",
			"process.executable.foo.bar": "baz",
		})
		require.NoError(t, err)

		record.SetObservedTimestamp(pcommon.Timestamp(1710273641123456789))
		logs.MarkReadOnly()

		var buf bytes.Buffer
		encoder, _ := newEncoder(MappingECS)
		err = encoder.encodeLog(
			encodingContext{resource: resource, scope: scope},
			record, elasticsearch.Index{}, &buf,
		)
		require.NoError(t, err)

		// process.executable should be string "/usr/bin/ssh", not an object
		// any other fields under process.executable should be ignored
		output := buf.String()
		assert.Equal(t, "/usr/bin/ssh", gjson.Get(output, "process.executable").String())
	})
}

// JSON serializable structs for OTel test convenience
type oTelRecord struct {
	TraceID                oTelTraceID          `json:"trace_id"`
	SpanID                 oTelSpanID           `json:"span_id"`
	SeverityNumber         int32                `json:"severity_number"`
	SeverityText           string               `json:"severity_text"`
	EventName              string               `json:"event_name"`
	Attributes             map[string]any       `json:"attributes"`
	DroppedAttributesCount uint32               `json:"dropped_attributes_count"`
	Scope                  oTelScope            `json:"scope"`
	Resource               oTelResource         `json:"resource"`
	Datastream             oTelRecordDatastream `json:"data_stream"`
}

type oTelRecordDatastream struct {
	Dataset   string `json:"dataset"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
}

type oTelScope struct {
	Name                   string         `json:"name"`
	Version                string         `json:"version"`
	Attributes             map[string]any `json:"attributes"`
	DroppedAttributesCount uint32         `json:"dropped_attributes_count"`
	SchemaURL              string         `json:"schema_url"`
}

type oTelResource struct {
	Attributes             map[string]any `json:"attributes"`
	DroppedAttributesCount uint32         `json:"dropped_attributes_count"`
	SchemaURL              string         `json:"schema_url"`
}

type oTelSpanID pcommon.SpanID

func (oTelSpanID) MarshalJSON() ([]byte, error) {
	return nil, nil
}

func (o *oTelSpanID) UnmarshalJSON(data []byte) error {
	b, err := decodeOTelID(data)
	if err != nil {
		return err
	}
	copy(o[:], b)
	return nil
}

type oTelTraceID pcommon.TraceID

func (oTelTraceID) MarshalJSON() ([]byte, error) {
	return nil, nil
}

func (o *oTelTraceID) UnmarshalJSON(data []byte) error {
	b, err := decodeOTelID(data)
	if err != nil {
		return err
	}
	copy(o[:], b)
	return nil
}

func decodeOTelID(data []byte) ([]byte, error) {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	return hex.DecodeString(s)
}

func TestEncodeLogOtelMode(t *testing.T) {
	randomString := strings.Repeat("abcdefghijklmnopqrstuvwxyz0123456789", 10)
	maxLenNamespace := maxDataStreamBytes - len(disallowedNamespaceRunes)
	maxLenDataset := maxDataStreamBytes - len(disallowedDatasetRunes) - len(".otel")

	tests := []struct {
		name   string
		rec    oTelRecord
		wantFn func(oTelRecord) oTelRecord // Allows each test to customized the expectations from the original test record data
	}{
		{
			name: "default", // Expecting default data_stream values
			rec:  buildOTelRecordTestData(t, nil),
			wantFn: func(or oTelRecord) oTelRecord {
				return assignDatastreamData(or)
			},
		},
		{
			name: "custom dataset",
			rec: buildOTelRecordTestData(t, func(or oTelRecord) oTelRecord {
				or.Attributes["data_stream.dataset"] = "custom"
				return or
			}),
			wantFn: func(or oTelRecord) oTelRecord {
				// Datastream attributes are expected to be deleted from under the attributes
				deleteDatasetAttributes(or)
				return assignDatastreamData(or, "", "custom.otel")
			},
		},
		{
			name: "custom dataset with otel suffix",
			rec: buildOTelRecordTestData(t, func(or oTelRecord) oTelRecord {
				or.Attributes["data_stream.dataset"] = "custom.otel"
				return or
			}),
			wantFn: func(or oTelRecord) oTelRecord {
				deleteDatasetAttributes(or)
				return assignDatastreamData(or, "", "custom.otel.otel")
			},
		},
		{
			name: "custom dataset/namespace",
			rec: buildOTelRecordTestData(t, func(or oTelRecord) oTelRecord {
				or.Attributes["data_stream.dataset"] = "customds"
				or.Attributes["data_stream.namespace"] = "customns"
				return or
			}),
			wantFn: func(or oTelRecord) oTelRecord {
				deleteDatasetAttributes(or)
				return assignDatastreamData(or, "", "customds.otel", "customns")
			},
		},
		{
			name: "dataset attributes priority",
			rec: buildOTelRecordTestData(t, func(or oTelRecord) oTelRecord {
				or.Attributes["data_stream.dataset"] = "first"
				or.Scope.Attributes["data_stream.dataset"] = "second"
				or.Resource.Attributes["data_stream.dataset"] = "third"
				return or
			}),
			wantFn: func(or oTelRecord) oTelRecord {
				deleteDatasetAttributes(or)
				return assignDatastreamData(or, "", "first.otel")
			},
		},
		{
			name: "dataset scope attribute priority",
			rec: buildOTelRecordTestData(t, func(or oTelRecord) oTelRecord {
				or.Scope.Attributes["data_stream.dataset"] = "second"
				or.Resource.Attributes["data_stream.dataset"] = "third"
				return or
			}),
			wantFn: func(or oTelRecord) oTelRecord {
				deleteDatasetAttributes(or)
				return assignDatastreamData(or, "", "second.otel")
			},
		},
		{
			name: "dataset resource attribute priority",
			rec: buildOTelRecordTestData(t, func(or oTelRecord) oTelRecord {
				or.Resource.Attributes["data_stream.dataset"] = "third"
				return or
			}),
			wantFn: func(or oTelRecord) oTelRecord {
				deleteDatasetAttributes(or)
				return assignDatastreamData(or, "", "third.otel")
			},
		},
		{
			name: "sanitize dataset/namespace",
			rec: buildOTelRecordTestData(t, func(or oTelRecord) oTelRecord {
				or.Attributes["data_stream.dataset"] = disallowedDatasetRunes + randomString
				or.Attributes["data_stream.namespace"] = disallowedNamespaceRunes + randomString
				return or
			}),
			wantFn: func(or oTelRecord) oTelRecord {
				deleteDatasetAttributes(or)
				ds := strings.Repeat("_", len(disallowedDatasetRunes)) + randomString[:maxLenDataset] + ".otel"
				ns := strings.Repeat("_", len(disallowedNamespaceRunes)) + randomString[:maxLenNamespace]
				return assignDatastreamData(or, "", ds, ns)
			},
		},
		{
			name: "event_name from attributes.event.name",
			rec: buildOTelRecordTestData(t, func(or oTelRecord) oTelRecord {
				or.Attributes["event.name"] = "foo"
				or.EventName = ""
				return or
			}),
			wantFn: func(or oTelRecord) oTelRecord {
				or.EventName = "foo"
				return assignDatastreamData(or)
			},
		},
		{
			name: "event_name takes precedent over attributes.event.name",
			rec: buildOTelRecordTestData(t, func(or oTelRecord) oTelRecord {
				or.Attributes["event.name"] = "foo"
				or.EventName = "bar"
				return or
			}),
			wantFn: func(or oTelRecord) oTelRecord {
				or.EventName = "bar"
				return assignDatastreamData(or)
			},
		},
	}

	encoder, _ := newEncoder(MappingOTel)

	for _, tc := range tests {
		record, scope, resource := createTestOTelLogRecord(t, tc.rec)
		router := newDocumentRouter(MappingOTel, "", &Config{})

		idx, err := router.routeLogRecord(resource, scope, record.Attributes())
		require.NoError(t, err)

		var buf bytes.Buffer
		err = encoder.encodeLog(
			encodingContext{
				resource:          resource,
				resourceSchemaURL: tc.rec.Resource.SchemaURL,
				scope:             scope,
				scopeSchemaURL:    tc.rec.Scope.SchemaURL,
			},
			record, idx, &buf,
		)
		require.NoError(t, err)

		want := tc.rec
		if tc.wantFn != nil {
			want = tc.wantFn(want)
		}

		var got oTelRecord
		err = json.Unmarshal(buf.Bytes(), &got)

		require.NoError(t, err)

		assert.Equal(t, want, got)
	}
}

// helper function that creates the OTel LogRecord from the test structure
func createTestOTelLogRecord(t *testing.T, rec oTelRecord) (plog.LogRecord, pcommon.InstrumentationScope, pcommon.Resource) {
	record := plog.NewLogRecord()

	record.SetTraceID(pcommon.TraceID(rec.TraceID))
	record.SetSpanID(pcommon.SpanID(rec.SpanID))
	record.SetSeverityNumber(plog.SeverityNumber(rec.SeverityNumber))
	record.SetSeverityText(rec.SeverityText)
	record.SetDroppedAttributesCount(rec.DroppedAttributesCount)
	record.SetEventName(rec.EventName)

	err := record.Attributes().FromRaw(rec.Attributes)
	require.NoError(t, err)

	scope := pcommon.NewInstrumentationScope()
	scope.SetName(rec.Scope.Name)
	scope.SetVersion(rec.Scope.Version)
	scope.SetDroppedAttributesCount(rec.Scope.DroppedAttributesCount)
	err = scope.Attributes().FromRaw(rec.Scope.Attributes)
	require.NoError(t, err)

	resource := pcommon.NewResource()
	resource.SetDroppedAttributesCount(rec.Resource.DroppedAttributesCount)
	err = resource.Attributes().FromRaw(rec.Resource.Attributes)
	require.NoError(t, err)

	return record, scope, resource
}

func buildOTelRecordTestData(t *testing.T, fn func(oTelRecord) oTelRecord) oTelRecord {
	s := `{
    "@timestamp": "2024-03-12T20:00:41.123456780Z",
    "attributes": {
        "event.name": "user-password-change",
        "foo.some": "bar"
    },
    "event_name": "user-password-change",
    "dropped_attributes_count": 1,
    "observed_timestamp": "2024-03-12T20:00:41.123456789Z",
    "resource": {
        "attributes": {
            "host.name": "lebuntu",
            "host.os.type": "linux"
        },
        "dropped_attributes_count": 2,
        "schema_url": "https://opentelemetry.io/schemas/1.6.0"
    },
    "scope": {
        "attributes": {
            "attr.num": 1234,
            "attr.str": "val1"
        },
        "dropped_attributes_count": 2,
        "name": "foobar",
        "schema_url": "https://opentelemetry.io/schemas/1.6.1",
        "version": "42"
    },
    "severity_number": 17,
    "severity_text": "ERROR",
    "span_id": "0102030405060708",
    "trace_id": "01020304050607080900010203040506"
}`

	var record oTelRecord
	err := json.Unmarshal([]byte(s), &record)
	assert.NoError(t, err)
	if fn != nil {
		record = fn(record)
	}
	return record
}

func deleteDatasetAttributes(or oTelRecord) {
	deleteDatasetAttributesFromMap(or.Attributes)
	deleteDatasetAttributesFromMap(or.Scope.Attributes)
	deleteDatasetAttributesFromMap(or.Resource.Attributes)
}

func deleteDatasetAttributesFromMap(m map[string]any) {
	delete(m, "data_stream.dataset")
	delete(m, "data_stream.namespace")
	delete(m, "data_stream.type")
}

func assignDatastreamData(or oTelRecord, a ...string) oTelRecord {
	r := oTelRecordDatastream{
		Dataset:   "generic.otel",
		Namespace: "default",
		Type:      "logs",
	}

	if len(a) > 0 && a[0] != "" {
		r.Type = a[0]
	}
	if len(a) > 1 && a[1] != "" {
		r.Dataset = a[1]
	}
	if len(a) > 2 && a[2] != "" {
		r.Namespace = a[2]
	}

	or.Datastream = r

	return or
}

func TestEncodeLogScalarObjectConflict(t *testing.T) {
	// If there is an attribute named "foo", and another called "foo.bar",
	// then "foo" will be renamed to "foo.value".
	encoder, _ := newEncoder(MappingNone)
	td := mockResourceLogs()
	td.ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("foo", "scalar")
	td.ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("foo.bar", "baz")
	var buf bytes.Buffer
	err := encoder.encodeLog(
		encodingContext{resource: td.Resource(), scope: td.ScopeLogs().At(0).Scope()},
		td.ScopeLogs().At(0).LogRecords().At(0), elasticsearch.Index{}, &buf,
	)
	assert.NoError(t, err)

	encoded := buf.Bytes()
	assert.True(t, gjson.ValidBytes(encoded))
	assert.False(t, gjson.GetBytes(encoded, "Attributes\\.foo").Exists())
	fooValue := gjson.GetBytes(encoded, "Attributes\\.foo\\.value")
	fooBar := gjson.GetBytes(encoded, "Attributes\\.foo\\.bar")
	assert.Equal(t, "scalar", fooValue.Str)
	assert.Equal(t, "baz", fooBar.Str)

	// If there is an attribute named "foo.value", then "foo" would be omitted rather than renamed.
	td.ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("foo.value", "foovalue")
	buf = bytes.Buffer{}
	err = encoder.encodeLog(
		encodingContext{resource: td.Resource(), scope: td.ScopeLogs().At(0).Scope()},
		td.ScopeLogs().At(0).LogRecords().At(0), elasticsearch.Index{}, &buf,
	)
	assert.NoError(t, err)

	encoded = buf.Bytes()
	assert.False(t, gjson.GetBytes(encoded, "Attributes\\.foo").Exists())
	fooValue = gjson.GetBytes(encoded, "Attributes\\.foo\\.value")
	assert.Equal(t, "foovalue", fooValue.Str)
}

func TestEncodeLogBodyMapMode(t *testing.T) {
	// craft a log record with a body map
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	logRecords := scopeLogs.LogRecords()
	observedTimestamp := pcommon.Timestamp(time.Now().UnixNano())

	logRecord := logRecords.AppendEmpty()
	logRecord.SetObservedTimestamp(observedTimestamp)

	bodyMap := pcommon.NewMap()
	bodyMap.PutStr("@timestamp", "2024-03-12T20:00:41.123456789Z")
	bodyMap.PutInt("id", 1)
	bodyMap.PutStr("key", "value")
	bodyMap.PutStr("key.a", "a")
	bodyMap.PutStr("key.a.b", "b")
	bodyMap.PutDouble("pi", 3.14)
	bodyMap.CopyTo(logRecord.Body().SetEmptyMap())

	encoder, _ := newEncoder(MappingBodyMap)
	var buf bytes.Buffer
	err := encoder.encodeLog(
		encodingContext{resource: resourceLogs.Resource(), scope: scopeLogs.Scope()},
		logRecord, elasticsearch.Index{}, &buf,
	)
	require.NoError(t, err)

	require.JSONEq(t, `{
		"@timestamp":                 "2024-03-12T20:00:41.123456789Z",
		"id":                         1,
		"key":                        "value",
		"key.a":                      "a",
		"key.a.b":                    "b",
		"pi":                         3.14
	}`, buf.String())

	// invalid body map
	logRecord.Body().SetEmptySlice()
	err = encoder.encodeLog(
		encodingContext{resource: resourceLogs.Resource(), scope: scopeLogs.Scope()},
		logRecord, elasticsearch.Index{}, &buf,
	)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidTypeForBodyMapMode)
}
