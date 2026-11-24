// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func TestPushTraceData(t *testing.T) {
	port, err := findRandomPort()
	require.NoError(t, err)

	config := createDefaultConfig().(*Config)
	config.ClientConfig.Endpoint = fmt.Sprintf("http://127.0.0.1:%d", port)
	config.CreateSchema = false
	require.NoError(t, config.Validate())

	exporter := newTracesExporter(zap.NewNop(), config, componenttest.NewNopTelemetrySettings())

	ctx := t.Context()

	client, err := createDorisHTTPClient(ctx, config, componenttest.NewNopHost(), componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	require.NotNil(t, client)

	exporter.client = client

	defer func() {
		_ = exporter.shutdown(ctx)
	}()

	srvMux := http.NewServeMux()
	server := &http.Server{
		ReadTimeout: 3 * time.Second,
		Addr:        fmt.Sprintf(":%d", port),
		Handler:     srvMux,
	}

	go func() {
		srvMux.HandleFunc("/api/otel/otel_traces/_stream_load", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Status":"Success"}`))
		})
		err = server.ListenAndServe()
		assert.Equal(t, http.ErrServerClosed, err)
	}()

	err0 := errors.New("Not Started")
	for i := 0; err0 != nil && i < 10; i++ { // until server started
		err0 = exporter.pushTraceData(ctx, simpleTraces(10))
		time.Sleep(100 * time.Millisecond)
	}
	require.NoError(t, err0)

	_ = server.Shutdown(ctx)
}

func simpleTraces(count int) ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.SetSchemaUrl("https://opentelemetry.io/schemas/1.4.0")
	rs.Resource().SetDroppedAttributesCount(10)
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("io.opentelemetry.contrib.doris")
	ss.Scope().SetVersion("1.0.0")
	ss.SetSchemaUrl("https://opentelemetry.io/schemas/1.7.0")
	ss.Scope().SetDroppedAttributesCount(20)
	ss.Scope().Attributes().PutStr("lib", "doris")
	timestamp := time.Now()
	for i := range count {
		s := ss.Spans().AppendEmpty()
		s.SetTraceID([16]byte{1, 2, 3, byte(i)})
		s.SetSpanID([8]byte{1, 2, 3, byte(i)})
		s.TraceState().FromRaw("trace state")
		s.SetParentSpanID([8]byte{1, 2, 4, byte(i)})
		s.SetName("call db")
		s.SetKind(ptrace.SpanKindInternal)
		s.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
		s.SetEndTimestamp(pcommon.NewTimestampFromTime(timestamp.Add(time.Minute)))
		s.Attributes().PutStr("service.name", "v")
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

func TestPushTraceData_IsRoot(t *testing.T) {
	port, err := findRandomPort()
	require.NoError(t, err)

	config := createDefaultConfig().(*Config)
	config.ClientConfig.Endpoint = fmt.Sprintf("http://127.0.0.1:%d", port)
	config.CreateSchema = false
	require.NoError(t, config.Validate())

	exporter := newTracesExporter(zap.NewNop(), config, componenttest.NewNopTelemetrySettings())
	ctx := t.Context()
	client, err := createDorisHTTPClient(ctx, config, componenttest.NewNopHost(), componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	exporter.client = client
	defer func() { _ = exporter.shutdown(ctx) }()

	received := make(chan []byte, 1)
	srvMux := http.NewServeMux()
	srvMux.HandleFunc("/api/otel/otel_traces/_stream_load", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		select {
		case received <- body:
		default:
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Status":"Success"}`))
	})
	server := &http.Server{ReadTimeout: 3 * time.Second, Addr: fmt.Sprintf(":%d", port), Handler: srvMux}
	go func() {
		err := server.ListenAndServe()
		assert.Equal(t, http.ErrServerClosed, err)
	}()
	defer func() { _ = server.Shutdown(ctx) }()

	// two spans: a root span (no parent) and a child span
	traces := simpleTraces(2)
	spans := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans()
	spans.At(0).SetParentSpanID(pcommon.NewSpanIDEmpty())

	err0 := errors.New("Not Started")
	for i := 0; err0 != nil && i < 10; i++ { // until server started
		err0 = exporter.pushTraceData(ctx, traces)
		time.Sleep(100 * time.Millisecond)
	}
	require.NoError(t, err0)

	var body []byte
	select {
	case body = <-received:
	case <-time.After(3 * time.Second):
		t.Fatal("no stream load request received")
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	require.Len(t, lines, 2)
	var root, child map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &root))
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &child))
	assert.Empty(t, root["parent_span_id"])
	assert.Equal(t, float64(1), root["is_root"])
	assert.NotEmpty(t, child["parent_span_id"])
	assert.Equal(t, float64(0), child["is_root"])
}

func TestTracesSchemaSQL(t *testing.T) {
	config := createDefaultConfig().(*Config)

	ddl := fmt.Sprintf(tracesDDL, config.Table.Traces, config.propertiesStr())
	assert.Contains(t, ddl, "is_root               TINYINT")
	assert.Contains(t, ddl, "INDEX idx_is_root(is_root) USING INVERTED")
	assert.Contains(t, ddl, "DUPLICATE KEY(service_name, timestamp)")
	assert.Contains(t, ddl, "DISTRIBUTED BY RANDOM BUCKETS AUTO")

	services := fmt.Sprintf(tracesServicesView, config.Table.Traces, config.Table.Traces)
	assert.Contains(t, services, "CREATE MATERIALIZED VIEW otel_traces_services AS")

	view := fmt.Sprintf(tracesView, config.Table.Traces, config.Table.Traces)
	assert.Contains(t, view, "CREATE MATERIALIZED VIEW otel_traces_summary AS")
	assert.Contains(t, view, "FROM otel_traces\n")
	// the DATE_FORMAT pattern must survive fmt.Sprintf (escaped as %% in the sql file)
	assert.Contains(t, view, "DATE_FORMAT(timestamp, '%Y%m%d%H%i%s%f')")
	assert.NotContains(t, view, "%!")
	// every output column must be aliased: Doris rejects sync MV columns named like base-table columns
	for _, col := range []string{"s_trace_id", "s_day", "s_start_time", "s_end_time", "s_span_count", "s_error_count", "s_first_root", "s_max_span_duration"} {
		assert.Contains(t, view, " AS "+col)
	}
}
