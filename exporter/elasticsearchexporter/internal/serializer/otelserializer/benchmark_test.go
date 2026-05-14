// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

func BenchmarkSerializeLog(b *testing.B) {
	for _, bb := range []struct {
		name          string
		logCustomizer func(resource pcommon.Resource, scope pcommon.InstrumentationScope, record plog.LogRecord)
	}{
		{
			name: "minimal",
			logCustomizer: func(_ pcommon.Resource, _ pcommon.InstrumentationScope, record plog.LogRecord) {
				record.SetTimestamp(1721314113467654123)
				record.SetObservedTimestamp(1721314113467654123)
				record.Body().SetStr("simple log message")
			},
		},
		{
			name: "with attributes",
			logCustomizer: func(resource pcommon.Resource, scope pcommon.InstrumentationScope, record plog.LogRecord) {
				record.SetTimestamp(1721314113467654123)
				record.SetObservedTimestamp(1721314113467654123)
				record.SetSeverityText("ERROR")
				record.SetSeverityNumber(plog.SeverityNumberError)
				record.Body().SetStr("an error occurred during processing")
				record.SetEventName("error.event")

				record.Attributes().PutStr("service.name", "my-service")
				record.Attributes().PutStr("host.name", "prod-server-01")
				record.Attributes().PutInt("http.status_code", 500)
				record.Attributes().PutStr("http.method", "POST")
				record.Attributes().PutStr("http.url", "https://api.example.com/v1/users")
				record.Attributes().PutBool("error", true)
				record.Attributes().PutDouble("request.duration_ms", 1234.56)

				resource.Attributes().PutStr("service.name", "my-service")
				resource.Attributes().PutStr("service.version", "1.2.3")
				resource.Attributes().PutStr("deployment.environment", "production")

				scope.Attributes().PutStr("otel.library.name", "my-library")
				scope.Attributes().PutStr("otel.library.version", "0.1.0")
			},
		},
		{
			name: "with map body",
			logCustomizer: func(resource pcommon.Resource, scope pcommon.InstrumentationScope, record plog.LogRecord) {
				record.SetTimestamp(1721314113467654123)
				record.SetObservedTimestamp(1721314113467654123)
				record.SetSeverityText("INFO")
				record.SetSeverityNumber(plog.SeverityNumberInfo)

				body := record.Body().SetEmptyMap()
				body.PutStr("message", "user logged in")
				body.PutStr("user_id", "user-12345")
				body.PutStr("ip_address", "192.168.1.1")
				body.PutInt("attempt", 3)

				resource.Attributes().PutStr("service.name", "auth-service")
				scope.Attributes().PutStr("otel.library.name", "auth-lib")
			},
		},
		{
			name: "with data stream index",
			logCustomizer: func(resource pcommon.Resource, _ pcommon.InstrumentationScope, record plog.LogRecord) {
				record.SetTimestamp(1721314113467654123)
				record.SetObservedTimestamp(1721314113467654123)
				record.Body().SetStr("log message with data stream")
				record.Attributes().PutStr("service.name", "my-service")
				resource.Attributes().PutStr("service.name", "my-service")
			},
		},
	} {
		b.Run(bb.name, func(b *testing.B) {
			logs := plog.NewLogs()
			resourceLogs := logs.ResourceLogs().AppendEmpty()
			scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
			record := scopeLogs.LogRecords().AppendEmpty()
			bb.logCustomizer(resourceLogs.Resource(), scopeLogs.Scope(), record)
			logs.MarkReadOnly()

			ser, err := New()
			require.NoError(b, err)

			idx := elasticsearch.Index{}
			if bb.name == "with data stream index" {
				idx = elasticsearch.NewDataStreamIndex("logs", "generic", "default")
			}

			var buf bytes.Buffer
			buf.Grow(1024)

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				buf.Reset()
				_ = ser.SerializeLog(resourceLogs.Resource(), "", scopeLogs.Scope(), "", record, idx, &buf)
			}
		})
	}
}

func BenchmarkSerializeMetrics(b *testing.B) {
	for _, bb := range []struct {
		name         string
		buildMetrics func() (pmetric.Metrics, []datapoints.DataPoint, elasticsearch.Index)
	}{
		{
			name: "single gauge",
			buildMetrics: func() (pmetric.Metrics, []datapoints.DataPoint, elasticsearch.Index) {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("service.name", "my-service")
				sm := rm.ScopeMetrics().AppendEmpty()
				m := sm.Metrics().AppendEmpty()
				m.SetName("system.cpu.utilization")
				m.SetUnit("%")
				dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
				dp.SetDoubleValue(0.75)
				dp.SetTimestamp(1721314113467654123)
				dp.Attributes().PutStr("cpu", "cpu0")
				dp.Attributes().PutStr("state", "user")
				return metrics, []datapoints.DataPoint{datapoints.NewNumber(m, dp)}, elasticsearch.Index{}
			},
		},
		{
			name: "multiple gauges",
			buildMetrics: func() (pmetric.Metrics, []datapoints.DataPoint, elasticsearch.Index) {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("service.name", "my-service")
				sm := rm.ScopeMetrics().AppendEmpty()

				var dps []datapoints.DataPoint
				for _, name := range []string{
					"system.cpu.utilization",
					"system.memory.usage",
					"system.disk.io",
					"system.network.io",
				} {
					m := sm.Metrics().AppendEmpty()
					m.SetName(name)
					m.SetUnit("1")
					dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
					dp.SetDoubleValue(0.75)
					dp.SetTimestamp(1721314113467654123)
					dps = append(dps, datapoints.NewNumber(m, dp))
				}
				return metrics, dps, elasticsearch.Index{}
			},
		},
		{
			name: "histogram",
			buildMetrics: func() (pmetric.Metrics, []datapoints.DataPoint, elasticsearch.Index) {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("service.name", "my-service")
				sm := rm.ScopeMetrics().AppendEmpty()
				m := sm.Metrics().AppendEmpty()
				m.SetName("http.server.request.duration")
				m.SetUnit("s")
				dp := m.SetEmptyHistogram().DataPoints().AppendEmpty()
				dp.SetTimestamp(1721314113467654123)
				dp.SetStartTimestamp(1721314113000000000)
				dp.SetCount(100)
				dp.SetSum(25.5)
				dp.SetMin(0.001)
				dp.SetMax(2.5)
				dp.BucketCounts().FromRaw([]uint64{10, 20, 30, 25, 10, 5})
				dp.ExplicitBounds().FromRaw([]float64{0.005, 0.01, 0.025, 0.05, 0.1})
				dp.Attributes().PutStr("http.method", "GET")
				dp.Attributes().PutInt("http.status_code", 200)
				return metrics, []datapoints.DataPoint{datapoints.NewHistogram(m, dp)}, elasticsearch.Index{}
			},
		},
		{
			name: "summary",
			buildMetrics: func() (pmetric.Metrics, []datapoints.DataPoint, elasticsearch.Index) {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("service.name", "my-service")
				sm := rm.ScopeMetrics().AppendEmpty()
				m := sm.Metrics().AppendEmpty()
				m.SetName("rpc.server.duration")
				m.SetUnit("ms")
				dp := m.SetEmptySummary().DataPoints().AppendEmpty()
				dp.SetTimestamp(1721314113467654123)
				dp.SetStartTimestamp(1721314113000000000)
				dp.SetCount(100)
				dp.SetSum(25500)
				qv := dp.QuantileValues().AppendEmpty()
				qv.SetQuantile(0.5)
				qv.SetValue(200)
				qv = dp.QuantileValues().AppendEmpty()
				qv.SetQuantile(0.99)
				qv.SetValue(500)
				return metrics, []datapoints.DataPoint{datapoints.NewSummary(m, dp)}, elasticsearch.Index{}
			},
		},
		{
			name: "exponential histogram",
			buildMetrics: func() (pmetric.Metrics, []datapoints.DataPoint, elasticsearch.Index) {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("service.name", "my-service")
				sm := rm.ScopeMetrics().AppendEmpty()
				m := sm.Metrics().AppendEmpty()
				m.SetName("http.server.request.duration")
				m.SetUnit("s")
				dp := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
				dp.SetTimestamp(1721314113467654123)
				dp.SetStartTimestamp(1721314113000000000)
				dp.SetCount(100)
				dp.SetSum(25.5)
				dp.SetMin(0.001)
				dp.SetMax(2.5)
				dp.SetScale(5)
				dp.SetZeroCount(2)
				dp.Positive().SetOffset(1)
				dp.Positive().BucketCounts().FromRaw([]uint64{1, 2, 3, 4, 5, 6, 7, 8})
				dp.Negative().SetOffset(1)
				dp.Negative().BucketCounts().FromRaw([]uint64{1, 1, 1, 1})
				return metrics, []datapoints.DataPoint{datapoints.NewExponentialHistogram(m, dp)}, elasticsearch.Index{}
			},
		},
	} {
		b.Run(bb.name, func(b *testing.B) {
			metrics, dps, idx := bb.buildMetrics()
			rm := metrics.ResourceMetrics().At(0)
			sm := rm.ScopeMetrics().At(0)
			metrics.MarkReadOnly()

			ser, err := New()
			require.NoError(b, err)

			var buf bytes.Buffer
			buf.Grow(1024)
			var validationErrors []error

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				buf.Reset()
				validationErrors = validationErrors[:0]
				_, _ = ser.SerializeMetrics(rm.Resource(), "", sm.Scope(), "", dps, &validationErrors, idx, &buf)
			}
		})
	}
}

func BenchmarkSerializeSpan(b *testing.B) {
	for _, bb := range []struct {
		name           string
		spanCustomizer func(resource pcommon.Resource, scope pcommon.InstrumentationScope, span ptrace.Span)
	}{
		{
			name: "minimal",
			spanCustomizer: func(_ pcommon.Resource, _ pcommon.InstrumentationScope, span ptrace.Span) {
				span.SetStartTimestamp(1721314113467654123)
				span.SetEndTimestamp(1721314113469654123)
				span.SetKind(ptrace.SpanKindServer)
				span.SetName("GET /api/users")
			},
		},
		{
			name: "with attributes and status",
			spanCustomizer: func(resource pcommon.Resource, scope pcommon.InstrumentationScope, span ptrace.Span) {
				span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
				span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
				span.SetParentSpanID([8]byte{8, 7, 6, 5, 4, 3, 2, 1})
				span.SetStartTimestamp(1721314113467654123)
				span.SetEndTimestamp(1721314113469654123)
				span.SetKind(ptrace.SpanKindServer)
				span.SetName("GET /api/users")
				span.Status().SetCode(ptrace.StatusCodeOk)
				span.Status().SetMessage("success")

				span.Attributes().PutStr("http.method", "GET")
				span.Attributes().PutStr("http.url", "https://api.example.com/v1/users")
				span.Attributes().PutInt("http.status_code", 200)
				span.Attributes().PutStr("net.peer.name", "api.example.com")
				span.Attributes().PutInt("net.peer.port", 443)
				span.Attributes().PutStr("http.user_agent", "Mozilla/5.0")
				span.Attributes().PutDouble("http.request_content_length", 1024.0)

				resource.Attributes().PutStr("service.name", "api-gateway")
				resource.Attributes().PutStr("service.version", "2.1.0")
				resource.Attributes().PutStr("deployment.environment", "production")
				resource.Attributes().PutStr("host.name", "prod-server-01")

				scope.Attributes().PutStr("otel.library.name", "http-instrumentation")
				scope.Attributes().PutStr("otel.library.version", "1.0.0")
			},
		},
		{
			name: "with links",
			spanCustomizer: func(resource pcommon.Resource, _ pcommon.InstrumentationScope, span ptrace.Span) {
				span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
				span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
				span.SetStartTimestamp(1721314113467654123)
				span.SetEndTimestamp(1721314113469654123)
				span.SetKind(ptrace.SpanKindInternal)
				span.SetName("process_batch")

				for i := range 3 {
					link := span.Links().AppendEmpty()
					link.SetTraceID([16]byte{byte(i + 1), 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
					link.SetSpanID([8]byte{byte(i + 1), 2, 3, 4, 5, 6, 7, 8})
					link.Attributes().PutStr("link.type", "child")
				}

				resource.Attributes().PutStr("service.name", "batch-processor")
			},
		},
	} {
		b.Run(bb.name, func(b *testing.B) {
			traces := ptrace.NewTraces()
			resourceSpans := traces.ResourceSpans().AppendEmpty()
			scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
			span := scopeSpans.Spans().AppendEmpty()
			bb.spanCustomizer(resourceSpans.Resource(), scopeSpans.Scope(), span)
			traces.MarkReadOnly()

			ser, err := New()
			require.NoError(b, err)
			idx := elasticsearch.Index{}

			var buf bytes.Buffer
			buf.Grow(1024)

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				buf.Reset()
				_ = ser.SerializeSpan(resourceSpans.Resource(), "", scopeSpans.Scope(), "", span, idx, &buf)
			}
		})
	}
}

func BenchmarkSerializeSpanEvent(b *testing.B) {
	for _, bb := range []struct {
		name           string
		spanCustomizer func(resource pcommon.Resource, scope pcommon.InstrumentationScope, span ptrace.Span, event ptrace.SpanEvent)
	}{
		{
			name: "minimal",
			spanCustomizer: func(_ pcommon.Resource, _ pcommon.InstrumentationScope, span ptrace.Span, event ptrace.SpanEvent) {
				span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
				span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
				event.SetTimestamp(1721314113467654123)
				event.SetName("exception")
			},
		},
		{
			name: "with attributes",
			spanCustomizer: func(resource pcommon.Resource, scope pcommon.InstrumentationScope, span ptrace.Span, event ptrace.SpanEvent) {
				span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
				span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
				event.SetTimestamp(1721314113467654123)
				event.SetName("exception")
				event.Attributes().PutStr("exception.type", "java.lang.NullPointerException")
				event.Attributes().PutStr("exception.message", "null pointer at line 42")
				event.Attributes().PutStr("exception.stacktrace", "java.lang.NullPointerException\n\tat com.example.MyClass.myMethod(MyClass.java:42)")

				resource.Attributes().PutStr("service.name", "api-gateway")
				resource.Attributes().PutStr("service.version", "2.1.0")

				scope.Attributes().PutStr("otel.library.name", "exception-handler")
			},
		},
	} {
		b.Run(bb.name, func(b *testing.B) {
			traces := ptrace.NewTraces()
			resourceSpans := traces.ResourceSpans().AppendEmpty()
			scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
			span := scopeSpans.Spans().AppendEmpty()
			event := span.Events().AppendEmpty()
			bb.spanCustomizer(resourceSpans.Resource(), scopeSpans.Scope(), span, event)
			traces.MarkReadOnly()

			ser, err := New()
			require.NoError(b, err)
			idx := elasticsearch.Index{}

			var buf bytes.Buffer
			buf.Grow(1024)

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				buf.Reset()
				ser.SerializeSpanEvent(resourceSpans.Resource(), "", scopeSpans.Scope(), "", span, event, idx, &buf)
			}
		})
	}
}
