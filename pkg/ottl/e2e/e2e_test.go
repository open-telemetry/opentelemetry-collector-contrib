// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

var (
	TestLogTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	TestLogTimestamp = pcommon.NewTimestampFromTime(TestLogTime)

	TestObservedTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	TestObservedTimestamp = pcommon.NewTimestampFromTime(TestObservedTime)

	traceID = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID  = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
)

func Test_e2e_editors(t *testing.T) {
	tests := []struct {
		statement string
		want      func(tCtx ottllog.TransformContext)
	}{
		{
			statement: `delete_key(attributes, "http.method")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().Remove("http.method")
			},
		},
		{
			statement: `delete_matching_keys(attributes, "^http")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().Remove("http.method")
				tCtx.GetLogRecord().Attributes().Remove("http.path")
				tCtx.GetLogRecord().Attributes().Remove("http.url")
			},
		},
		{
			statement: `keep_matching_keys(attributes, "^http")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().Remove("flags")
				tCtx.GetLogRecord().Attributes().Remove("total.string")
				tCtx.GetLogRecord().Attributes().Remove("foo")
				tCtx.GetLogRecord().Attributes().Remove("things")
			},
		},
		{
			statement: `flatten(attributes)`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().Remove("foo")
				tCtx.GetLogRecord().Attributes().PutStr("foo.bar", "pass")
				tCtx.GetLogRecord().Attributes().PutStr("foo.flags", "pass")
				tCtx.GetLogRecord().Attributes().PutStr("foo.slice.0", "val")
				tCtx.GetLogRecord().Attributes().PutStr("foo.nested.test", "pass")

				tCtx.GetLogRecord().Attributes().Remove("things")
				tCtx.GetLogRecord().Attributes().PutStr("things.0.name", "foo")
				tCtx.GetLogRecord().Attributes().PutInt("things.0.value", 2)
				tCtx.GetLogRecord().Attributes().PutStr("things.1.name", "bar")
				tCtx.GetLogRecord().Attributes().PutInt("things.1.value", 5)
			},
		},
		{
			statement: `flatten(attributes, "test")`,
			want: func(tCtx ottllog.TransformContext) {
				m := pcommon.NewMap()
				m.PutStr("test.http.method", "get")
				m.PutStr("test.http.path", "/health")
				m.PutStr("test.http.url", "http://localhost/health")
				m.PutStr("test.flags", "A|B|C")
				m.PutStr("test.total.string", "123456789")

				m.PutStr("test.foo.bar", "pass")
				m.PutStr("test.foo.flags", "pass")
				m.PutStr("test.foo.bar", "pass")
				m.PutStr("test.foo.flags", "pass")
				m.PutStr("test.foo.slice.0", "val")
				m.PutStr("test.foo.nested.test", "pass")

				m.PutStr("test.things.0.name", "foo")
				m.PutInt("test.things.0.value", 2)
				m.PutStr("test.things.1.name", "bar")
				m.PutInt("test.things.1.value", 5)
				m.CopyTo(tCtx.GetLogRecord().Attributes())
			},
		},
		{
			statement: `flatten(attributes, depth=1)`,
			want: func(tCtx ottllog.TransformContext) {
				m := pcommon.NewMap()
				m.PutStr("http.method", "get")
				m.PutStr("http.path", "/health")
				m.PutStr("http.url", "http://localhost/health")
				m.PutStr("flags", "A|B|C")
				m.PutStr("total.string", "123456789")
				m.PutStr("foo.bar", "pass")
				m.PutStr("foo.flags", "pass")
				m.PutStr("foo.bar", "pass")
				m.PutStr("foo.flags", "pass")
				m.PutEmptySlice("foo.slice").AppendEmpty().SetStr("val")

				m1 := m.PutEmptyMap("things.0")
				m1.PutStr("name", "foo")
				m1.PutInt("value", 2)

				m2 := m.PutEmptyMap("things.1")
				m2.PutStr("name", "bar")
				m2.PutInt("value", 5)

				m3 := m.PutEmptyMap("foo.nested")
				m3.PutStr("test", "pass")
				m.CopyTo(tCtx.GetLogRecord().Attributes())
			},
		},
		{
			statement: `keep_keys(attributes, ["flags", "total.string"])`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().Remove("http.method")
				tCtx.GetLogRecord().Attributes().Remove("http.path")
				tCtx.GetLogRecord().Attributes().Remove("http.url")
				tCtx.GetLogRecord().Attributes().Remove("foo")
				tCtx.GetLogRecord().Attributes().Remove("things")
			},
		},
		{
			statement: `limit(attributes, 100, [])`,
			want:      func(_ ottllog.TransformContext) {},
		},
		{
			statement: `limit(attributes, 1, ["total.string"])`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().Remove("http.method")
				tCtx.GetLogRecord().Attributes().Remove("http.path")
				tCtx.GetLogRecord().Attributes().Remove("http.url")
				tCtx.GetLogRecord().Attributes().Remove("flags")
				tCtx.GetLogRecord().Attributes().Remove("foo")
				tCtx.GetLogRecord().Attributes().Remove("things")
			},
		},
		{
			statement: `merge_maps(attributes, attributes["foo"], "insert")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("bar", "pass")
				s := tCtx.GetLogRecord().Attributes().PutEmptySlice("slice")
				v := s.AppendEmpty()
				v.SetStr("val")
				m2 := tCtx.GetLogRecord().Attributes().PutEmptyMap("nested")
				m2.PutStr("test", "pass")
			},
		},
		{
			statement: `merge_maps(attributes, attributes["foo"], "update")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("flags", "pass")
			},
		},
		{
			statement: `merge_maps(attributes, attributes["foo"], "upsert")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("bar", "pass")
				tCtx.GetLogRecord().Attributes().PutStr("flags", "pass")
				s := tCtx.GetLogRecord().Attributes().PutEmptySlice("slice")
				v := s.AppendEmpty()
				v.SetStr("val")
				m2 := tCtx.GetLogRecord().Attributes().PutEmptyMap("nested")
				m2.PutStr("test", "pass")
			},
		},
		{
			statement: `replace_all_matches(attributes, "*/*", "test")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("http.path", "test")
				tCtx.GetLogRecord().Attributes().PutStr("http.url", "test")
			},
		},
		{
			statement: `replace_all_patterns(attributes, "key", "^http", "test")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().Remove("http.method")
				tCtx.GetLogRecord().Attributes().Remove("http.path")
				tCtx.GetLogRecord().Attributes().Remove("http.url")
				tCtx.GetLogRecord().Attributes().PutStr("test.method", "get")
				tCtx.GetLogRecord().Attributes().PutStr("test.path", "/health")
				tCtx.GetLogRecord().Attributes().PutStr("test.url", "http://localhost/health")
			},
		},
		{
			statement: `replace_all_patterns(attributes, "value", "/", "@")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("http.path", "@health")
				tCtx.GetLogRecord().Attributes().PutStr("http.url", "http:@@localhost@health")
			},
		},
		{
			statement: `replace_match(attributes["http.path"], "*/*", "test")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("http.path", "test")
			},
		},
		{
			statement: `replace_pattern(attributes["http.path"], "/", "@")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("http.path", "@health")
			},
		},
		{
			statement: `replace_pattern(attributes["http.path"], "/", "@", SHA256)`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("http.path", "c3641f8544d7c02f3580b07c0f9887f0c6a27ff5ab1d4a3e29caf197cfc299aehealth")
			},
		},
		{
			statement: `set(attributes["test"], "pass")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], nil)`,
			want:      func(_ ottllog.TransformContext) {},
		},
		{
			statement: `set(attributes["test"], attributes["unknown"])`,
			want:      func(_ ottllog.TransformContext) {},
		},
		{
			statement: `set(attributes["foo"]["test"], "pass")`,
			want: func(tCtx ottllog.TransformContext) {
				v, _ := tCtx.GetLogRecord().Attributes().Get("foo")
				v.Map().PutStr("test", "pass")
			},
		},
		{
			statement: `truncate_all(attributes, 100)`,
			want:      func(_ ottllog.TransformContext) {},
		},
		{
			statement: `truncate_all(attributes, 1)`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("http.method", "g")
				tCtx.GetLogRecord().Attributes().PutStr("http.path", "/")
				tCtx.GetLogRecord().Attributes().PutStr("http.url", "h")
				tCtx.GetLogRecord().Attributes().PutStr("flags", "A")
				tCtx.GetLogRecord().Attributes().PutStr("total.string", "1")
			},
		},
		{
			statement: `append(attributes["foo"]["slice"], "sample_value")`,
			want: func(tCtx ottllog.TransformContext) {
				v, _ := tCtx.GetLogRecord().Attributes().Get("foo")
				sv, _ := v.Map().Get("slice")
				s := sv.Slice()
				s.AppendEmpty().SetStr("sample_value")
			},
		},
		{
			statement: `append(attributes["foo"]["flags"], "sample_value")`,
			want: func(tCtx ottllog.TransformContext) {
				v, _ := tCtx.GetLogRecord().Attributes().Get("foo")
				s := v.Map().PutEmptySlice("flags")
				s.AppendEmpty().SetStr("pass")
				s.AppendEmpty().SetStr("sample_value")
			},
		},
		{
			statement: `append(attributes["foo"]["slice"], values=[5,6])`,
			want: func(tCtx ottllog.TransformContext) {
				v, _ := tCtx.GetLogRecord().Attributes().Get("foo")
				sv, _ := v.Map().Get("slice")
				s := sv.Slice()
				s.AppendEmpty().SetInt(5)
				s.AppendEmpty().SetInt(6)
			},
		},
		{
			statement: `append(attributes["foo"]["new_slice"], values=[5,6])`,
			want: func(tCtx ottllog.TransformContext) {
				v, _ := tCtx.GetLogRecord().Attributes().Get("foo")
				s := v.Map().PutEmptySlice("new_slice")
				s.AppendEmpty().SetInt(5)
				s.AppendEmpty().SetInt(6)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			settings := componenttest.NewNopTelemetrySettings()
			logParser, err := ottllog.NewParser(ottlfuncs.StandardFuncs[ottllog.TransformContext](), settings)
			assert.NoError(t, err)
			logStatements, err := logParser.ParseStatement(tt.statement)
			assert.NoError(t, err)

			tCtx := constructLogTransformContext()
			_, _, _ = logStatements.Execute(context.Background(), tCtx)

			exTCtx := constructLogTransformContext()
			tt.want(exTCtx)

			assert.NoError(t, plogtest.CompareResourceLogs(newResourceLogs(exTCtx), newResourceLogs(tCtx)))
		})
	}
}

func Test_e2e_converters(t *testing.T) {
	tests := []struct {
		statement string
		want      func(tCtx ottllog.TransformContext)
	}{
		{
			statement: `set(attributes[attributes["flags"]], attributes["total.string"])`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("A|B|C", "123456789")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			settings := componenttest.NewNopTelemetrySettings()
			logParser, err := ottllog.NewParser(ottlfuncs.StandardFuncs[ottllog.TransformContext](), settings)
			assert.NoError(t, err)
			logStatements, err := logParser.ParseStatement(tt.statement)
			require.Nil(t, err)

			tCtx := constructLogTransformContext()
			_, _, _ = logStatements.Execute(context.Background(), tCtx)

			exTCtx := constructLogTransformContext()
			tt.want(exTCtx)

			assert.NoError(t, plogtest.CompareResourceLogs(newResourceLogs(exTCtx), newResourceLogs(tCtx)))
		})
	}
}

func Test_e2e_ottl_features(t *testing.T) {
	tests := []struct {
		name      string
		statement string
		want      func(tCtx ottllog.TransformContext)
	}{
		{
			name:      "where clause",
			statement: `set(attributes["test"], "pass") where body == "operationB"`,
			want:      func(_ ottllog.TransformContext) {},
		},
		{
			name:      "reach upwards",
			statement: `set(attributes["test"], "pass") where resource.attributes["host.name"] == "localhost"`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			name:      "Using enums",
			statement: `set(severity_number, SEVERITY_NUMBER_TRACE2) where severity_number == SEVERITY_NUMBER_TRACE`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().SetSeverityNumber(2)
			},
		},
		{
			name:      "Using hex",
			statement: `set(attributes["test"], "pass") where trace_id == TraceID(0x0102030405060708090a0b0c0d0e0f10)`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			name:      "where clause without comparator",
			statement: `set(attributes["test"], "pass") where IsMatch(body, "operation[AC]")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			name:      "where clause with Converter return value",
			statement: `set(attributes["test"], "pass") where body == Concat(["operation", "A"], "")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			name:      "composing functions",
			statement: `merge_maps(attributes, ParseJSON("{\"json_test\":\"pass\"}"), "insert") where body == "operationA"`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("json_test", "pass")
			},
		},
		{
			name:      "complex indexing found",
			statement: `set(attributes["test"], attributes["foo"]["bar"])`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			name:      "complex indexing not found",
			statement: `set(attributes["test"], attributes["metadata"]["uid"])`,
			want:      func(_ ottllog.TransformContext) {},
		},
		{
			name:      "map value",
			statement: `set(body, {"_raw": body, "test": {"result": attributes["foo"]["bar"], "time": UnixNano(time)}})`,
			want: func(tCtx ottllog.TransformContext) {
				originalBody := tCtx.GetLogRecord().Body().AsString()
				mapValue := tCtx.GetLogRecord().Body().SetEmptyMap()
				mapValue.PutStr("_raw", originalBody)
				mv1 := mapValue.PutEmptyMap("test")
				mv1.PutStr("result", "pass")
				mv1.PutInt("time", 1581452772000000321)
			},
		},
		{
			name:      "map value as input to function",
			statement: `set(attributes["isMap"], IsMap({"foo": {"bar": "baz", "test": "pass"}}))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutBool("isMap", true)
			},
		},
		{
			name:      "extract value from Split function result slice of type []string",
			statement: `set(attributes["my.environment.2"], Split(resource.attributes["host.name"],"h")[1])`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("my.environment.2", "ost")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			settings := componenttest.NewNopTelemetrySettings()
			logParser, err := ottllog.NewParser(ottlfuncs.StandardFuncs[ottllog.TransformContext](), settings)
			assert.NoError(t, err)
			logStatements, err := logParser.ParseStatement(tt.statement)
			assert.NoError(t, err)

			tCtx := constructLogTransformContext()
			_, _, _ = logStatements.Execute(context.Background(), tCtx)

			exTCtx := constructLogTransformContext()
			tt.want(exTCtx)

			assert.NoError(t, plogtest.CompareResourceLogs(newResourceLogs(exTCtx), newResourceLogs(tCtx)))
		})
	}
}

func Test_ProcessTraces_TraceContext(t *testing.T) {
	tests := []struct {
		statement string
		want      func(_ ottlspan.TransformContext)
	}{
		{
			statement: `set(attributes["entrypoint-root"], name) where IsRootSpan()`,
			want: func(tCtx ottlspan.TransformContext) {
				tCtx.GetSpan().Attributes().PutStr("entrypoint-root", "operationB")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			settings := componenttest.NewNopTelemetrySettings()
			funcs := ottlfuncs.StandardFuncs[ottlspan.TransformContext]()
			isRootSpanFactory := ottlfuncs.NewIsRootSpanFactory()
			funcs[isRootSpanFactory.Name()] = isRootSpanFactory
			spanParser, err := ottlspan.NewParser(funcs, settings)
			assert.NoError(t, err)
			spanStatements, err := spanParser.ParseStatement(tt.statement)
			assert.NoError(t, err)

			tCtx := constructSpanTransformContext()
			_, _, _ = spanStatements.Execute(context.Background(), tCtx)

			exTCtx := constructSpanTransformContext()
			tt.want(exTCtx)

			assert.NoError(t, ptracetest.CompareResourceSpans(newResourceSpans(exTCtx), newResourceSpans(tCtx)))
		})
	}
}

func constructLogTransformContext() ottllog.TransformContext {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("host.name", "localhost")

	scope := pcommon.NewInstrumentationScope()
	scope.SetName("scope")

	logRecord := plog.NewLogRecord()
	logRecord.Body().SetStr("operationA")
	logRecord.SetTimestamp(TestLogTimestamp)
	logRecord.SetObservedTimestamp(TestObservedTimestamp)
	logRecord.SetDroppedAttributesCount(1)
	logRecord.SetFlags(plog.DefaultLogRecordFlags.WithIsSampled(true))
	logRecord.SetSeverityNumber(1)
	logRecord.SetTraceID(traceID)
	logRecord.SetSpanID(spanID)
	logRecord.Attributes().PutStr("http.method", "get")
	logRecord.Attributes().PutStr("http.path", "/health")
	logRecord.Attributes().PutStr("http.url", "http://localhost/health")
	logRecord.Attributes().PutStr("flags", "A|B|C")
	logRecord.Attributes().PutStr("total.string", "123456789")
	m := logRecord.Attributes().PutEmptyMap("foo")
	m.PutStr("bar", "pass")
	m.PutStr("flags", "pass")
	s := m.PutEmptySlice("slice")
	v := s.AppendEmpty()
	v.SetStr("val")
	m2 := m.PutEmptyMap("nested")
	m2.PutStr("test", "pass")

	s2 := logRecord.Attributes().PutEmptySlice("things")
	thing1 := s2.AppendEmpty().SetEmptyMap()
	thing1.PutStr("name", "foo")
	thing1.PutInt("value", 2)

	thing2 := s2.AppendEmpty().SetEmptyMap()
	thing2.PutStr("name", "bar")
	thing2.PutInt("value", 5)

	return ottllog.NewTransformContext(logRecord, scope, resource, plog.NewScopeLogs(), plog.NewResourceLogs())
}

func constructSpanTransformContext() ottlspan.TransformContext {
	resource := pcommon.NewResource()

	scope := pcommon.NewInstrumentationScope()
	scope.SetName("scope")

	td := ptrace.NewSpan()
	fillSpanOne(td)

	return ottlspan.NewTransformContext(td, scope, resource, ptrace.NewScopeSpans(), ptrace.NewResourceSpans())
}

func newResourceLogs(tCtx ottllog.TransformContext) plog.ResourceLogs {
	rl := plog.NewResourceLogs()
	tCtx.GetResource().CopyTo(rl.Resource())
	sl := rl.ScopeLogs().AppendEmpty()
	tCtx.GetInstrumentationScope().CopyTo(sl.Scope())
	l := sl.LogRecords().AppendEmpty()
	tCtx.GetLogRecord().CopyTo(l)
	return rl
}

func newResourceSpans(tCtx ottlspan.TransformContext) ptrace.ResourceSpans {
	rl := ptrace.NewResourceSpans()
	tCtx.GetResource().CopyTo(rl.Resource())
	sl := rl.ScopeSpans().AppendEmpty()
	tCtx.GetInstrumentationScope().CopyTo(sl.Scope())
	l := sl.Spans().AppendEmpty()
	tCtx.GetSpan().CopyTo(l)
	return rl
}

func fillSpanOne(span ptrace.Span) {
	span.SetName("operationB")
	span.SetSpanID(spanID)
	span.SetTraceID(traceID)
}

func Benchmark_XML_Functions(b *testing.B) {
	testXML := `<Data><From><Test>1</Test><Test>2</Test></From><To></To></Data>`
	tCtxWithTestBody := func() ottllog.TransformContext {
		resource := pcommon.NewResource()
		scope := pcommon.NewInstrumentationScope()
		logRecord := plog.NewLogRecord()
		logRecord.Body().SetStr(testXML)
		return ottllog.NewTransformContext(logRecord, scope, resource, plog.NewScopeLogs(), plog.NewResourceLogs())
	}

	settings := componenttest.NewNopTelemetrySettings()
	logParser, err := ottllog.NewParser(ottlfuncs.StandardFuncs[ottllog.TransformContext](), settings)
	assert.NoError(b, err)

	// Use a round trip composition to ensure each iteration of the benchmark is the same.
	// GetXML(body, "/Data/From/Test") returns "<Test>1</Test><Test>2</Test>"
	// InsertXML(body, "/Data/To", GetXML(...)) adds the two Test elements to the To element
	// RemoveXML(InsertXML(...) "/Data/To/Test") removes the Test elements which were just added
	// set overwrites the body, but the result should be the same as the original body
	roundTrip := `set(body, RemoveXML(InsertXML(body, "/Data/To", GetXML(body, "/Data/From/Test")), "/Data/To/Test"))`
	logStatements, err := logParser.ParseStatement(roundTrip)
	assert.NoError(b, err)

	actualCtx := tCtxWithTestBody()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = logStatements.Execute(context.Background(), actualCtx)
	}

	// Ensure correctness
	assert.NoError(b, plogtest.CompareResourceLogs(newResourceLogs(tCtxWithTestBody()), newResourceLogs(actualCtx)))
}
