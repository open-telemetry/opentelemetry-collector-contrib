// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
	traceParserConfig := NewConfig()
	_, err := traceParserConfig.Build(testutil.Logger(t))
	require.NoError(t, err)
}

func TestBuild(t *testing.T) {
	testCases := []struct {
		name      string
		input     func() (*Config, error)
		expectErr bool
	}{
		{
			"empty",
			func() (*Config, error) {
				return &Config{}, nil
			},
			true,
		},
		{
			"default",
			func() (*Config, error) {
				cfg := NewConfigWithID("test_id")
				return cfg, nil
			},
			false,
		},
		{
			"spanid",
			func() (*Config, error) {
				parseFrom := entry.NewBodyField("app_span_id")
				cfg := NewConfigWithID("test_id")
				cfg.SpanID.ParseFrom = &parseFrom
				return cfg, nil
			},
			false,
		},
		{
			"traceid",
			func() (*Config, error) {
				parseFrom := entry.NewBodyField("app_trace_id")
				cfg := NewConfigWithID("test_id")
				cfg.TraceID.ParseFrom = &parseFrom
				return cfg, nil
			},
			false,
		},
		{
			"trace-flags",
			func() (*Config, error) {
				parseFrom := entry.NewBodyField("trace-flags-field")
				cfg := NewConfigWithID("test_id")
				cfg.TraceFlags.ParseFrom = &parseFrom
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
				cfg := NewConfigWithID("test_id")
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
				cfg := NewConfigWithID("test_id")
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
				Body: map[string]interface{}{
					"app_span_id":       "480140f3d770a5ae32f0a22b6a812cff",
					"app_trace_id":      "92c3792d54ba94f3",
					"trace_flags_field": "01",
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
			map[string]interface{}{
				"trace_id":    "480140f3d770a5ae32f0a22b6a812cff",
				"span_id":     "92c3792d54ba94f3",
				"trace_flags": "01",
			},
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
			map[string]interface{}{
				"trace_id": "480140f3d770a5ae32f0a22b6a812cff",
			},
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
			map[string]interface{}{
				"trace_id":    "foo_bar",
				"span_id":     "92c3792d54ba94f3",
				"trace_flags": "01",
			},
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
			map[string]interface{}{
				"trace_id":    "480140f3d770a5ae32f0a22b6a812cff",
				"span_id":     "92c3792d54ba94f3",
				"trace_flags": "foo_bar",
			},
			true,
			"480140f3d770a5ae32f0a22b6a812cff",
			"92c3792d54ba94f3",
			"",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			traceParserConfig := NewConfigWithID("")
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
