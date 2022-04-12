// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package trace

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestInit(t *testing.T) {
	builder, ok := operator.DefaultRegistry.Lookup("trace_parser")
	require.True(t, ok, "expected time_parser to be registered")
	require.Equal(t, "trace_parser", builder().Type())
}
func TestDefaultParser(t *testing.T) {
	parserConfig := NewParserConfig("")
	_, err := parserConfig.Build(testutil.Logger(t))
	require.NoError(t, err)
}

func TestBuild(t *testing.T) {
	testCases := []struct {
		name      string
		input     func() (*ParserConfig, error)
		expectErr bool
	}{
		{
			"empty",
			func() (*ParserConfig, error) {
				return &ParserConfig{}, nil
			},
			true,
		},
		{
			"default",
			func() (*ParserConfig, error) {
				cfg := NewParserConfig("test_id")
				return cfg, nil
			},
			false,
		},
		{
			"spanid",
			func() (*ParserConfig, error) {
				parseFrom := entry.NewBodyField("app_span_id")
				preserveTo := entry.NewBodyField("orig_span_id")
				cfg := NewParserConfig("test_id")
				cfg.SpanID.ParseFrom = &parseFrom
				cfg.SpanID.PreserveTo = &preserveTo
				return cfg, nil
			},
			false,
		},
		{
			"traceid",
			func() (*ParserConfig, error) {
				parseFrom := entry.NewBodyField("app_trace_id")
				preserveTo := entry.NewBodyField("orig_trace_id")
				cfg := NewParserConfig("test_id")
				cfg.TraceID.ParseFrom = &parseFrom
				cfg.TraceID.PreserveTo = &preserveTo
				return cfg, nil
			},
			false,
		},
		{
			"trace-flags",
			func() (*ParserConfig, error) {
				parseFrom := entry.NewBodyField("trace-flags-field")
				preserveTo := entry.NewBodyField("parsed-trace-flags")
				cfg := NewParserConfig("test_id")
				cfg.TraceFlags.ParseFrom = &parseFrom
				cfg.TraceFlags.PreserveTo = &preserveTo
				return cfg, nil
			},
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := tc.input()
			require.NoError(t, err, "expected nil error when running test cases input func")
			op, err := cfg.Build(testutil.Logger(t))
			if tc.expectErr {
				require.Error(t, err, "expected error while building trace_parser operator")
				return
			}
			require.NoError(t, err, "did not expect error while building trace_parser operator")
			require.NotNil(t, op, "expected Build to return an operator")
		})
	}
}

func TestProcess(t *testing.T) {
	testSpanIDBytes, _ := hex.DecodeString("480140f3d770a5ae32f0a22b6a812cff")
	testTraceIDBytes, _ := hex.DecodeString("92c3792d54ba94f3")
	testTraceFlagsBytes, _ := hex.DecodeString("01")

	cases := []struct {
		name   string
		op     func() (operator.Operator, error)
		input  *entry.Entry
		expect *entry.Entry
	}{
		{
			"no-op",
			func() (operator.Operator, error) {
				cfg := NewParserConfig("test_id")
				return cfg.Build(testutil.Logger(t))
			},
			&entry.Entry{
				Body: "https://google.com:443/path?user=dev",
			},
			&entry.Entry{
				Body: "https://google.com:443/path?user=dev",
			},
		},
		{
			"all",
			func() (operator.Operator, error) {
				cfg := NewParserConfig("test_id")
				spanFrom := entry.NewBodyField("app_span_id")
				traceFrom := entry.NewBodyField("app_trace_id")
				flagsFrom := entry.NewBodyField("trace_flags_field")
				cfg.SpanID.ParseFrom = &spanFrom
				cfg.TraceID.ParseFrom = &traceFrom
				cfg.TraceFlags.ParseFrom = &flagsFrom
				return cfg.Build(testutil.Logger(t))
			},
			&entry.Entry{
				Body: map[string]interface{}{
					"app_span_id":       "480140f3d770a5ae32f0a22b6a812cff",
					"app_trace_id":      "92c3792d54ba94f3",
					"trace_flags_field": "01",
				},
			},
			&entry.Entry{
				SpanID:     testSpanIDBytes,
				TraceID:    testTraceIDBytes,
				TraceFlags: testTraceFlagsBytes,
				Body:       map[string]interface{}{},
			},
		},
		{
			"preserve",
			func() (operator.Operator, error) {
				cfg := NewParserConfig("test_id")
				spanFrom := entry.NewBodyField("app_span_id")
				spanTo := entry.NewBodyField("orig_span_id")
				traceFrom := entry.NewBodyField("app_trace_id")
				traceTo := entry.NewBodyField("orig_trace_id")
				flagsFrom := entry.NewBodyField("trace_flags_field")
				flagsTo := entry.NewBodyField("orig_trace_flags")
				cfg.SpanID.ParseFrom = &spanFrom
				cfg.SpanID.PreserveTo = &spanTo
				cfg.TraceID.ParseFrom = &traceFrom
				cfg.TraceID.PreserveTo = &traceTo
				cfg.TraceFlags.ParseFrom = &flagsFrom
				cfg.TraceFlags.PreserveTo = &flagsTo
				return cfg.Build(testutil.Logger(t))
			},
			&entry.Entry{
				Body: map[string]interface{}{
					"app_span_id":       "480140f3d770a5ae32f0a22b6a812cff",
					"app_trace_id":      "92c3792d54ba94f3",
					"trace_flags_field": "01",
				},
			},
			&entry.Entry{
				SpanID:     testSpanIDBytes,
				TraceID:    testTraceIDBytes,
				TraceFlags: testTraceFlagsBytes,
				Body: map[string]interface{}{
					"orig_span_id":     "480140f3d770a5ae32f0a22b6a812cff",
					"orig_trace_id":    "92c3792d54ba94f3",
					"orig_trace_flags": "01",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op, err := tc.op()
			require.NoError(t, err, "did not expect operator function to return an error, this is a bug with the test case")

			err = op.Process(context.Background(), tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expect, tc.input)
		})
	}
}

func TestTraceParserParse(t *testing.T) {
	cases := []struct {
		name           string
		inputRecord    map[string]interface{}
		expectedRecord map[string]interface{}
		expectErr      bool
		traceID        string
		spanID         string
		traceFlags     string
	}{
		{
			"AllFields",
			map[string]interface{}{
				"trace_id":    "480140f3d770a5ae32f0a22b6a812cff",
				"span_id":     "92c3792d54ba94f3",
				"trace_flags": "01",
			},
			map[string]interface{}{},
			false,
			"480140f3d770a5ae32f0a22b6a812cff",
			"92c3792d54ba94f3",
			"01",
		},
		{
			"WrongFields",
			map[string]interface{}{
				"traceId":    "480140f3d770a5ae32f0a22b6a812cff",
				"traceFlags": "01",
				"spanId":     "92c3792d54ba94f3",
			},
			map[string]interface{}{
				"traceId":    "480140f3d770a5ae32f0a22b6a812cff",
				"spanId":     "92c3792d54ba94f3",
				"traceFlags": "01",
			},
			false,
			"",
			"",
			"",
		},
		{
			"OnlyTraceId",
			map[string]interface{}{
				"trace_id": "480140f3d770a5ae32f0a22b6a812cff",
			},
			map[string]interface{}{},
			false,
			"480140f3d770a5ae32f0a22b6a812cff",
			"",
			"",
		},
		{
			"WrongTraceIdFormat",
			map[string]interface{}{
				"trace_id":    "foo_bar",
				"span_id":     "92c3792d54ba94f3",
				"trace_flags": "01",
			},
			map[string]interface{}{},
			true,
			"",
			"92c3792d54ba94f3",
			"01",
		},
		{
			"WrongTraceFlagFormat",
			map[string]interface{}{
				"trace_id":    "480140f3d770a5ae32f0a22b6a812cff",
				"span_id":     "92c3792d54ba94f3",
				"trace_flags": "foo_bar",
			},
			map[string]interface{}{},
			true,
			"480140f3d770a5ae32f0a22b6a812cff",
			"92c3792d54ba94f3",
			"",
		},
		{
			"AllFields",
			map[string]interface{}{
				"trace_id":    "480140f3d770a5ae32f0a22b6a812cff",
				"span_id":     "92c3792d54ba94f3",
				"trace_flags": "01",
			},
			map[string]interface{}{},
			false,
			"480140f3d770a5ae32f0a22b6a812cff",
			"92c3792d54ba94f3",
			"01",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			traceParserConfig := NewParserConfig("")
			_, _ = traceParserConfig.Build(testutil.Logger(t))
			e := entry.New()
			e.Body = tc.inputRecord
			err := traceParserConfig.Parse(e)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.expectedRecord, e.Body)
			traceID, _ := hex.DecodeString(tc.traceID)
			if len(tc.traceID) == 0 {
				require.Nil(t, e.TraceID)
			} else {
				require.Equal(t, traceID, e.TraceID)
			}
			spanID, _ := hex.DecodeString(tc.spanID)
			if len(tc.spanID) == 0 {
				require.Nil(t, e.SpanID)
			} else {
				require.Equal(t, spanID, e.SpanID)
			}
			traceFlags, _ := hex.DecodeString(tc.traceFlags)
			if len(tc.traceFlags) == 0 {
				require.Nil(t, e.TraceFlags)
			} else {
				require.Equal(t, traceFlags, e.TraceFlags)
			}
		})
	}
}
