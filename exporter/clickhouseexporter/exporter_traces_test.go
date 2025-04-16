// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"context"
	"database/sql/driver"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/zap/zaptest"
)

func TestExporter_pushTracesData(t *testing.T) {
	t.Run("push success", func(t *testing.T) {
		var items int
		initClickhouseTestServer(t, func(query string, values []driver.Value) error {
			t.Logf("%d, values:%+v", items, values)
			if strings.HasPrefix(query, "INSERT") {
				items++
			}
			return nil
		})

		exporter := newTestTracesExporter(t, defaultEndpoint, withDriverName(t.Name()))
		mustPushTracesData(t, exporter, simpleTraces(1))
		mustPushTracesData(t, exporter, simpleTraces(2))

		require.Equal(t, 3, items)
	})
	t.Run("check insert scopeName and ScopeVersion", func(t *testing.T) {
		initClickhouseTestServer(t, func(query string, values []driver.Value) error {
			if strings.HasPrefix(query, "INSERT") {
				require.Equal(t, "io.opentelemetry.contrib.clickhouse", values[9])
				require.Equal(t, "1.0.0", values[10])
			}
			return nil
		})

		exporter := newTestTracesExporter(t, defaultEndpoint, withDriverName(t.Name()))
		mustPushTracesData(t, exporter, simpleTraces(1))
	})
}

func newTestTracesExporter(t *testing.T, dsn string, fns ...func(*Config)) *tracesExporter {
	exporter, err := newTracesExporter(zaptest.NewLogger(t), withTestExporterConfig(fns...)(dsn))
	require.NoError(t, err)
	require.NoError(t, exporter.start(context.TODO(), nil))

	t.Cleanup(func() { _ = exporter.shutdown(context.TODO()) })
	return exporter
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
	timestamp := time.Unix(1703498029, 0)
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
		s.Attributes().PutStr(conventions.AttributeServiceName, "v")
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

func mustPushTracesData(t *testing.T, exporter *tracesExporter, td ptrace.Traces) {
	err := exporter.pushTraceData(context.TODO(), td)
	require.NoError(t, err)
}

func TestTracesClusterConfig(t *testing.T) {
	testClusterConfig(t, func(t *testing.T, dsn string, clusterTest clusterTestConfig, fns ...func(*Config)) {
		fns = append(fns, withDriverName(t.Name()))
		exporter := newTestTracesExporter(t, dsn, fns...)
		clusterTest.verifyConfig(t, exporter.cfg)
	})
}

func TestTracesTableEngineConfig(t *testing.T) {
	testTableEngineConfig(t, func(t *testing.T, dsn string, engineTest tableEngineTestConfig, fns ...func(*Config)) {
		fns = append(fns, withDriverName(t.Name()))
		exporter := newTestTracesExporter(t, dsn, fns...)
		engineTest.verifyConfig(t, exporter.cfg.TableEngine)
	})
}
