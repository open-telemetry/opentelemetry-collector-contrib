// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/mapping"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
)

var expectedSpanBody = `{"@timestamp":"2023-04-19T03:04:05.000000006Z","Attributes.service.instance.id":"23","Duration":1000000,"EndTimestamp":"2023-04-19T03:04:06.000000006Z","Events.fooEvent.evnetMockBar":"bar","Events.fooEvent.evnetMockFoo":"foo","Events.fooEvent.time":"2023-04-19T03:04:05.000000006Z","Kind":"SPAN_KIND_CLIENT","Link":"[{\"attribute\":{},\"spanID\":\"\",\"traceID\":\"01020304050607080807060504030200\"}]","Name":"client span","Resource.cloud.platform":"aws_elastic_beanstalk","Resource.cloud.provider":"aws","Resource.deployment.environment":"BETA","Resource.service.instance.id":"23","Resource.service.name":"some-service","Resource.service.version":"env-version-1234","Scope.lib-foo":"lib-bar","Scope.name":"io.opentelemetry.rabbitmq-2.7","Scope.version":"1.30.0-alpha","SpanId":"1920212223242526","TraceId":"01020304050607080807060504030201","TraceStatus":2,"TraceStatusDescription":"Test"}`

var expectedMetricsEncoded = `{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"idle","system":{"cpu":{"time":440.23}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"interrupt","system":{"cpu":{"time":0.0}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"nice","system":{"cpu":{"time":0.14}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"softirq","system":{"cpu":{"time":0.77}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"steal","system":{"cpu":{"time":0.0}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"system","system":{"cpu":{"time":24.8}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"user","system":{"cpu":{"time":64.78}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu0","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"wait","system":{"cpu":{"time":1.65}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"idle","system":{"cpu":{"time":475.69}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"interrupt","system":{"cpu":{"time":0.0}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"nice","system":{"cpu":{"time":0.1}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"softirq","system":{"cpu":{"time":0.57}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"steal","system":{"cpu":{"time":0.0}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"system","system":{"cpu":{"time":15.88}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"user","system":{"cpu":{"time":50.09}}}
{"@timestamp":"2024-06-12T10:20:16.419290690Z","cpu":"cpu1","host":{"hostname":"my-host","name":"my-host","os":{"platform":"linux"}},"state":"wait","system":{"cpu":{"time":0.95}}}`

func TestEncodeSpan(t *testing.T) {
	model := &encodeModel{dedot: false}
	td := mockResourceSpans()
	spanByte, err := model.encodeSpan(td.ResourceSpans().At(0).Resource(), "", td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0), td.ResourceSpans().At(0).ScopeSpans().At(0).Scope(), "")
	assert.NoError(t, err)
	assert.Equal(t, expectedSpanBody, string(spanByte))
}

func TestEncodeMetric(t *testing.T) {
	// Prepare metrics to test.
	metrics := createTestMetrics(t)

	// Encode the metrics.
	model := &encodeModel{
		dedot: true,
		mode:  mapping.ModeECS,
	}

	docs := make(map[uint32]objmodel.Document)

	var docsBytes [][]byte
	for i := 0; i < metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().Len(); i++ {
		err := model.upsertMetricDataPointValue(
			docs,
			metrics.ResourceMetrics().At(0).Resource(),
			"",
			metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope(),
			"",
			metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0),
			newNumberDataPoint(metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(i)),
		)
		require.NoError(t, err)
	}

	for _, doc := range docs {
		bytes, err := model.encodeDocument(doc)
		require.NoError(t, err)
		docsBytes = append(docsBytes, bytes)
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

	resourceSpans.Resource().Attributes().PutStr(semconv.AttributeServiceName, "some-service")

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
	event.Attributes().PutStr("evnetMockFoo", "foo")
	event.Attributes().PutStr("evnetMockBar", "bar")
	return traces
}

func TestEncodeEvents(t *testing.T) {
	t.Parallel()

	events := ptrace.NewSpanEventSlice()
	events.EnsureCapacity(4)
	for i := 0; i < 4; i++ {
		event := events.AppendEmpty()
		event.SetTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Duration(i) * time.Minute)))
		event.SetName(fmt.Sprintf("event_%d", i))
	}

	tests := map[string]struct {
		mappingMode mapping.Mode
		want        func() objmodel.Document
	}{
		"raw": {
			mappingMode: mapping.ModeRaw,
			want: func() objmodel.Document {
				doc := objmodel.Document{}
				doc.AddEvents("", events)
				return doc
			},
		},
		"none": {
			mappingMode: mapping.ModeNone,
			want: func() objmodel.Document {
				doc := objmodel.Document{}
				doc.AddEvents("Events", events)
				return doc
			},
		},
		"ecs": {
			mappingMode: mapping.ModeECS,
			want: func() objmodel.Document {
				doc := objmodel.Document{}
				doc.AddEvents("Events", events)
				return doc
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			m := encodeModel{
				mode: test.mappingMode,
			}

			doc := objmodel.Document{}
			m.encodeEvents(&doc, events)
			require.Equal(t, test.want(), doc)
		})
	}
}
