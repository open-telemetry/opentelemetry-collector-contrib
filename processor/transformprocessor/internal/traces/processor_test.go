// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

var (
	TestSpanStartTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	TestSpanStartTimestamp = pcommon.NewTimestampFromTime(TestSpanStartTime)

	TestSpanEndTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	TestSpanEndTimestamp = pcommon.NewTimestampFromTime(TestSpanEndTime)

	traceID = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID  = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	spanID2 = [8]byte{8, 7, 6, 5, 4, 3, 2, 1}
)

func Test_ProcessTraces_ResourceContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td ptrace.Traces)
	}{
		{
			statement: `set(attributes["test"], "pass")`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).Resource().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where attributes["host.name"] == "wrong"`,
			want: func(_ ptrace.Traces) {
			},
		},
		{
			statement: `set(schema_url, "test_schema_url")`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).SetSchemaUrl("test_schema_url")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructTraces()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "resource", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessTraces(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructTraces()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessTraces_InferredResourceContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td ptrace.Traces)
	}{
		{
			statement: `set(resource.attributes["test"], "pass")`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).Resource().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(resource.attributes["test"], "pass") where resource.attributes["host.name"] == "wrong"`,
			want: func(_ ptrace.Traces) {
			},
		},
		{
			statement: `set(resource.schema_url, "test_schema_url")`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).SetSchemaUrl("test_schema_url")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructTraces()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessTraces(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructTraces()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessTraces_ScopeContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td ptrace.Traces)
	}{
		{
			statement: `set(attributes["test"], "pass") where name == "scope"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where version == 2`,
			want: func(_ ptrace.Traces) {
			},
		},
		{
			statement: `set(schema_url, "test_schema_url")`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).SetSchemaUrl("test_schema_url")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructTraces()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "scope", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessTraces(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructTraces()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessTraces_InferredScopeContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td ptrace.Traces)
	}{
		{
			statement: `set(scope.attributes["test"], "pass") where scope.name == "scope"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(scope.attributes["test"], "pass") where scope.version == 2`,
			want: func(_ ptrace.Traces) {
			},
		},
		{
			statement: `set(scope.schema_url, "test_schema_url")`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).SetSchemaUrl("test_schema_url")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructTraces()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessTraces(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructTraces()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessTraces_TraceContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td ptrace.Traces)
	}{
		{
			statement: `set(attributes["test"], "pass") where name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where resource.attributes["host.name"] == "localhost"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `keep_keys(attributes, ["http.method"]) where name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().Clear()
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("http.method", "get")
			},
		},
		{
			statement: `set(status.code, 1) where attributes["http.path"] == "/health"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Status().SetCode(ptrace.StatusCodeOk)
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Status().SetCode(ptrace.StatusCodeOk)
			},
		},
		{
			statement: `set(attributes["test"], "pass") where dropped_attributes_count == 1`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where dropped_events_count == 1`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where dropped_links_count == 1`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where span_id == SpanID(0x0102030405060708)`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where parent_span_id == SpanID(0x0807060504030201)`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where trace_id == TraceID(0x0102030405060708090a0b0c0d0e0f10)`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where trace_state == "new"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `replace_pattern(attributes["http.method"], "get", "post")`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("http.method", "post")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("http.method", "post")
			},
		},
		{
			statement: `replace_all_patterns(attributes, "value", "get", "post")`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("http.method", "post")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("http.method", "post")
			},
		},
		{
			statement: `replace_all_patterns(attributes, "key", "http.url", "url")`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().Clear()
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("http.method", "get")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("http.path", "/health")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("url", "http://localhost/health")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("flags", "A|B|C")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("total.string", "123456789")

				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().Clear()
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("http.method", "get")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("http.path", "/health")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("url", "http://localhost/health")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("flags", "C|D")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("total.string", "345678")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where IsMatch(name, "operation[AC]")`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where attributes["doesnt exist"] == nil`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `delete_key(attributes, "http.url") where name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().Clear()
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("http.method", "get")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("http.path", "/health")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("total.string", "123456789")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("flags", "A|B|C")
			},
		},
		{
			statement: `delete_matching_keys(attributes, "http.*t.*") where name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().Clear()
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("http.url", "http://localhost/health")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("flags", "A|B|C")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("total.string", "123456789")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where kind == SPAN_KIND_INTERNAL`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(kind, SPAN_KIND_SERVER) where kind == 1`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).SetKind(2)
			},
		},
		{
			statement: `set(attributes["test"], Concat([attributes["http.method"], attributes["http.url"]], ": "))`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "get: http://localhost/health")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("test", "get: http://localhost/health")
			},
		},
		{
			statement: `set(attributes["test"], Concat([attributes["http.method"], ": ", attributes["http.url"]], ""))`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "get: http://localhost/health")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("test", "get: http://localhost/health")
			},
		},
		{
			statement: `set(attributes["test"], Concat([attributes["http.method"], attributes["http.url"]], ": ")) where name == Concat(["operation", "A"], "")`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "get: http://localhost/health")
			},
		},
		{
			statement: `set(attributes["kind"], Concat(["kind", ": ", kind], "")) where kind == 1`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("kind", "kind: 1")
			},
		},
		{
			statement: `set(attributes["test"], Split(attributes["flags"], "|"))`,
			want: func(td ptrace.Traces) {
				v1 := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutEmptySlice("test")
				v1.AppendEmpty().SetStr("A")
				v1.AppendEmpty().SetStr("B")
				v1.AppendEmpty().SetStr("C")
				v2 := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutEmptySlice("test")
				v2.AppendEmpty().SetStr("C")
				v2.AppendEmpty().SetStr("D")
			},
		},
		{
			statement: `set(attributes["test"], Split(attributes["flags"], "|")) where name == "operationA"`,
			want: func(td ptrace.Traces) {
				v1 := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutEmptySlice("test")
				v1.AppendEmpty().SetStr("A")
				v1.AppendEmpty().SetStr("B")
				v1.AppendEmpty().SetStr("C")
			},
		},
		{
			statement: `set(attributes["test"], Split(attributes["not_exist"], "|"))`,
			want:      func(_ ptrace.Traces) {},
		},
		{
			statement: `set(attributes["test"], Substring(attributes["total.string"], 3, 3))`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "456")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("test", "678")
			},
		},
		{
			statement: `set(attributes["test"], Substring(attributes["total.string"], 3, 3)) where name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "456")
			},
		},
		{
			statement: `set(attributes["test"], Substring(attributes["not_exist"], 3, 3))`,
			want:      func(_ ptrace.Traces) {},
		},
		{
			statement: `set(attributes["test"], ["A", "B", "C"]) where name == "operationA"`,
			want: func(td ptrace.Traces) {
				v1 := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutEmptySlice("test")
				v1.AppendEmpty().SetStr("A")
				v1.AppendEmpty().SetStr("B")
				v1.AppendEmpty().SetStr("C")
			},
		},
		{
			statement: `set(attributes["entrypoint"], name) where parent_span_id == SpanID(0x0000000000000000)`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("entrypoint", "operationB")
			},
		},
		{
			statement: `set(attributes["entrypoint-root"], name) where IsRootSpan()`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("entrypoint-root", "operationB")
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase(name, "lower")) where name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "operationa")
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase(name, "upper")) where name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "OPERATIONA")
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase(name, "snake")) where name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "operation_a")
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase(name, "camel")) where name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "OperationA")
			},
		},
		{
			statement: `merge_maps(attributes, ParseJSON("{\"json_test\":\"pass\"}"), "insert") where name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("json_test", "pass")
			},
		},
		{
			statement: `limit(attributes, 0, []) where name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().RemoveIf(func(_ string, _ pcommon.Value) bool { return true })
			},
		},
		{
			statement: `set(attributes["test"], Log(1)) where name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutDouble("test", 0.0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructTraces()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "span", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessTraces(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructTraces()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessTraces_InferredTraceContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td ptrace.Traces)
	}{
		{
			statement: `set(span.attributes["test"], "pass") where span.name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(span.attributes["test"], "pass") where resource.attributes["host.name"] == "localhost"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `keep_keys(span.attributes, ["http.method"]) where span.name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().Clear()
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("http.method", "get")
			},
		},
		{
			statement: `set(span.status.code, 1) where span.attributes["http.path"] == "/health"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Status().SetCode(ptrace.StatusCodeOk)
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Status().SetCode(ptrace.StatusCodeOk)
			},
		},
		{
			statement: `set(span.attributes["test"], "pass") where span.dropped_attributes_count == 1`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(span.attributes["test"], "pass") where span.dropped_events_count == 1`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(span.attributes["test"], "pass") where span.dropped_links_count == 1`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(span.attributes["test"], "pass") where span.span_id == SpanID(0x0102030405060708)`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(span.attributes["test"], "pass") where span.parent_span_id == SpanID(0x0807060504030201)`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(span.attributes["test"], "pass") where span.trace_id == TraceID(0x0102030405060708090a0b0c0d0e0f10)`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(span.attributes["test"], "pass") where span.trace_state == "new"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `replace_pattern(span.attributes["http.method"], "get", "post")`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("http.method", "post")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("http.method", "post")
			},
		},
		{
			statement: `replace_all_patterns(span.attributes, "value", "get", "post")`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("http.method", "post")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("http.method", "post")
			},
		},
		{
			statement: `replace_all_patterns(span.attributes, "key", "http.url", "url")`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().Clear()
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("http.method", "get")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("http.path", "/health")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("url", "http://localhost/health")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("flags", "A|B|C")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("total.string", "123456789")

				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().Clear()
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("http.method", "get")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("http.path", "/health")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("url", "http://localhost/health")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("flags", "C|D")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("total.string", "345678")
			},
		},
		{
			statement: `set(span.attributes["test"], "pass") where IsMatch(span.name, "operation[AC]")`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(span.attributes["test"], "pass") where span.attributes["doesnt exist"] == nil`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `delete_key(span.attributes, "http.url") where span.name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().Clear()
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("http.method", "get")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("http.path", "/health")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("total.string", "123456789")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("flags", "A|B|C")
			},
		},
		{
			statement: `delete_matching_keys(span.attributes, "http.*t.*") where span.name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().Clear()
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("http.url", "http://localhost/health")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("flags", "A|B|C")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("total.string", "123456789")
			},
		},
		{
			statement: `set(span.attributes["test"], "pass") where span.kind == SPAN_KIND_INTERNAL`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(span.kind, SPAN_KIND_SERVER) where span.kind == 1`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).SetKind(2)
			},
		},
		{
			statement: `set(span.attributes["test"], Concat([span.attributes["http.method"], span.attributes["http.url"]], ": "))`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "get: http://localhost/health")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("test", "get: http://localhost/health")
			},
		},
		{
			statement: `set(span.attributes["test"], Concat([span.attributes["http.method"], ": ", span.attributes["http.url"]], ""))`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "get: http://localhost/health")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("test", "get: http://localhost/health")
			},
		},
		{
			statement: `set(span.attributes["test"], Concat([span.attributes["http.method"], span.attributes["http.url"]], ": ")) where span.name == Concat(["operation", "A"], "")`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "get: http://localhost/health")
			},
		},
		{
			statement: `set(span.attributes["kind"], Concat(["kind", ": ", span.kind], "")) where span.kind == 1`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("kind", "kind: 1")
			},
		},
		{
			statement: `set(span.attributes["test"], Split(span.attributes["flags"], "|"))`,
			want: func(td ptrace.Traces) {
				v1 := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutEmptySlice("test")
				v1.AppendEmpty().SetStr("A")
				v1.AppendEmpty().SetStr("B")
				v1.AppendEmpty().SetStr("C")
				v2 := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutEmptySlice("test")
				v2.AppendEmpty().SetStr("C")
				v2.AppendEmpty().SetStr("D")
			},
		},
		{
			statement: `set(span.attributes["test"], Split(span.attributes["flags"], "|")) where span.name == "operationA"`,
			want: func(td ptrace.Traces) {
				v1 := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutEmptySlice("test")
				v1.AppendEmpty().SetStr("A")
				v1.AppendEmpty().SetStr("B")
				v1.AppendEmpty().SetStr("C")
			},
		},
		{
			statement: `set(span.attributes["test"], Split(span.attributes["not_exist"], "|"))`,
			want:      func(_ ptrace.Traces) {},
		},
		{
			statement: `set(span.attributes["test"], Substring(span.attributes["total.string"], 3, 3))`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "456")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("test", "678")
			},
		},
		{
			statement: `set(span.attributes["test"], Substring(span.attributes["total.string"], 3, 3)) where span.name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "456")
			},
		},
		{
			statement: `set(span.attributes["test"], Substring(span.attributes["not_exist"], 3, 3))`,
			want:      func(_ ptrace.Traces) {},
		},
		{
			statement: `set(span.attributes["test"], ["A", "B", "C"]) where span.name == "operationA"`,
			want: func(td ptrace.Traces) {
				v1 := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutEmptySlice("test")
				v1.AppendEmpty().SetStr("A")
				v1.AppendEmpty().SetStr("B")
				v1.AppendEmpty().SetStr("C")
			},
		},
		{
			statement: `set(span.attributes["entrypoint"], span.name) where span.parent_span_id == SpanID(0x0000000000000000)`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("entrypoint", "operationB")
			},
		},
		{
			statement: `set(span.attributes["entrypoint-root"], span.name) where IsRootSpan()`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("entrypoint-root", "operationB")
			},
		},
		{
			statement: `set(span.attributes["test"], ConvertCase(span.name, "lower")) where span.name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "operationa")
			},
		},
		{
			statement: `set(span.attributes["test"], ConvertCase(span.name, "upper")) where span.name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "OPERATIONA")
			},
		},
		{
			statement: `set(span.attributes["test"], ConvertCase(span.name, "snake")) where span.name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "operation_a")
			},
		},
		{
			statement: `set(span.attributes["test"], ConvertCase(span.name, "camel")) where span.name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "OperationA")
			},
		},
		{
			statement: `merge_maps(span.attributes, ParseJSON("{\"json_test\":\"pass\"}"), "insert") where span.name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("json_test", "pass")
			},
		},
		{
			statement: `limit(span.attributes, 0, []) where span.name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().RemoveIf(func(_ string, _ pcommon.Value) bool { return true })
			},
		},
		{
			statement: `set(span.attributes["test"], Log(1)) where span.name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutDouble("test", 0.0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructTraces()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessTraces(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructTraces()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessTraces_SpanEventContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td ptrace.Traces)
	}{
		{
			statement: `set(attributes["test"], "pass") where name == "eventA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Events().At(0).Attributes().PutStr("test", "pass")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructTraces()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "spanevent", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessTraces(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructTraces()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessTraces_InferredSpanEventContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td ptrace.Traces)
	}{
		{
			statement: `set(spanevent.attributes["test"], "pass") where spanevent.name == "eventA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Events().At(0).Attributes().PutStr("test", "pass")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructTraces()
			processor, err := NewProcessor([]common.ContextStatements{{Context: "", Statements: []string{tt.statement}}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessTraces(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructTraces()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessTraces_MixContext(t *testing.T) {
	tests := []struct {
		name              string
		contextStatements []common.ContextStatements
		want              func(td ptrace.Traces)
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
					Context: "span",
					Statements: []string{
						`set(attributes["test"], "pass") where resource.attributes["test"] == "pass"`,
					},
				},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).Resource().Attributes().PutStr("test", "pass")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("test", "pass")
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
					Context: "span",
					Statements: []string{
						`set(attributes["test"], "pass") where instrumentation_scope.attributes["test"] == "pass"`,
					},
				},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes().PutStr("test", "pass")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "order matters",
			contextStatements: []common.ContextStatements{
				{
					Context: "span",
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
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes().PutStr("test", "pass")
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
					Context: "span",
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
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes().PutStr("test", "fail")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("test", "pass")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := constructTraces()
			processor, err := NewProcessor(tt.contextStatements, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessTraces(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructTraces()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessTraces_ErrorMode(t *testing.T) {
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
			context: "span",
		},
		{
			context: "spanevent",
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.context), func(t *testing.T) {
			td := constructTraces()
			processor, err := NewProcessor([]common.ContextStatements{{Context: tt.context, Statements: []string{`set(attributes["test"], ParseJSON(1))`}}}, ottl.PropagateError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessTraces(context.Background(), td)
			assert.Error(t, err)
		})
	}
}

func Test_ProcessTraces_StatementsErrorMode(t *testing.T) {
	tests := []struct {
		name          string
		errorMode     ottl.ErrorMode
		statements    []common.ContextStatements
		want          func(td ptrace.Traces)
		wantErrorWith string
	}{
		{
			name:      "span: statements group with error mode",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(span.attributes["test"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(span.attributes["test"], "pass") where span.name == "operationA" `}},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			name:      "span: statements group error mode does not affect default",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(span.attributes["test"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(span.attributes["test"], ParseJSON(true))`}},
			},
			wantErrorWith: "expected string but got bool",
		},
		{
			name:      "spanevent: statements group with error mode",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(spanevent.attributes["test"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(spanevent.attributes["test"], "pass") where spanevent.name == "eventA" `}},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Events().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			name:      "spanevent: statements group error mode does not affect default",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(spanevent.attributes["test"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(spanevent.attributes["test"], ParseJSON(true))`}},
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
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).Resource().Attributes().PutStr("test", "pass")
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
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			name:      "scope: statements group error mode does not affect default",
			errorMode: ottl.PropagateError,
			statements: []common.ContextStatements{
				{Statements: []string{`set(scope.attributes["pass"], ParseJSON(1))`}, ErrorMode: ottl.IgnoreError},
				{Statements: []string{`set(scope.attributes["pass"], ParseJSON(true))`}},
			},
			wantErrorWith: `expected string but got bool`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := constructTraces()
			processor, err := NewProcessor(tt.statements, tt.errorMode, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)
			_, err = processor.ProcessTraces(context.Background(), td)
			if tt.wantErrorWith != "" {
				if err == nil {
					t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
				}
				assert.Contains(t, err.Error(), tt.wantErrorWith)
				return
			}
			assert.NoError(t, err)
			exTd := constructTraces()
			tt.want(exTd)
			assert.Equal(t, exTd, td)
		})
	}
}

func Test_ProcessTraces_CacheAccess(t *testing.T) {
	tests := []struct {
		name       string
		statements []common.ContextStatements
		want       func(td ptrace.Traces)
	}{
		{
			name: "resource:resource.cache",
			statements: []common.ContextStatements{
				{Statements: []string{
					`set(resource.cache["test"], "pass")`,
					`set(resource.attributes["test"], resource.cache["test"])`,
				}},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).Resource().Attributes().PutStr("test", "pass")
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
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).Resource().Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "scope:scope.cache",
			statements: []common.ContextStatements{
				{Statements: []string{
					`set(scope.cache["test"], "pass")`,
					`set(scope.attributes["test"], scope.cache["test"])`,
				}},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes().PutStr("test", "pass")
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
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "span:span.cache",
			statements: []common.ContextStatements{
				{Statements: []string{
					`set(span.cache["test"], "pass")`,
					`set(span.attributes["test"], span.cache["test"]) where span.name == "operationA"`,
				}},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "span:span.cache multiple entries",
			statements: []common.ContextStatements{
				{Statements: []string{
					`set(span.cache["test"], Concat([span.name, "cache"], "-"))`,
					`set(span.attributes["test"], span.cache["test"])`,
				}},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "operationA-cache")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("test", "operationB-cache")
			},
		},
		{
			name: "span:cache",
			statements: []common.ContextStatements{{
				Context: common.Span,
				Statements: []string{
					`set(cache["test"], "pass")`,
					`set(attributes["test"], cache["test"]) where name == "operationA"`,
				},
			}},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "spanevent:spanevent.cache",
			statements: []common.ContextStatements{
				{Statements: []string{
					`set(spanevent.cache["test"], "pass")`,
					`set(spanevent.attributes["test"], spanevent.cache["test"]) where spanevent.name == "eventA"`,
				}},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Events().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			name: "spanevent:spanevent.cache multiple entries",
			statements: []common.ContextStatements{
				{Statements: []string{
					`set(spanevent.cache["test"], Concat([spanevent.name, "cache"], "-"))`,
					`set(spanevent.attributes["test"], spanevent.cache["test"]) where span.name == "operationB"`,
				}},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Events().At(0).Attributes().PutStr("test", "eventB-cache")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Events().At(1).Attributes().PutStr("test", "eventB2-cache")
			},
		},
		{
			name: "spanevent:cache",
			statements: []common.ContextStatements{{
				Context: common.SpanEvent,
				Statements: []string{
					`set(cache["test"], "pass")`,
					`set(attributes["test"], cache["test"]) where name == "eventA"`,
				},
			}},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Events().At(0).Attributes().PutStr("test", "pass")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := constructTraces()
			processor, err := NewProcessor(tt.statements, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessTraces(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructTraces()
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

	contextsTests := map[string][]testCase{"span": nil, "spanevent": nil, "resource": nil, "scope": nil}
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
						Statements: []string{`set(cache["test"], "pass")`},
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
					_, err := NewProcessor(tt.statements, ottl.PropagateError, componenttest.NewNopTelemetrySettings())
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

func BenchmarkTwoSpans(b *testing.B) {
	tests := []struct {
		name       string
		statements []string
	}{
		{
			name:       "no processing",
			statements: []string{},
		},
		{
			name:       "set attribute",
			statements: []string{`set(attributes["test"], "pass") where name == "operationA"`},
		},
		{
			name:       "keep_keys attribute",
			statements: []string{`keep_keys(attributes, ["http.method"]) where name == "operationA"`},
		},
		{
			name:       "no match",
			statements: []string{`keep_keys(attributes, ["http.method"]) where name == "unknownOperation"`},
		},
		{
			name:       "inner field",
			statements: []string{`set(status.code, 1) where attributes["http.path"] == "/health"`},
		},
		{
			name: "inner field both spans",
			statements: []string{
				`set(status.code, 1) where name == "operationA"`,
				`set(status.code, 2) where name == "operationB"`,
			},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			processor, err := NewProcessor([]common.ContextStatements{{Context: "span", Statements: tt.statements}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(b, err)
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				td := constructTraces()
				_, err = processor.ProcessTraces(context.Background(), td)
				assert.NoError(b, err)
			}
		})
	}
}

func BenchmarkHundredSpans(b *testing.B) {
	tests := []struct {
		name       string
		statements []string
	}{
		{
			name:       "no processing",
			statements: []string{},
		},
		{
			name: "set status code",
			statements: []string{
				`set(status.code, 1) where name == "operationA"`,
				`set(status.code, 2) where name == "operationB"`,
			},
		},
		{
			name: "hundred statements",
			statements: func() []string {
				var statements []string
				statements = append(statements, `set(status.code, 1) where name == "operationA"`)
				for i := 0; i < 99; i++ {
					statements = append(statements, `keep_keys(attributes, ["http.method"]) where name == "unknownOperation"`)
				}
				return statements
			}(),
		},
	}
	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			processor, err := NewProcessor([]common.ContextStatements{{Context: "span", Statements: tt.statements}}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings())
			assert.NoError(b, err)
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				td := constructTracesNum(100)
				_, err = processor.ProcessTraces(context.Background(), td)
				assert.NoError(b, err)
			}
		})
	}
}

func constructTraces() ptrace.Traces {
	td := ptrace.NewTraces()
	rs0 := td.ResourceSpans().AppendEmpty()
	rs0.SetSchemaUrl("test_schema_url")
	rs0.Resource().Attributes().PutStr("host.name", "localhost")
	rs0ils0 := rs0.ScopeSpans().AppendEmpty()
	rs0ils0.SetSchemaUrl("test_schema_url")
	rs0ils0.Scope().SetName("scope")
	fillSpanOne(rs0ils0.Spans().AppendEmpty())
	fillSpanTwo(rs0ils0.Spans().AppendEmpty())
	return td
}

func constructTracesNum(num int) ptrace.Traces {
	td := ptrace.NewTraces()
	rs0 := td.ResourceSpans().AppendEmpty()
	rs0ils0 := rs0.ScopeSpans().AppendEmpty()
	for i := 0; i < num; i++ {
		fillSpanOne(rs0ils0.Spans().AppendEmpty())
	}
	return td
}

func fillSpanOne(span ptrace.Span) {
	span.SetName("operationA")
	span.SetSpanID(spanID)
	span.SetParentSpanID(spanID2)
	span.SetTraceID(traceID)
	span.SetStartTimestamp(TestSpanStartTimestamp)
	span.SetEndTimestamp(TestSpanEndTimestamp)
	span.SetDroppedAttributesCount(1)
	span.SetDroppedLinksCount(1)
	span.SetDroppedEventsCount(1)
	span.SetKind(1)
	span.TraceState().FromRaw("new")
	span.Attributes().PutStr("http.method", "get")
	span.Attributes().PutStr("http.path", "/health")
	span.Attributes().PutStr("http.url", "http://localhost/health")
	span.Attributes().PutStr("flags", "A|B|C")
	span.Attributes().PutStr("total.string", "123456789")
	status := span.Status()
	status.SetCode(ptrace.StatusCodeError)
	status.SetMessage("status-cancelled")
	event := span.Events().AppendEmpty()
	event.SetName("eventA")
}

func fillSpanTwo(span ptrace.Span) {
	span.SetName("operationB")
	span.SetStartTimestamp(TestSpanStartTimestamp)
	span.SetEndTimestamp(TestSpanEndTimestamp)
	span.Attributes().PutStr("http.method", "get")
	span.Attributes().PutStr("http.path", "/health")
	span.Attributes().PutStr("http.url", "http://localhost/health")
	span.Attributes().PutStr("flags", "C|D")
	span.Attributes().PutStr("total.string", "345678")
	link0 := span.Links().AppendEmpty()
	link0.SetDroppedAttributesCount(4)
	link1 := span.Links().AppendEmpty()
	link1.SetDroppedAttributesCount(4)
	span.SetDroppedLinksCount(3)
	status := span.Status()
	status.SetCode(ptrace.StatusCodeError)
	status.SetMessage("status-cancelled")
	event := span.Events().AppendEmpty()
	event.SetName("eventB")
	eventB2 := span.Events().AppendEmpty()
	eventB2.SetName("eventB2")
}
