// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	xottlfuncs "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/xottl/ottlfuncs"
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

func Test_e2e_converters(t *testing.T) {
	t.Cleanup(testutil.SetFeatureGateForTest(t, metadata.OttlFunctionsEnableLambdaFeatureGate, true))

	tests := []struct {
		statement string
		want      func(tCtx *ottllog.TransformContext)
		errMsg    string
	}{
		{
			statement: `set(attributes["filtered_slice"], Filter(attributes["primitiveValuesSlice"], (_, v) => v == "value1"))`,
			want: func(tCtx *ottllog.TransformContext) {
				filtered := tCtx.GetLogRecord().Attributes().PutEmptySlice("filtered_slice")
				filtered.AppendEmpty().SetStr("value1")
			},
		},
		{
			statement: `set(attributes["filtered_map"], Filter(attributes["foo"], (k, _) => k == "bar"))`,
			want: func(tCtx *ottllog.TransformContext) {
				filtered := tCtx.GetLogRecord().Attributes().PutEmptyMap("filtered_map")
				filtered.PutStr("bar", "pass")
			},
		},
		{
			statement: `set(attributes["mapped_slice"], MapEach(attributes["primitiveValuesSlice"], (i, v) => Concat([String(i), ":", String(v)], "")))`,
			want: func(tCtx *ottllog.TransformContext) {
				mapped := tCtx.GetLogRecord().Attributes().PutEmptySlice("mapped_slice")
				mapped.AppendEmpty().SetStr("0:value1")
				mapped.AppendEmpty().SetStr("1:42")
				mapped.AppendEmpty().SetStr("2:true")
			},
		},
		{
			statement: `set(attributes["mapped_map"], MapEach(attributes["foo"], (k, v) => Concat([k, ":", String(v)], "")))`,
			want: func(tCtx *ottllog.TransformContext) {
				mapped := tCtx.GetLogRecord().Attributes().PutEmptyMap("mapped_map")
				mapped.PutStr("bar", "bar:pass")
				mapped.PutStr("flags", "flags:pass")
				mapped.PutStr("slice", `slice:["val"]`)
				mapped.PutStr("nested", `nested:{"test":"pass"}`)
			},
		},
		{
			statement: `set(attributes["pdata"], MapEach(["things"], (_, v) => {"result":v}))`,
			want: func(tCtx *ottllog.TransformContext) {
				mapped := tCtx.GetLogRecord().Attributes().PutEmptySlice("pdata")
				mapped.AppendEmpty().SetEmptyMap().PutStr("result", "things")
			},
		},
		{
			statement: `set(attributes["pdata"], MapEach({"key":"val"}, (_, _) => attributes))`,
			want: func(tCtx *ottllog.TransformContext) {
				orig := pcommon.NewMap()
				tCtx.GetLogRecord().Attributes().CopyTo(orig)
				m := tCtx.GetLogRecord().Attributes().PutEmptyMap("pdata")
				v := m.PutEmptyMap("key")
				orig.CopyTo(v)
			},
		},
		{
			statement: `set(attributes["all_slice"], All(attributes["primitiveValuesSlice"], (_, v) => v == "value1"))`,
			want: func(tCtx *ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutBool("all_slice", false)
			},
		},
		{
			statement: `set(attributes["all_map"], All(attributes["foo"], (k, _) => k != "missing"))`,
			want: func(tCtx *ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutBool("all_map", true)
			},
		},
		{
			statement: `set(attributes["any_slice"], Any(attributes["primitiveValuesSlice"], (_, v) => v == "value1"))`,
			want: func(tCtx *ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutBool("any_slice", true)
			},
		},
		{
			statement: `set(attributes["any_map"], Any(attributes["foo"], (k, _) => k == "bar"))`,
			want: func(tCtx *ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutBool("any_map", true)
			},
		},
		{
			statement: `set(attributes["found_slice"], Find(attributes["primitiveValuesSlice"], (_, v) => v == "value1"))`,
			want: func(tCtx *ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("found_slice", "value1")
			},
		},
		{
			statement: `set(attributes["found_map"], Find(attributes["foo"], (k, _) => k == "bar"))`,
			want: func(tCtx *ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("found_map", "pass")
			},
		},
		{
			statement: `set(attributes["found_map_mapped"], Find(attributes["foo"], (k, _) => k == "bar", (k, v) => Concat([k, ":", String(v)], "")))`,
			want: func(tCtx *ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("found_map_mapped", "bar:pass")
			},
		},
		{
			statement: `set(attributes["found_slice_mapped"], Find(attributes["primitiveValuesSlice"], (_, v) => v == "value1", (i, v) => Concat([String(i), ":", String(v)], "")))`,
			want: func(tCtx *ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("found_slice_mapped", "0:value1")
			},
		},
		{
			statement: `set(attributes["slice_sum"], Reduce([1, 2, 3], 0, (acc, _, v) => acc + Int(v)))`,
			want: func(tCtx *ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutInt("slice_sum", 6)
			},
		},
		{
			statement: `set(attributes["labels_str"], Reduce({"env": "prod"}, "", (acc, k, v) => Concat([acc, k, "=", String(v), ";"], "")))`,
			want: func(tCtx *ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("labels_str", "env=prod;")
			},
		},
		{
			statement: `set(attributes["prefixed_foo"], MapKeys(attributes["foo"], (k, _) => Concat(["http.", k], "")))`,
			want: func(tCtx *ottllog.TransformContext) {
				prefixed := tCtx.GetLogRecord().Attributes().PutEmptyMap("prefixed_foo")
				prefixed.PutStr("http.bar", "pass")
				prefixed.PutStr("http.flags", "pass")
				s := prefixed.PutEmptySlice("http.slice")
				s.AppendEmpty().SetStr("val")
				nested := prefixed.PutEmptyMap("http.nested")
				nested.PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["renamed_foo"], MapKeys(attributes["foo"], (k, v) => Concat([k, ":", String(v)], "")))`,
			want: func(tCtx *ottllog.TransformContext) {
				renamed := tCtx.GetLogRecord().Attributes().PutEmptyMap("renamed_foo")
				renamed.PutStr("bar:pass", "pass")
				renamed.PutStr("flags:pass", "pass")
				s := renamed.PutEmptySlice(`slice:["val"]`)
				s.AppendEmpty().SetStr("val")
				nested := renamed.PutEmptyMap(`nested:{"test":"pass"}`)
				nested.PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], When(() => attributes["int_value"] > 0, "positive", "negative"))`,
			want: func(tCtx *ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "negative")
			},
		},
		{
			statement: `set(attributes["test"], When(() => IsMap(attributes["foo"]), attributes["foo"]["bar"], "fail"))`,
			want: func(tCtx *ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], When(() => IsMap(attributes["foo"]), When(() => attributes["foo"]["bar"] == "pass", "pass", "fail"), "fail"))`,
			want: func(tCtx *ottllog.TransformContext) {
				tCtx.GetLogRecord().Attributes().PutStr("test", "pass")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			logStatements, err := parseStatementWithAndWithoutPathContext(tt.statement)
			require.NoError(t, err)

			for _, statement := range logStatements {
				tCtx := constructLogTransformContext()
				_, _, err = statement.Execute(t.Context(), tCtx)
				if tt.errMsg == "" {
					require.NoError(t, err)
				} else {
					assert.ErrorContains(t, err, tt.errMsg)
				}

				exTCtx := constructLogTransformContext()
				tt.want(exTCtx)

				require.NoError(t, plogtest.CompareResourceLogs(newResourceLogs(exTCtx), newResourceLogs(tCtx)))
				tCtx.Close()
				exTCtx.Close()
			}
		})
	}
}

func Test_e2e_lambda_gate_disabled(t *testing.T) {
	t.Cleanup(testutil.SetFeatureGateForTest(t, metadata.OttlFunctionsEnableLambdaFeatureGate, false))

	_, err := parseStatementWithAndWithoutPathContext(`set(attributes["test"], When(() => true, "pass", "fail"))`)
	require.ErrorContains(t, err, "function \"When\" is experimental and requires the `ottl.functions.enableLambda` feature gate to be enabled")
}

func parseStatementWithAndWithoutPathContext(statement string) ([]*ottl.Statement[*ottllog.TransformContext], error) {
	settings := componenttest.NewNopTelemetrySettings()
	functions := xottlfuncs.WithExperimentalConverters(ottlfuncs.StandardFuncs[*ottllog.TransformContext]())
	parserWithoutPathCtx, err := ottllog.NewParser(functions, settings)
	if err != nil {
		return nil, err
	}

	withoutPathCtxResult, err := parserWithoutPathCtx.ParseStatement(statement)
	if err != nil {
		return nil, err
	}

	parserWithPathCtx, err := ottllog.NewParser(functions, settings, ottllog.EnablePathContextNames())
	if err != nil {
		return nil, err
	}

	pc, err := ottl.NewParserCollection(settings,
		ottl.WithParserCollectionContext[*ottllog.TransformContext, *ottl.Statement[*ottllog.TransformContext]](
			ottllog.ContextName,
			&parserWithPathCtx,
			ottl.WithStatementConverter(func(_ *ottl.ParserCollection[*ottl.Statement[*ottllog.TransformContext]], _ ottl.StatementsGetter, parsedStatements []*ottl.Statement[*ottllog.TransformContext]) (*ottl.Statement[*ottllog.TransformContext], error) {
				return parsedStatements[0], nil
			}),
		))
	if err != nil {
		return nil, err
	}

	withPathCtxResult, err := pc.ParseStatementsWithContext(ottllog.ContextName, ottl.NewStatementsGetter([]string{statement}), true)
	if err != nil {
		return nil, err
	}

	return []*ottl.Statement[*ottllog.TransformContext]{withoutPathCtxResult, withPathCtxResult}, nil
}

func constructLogTransformContext() *ottllog.TransformContext {
	rLogs := plog.NewResourceLogs()
	rLogs.Resource().Attributes().PutStr("host.name", "localhost")
	rLogs.Resource().Attributes().PutStr("A|B|C", "newValue")

	scope := rLogs.ScopeLogs().AppendEmpty().Scope()
	scope.SetName("scope")

	logRecord := rLogs.ScopeLogs().At(0).LogRecords().AppendEmpty()
	logRecord.Body().SetStr("operationA")
	logRecord.SetTimestamp(TestLogTimestamp)
	logRecord.SetObservedTimestamp(TestObservedTimestamp)
	logRecord.SetDroppedAttributesCount(1)
	logRecord.SetFlags(plog.DefaultLogRecordFlags.WithIsSampled(true))
	logRecord.SetSeverityNumber(1)
	logRecord.SetTraceID(traceID)
	logRecord.SetSpanID(spanID)
	logRecord.Attributes().PutStr("encoding", "base64")
	logRecord.Attributes().PutStr("http.method", "get")
	logRecord.Attributes().PutStr("split_delimiter", "|")
	logRecord.Attributes().PutStr("dynamicprefix", "operation")
	logRecord.Attributes().PutStr("dynamicsuffix", "tionA")
	logRecord.Attributes().PutStr("http.path", "/health")
	logRecord.Attributes().PutStr("http.url", "http://localhost/health")
	logRecord.Attributes().PutStr("flags", "A|B|C")
	logRecord.Attributes().PutStr("total.string", "123456789")
	logRecord.Attributes().PutStr("A|B|C", "something")
	logRecord.Attributes().PutStr("foo", "foo")
	logRecord.Attributes().PutStr("slice", "slice")
	logRecord.Attributes().PutStr("val", "val2")
	logRecord.Attributes().PutInt("int_value", 0)
	logRecord.Attributes().PutStr("int_value_str", "0")
	logRecord.Attributes().PutStr("nil_string", "nil")
	logRecord.Attributes().PutEmpty("empty_value")
	logRecord.Attributes().PutStr("server.ip", "192.168.0.1")
	arr := logRecord.Attributes().PutEmptySlice("array")
	arr0 := arr.AppendEmpty()
	arr0.SetStr("looong")
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

	s3 := logRecord.Attributes().PutEmptySlice("slices")
	s3.AppendEmpty().SetStr("slice1")
	s3.AppendEmpty().SetStr("slice2")
	s3m1 := s3.AppendEmpty().SetEmptyMap()
	s3m1.PutStr("name", "foo")

	s4 := logRecord.Attributes().PutEmptySlice("primitiveValuesSlice")
	s4.AppendEmpty().SetStr("value1")
	s4.AppendEmpty().SetInt(42)
	s4.AppendEmpty().SetBool(true)

	return ottllog.NewTransformContext(rLogs, rLogs.ScopeLogs().At(0), logRecord)
}

func newResourceLogs(tCtx *ottllog.TransformContext) plog.ResourceLogs {
	rl := plog.NewResourceLogs()
	tCtx.GetResource().CopyTo(rl.Resource())
	sl := rl.ScopeLogs().AppendEmpty()
	tCtx.GetInstrumentationScope().CopyTo(sl.Scope())
	l := sl.LogRecords().AppendEmpty()
	tCtx.GetLogRecord().CopyTo(l)
	return rl
}
