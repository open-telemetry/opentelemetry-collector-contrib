// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"bytes"
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
)

// The tests in this file keep the "Indexed document fields by mapping mode" tables in
// README.md in sync with the encoders. Each case encodes a sample signal and asserts
// that the field paths documented in the README are present in the indexed document,
// so the tables cannot silently drift away from the implementation.

// assertDocumentedFields checks that every documented path exists in the document and
// that no undocumented path does.
func assertDocumentedFields(t *testing.T, body string, want, notWant []string) {
	t.Helper()
	for _, path := range want {
		assert.Truef(t, gjson.Get(body, path).Exists(), "documented field %q missing from document: %s", path, body)
	}
	for _, path := range notWant {
		assert.Falsef(t, gjson.Get(body, path).Exists(), "undocumented field %q present in document: %s", path, body)
	}
}

func documentationLogFixture() (encodingContext, plog.LogRecord) {
	rl := plog.NewResourceLogs()
	rl.Resource().Attributes().PutStr("service.name", "my-service")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("my-scope")
	sl.Scope().SetVersion("v1.2.3")
	sl.Scope().Attributes().PutStr("scope-attr", "scope-value")
	rec := sl.LogRecords().AppendEmpty()
	rec.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1700000000, 0)))
	rec.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Unix(1700000001, 0)))
	rec.SetSeverityText("ERROR")
	rec.SetSeverityNumber(plog.SeverityNumberError)
	rec.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 8, 7, 6, 5, 4, 3, 2, 1})
	rec.SetSpanID([8]byte{0x19, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26})
	rec.Body().SetStr("log-body")
	rec.Attributes().PutStr("log-attr", "value1")
	return encodingContext{resource: rl.Resource(), scope: sl.Scope()}, rec
}

func documentationSpanFixture() (encodingContext, ptrace.Span) {
	rs := ptrace.NewResourceSpans()
	rs.Resource().Attributes().PutStr("service.name", "my-service")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("my-scope")
	ss.Scope().SetVersion("v1.2.3")
	span := ss.Spans().AppendEmpty()
	span.SetName("client span")
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1700000000, 0)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(1700000001, 0)))
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 8, 7, 6, 5, 4, 3, 2, 1})
	span.SetSpanID([8]byte{0x19, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26})
	span.SetParentSpanID([8]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18})
	span.TraceState().FromRaw("vendor=state")
	span.Status().SetCode(ptrace.StatusCodeError)
	span.Status().SetMessage("boom")
	span.Attributes().PutStr("span-attr", "value2")
	span.SetDroppedAttributesCount(3)
	span.SetDroppedEventsCount(2)
	span.SetDroppedLinksCount(1)
	event := span.Events().AppendEmpty()
	event.SetName("fooEvent")
	event.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1700000000, 0)))
	event.Attributes().PutStr("foo", "bar")
	link := span.Links().AppendEmpty()
	link.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 8, 7, 6, 5, 4, 3, 2, 0})
	link.SetSpanID([8]byte{0x19, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x27})
	return encodingContext{resource: rs.Resource(), scope: ss.Scope()}, span
}

func documentationMetricsContext() encodingContext {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("cloud.platform", "aws")
	scope := pcommon.NewInstrumentationScope()
	scope.SetName("my-scope")
	scope.SetVersion("v1.2.3")
	scope.Attributes().PutStr("scope-attr", "scope-value")
	return encodingContext{resource: resource, scope: scope}
}

func documentationMetricsFixture() []datapoints.DataPoint {
	m := pmetric.NewMetric()
	m.SetName("system.cpu.usage")
	m.SetUnit("s")
	sum := m.SetEmptySum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp := sum.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1700000000, 0)))
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1699999999, 0)))
	dp.SetDoubleValue(1.5)
	dp.Attributes().PutStr("client.address", "10.0.0.1")
	return []datapoints.DataPoint{datapoints.NewNumber(m, dp)}
}

// documentationHistogramFixture returns a histogram data point, which is the data
// point type that carries a temporality.
func documentationHistogramFixture() []datapoints.DataPoint {
	m := pmetric.NewMetric()
	m.SetName("system.cpu.time")
	m.SetUnit("s")
	hist := m.SetEmptyHistogram()
	hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp := hist.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1700000000, 0)))
	dp.ExplicitBounds().FromRaw([]float64{1, 2})
	dp.BucketCounts().FromRaw([]uint64{1, 2, 3})
	dp.SetCount(6)
	dp.SetSum(10)
	return []datapoints.DataPoint{datapoints.NewHistogram(m, dp, datapoints.HistogramMappingRaw)}
}

func TestDocumentedLogFields(t *testing.T) {
	idx := elasticsearch.NewDataStreamIndex("logs", "dataset", "namespace")

	t.Run("none", func(t *testing.T) {
		encoder, err := newEncoder(MappingNone)
		require.NoError(t, err)
		ec, rec := documentationLogFixture()
		var buf bytes.Buffer
		require.NoError(t, encoder.encodeLog(ec, rec, idx, &buf))
		assertDocumentedFields(t, buf.String(), []string{
			"@timestamp",
			"TraceId",
			"SpanId",
			"TraceFlags",
			"SeverityText",
			"SeverityNumber",
			"Body",
			`Resource\.service\.name`,
			`Scope\.name`,
			`Scope\.version`,
			`Attributes\.log-attr`,
			`Attributes\.data_stream\.type`,
			`Attributes\.data_stream\.dataset`,
			`Attributes\.data_stream\.namespace`,
		}, nil)
	})

	t.Run("raw", func(t *testing.T) {
		encoder, err := newEncoder(MappingRaw)
		require.NoError(t, err)
		ec, rec := documentationLogFixture()
		var buf bytes.Buffer
		require.NoError(t, encoder.encodeLog(ec, rec, idx, &buf))
		assertDocumentedFields(t, buf.String(), []string{
			"@timestamp",
			"TraceId",
			"SpanId",
			"TraceFlags",
			"SeverityText",
			"SeverityNumber",
			"Body",
			`Resource\.service\.name`,
			`Scope\.name`,
			"log-attr",
			`data_stream\.type`,
			`data_stream\.dataset`,
			`data_stream\.namespace`,
		}, []string{`Attributes\.log-attr`, `Attributes\.data_stream\.type`})
	})

	t.Run("ecs", func(t *testing.T) {
		encoder, err := newEncoder(MappingECS)
		require.NoError(t, err)
		ec, rec := documentationLogFixture()
		var buf bytes.Buffer
		require.NoError(t, encoder.encodeLog(ec, rec, idx, &buf))
		assertDocumentedFields(t, buf.String(), []string{
			"@timestamp",
			"message",
			"log.level",
			"event.severity",
			"trace.id",
			"span.id",
			"data_stream.type",
			"data_stream.dataset",
			"data_stream.namespace",
		}, nil)
	})

	t.Run("otel", func(t *testing.T) {
		encoder, err := newEncoder(MappingOTel)
		require.NoError(t, err)
		ec, rec := documentationLogFixture()
		var buf bytes.Buffer
		require.NoError(t, encoder.encodeLog(ec, rec, idx, &buf))
		assertDocumentedFields(t, buf.String(), []string{
			"@timestamp",
			"observed_timestamp",
			"data_stream.type",
			"data_stream.dataset",
			"data_stream.namespace",
			"severity_text",
			"severity_number",
			"trace_id",
			"span_id",
			"attributes.log-attr",
			"resource",
			"scope",
			"body.text",
		}, nil)
	})
}

func TestDocumentedSpanFields(t *testing.T) {
	idx := elasticsearch.NewDataStreamIndex("traces", "dataset", "namespace")

	t.Run("none", func(t *testing.T) {
		encoder, err := newEncoder(MappingNone)
		require.NoError(t, err)
		ec, span := documentationSpanFixture()
		var buf bytes.Buffer
		require.NoError(t, encoder.encodeSpan(ec, span, idx, &buf))
		assertDocumentedFields(t, buf.String(), []string{
			"@timestamp",
			"EndTimestamp",
			"TraceId",
			"SpanId",
			"ParentSpanId",
			"Name",
			"Kind",
			"TraceStatus",
			"TraceStatusDescription",
			"Duration",
			"Link",
			`Resource\.service\.name`,
			`Scope\.name`,
			`Scope\.version`,
			`Attributes\.span-attr`,
			`Attributes\.data_stream\.type`,
			`Events\.fooEvent\.time`,
			`Events\.fooEvent\.foo`,
		}, nil)
	})

	t.Run("raw", func(t *testing.T) {
		encoder, err := newEncoder(MappingRaw)
		require.NoError(t, err)
		ec, span := documentationSpanFixture()
		var buf bytes.Buffer
		require.NoError(t, encoder.encodeSpan(ec, span, idx, &buf))
		assertDocumentedFields(t, buf.String(), []string{
			"@timestamp",
			"EndTimestamp",
			"TraceId",
			"SpanId",
			"ParentSpanId",
			"Name",
			"Kind",
			"TraceStatus",
			"TraceStatusDescription",
			"Duration",
			"Link",
			`Resource\.service\.name`,
			`Scope\.name`,
			"span-attr",
			`data_stream\.type`,
			`fooEvent\.time`,
			`fooEvent\.foo`,
		}, []string{`Attributes\.span-attr`, `Events\.fooEvent\.time`})
	})

	t.Run("ecs", func(t *testing.T) {
		encoder, err := newEncoder(MappingECS)
		require.NoError(t, err)
		ec, span := documentationSpanFixture()
		var buf bytes.Buffer
		require.NoError(t, encoder.encodeSpan(ec, span, idx, &buf))
		assertDocumentedFields(t, buf.String(), []string{
			"@timestamp",
			"span.name",
			"span.kind",
			"trace.id",
			"span.id",
			"parent.id",
			"event.outcome",
			"span.links",
			"data_stream.type",
			"data_stream.dataset",
			"data_stream.namespace",
		}, nil)
	})

	t.Run("otel", func(t *testing.T) {
		encoder, err := newEncoder(MappingOTel)
		require.NoError(t, err)
		ec, span := documentationSpanFixture()
		var buf bytes.Buffer
		require.NoError(t, encoder.encodeSpan(ec, span, idx, &buf))
		assertDocumentedFields(t, buf.String(), []string{
			"@timestamp",
			"data_stream.type",
			"name",
			"kind",
			"duration",
			"status.code",
			"status.message",
			"trace_id",
			"span_id",
			"parent_span_id",
			"trace_state",
			"links",
			"attributes.span-attr",
			"resource",
			"scope",
			"dropped_attributes_count",
			"dropped_events_count",
			"dropped_links_count",
		}, nil)
	})
}

func TestDocumentedMetricFields(t *testing.T) {
	idx := elasticsearch.NewDataStreamIndex("metrics", "dataset", "namespace")

	t.Run("ecs", func(t *testing.T) {
		encoder, err := newEncoder(MappingECS)
		require.NoError(t, err)
		ec := documentationMetricsContext()
		var validationErrors []error
		var buf bytes.Buffer
		_, err = encoder.encodeMetrics(ec, documentationMetricsFixture(), &validationErrors, idx, &buf)
		require.NoError(t, err)
		assert.Empty(t, validationErrors)
		assertDocumentedFields(t, buf.String(), []string{
			"@timestamp",
			// The metric is indexed at the metric name as is, not under a "metric." prefix.
			"system.cpu.usage",
			"data_stream.type",
			"data_stream.dataset",
			"data_stream.namespace",
			// Resource attributes go through the ECS conversion map.
			"cloud.service.name",
			// Data point attributes keep their original names.
			"client.address",
		}, []string{"metric.system.cpu.usage", "client.ip"})
	})

	t.Run("otel", func(t *testing.T) {
		encoder, err := newEncoder(MappingOTel)
		require.NoError(t, err)
		ec := documentationMetricsContext()
		var validationErrors []error
		var buf bytes.Buffer
		_, err = encoder.encodeMetrics(ec, documentationMetricsFixture(), &validationErrors, idx, &buf)
		require.NoError(t, err)
		assert.Empty(t, validationErrors)
		assertDocumentedFields(t, buf.String(), []string{
			"@timestamp",
			"start_timestamp",
			"unit",
			`metrics.system\.cpu\.usage`,
			"data_stream.type",
			"data_stream.dataset",
			"data_stream.namespace",
			"_metric_names_hash",
			`attributes.client\.address`,
		}, nil)

		// Temporality is carried by the data point types that have one.
		var histBuf bytes.Buffer
		_, err = encoder.encodeMetrics(ec, documentationHistogramFixture(), &validationErrors, idx, &histBuf)
		require.NoError(t, err)
		assert.Empty(t, validationErrors)
		assertDocumentedFields(t, histBuf.String(), []string{
			"temporality",
			`metrics.system\.cpu\.time`,
		}, nil)
	})
}

func TestDocumentedBodyMapFields(t *testing.T) {
	idx := elasticsearch.NewDataStreamIndex("logs", "dataset", "namespace")

	t.Run("map body is indexed as is", func(t *testing.T) {
		encoder, err := newEncoder(MappingBodyMap)
		require.NoError(t, err)
		ec, rec := documentationLogFixture()
		body := rec.Body().SetEmptyMap()
		body.PutStr("message", "log-body")
		body.PutStr("service.name", "my-service")
		var buf bytes.Buffer
		require.NoError(t, encoder.encodeLog(ec, rec, idx, &buf))
		assertDocumentedFields(t, buf.String(), []string{
			"message",
			`service\.name`,
		}, []string{"@timestamp", "TraceId", "data_stream.type"})
	})

	t.Run("non-map body is rejected", func(t *testing.T) {
		encoder, err := newEncoder(MappingBodyMap)
		require.NoError(t, err)
		ec, rec := documentationLogFixture()
		rec.Body().SetStr(`{"message":"log-body"}`)
		var buf bytes.Buffer
		err = encoder.encodeLog(ec, rec, idx, &buf)
		require.ErrorIs(t, err, ErrInvalidTypeForBodyMapMode)
	})
}
