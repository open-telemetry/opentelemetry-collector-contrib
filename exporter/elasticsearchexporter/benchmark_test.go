// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

const benchECSBatch = 32

func BenchmarkEncodeECS(b *testing.B) {
	// Measures ECS encode time and allocs for logs and spans. Cases cover a
	// small resource (2 attributes) and a large resource. A new encoding
	// context is created every benchECSBatch records, matching one
	// Resource+Scope batch in the exporter.
	encoder, err := newEncoder(MappingECS)
	require.NoError(b, err)
	scope := pcommon.NewInstrumentationScope()
	scope.Attributes().PutStr("otel.scope.name", "go.opentelemetry.io/contrib/instrumentation/net/http")
	scope.Attributes().PutStr("otel.scope.version", "0.49.0")
	logs := benchECSLogRecords(benchECSBatch)
	spans := benchECSSpans(benchECSBatch)
	logIdx := elasticsearch.NewDataStreamIndex("logs", "app", "default")
	traceIdx := elasticsearch.NewDataStreamIndex("traces", "app", "default")

	resources := []struct {
		name     string
		resource pcommon.Resource
	}{
		{
			name:     "small",
			resource: benchECSSmallResource(),
		},
		{
			name:     "large",
			resource: ecsModeResource(b),
		},
	}
	encodes := []struct {
		name   string
		encode func(ec encodingContext, i int, buf *bytes.Buffer) error
	}{
		{
			name: "log",
			encode: func(ec encodingContext, i int, buf *bytes.Buffer) error {
				return encoder.encodeLog(ec, logs[i], logIdx, buf)
			},
		},
		{
			name: "span",
			encode: func(ec encodingContext, i int, buf *bytes.Buffer) error {
				return encoder.encodeSpan(ec, spans[i], traceIdx, buf)
			},
		},
	}

	for _, tc := range encodes {
		for _, rc := range resources {
			b.Run(tc.name+"/"+rc.name, func(b *testing.B) {
				ec := benchECSEncodingContext(rc.resource, scope)
				var buf bytes.Buffer
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if i%benchECSBatch == 0 {
						ec = benchECSEncodingContext(rc.resource, scope)
					}
					buf.Reset()
					if err := tc.encode(ec, i%benchECSBatch, &buf); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func benchECSEncodingContext(resource pcommon.Resource, scope pcommon.InstrumentationScope) encodingContext {
	return encodingContext{
		resource: resource,
		scope:    scope,
		ecsDoc:   newECSDocument(MappingECS),
	}
}

func benchECSSmallResource() pcommon.Resource {
	r := pcommon.NewResource()
	r.Attributes().PutStr("service.name", "api")
	r.Attributes().PutStr("deployment.environment", "prod")
	return r
}

func benchECSLogRecords(n int) []plog.LogRecord {
	records := make([]plog.LogRecord, n)
	for i := range records {
		rec := plog.NewLogRecord()
		rec.SetTimestamp(1710273639345678901)
		rec.SetObservedTimestamp(1710273641123456789)
		rec.SetSeverityText("INFO")
		rec.SetSeverityNumber(plog.SeverityNumberInfo)
		rec.Body().SetStr("request completed")
		rec.Attributes().PutStr("event.name", "http.request")
		rec.Attributes().PutStr("http.request.method", "GET")
		rec.Attributes().PutInt("http.response.status_code", 200)
		rec.Attributes().PutStr("url.path", "/v1/users")
		rec.Attributes().PutInt("http.response.body.size", int64(512+i))
		records[i] = rec
	}
	return records
}

func benchECSSpans(n int) []ptrace.Span {
	spans := make([]ptrace.Span, n)
	var tid pcommon.TraceID
	tid[0] = 1
	for i := range spans {
		sp := ptrace.NewSpan()
		sp.SetTraceID(tid)
		var sid pcommon.SpanID
		sid[0] = byte(i + 1)
		sp.SetSpanID(sid)
		sp.SetName("GET /v1/users")
		sp.SetKind(ptrace.SpanKindServer)
		sp.SetStartTimestamp(1710273639345678901)
		sp.SetEndTimestamp(1710273641123456789)
		sp.Status().SetCode(ptrace.StatusCodeOk)
		sp.Attributes().PutStr("http.request.method", "GET")
		sp.Attributes().PutInt("http.response.status_code", 200)
		spans[i] = sp
	}
	return spans
}
