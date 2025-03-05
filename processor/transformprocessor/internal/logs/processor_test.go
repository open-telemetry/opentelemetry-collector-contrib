// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

var (
	TestLogTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	TestLogTimestamp = pcommon.NewTimestampFromTime(TestLogTime)

	TestObservedTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	TestObservedTimestamp = pcommon.NewTimestampFromTime(TestObservedTime)

	traceID = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID  = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
)

func Test_ProcessLogs_ResourceContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td plog.Logs)
	}{
		{
			statement: `set(attributes["test"], "pass")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).Resource().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where attributes["host.name"] == "wrong"`,
			want: func(_ plog.Logs) {
			},
		},
		{
			statement: `set(schema_url, "test_schema_url")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).SetSchemaUrl("test_schema_url")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructLogs()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "resource", Statements: []string{tt.statement}}}, ottl.IgnoreError, false, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessLogs(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructLogs()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessLogs_InferredResourceContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td plog.Logs)
	}{
		{
			statement: `set(resource.attributes["test"], "pass")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).Resource().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(resource.attributes["test"], "pass") where resource.attributes["host.name"] == "wrong"`,
			want: func(_ plog.Logs) {
			},
		},
		{
			statement: `set(resource.schema_url, "test_schema_url")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).SetSchemaUrl("test_schema_url")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructLogs()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "", Statements: []string{tt.statement}}}, ottl.IgnoreError, false, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessLogs(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructLogs()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessLogs_ScopeContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td plog.Logs)
	}{
		{
			statement: `set(attributes["test"], "pass") where name == "scope"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where version == 2`,
			want: func(_ plog.Logs) {
			},
		},
		{
			statement: `set(schema_url, "test_schema_url")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).SetSchemaUrl("test_schema_url")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructLogs()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "scope", Statements: []string{tt.statement}}}, ottl.IgnoreError, false, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessLogs(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructLogs()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessLogs_InferredScopeContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td plog.Logs)
	}{
		{
			statement: `set(scope.attributes["test"], "pass") where scope.name == "scope"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(scope.attributes["test"], "pass") where scope.version == 2`,
			want: func(_ plog.Logs) {
			},
		},
		{
			statement: `set(scope.schema_url, "test_schema_url")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).SetSchemaUrl("test_schema_url")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructLogs()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "", Statements: []string{tt.statement}}}, ottl.IgnoreError, false, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessLogs(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructLogs()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessLogs_LogContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td plog.Logs)
	}{
		{
			statement: `set(attributes["test"], "pass") where body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where resource.attributes["host.name"] == "localhost"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `keep_keys(attributes, ["http.method"]) where body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Clear()
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.method",
					"get")
			},
		},
		{
			statement: `set(severity_text, "ok") where attributes["http.path"] == "/health"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).SetSeverityText("ok")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).SetSeverityText("ok")
			},
		},
		{
			statement: `replace_pattern(attributes["http.method"], "get", "post")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.method", "post")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("http.method", "post")
			},
		},
		{
			statement: `replace_all_patterns(attributes, "value", "get", "post")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.method", "post")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("http.method", "post")
			},
		},
		{
			statement: `replace_all_patterns(attributes, "key", "http.url", "url")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Clear()
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.method", "get")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.path", "/health")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("url", "http://localhost/health")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("flags", "A|B|C")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("total.string", "123456789")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().Clear()
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("http.method", "get")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("http.path", "/health")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("url", "http://localhost/health")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("flags", "C|D")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("total.string", "345678")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where dropped_attributes_count == 1`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where flags == 1`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where severity_number == SEVERITY_NUMBER_TRACE`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(severity_number, SEVERITY_NUMBER_TRACE2) where severity_number == 1`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).SetSeverityNumber(2)
			},
		},
		{
			statement: `set(attributes["test"], "pass") where trace_id == TraceID(0x0102030405060708090a0b0c0d0e0f10)`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where span_id == SpanID(0x0102030405060708)`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where IsMatch(body, "operation[AC]")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `delete_key(attributes, "http.url") where body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Clear()
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.method",
					"get")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.path",
					"/health")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("total.string",
					"123456789")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("flags",
					"A|B|C")
			},
		},
		{
			statement: `delete_matching_keys(attributes, "http.*t.*") where body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Clear()
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.url",
					"http://localhost/health")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("flags",
					"A|B|C")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("total.string",
					"123456789")
			},
		},
		{
			statement: `set(attributes["test"], Concat([attributes["http.method"], attributes["http.url"]], ": ")) where body == Concat(["operation", "A"], "")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "get: http://localhost/health")
			},
		},
		{
			statement: `set(attributes["test"], Split(attributes["flags"], "|"))`,
			want: func(td plog.Logs) {
				v1 := td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutEmptySlice("test")
				v1.AppendEmpty().SetStr("A")
				v1.AppendEmpty().SetStr("B")
				v1.AppendEmpty().SetStr("C")
				v2 := td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutEmptySlice("test")
				v2.AppendEmpty().SetStr("C")
				v2.AppendEmpty().SetStr("D")
			},
		},
		{
			statement: `set(attributes["test"], Split(attributes["flags"], "|")) where body == "operationA"`,
			want: func(td plog.Logs) {
				newValue := td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutEmptySlice("test")
				newValue.AppendEmpty().SetStr("A")
				newValue.AppendEmpty().SetStr("B")
				newValue.AppendEmpty().SetStr("C")
			},
		},
		{
			statement: `set(attributes["test"], Split(attributes["not_exist"], "|"))`,
			want:      func(_ plog.Logs) {},
		},
		{
			statement: `set(attributes["test"], Substring(attributes["total.string"], 3, 3))`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "456")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("test", "678")
			},
		},
		{
			statement: `set(attributes["test"], Substring(attributes["total.string"], 3, 3)) where body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "456")
			},
		},
		{
			statement: `set(attributes["test"], Substring(attributes["not_exist"], 3, 3))`,
			want:      func(_ plog.Logs) {},
		},
		{
			statement: `set(attributes["test"], ["A", "B", "C"]) where body == "operationA"`,
			want: func(td plog.Logs) {
				v1 := td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutEmptySlice("test")
				v1.AppendEmpty().SetStr("A")
				v1.AppendEmpty().SetStr("B")
				v1.AppendEmpty().SetStr("C")
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase(body, "lower")) where body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "operationa")
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase(body, "upper")) where body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "OPERATIONA")
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase(body, "snake")) where body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "operation_a")
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase(body, "camel")) where body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "OperationA")
			},
		},
		{
			statement: `merge_maps(attributes, ParseJSON("{\"json_test\":\"pass\"}"), "insert") where body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("json_test", "pass")
			},
		},
		{
			statement: `limit(attributes, 0, []) where body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().RemoveIf(func(_ string, _ pcommon.Value) bool { return true })
			},
		},
		{
			statement: `set(attributes["test"], Log(1)) where body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutDouble("test", 0.0)
			},
		},
		{
			statement: `replace_match(body["metadata"]["uid"], "*", "12345")`,
			want:      func(_ plog.Logs) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructLogs()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "log", Statements: []string{tt.statement}}}, ottl.IgnoreError, false, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessLogs(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructLogs()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessLogs_InferredLogContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td plog.Logs)
	}{
		{
			statement: `set(log.attributes["test"], "pass") where log.body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(log.attributes["test"], "pass") where resource.attributes["host.name"] == "localhost"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `keep_keys(log.attributes, ["http.method"]) where log.body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Clear()
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.method",
					"get")
			},
		},
		{
			statement: `set(log.severity_text, "ok") where log.attributes["http.path"] == "/health"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).SetSeverityText("ok")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).SetSeverityText("ok")
			},
		},
		{
			statement: `replace_pattern(log.attributes["http.method"], "get", "post")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.method", "post")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("http.method", "post")
			},
		},
		{
			statement: `replace_all_patterns(log.attributes, "value", "get", "post")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.method", "post")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("http.method", "post")
			},
		},
		{
			statement: `replace_all_patterns(log.attributes, "key", "http.url", "url")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Clear()
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.method", "get")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.path", "/health")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("url", "http://localhost/health")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("flags", "A|B|C")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("total.string", "123456789")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().Clear()
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("http.method", "get")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("http.path", "/health")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("url", "http://localhost/health")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("flags", "C|D")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("total.string", "345678")
			},
		},
		{
			statement: `set(log.attributes["test"], "pass") where log.dropped_attributes_count == 1`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(log.attributes["test"], "pass") where log.flags == 1`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(log.attributes["test"], "pass") where log.severity_number == SEVERITY_NUMBER_TRACE`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(log.severity_number, SEVERITY_NUMBER_TRACE2) where log.severity_number == 1`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).SetSeverityNumber(2)
			},
		},
		{
			statement: `set(log.attributes["test"], "pass") where log.trace_id == TraceID(0x0102030405060708090a0b0c0d0e0f10)`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(log.attributes["test"], "pass") where log.span_id == SpanID(0x0102030405060708)`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(log.attributes["test"], "pass") where IsMatch(log.body, "operation[AC]")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `delete_key(log.attributes, "http.url") where log.body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Clear()
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.method",
					"get")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.path",
					"/health")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("total.string",
					"123456789")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("flags",
					"A|B|C")
			},
		},
		{
			statement: `delete_matching_keys(log.attributes, "http.*t.*") where log.body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Clear()
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.url",
					"http://localhost/health")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("flags",
					"A|B|C")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("total.string",
					"123456789")
			},
		},
		{
			statement: `set(log.attributes["test"], Concat([log.attributes["http.method"], log.attributes["http.url"]], ": ")) where log.body == Concat(["operation", "A"], "")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "get: http://localhost/health")
			},
		},
		{
			statement: `set(log.attributes["test"], Split(log.attributes["flags"], "|"))`,
			want: func(td plog.Logs) {
				v1 := td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutEmptySlice("test")
				v1.AppendEmpty().SetStr("A")
				v1.AppendEmpty().SetStr("B")
				v1.AppendEmpty().SetStr("C")
				v2 := td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutEmptySlice("test")
				v2.AppendEmpty().SetStr("C")
				v2.AppendEmpty().SetStr("D")
			},
		},
		{
			statement: `set(log.attributes["test"], Split(log.attributes["flags"], "|")) where log.body == "operationA"`,
			want: func(td plog.Logs) {
				newValue := td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutEmptySlice("test")
				newValue.AppendEmpty().SetStr("A")
				newValue.AppendEmpty().SetStr("B")
				newValue.AppendEmpty().SetStr("C")
			},
		},
		{
			statement: `set(log.attributes["test"], Split(log.attributes["not_exist"], "|"))`,
			want:      func(_ plog.Logs) {},
		},
		{
			statement: `set(log.attributes["test"], Substring(log.attributes["total.string"], 3, 3))`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "456")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("test", "678")
			},
		},
		{
			statement: `set(log.attributes["test"], Substring(log.attributes["total.string"], 3, 3)) where log.body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "456")
			},
		},
		{
			statement: `set(log.attributes["test"], Substring(log.attributes["not_exist"], 3, 3))`,
			want:      func(_ plog.Logs) {},
		},
		{
			statement: `set(log.attributes["test"], ["A", "B", "C"]) where log.body == "operationA"`,
			want: func(td plog.Logs) {
				v1 := td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutEmptySlice("test")
				v1.AppendEmpty().SetStr("A")
				v1.AppendEmpty().SetStr("B")
				v1.AppendEmpty().SetStr("C")
			},
		},
		{
			statement: `set(log.attributes["test"], ConvertCase(log.body, "lower")) where log.body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "operationa")
			},
		},
		{
			statement: `set(log.attributes["test"], ConvertCase(log.body, "upper")) where log.body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "OPERATIONA")
			},
		},
		{
			statement: `set(log.attributes["test"], ConvertCase(log.body, "snake")) where log.body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "operation_a")
			},
		},
		{
			statement: `set(log.attributes["test"], ConvertCase(log.body, "camel")) where log.body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "OperationA")
			},
		},
		{
			statement: `merge_maps(log.attributes, ParseJSON("{\"json_test\":\"pass\"}"), "insert") where log.body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("json_test", "pass")
			},
		},
		{
			statement: `limit(log.attributes, 0, []) where log.body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().RemoveIf(func(_ string, _ pcommon.Value) bool { return true })
			},
		},
		{
			statement: `set(log.attributes["test"], Log(1)) where log.body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutDouble("test", 0.0)
			},
		},
		{
			statement: `replace_match(log.body["metadata"]["uid"], "*", "12345")`,
			want:      func(_ plog.Logs) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructLogs()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "", Statements: []string{tt.statement}}}, ottl.IgnoreError, false, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessLogs(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructLogs()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessLogs_MixContext(t *testing.T) {
	tests := []struct {
		name              string
		contextStatements []common.ContextStatements
		want              func(td plog.Logs)
	}{
		{
			name: "set resource and then use",
			contextStatements: []common.ContextStatements{
				{
					Context: "resource",
					Statements: []string{
						`set(attributes["test"], "pass")`,
					},
				},
				{
					Context: "log",
					Statements: []string{
						`set(attributes["test"], "pass") where resource.attributes["test"] == "pass"`,
					},
				},
			},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).Resource().Attributes().PutStr("test", "pass")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "set scope and then use",
			contextStatements: []common.ContextStatements{
				{
					Context: "scope",
					Statements: []string{
						`set(attributes["test"], "pass")`,
					},
				},
				{
					Context: "log",
					Statements: []string{
						`set(attributes["test"], "pass") where instrumentation_scope.attributes["test"] == "pass"`,
					},
				},
			},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes().PutStr("test", "pass")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "order matters",
			contextStatements: []common.ContextStatements{
				{
					Context: "log",
					Statements: []string{
						`set(attributes["test"], "pass") where instrumentation_scope.attributes["test"] == "pass"`,
					},
				},
				{
					Context: "scope",
					Statements: []string{
						`set(attributes["test"], "pass")`,
					},
				},
			},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "reuse context",
			contextStatements: []common.ContextStatements{
				{
					Context: "scope",
					Statements: []string{
						`set(attributes["test"], "pass")`,
					},
				},
				{
					Context: "log",
					Statements: []string{
						`set(attributes["test"], "pass") where instrumentation_scope.attributes["test"] == "pass"`,
					},
				},
				{
					Context: "scope",
					Statements: []string{
						`set(attributes["test"], "fail")`,
					},
				},
			},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes().PutStr("test", "fail")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("test", "pass")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := constructLogs()
			processor, err := NewProcessor(tt.contextStatements, ottl.IgnoreError, false, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessLogs(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructLogs()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessLogs_InferredMixContext(t *testing.T) {
	tests := []struct {
		name              string
		contextStatements []common.ContextStatements
		want              func(td plog.Logs)
	}{
		{
			name: "set resource and then use",
			contextStatements: []common.ContextStatements{
				{
					Statements: []string{`set(resource.attributes["test"], "pass")`},
				},
				{
					Statements: []string{`set(log.attributes["test"], "pass") where resource.attributes["test"] == "pass"`},
				},
			},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).Resource().Attributes().PutStr("test", "pass")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "set scope and then use",
			contextStatements: []common.ContextStatements{
				{
					Statements: []string{`set(scope.attributes["test"], "pass")`},
				},
				{
					Statements: []string{`set(log.attributes["test"], "pass") where instrumentation_scope.attributes["test"] == "pass"`},
				},
			},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes().PutStr("test", "pass")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "order matters",
			contextStatements: []common.ContextStatements{
				{
					Statements: []string{`set(log.attributes["test"], "pass") where instrumentation_scope.attributes["test"] == "pass"`},
				},
				{
					Statements: []string{`set(scope.attributes["test"], "pass")`},
				},
			},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "reuse context",
			contextStatements: []common.ContextStatements{
				{
					Statements: []string{`set(scope.attributes["test"], "pass")`},
				},
				{
					Statements: []string{`set(log.attributes["test"], "pass") where instrumentation_scope.attributes["test"] == "pass"`},
				},
				{
					Statements: []string{`set(scope.attributes["test"], "fail")`},
				},
			},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes().PutStr("test", "fail")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("test", "pass")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := constructLogs()
			processor, err := NewProcessor(tt.contextStatements, ottl.IgnoreError, false, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessLogs(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructLogs()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessLogs_ErrorMode(t *testing.T) {
	tests := []struct {
		statement string
		context   common.ContextID
	}{
		{
			context: "resource",
		},
		{
			context: "scope",
		},
		{
			context: "log",
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.context), func(t *testing.T) {
			td := constructLogs()
			processor, err := NewProcessor([]common.ContextStatements{{Context: tt.context, Statements: []string{`set(attributes["test"], ParseJSON(1))`}}}, ottl.PropagateError, false, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessLogs(context.Background(), td)
			assert.Error(t, err)
		})
	}
}

func Test_ProcessLogs_StatementsErrorMode(t *testing.T) {
	tests := []struct {
		name          string
		errorMode     ottl.ErrorMode
		statements    []common.ContextStatements
		want          func(td plog.Logs)
		wantErrorWith string
	}{
		{
			name:      "log: statements group with error mode",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(log.attributes["pass"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(log.attributes["test"], "pass") where log.body == "operationA"`}},
			},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			name:      "log: statements group error mode does not affect default",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(log.attributes["pass"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(log.attributes["pass"], ParseJSON(true))`}},
			},
			wantErrorWith: "expected string but got bool",
		},
		{
			name:      "resource: statements group with error mode",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(resource.attributes["pass"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(resource.attributes["test"], "pass")`}},
			},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).Resource().Attributes().PutStr("test", "pass")
			},
		},
		{
			name:      "resource: statements group error mode does not affect default",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(resource.attributes["pass"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(resource.attributes["pass"], ParseJSON(true))`}},
			},
			wantErrorWith: "expected string but got bool",
		},
		{
			name:      "scope: statements group with error mode",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(scope.attributes["pass"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(scope.attributes["test"], "pass")`}},
			},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			name:      "scope: statements group error mode does not affect default",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(scope.attributes["pass"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(scope.attributes["pass"], ParseJSON(true))`}},
			},
			wantErrorWith: "expected string but got bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := constructLogs()
			processor, err := NewProcessor(tt.statements, tt.errorMode, false, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)
			_, err = processor.ProcessLogs(context.Background(), td)
			if tt.wantErrorWith != "" {
				if err == nil {
					t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
				}
				assert.Contains(t, err.Error(), tt.wantErrorWith)
				return
			}
			assert.NoError(t, err)
			exTd := constructLogs()
			tt.want(exTd)
			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessLogs_CacheAccess(t *testing.T) {
	tests := []struct {
		name       string
		statements []common.ContextStatements
		want       func(td plog.Logs)
	}{
		{
			name: "resource:resource.cache",
			statements: []common.ContextStatements{
				{Statements: []string{`set(resource.cache["test"], "pass")`}, SharedCache: true},
				{Statements: []string{`set(resource.attributes["test"], resource.cache["test"])`}, SharedCache: true},
			},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).Resource().Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "resource:cache",
			statements: []common.ContextStatements{
				{
					Context: common.Resource,
					Statements: []string{
						`set(cache["test"], "pass")`,
						`set(attributes["test"], cache["test"])`,
					},
				},
			},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).Resource().Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "scope:scope.cache",
			statements: []common.ContextStatements{
				{Statements: []string{`set(scope.cache["test"], "pass")`}, SharedCache: true},
				{Statements: []string{`set(scope.attributes["test"], scope.cache["test"])`}, SharedCache: true},
			},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "scope:cache",
			statements: []common.ContextStatements{{
				Context: common.Scope,
				Statements: []string{
					`set(cache["test"], "pass")`,
					`set(attributes["test"], cache["test"])`,
				},
			}},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "log:log.cache",
			statements: []common.ContextStatements{
				{Statements: []string{`set(log.cache["test"], "pass")`}, SharedCache: true},
				{Statements: []string{`set(log.attributes["test"], log.cache["test"])`}, SharedCache: true},
			},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "log:cache",
			statements: []common.ContextStatements{{
				Context: common.Log,
				Statements: []string{
					`set(cache["test"], "pass")`,
					`set(attributes["test"], cache["test"])`,
				},
			}},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "cache isolation",
			statements: []common.ContextStatements{
				{
					Statements:  []string{`set(log.cache["shared"], "fail")`},
					SharedCache: true,
				},
				{
					Statements: []string{
						`set(log.cache["test"], "pass")`,
						`set(log.attributes["test"], log.cache["test"])`,
						`set(log.attributes["test"], log.cache["shared"])`,
					},
				},
				{
					Context: common.Log,
					Statements: []string{
						`set(cache["test"], "pass")`,
						`set(attributes["test"], cache["test"])`,
						`set(attributes["test"], cache["shared"])`,
						`set(attributes["test"], log.cache["shared"])`,
					},
				},
				{
					Statements:  []string{`set(log.attributes["test"], "pass") where log.cache["shared"] == "fail"`},
					SharedCache: true,
				},
			},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("test", "pass")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := constructLogs()
			processor, err := NewProcessor(tt.statements, ottl.IgnoreError, false, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessLogs(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructLogs()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_NewProcessor_ConditionsParse(t *testing.T) {
	type testCase struct {
		name          string
		statements    []common.ContextStatements
		wantErrorWith string
	}

	contextsTests := map[string][]testCase{"log": nil, "resource": nil, "scope": nil}
	for ctx := range contextsTests {
		contextsTests[ctx] = []testCase{
			{
				name: "inferred: condition with context",
				statements: []common.ContextStatements{
					{
						Statements: []string{fmt.Sprintf(`set(%s.cache["test"], "pass")`, ctx)},
						Conditions: []string{fmt.Sprintf(`%s.cache["test"] == ""`, ctx)},
					},
				},
			},
			{
				name: "inferred: condition without context",
				statements: []common.ContextStatements{
					{
						Statements: []string{fmt.Sprintf(`set(%s.cache["test"], "pass")`, ctx)},
						Conditions: []string{`cache["test"] == ""`},
					},
				},
				wantErrorWith: `missing context name for path "cache[test]"`,
			},
			{
				name: "context defined: condition without context",
				statements: []common.ContextStatements{
					{
						Context:    common.ContextID(ctx),
						Statements: []string{`set(cache["test"], "pass")`},
						Conditions: []string{`cache["test"] == ""`},
					},
				},
			},
			{
				name: "context defined: condition with context",
				statements: []common.ContextStatements{
					{
						Context:    common.ContextID(ctx),
						Statements: []string{`set(attributes["test"], "pass")`},
						Conditions: []string{fmt.Sprintf(`%s.cache["test"] == ""`, ctx)},
					},
				},
			},
		}
	}

	for ctx, tests := range contextsTests {
		t.Run(ctx, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					_, err := NewProcessor(tt.statements, ottl.PropagateError, false, componenttest.NewNopTelemetrySettings())
					if tt.wantErrorWith != "" {
						if err == nil {
							t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
						}
						assert.Contains(t, err.Error(), tt.wantErrorWith)
						return
					}
					require.NoError(t, err)
				})
			}
		})
	}
}

func constructLogs() plog.Logs {
	td := plog.NewLogs()
	rs0 := td.ResourceLogs().AppendEmpty()
	rs0.SetSchemaUrl("test_schema_url")
	rs0.Resource().Attributes().PutStr("host.name", "localhost")
	rs0ils0 := rs0.ScopeLogs().AppendEmpty()
	rs0ils0.SetSchemaUrl("test_schema_url")
	rs0ils0.Scope().SetName("scope")
	fillLogOne(rs0ils0.LogRecords().AppendEmpty())
	fillLogTwo(rs0ils0.LogRecords().AppendEmpty())
	return td
}

func fillLogOne(log plog.LogRecord) {
	log.Body().SetStr("operationA")
	log.SetTimestamp(TestLogTimestamp)
	log.SetObservedTimestamp(TestObservedTimestamp)
	log.SetDroppedAttributesCount(1)
	log.SetFlags(plog.DefaultLogRecordFlags.WithIsSampled(true))
	log.SetSeverityNumber(1)
	log.SetTraceID(traceID)
	log.SetSpanID(spanID)
	log.Attributes().PutStr("http.method", "get")
	log.Attributes().PutStr("http.path", "/health")
	log.Attributes().PutStr("http.url", "http://localhost/health")
	log.Attributes().PutStr("flags", "A|B|C")
	log.Attributes().PutStr("total.string", "123456789")
}

func fillLogTwo(log plog.LogRecord) {
	log.Body().SetStr("operationB")
	log.SetTimestamp(TestLogTimestamp)
	log.SetObservedTimestamp(TestObservedTimestamp)
	log.Attributes().PutStr("http.method", "get")
	log.Attributes().PutStr("http.path", "/health")
	log.Attributes().PutStr("http.url", "http://localhost/health")
	log.Attributes().PutStr("flags", "C|D")
	log.Attributes().PutStr("total.string", "345678")
}
