// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
			statement: `set(attributes["test"], Base64Decode("cGFzcw=="))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], Decode("cGFzcw==", "base64"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], Concat(["A","B"], ":"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "A:B")
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase(attributes["http.method"], "upper"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", http.MethodGet)
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
			statement: `set(attributes["test"], ConvertAttributesToElementsXML("<Log id=\"1\"><Message>This is a log message!</Message></Log>"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", `<Log><Message>This is a log message!</Message><id>1</id></Log>`)
			},
		},
		{
			statement: `set(body, ConvertTextToElementsXML("<a><b/>foo</a>"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Body().SetStr("<a><b></b><value>foo</value></a>")
			},
		},
		{
			statement: `set(body, ConvertTextToElementsXML("<a><b/>foo</a><c><b/>bar</c>", "/a", "custom"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Body().SetStr("<a><b></b><custom>foo</custom></a><c><b></b>bar</c>")
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
			statement: `set(attributes["test"], ExtractGrokPatterns("http://user:password@example.com:80/path?query=string", "%{ELB_URI}", true))`,
			want: func(tCtx ottllog.TransformContext) {
				m := tCtx.GetLogRecord().Attributes().PutEmptyMap("test")
				m.PutStr("url.scheme", "http")
				m.PutStr("url.username", "user")
				m.PutStr("url.domain", "example.com")
				m.PutInt("url.port", 80)
				m.PutStr("url.path", "/path")
				m.PutStr("url.query", "query=string")
			},
		},
		{
			statement: `set(attributes["test"], FNV("pass"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutInt("test", 266877920130663416)
			},
		},
		{
			statement: `set(attributes["test"], Format("%03d-%s", [7, "test"]))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "007-test")
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
			statement: `set(attributes["test"], InsertXML("<a></a>", "/a", "<b></b>"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "<a><b></b></a>")
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
			statement: `set(attributes["test"], GetXML("<a><b>1</b><c><b>2</b></c></a>", "/a//b"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "<b>1</b><b>2</b>")
			},
		},
		{
			statement: `set(attributes["test"], Hex(1.0))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "3ff0000000000000")
			},
		},
		{
			statement: `set(attributes["test"], Hex(true))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "01")
			},
		},
		{
			statement: `set(attributes["test"], Hex(12))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "000000000000000c")
			},
		},
		{
			statement: `set(attributes["test"], Hex("12"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "3132")
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
			statement: `set(attributes["test"], "pass") where IsList(attributes["foo"]["slice"])`,
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
				tCtx.GetLogRecord().Attributes().PutInt("test", 4)
			},
		},
		{
			statement: `set(attributes["test"], Log(1))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutDouble("test", 0)
			},
		},
		{
			statement: `set(attributes["test"], MD5("pass"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "1a1dc91c907325c69271ddf0c944bc72")
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
			statement: `set(attributes["test"], ParseCSV("val1;val2;val3","header1|header2|header3",";","|","strict"))`,
			want: func(tCtx ottllog.TransformContext) {
				m := tCtx.GetLogRecord().Attributes().PutEmptyMap("test")
				m.PutStr("header1", "val1")
				m.PutStr("header2", "val2")
				m.PutStr("header3", "val3")
			},
		},
		{
			statement: `set(attributes["test"], ParseCSV("val1,val2,val3","header1|header2|header3",headerDelimiter="|",mode="strict"))`,
			want: func(tCtx ottllog.TransformContext) {
				m := tCtx.GetLogRecord().Attributes().PutEmptyMap("test")
				m.PutStr("header1", "val1")
				m.PutStr("header2", "val2")
				m.PutStr("header3", "val3")
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
			statement: `set(attributes["test"], ParseJSON("[\"value1\",\"value2\"]"))`,
			want: func(tCtx ottllog.TransformContext) {
				m := tCtx.GetLogRecord().Attributes().PutEmptySlice("test")
				m.AppendEmpty().SetStr("value1")
				m.AppendEmpty().SetStr("value2")
			},
		},
		{
			statement: `set(attributes["test"], ParseKeyValue("k1=v1 k2=v2"))`,
			want: func(tCtx ottllog.TransformContext) {
				m := tCtx.GetLogRecord().Attributes().PutEmptyMap("test")
				m.PutStr("k1", "v1")
				m.PutStr("k2", "v2")
			},
		},
		{
			statement: `set(attributes["test"], ParseKeyValue("k1!v1_k2!v2", "!", "_"))`,
			want: func(tCtx ottllog.TransformContext) {
				m := tCtx.GetLogRecord().Attributes().PutEmptyMap("test")
				m.PutStr("k1", "v1")
				m.PutStr("k2", "v2")
			},
		},
		{
			statement: `set(attributes["test"], ParseKeyValue("k1!v1_k2!\"v2__!__v2\"", "!", "_"))`,
			want: func(tCtx ottllog.TransformContext) {
				m := tCtx.GetLogRecord().Attributes().PutEmptyMap("test")
				m.PutStr("k1", "v1")
				m.PutStr("k2", "v2__!__v2")
			},
		},
		{
			statement: `set(attributes["test"], ToKeyValueString(ParseKeyValue("k1=v1 k2=v2"), "=", " ", true))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "k1=v1 k2=v2")
			},
		},
		{
			statement: `set(attributes["test"], ToKeyValueString(ParseKeyValue("k1:v1,k2:v2", ":" , ","), ":", ",", true))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "k1:v1,k2:v2")
			},
		},
		{
			statement: `set(attributes["test"], ToKeyValueString(ParseKeyValue("k1=v1 k2=v2"), "!", "+", true))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "k1!v1+k2!v2")
			},
		},
		{
			statement: `set(attributes["test"], ToKeyValueString(ParseKeyValue("k1=v1 k2=v2=v3"), "=", " ", true))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "k1=v1 k2=\"v2=v3\"")
			},
		},
		{
			statement: `set(attributes["test"], ParseSimplifiedXML("<Log><id>1</id><Message>This is a log message!</Message></Log>"))`,
			want: func(tCtx ottllog.TransformContext) {
				attr := tCtx.GetLogRecord().Attributes().PutEmptyMap("test")
				log := attr.PutEmptyMap("Log")
				log.PutStr("id", "1")
				log.PutStr("Message", "This is a log message!")
			},
		},
		{
			statement: `set(attributes["test"], ParseXML("<Log id=\"1\"><Message>This is a log message!</Message></Log>"))`,
			want: func(tCtx ottllog.TransformContext) {
				log := tCtx.GetLogRecord().Attributes().PutEmptyMap("test")
				log.PutStr("tag", "Log")

				attrs := log.PutEmptyMap("attributes")
				attrs.PutStr("id", "1")

				logChildren := log.PutEmptySlice("children")

				message := logChildren.AppendEmpty().SetEmptyMap()
				message.PutStr("tag", "Message")
				message.PutStr("content", "This is a log message!")
			},
		},
		{
			statement: `set(attributes["test"], RemoveXML("<Log id=\"1\"><Message>This is a log message!</Message></Log>", "/Log/Message"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", `<Log id="1"></Log>`)
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
			statement: `set(attributes["test"], SHA512("pass"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "5b722b307fce6c944905d132691d5e4a2214b7fe92b738920eb3fce3a90420a19511c3010a0e7712b054daef5b57bad59ecbd93b3280f210578f547f4aed4d25")
			},
		},
		{
			statement: `set(attributes["test"], Sort(Split(attributes["flags"], "|"), "desc"))`,
			want: func(tCtx ottllog.TransformContext) {
				s := tCtx.GetLogRecord().Attributes().PutEmptySlice("test")
				s.AppendEmpty().SetStr("C")
				s.AppendEmpty().SetStr("B")
				s.AppendEmpty().SetStr("A")
			},
		},
		{
			statement: `set(attributes["test"], Sort([true, false, false]))`,
			want: func(tCtx ottllog.TransformContext) {
				s := tCtx.GetLogRecord().Attributes().PutEmptySlice("test")
				s.AppendEmpty().SetBool(false)
				s.AppendEmpty().SetBool(false)
				s.AppendEmpty().SetBool(true)
			},
		},
		{
			statement: `set(attributes["test"], Sort([3, 6, 9], "desc"))`,
			want: func(tCtx ottllog.TransformContext) {
				s := tCtx.GetLogRecord().Attributes().PutEmptySlice("test")
				s.AppendEmpty().SetInt(9)
				s.AppendEmpty().SetInt(6)
				s.AppendEmpty().SetInt(3)
			},
		},
		{
			statement: `set(attributes["test"], Sort([Double(1.5), Double(10.2), Double(2.3), Double(0.5)]))`,
			want: func(tCtx ottllog.TransformContext) {
				s := tCtx.GetLogRecord().Attributes().PutEmptySlice("test")
				s.AppendEmpty().SetDouble(0.5)
				s.AppendEmpty().SetDouble(1.5)
				s.AppendEmpty().SetDouble(2.3)
				s.AppendEmpty().SetDouble(10.2)
			},
		},
		{
			statement: `set(attributes["test"], Sort([Int(11), Double(2.2), Double(-1)]))`,
			want: func(tCtx ottllog.TransformContext) {
				s := tCtx.GetLogRecord().Attributes().PutEmptySlice("test")
				s.AppendEmpty().SetDouble(-1)
				s.AppendEmpty().SetDouble(2.2)
				s.AppendEmpty().SetInt(11)
			},
		},
		{
			statement: `set(attributes["test"], Sort([false, Int(11), Double(2.2), "three"]))`,
			want: func(tCtx ottllog.TransformContext) {
				s := tCtx.GetLogRecord().Attributes().PutEmptySlice("test")
				s.AppendEmpty().SetInt(11)
				s.AppendEmpty().SetDouble(2.2)
				s.AppendEmpty().SetBool(false)
				s.AppendEmpty().SetStr("three")
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
			statement: `set(attributes["test"], String("test"))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "test")
			},
		},
		{
			statement: `set(attributes["test"], String(attributes["http.method"]))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "get")
			},
		},
		{
			statement: `set(attributes["test"], String(span_id))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "[1,2,3,4,5,6,7,8]")
			},
		},
		{
			statement: `set(attributes["test"], String([1,2,3]))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "[1,2,3]")
			},
		},
		{
			statement: `set(attributes["test"], String(true))`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "true")
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
		{
			statement: `set(attributes["test"], "\\")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "\\")
			},
		},
		{
			statement: `set(attributes["test"], "\\\\")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "\\\\")
			},
		},
		{
			statement: `set(attributes["test"], "\\\\\\")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "\\\\\\")
			},
		},
		{
			statement: `set(attributes["test"], "\\\\\\\\")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "\\\\\\\\")
			},
		},
		{
			statement: `set(attributes["test"], "\"")`,
			want: func(tCtx ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", `"`)
			},
		},
		{
			statement: `keep_keys(attributes["foo"], ["\\", "bar"])`,
			want: func(tCtx ottllog.TransformContext) {
				// keep_keys should see two arguments
				m := tCtx.GetLogRecord().Attributes().PutEmptyMap("foo")
				m.PutStr("bar", "pass")
			},
		},
		{
			statement: `set(attributes["test"], UserAgent("curl/7.81.0"))`,
			want: func(tCtx ottllog.TransformContext) {
				m := tCtx.GetLogRecord().Attributes().PutEmptyMap("test")
				m.PutStr("user_agent.original", "curl/7.81.0")
				m.PutStr("user_agent.name", "curl")
				m.PutStr("user_agent.version", "7.81.0")
			},
		},
		{
			statement: `set(attributes["test"], SliceToMap(attributes["things"], ["name"]))`,
			want: func(tCtx ottllog.TransformContext) {
				m := tCtx.GetLogRecord().Attributes().PutEmptyMap("test")
				thing1 := m.PutEmptyMap("foo")
				thing1.PutStr("name", "foo")
				thing1.PutInt("value", 2)

				thing2 := m.PutEmptyMap("bar")
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)
			},
		},
		{
			statement: `set(attributes["test"], SliceToMap(attributes["things"], ["name"], ["value"]))`,
			want: func(tCtx ottllog.TransformContext) {
				m := tCtx.GetLogRecord().Attributes().PutEmptyMap("test")
				m.PutInt("foo", 2)
				m.PutInt("bar", 5)
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

func Test_ProcessSpanEvents(t *testing.T) {
	tests := []struct {
		statement string
		want      func(_ ottlspanevent.TransformContext)
	}{
		{
			statement: `set(attributes["index"], event_index)`,
			want: func(tCtx ottlspanevent.TransformContext) {
				tCtx.GetSpanEvent().Attributes().PutInt("index", 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			settings := componenttest.NewNopTelemetrySettings()
			funcs := ottlfuncs.StandardFuncs[ottlspanevent.TransformContext]()

			spanEventParser, err := ottlspanevent.NewParser(funcs, settings)
			assert.NoError(t, err)
			spanStatements, err := spanEventParser.ParseStatement(tt.statement)
			assert.NoError(t, err)

			tCtx := constructSpanEventTransformContext()
			_, _, _ = spanStatements.Execute(context.Background(), tCtx)

			exTCtx := constructSpanEventTransformContext()
			tt.want(exTCtx)

			assert.NoError(t, ptracetest.CompareSpanEvent(newSpanEvent(exTCtx), newSpanEvent(tCtx)))
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

func constructSpanEventTransformContext() ottlspanevent.TransformContext {
	resource := pcommon.NewResource()

	scope := pcommon.NewInstrumentationScope()
	scope.SetName("scope")

	span := ptrace.NewSpan()
	fillSpanOne(span)

	ev1 := span.Events().AppendEmpty()
	ev1.SetName("event-1")

	return ottlspanevent.NewTransformContext(ev1, span, scope, resource, ptrace.NewScopeSpans(), ptrace.NewResourceSpans())
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

func newSpanEvent(tCtx ottlspanevent.TransformContext) ptrace.SpanEvent {
	dst := ptrace.NewSpanEvent()
	tCtx.GetSpanEvent().CopyTo(dst)
	return dst
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
