// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package clickhouseexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap/zaptest"
)

func testTracesExporter(t *testing.T, endpoint string) {
	exporter := newTestTracesExporter(t, endpoint)
	verifyExportTraces(t, exporter)
}

func newTestTracesExporter(t *testing.T, dsn string, fns ...func(*Config)) *tracesExporter {
	exporter := newTracesExporter(zaptest.NewLogger(t), withTestExporterConfig(fns...)(dsn))

	require.NoError(t, exporter.start(context.Background(), nil))

	t.Cleanup(func() { _ = exporter.shutdown(context.Background()) })
	return exporter
}

func verifyExportTraces(t *testing.T, exporter *tracesExporter) {
	// 3 pushes
	mustPushTracesData(t, exporter, simpleTraces(5000))
	mustPushTracesData(t, exporter, simpleTraces(5000))
	mustPushTracesData(t, exporter, simpleTraces(5000))

	type trace struct {
		Timestamp          time.Time           `ch:"Timestamp"`
		TraceID            string              `ch:"TraceId"`
		SpanID             string              `ch:"SpanId"`
		ParentSpanID       string              `ch:"ParentSpanId"`
		TraceState         string              `ch:"TraceState"`
		SpanName           string              `ch:"SpanName"`
		SpanKind           string              `ch:"SpanKind"`
		ServiceName        string              `ch:"ServiceName"`
		ResourceAttributes map[string]string   `ch:"ResourceAttributes"`
		ScopeName          string              `ch:"ScopeName"`
		ScopeVersion       string              `ch:"ScopeVersion"`
		SpanAttributes     map[string]string   `ch:"SpanAttributes"`
		Duration           uint64              `ch:"Duration"`
		StatusCode         string              `ch:"StatusCode"`
		StatusMessage      string              `ch:"StatusMessage"`
		EventsTimestamp    []time.Time         `ch:"Events.Timestamp"`
		EventsName         []string            `ch:"Events.Name"`
		EventsAttributes   []map[string]string `ch:"Events.Attributes"`
		LinksTraceID       []string            `ch:"Links.TraceId"`
		LinksSpanID        []string            `ch:"Links.SpanId"`
		LinksTraceState    []string            `ch:"Links.TraceState"`
		LinksAttributes    []map[string]string `ch:"Links.Attributes"`
	}

	expectedTrace := trace{
		Timestamp:    telemetryTimestamp,
		TraceID:      "01020300000000000000000000000000",
		SpanID:       "0102030000000000",
		ParentSpanID: "0102040000000000",
		TraceState:   "trace state",
		SpanName:     "call db",
		SpanKind:     "Internal",
		ServiceName:  "test-service",
		ResourceAttributes: map[string]string{
			"service.name": "test-service",
		},
		ScopeName:    "io.opentelemetry.contrib.clickhouse",
		ScopeVersion: "1.0.0",
		SpanAttributes: map[string]string{
			"service.name": "v",
		},
		Duration:      60000000000,
		StatusCode:    "Error",
		StatusMessage: "error",
		EventsTimestamp: []time.Time{
			telemetryTimestamp,
		},
		EventsName: []string{"event1"},
		EventsAttributes: []map[string]string{
			{
				"level": "info",
			},
		},
		LinksTraceID: []string{
			"01020500000000000000000000000000",
		},
		LinksSpanID: []string{
			"0102050000000000",
		},
		LinksTraceState: []string{
			"error",
		},
		LinksAttributes: []map[string]string{
			{
				"k": "v",
			},
		},
	}

	row := exporter.db.QueryRow(context.Background(), "SELECT * FROM otel_int_test.otel_traces")
	require.NoError(t, row.Err())

	var actualTrace trace
	err := row.ScanStruct(&actualTrace)
	require.NoError(t, err)

	require.Equal(t, expectedTrace, actualTrace)
}

func mustPushTracesData(t *testing.T, exporter *tracesExporter, td ptrace.Traces) {
	err := exporter.pushTraceData(context.Background(), td)
	require.NoError(t, err)
}

func simpleTraces(count int) ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.SetSchemaUrl("https://opentelemetry.io/schemas/1.4.0")
	rs.Resource().SetDroppedAttributesCount(10)
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("io.opentelemetry.contrib.clickhouse")
	ss.Scope().SetVersion("1.0.0")
	ss.SetSchemaUrl("https://opentelemetry.io/schemas/1.7.0")
	ss.Scope().SetDroppedAttributesCount(20)
	ss.Scope().Attributes().PutStr("lib", "clickhouse")
	timestamp := telemetryTimestamp

	for i := 0; i < count; i++ {
		s := ss.Spans().AppendEmpty()
		s.SetTraceID([16]byte{1, 2, 3, byte(i)})
		s.SetSpanID([8]byte{1, 2, 3, byte(i)})
		s.TraceState().FromRaw("trace state")
		s.SetParentSpanID([8]byte{1, 2, 4, byte(i)})
		s.SetName("call db")
		s.SetKind(ptrace.SpanKindInternal)
		s.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
		s.SetEndTimestamp(pcommon.NewTimestampFromTime(timestamp.Add(time.Minute)))
		s.Attributes().PutStr(string(conventions.ServiceNameKey), "v")
		s.Status().SetMessage("error")
		s.Status().SetCode(ptrace.StatusCodeError)
		event := s.Events().AppendEmpty()
		event.SetName("event1")
		event.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		event.Attributes().PutStr("level", "info")
		link := s.Links().AppendEmpty()
		link.SetTraceID([16]byte{1, 2, 5, byte(i)})
		link.SetSpanID([8]byte{1, 2, 5, byte(i)})
		link.TraceState().FromRaw("error")
		link.Attributes().PutStr("k", "v")
	}

	return traces
}
