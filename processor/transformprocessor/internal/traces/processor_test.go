// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package traces

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
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

func TestProcess(t *testing.T) {
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

				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().Clear()
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("http.method", "get")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("http.path", "/health")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("url", "http://localhost/health")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Attributes().PutStr("flags", "C|D")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where IsMatch(name, "operation[AC]") == true`,
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
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("flags", "A|B|C")
			},
		},
		{
			statement: `delete_matching_keys(attributes, "http.*t.*") where name == "operationA"`,
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().Clear()
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("http.url", "http://localhost/health")
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("flags", "A|B|C")
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
			want:      func(td ptrace.Traces) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructTraces()
			processor, err := NewProcessor([]string{tt.statement}, Functions(), componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)

			_, err = processor.ProcessTraces(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructTraces()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
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
			processor, err := NewProcessor(tt.statements, Functions(), componenttest.NewNopTelemetrySettings())
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
			processor, err := NewProcessor(tt.statements, Functions(), componenttest.NewNopTelemetrySettings())
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
	rs0.Resource().Attributes().PutStr("host.name", "localhost")
	rs0ils0 := rs0.ScopeSpans().AppendEmpty()
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
	status := span.Status()
	status.SetCode(ptrace.StatusCodeError)
	status.SetMessage("status-cancelled")
}

func fillSpanTwo(span ptrace.Span) {
	span.SetName("operationB")
	span.SetStartTimestamp(TestSpanStartTimestamp)
	span.SetEndTimestamp(TestSpanEndTimestamp)
	span.Attributes().PutStr("http.method", "get")
	span.Attributes().PutStr("http.path", "/health")
	span.Attributes().PutStr("http.url", "http://localhost/health")
	span.Attributes().PutStr("flags", "C|D")
	link0 := span.Links().AppendEmpty()
	link0.SetDroppedAttributesCount(4)
	link1 := span.Links().AppendEmpty()
	link1.SetDroppedAttributesCount(4)
	span.SetDroppedLinksCount(3)
	status := span.Status()
	status.SetCode(ptrace.StatusCodeError)
	status.SetMessage("status-cancelled")
}
