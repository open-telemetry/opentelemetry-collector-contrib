// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
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
			statement: `keep_keys(attributes, ["flags", "total.string"])`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().Remove("http.method")
				tCtx.GetLogRecord().Attributes().Remove("http.path")
				tCtx.GetLogRecord().Attributes().Remove("http.url")
				tCtx.GetLogRecord().Attributes().Remove("foo")
			},
		},
		{
			statement: `limit(attributes, 100, [])`,
			want:      func(tCtx ottllog.TransformContext) {},
		},
		{
			statement: `limit(attributes, 1, ["total.string"])`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().Remove("http.method")
				tCtx.GetLogRecord().Attributes().Remove("http.path")
				tCtx.GetLogRecord().Attributes().Remove("http.url")
				tCtx.GetLogRecord().Attributes().Remove("flags")
				tCtx.GetLogRecord().Attributes().Remove("foo")
			},
		},
		{
			statement: `merge_maps(attributes, attributes["foo"], "insert")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("bar", "pass")
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
			statement: `set(attributes["test"], "pass")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], nil)`,
			want:      func(tCtx ottllog.TransformContext) {},
		},
		{
			statement: `set(attributes["test"], attributes["unknown"])`,
			want:      func(tCtx ottllog.TransformContext) {},
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
			want:      func(tCtx ottllog.TransformContext) {},
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
			statement: `set(attributes["test"], Concat(["A","B"], ":"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "A:B")
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase(attributes["http.method"], "upper"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "GET")
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase("PASS", "lower"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase("fooBar", "snake"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "foo_bar")
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase("foo_bar", "camel"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "FooBar")
			},
		},
		{
			statement: `set(attributes["test"], Double(1.0))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutDouble("test", 1.0)
			},
		},
		{
			statement: `set(attributes["test"], Double("1"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutDouble("test", 1.0)
			},
		},
		{
			statement: `set(attributes["test"], Double(true))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutDouble("test", 1.0)
			},
		},
		{
			statement: `set(attributes["test"], Double(1))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutDouble("test", 1.0)
			},
		},
		{
			statement: `set(attributes["test"], "pass") where Time("10", "%M") - Time("01", "%M") < Duration("10m")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], ExtractPatterns("aa123bb", "(?P<numbers>\\d+)"))`,
			want: func(tCtx ottllog.TransformContext) {
				m := tCtx.GetLogRecord().Attributes().PutEmptyMap("test")
				m.PutStr("numbers", "123")
			},
		},
		{
			statement: `set(attributes["test"], FNV("pass"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutInt("test", 266877920130663416)
			},
		},
		{
			statement: `set(attributes["test"], Hour(Time("12", "%H")))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutInt("test", 12)
			},
		},
		{
			statement: `set(attributes["test"], Hours(Duration("90m")))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutDouble("test", 1.5)
			},
		},
		{
			statement: `set(attributes["test"], Int(1.0))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutInt("test", 1)
			},
		},
		{
			statement: `set(attributes["test"], Int("1"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutInt("test", 1)
			},
		},
		{
			statement: `set(attributes["test"], Int(true))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutInt("test", 1)
			},
		},
		{
			statement: `set(attributes["test"], Int(1))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutInt("test", 1)
			},
		},
		{
			statement: `set(attributes["test"], "pass") where IsBool(false)`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where IsDouble(1.0)`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where IsMap(attributes["foo"])`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where IsMatch("aa123bb", "\\d{3}")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where IsString("")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], Len(attributes["foo"]))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutInt("test", 2)
			},
		},
		{
			statement: `set(attributes["test"], Log(1))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutDouble("test", 0)
			},
		},
		{
			statement: `set(attributes["test"], Microseconds(Duration("1ms")))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutInt("test", 1000)
			},
		},
		{
			statement: `set(attributes["test"], Milliseconds(Duration("1s")))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutInt("test", 1000)
			},
		},
		{
			statement: `set(attributes["test"], Minutes(Duration("1h")))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutDouble("test", 60)
			},
		},
		{
			statement: `set(attributes["test"], Nanoseconds(Duration("1ms")))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutInt("test", 1000000)
			},
		},
		{
			statement: `set(attributes["test"], "pass") where Now() - Now() < Duration("1h")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], ParseJSON("{\"id\":1}"))`,
			want: func(tCtx ottllog.TransformContext) {
				m := tCtx.GetLogRecord().Attributes().PutEmptyMap("test")
				m.PutDouble("id", 1)
			},
		},
		{
			statement: `set(attributes["test"], Seconds(Duration("1m")))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutDouble("test", 60)
			},
		},
		{
			statement: `set(attributes["test"], SHA1("pass"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "9d4e1e23bd5b727046a9e3b4b7db57bd8d6ee684")
			},
		},
		{
			statement: `set(attributes["test"], SHA256("pass"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "d74ff0ee8da3b9806b18c877dbf29bbde50b5bd8e4dad7a3a725000feb82e8f1")
			},
		},
		{
			statement: `set(span_id, SpanID(0x0000000000000000))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().SetSpanID(pcommon.NewSpanIDEmpty())
			},
		},
		{
			statement: `set(attributes["test"], Split(attributes["flags"], "|"))`,
			want: func(tCtx ottllog.TransformContext) {
				s := tCtx.GetLogRecord().Attributes().PutEmptySlice("test")
				s.AppendEmpty().SetStr("A")
				s.AppendEmpty().SetStr("B")
				s.AppendEmpty().SetStr("C")
			},
		},
		{
			statement: `set(attributes["test"], Substring("pass", 0, 2))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pa")
			},
		},
		{
			statement: `set(trace_id, TraceID(0x00000000000000000000000000000000))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().SetTraceID(pcommon.NewTraceIDEmpty())
			},
		},
		{
			statement: `set(time, TruncateTime(time, Duration("1s")))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().SetTimestamp(pcommon.NewTimestampFromTime(TestLogTimestamp.AsTime().Truncate(time.Second)))
			},
		},
		{
			statement: `set(attributes["test"], "pass") where UnixMicro(time) > 0`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where UnixMilli(time) > 0`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where UnixNano(time) > 0`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where UnixSeconds(time) > 0`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where IsString(UUID())`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
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

func Test_e2e_ottl_features(t *testing.T) {
	tests := []struct {
		name      string
		statement string
		want      func(tCtx ottllog.TransformContext)
	}{
		{
			name:      "where clause",
			statement: `set(attributes["test"], "pass") where body == "operationB"`,
			want:      func(tCtx ottllog.TransformContext) {},
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
			want:      func(tCtx ottllog.TransformContext) {},
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

	return ottllog.NewTransformContext(logRecord, scope, resource)
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
