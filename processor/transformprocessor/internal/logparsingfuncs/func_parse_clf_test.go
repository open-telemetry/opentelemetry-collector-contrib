// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logparsingfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

func Test_parseCLF(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		format   string
		expected map[string]any
		absent   []string
	}{
		{
			name:  "canonical example from the W3C spec",
			input: `127.0.0.1 - frank [10/Oct/2000:13:55:36 -0700] "GET /apache_pb.gif HTTP/1.0" 200 2326`,
			expected: map[string]any{
				"remote_host": "127.0.0.1",
				"rfc931":      "-",
				"authuser":    "frank",
				"timestamp":   "10/Oct/2000:13:55:36 -0700",
				"request":     "GET /apache_pb.gif HTTP/1.0",
				"method":      "GET",
				"request_uri": "/apache_pb.gif",
				"protocol":    "HTTP/1.0",
				"status":      int64(200),
				"bytes":       int64(2326),
			},
		},
		{
			name:  "all dashes for unknown fields",
			input: `- - - [10/Oct/2000:13:55:36 -0700] "GET / HTTP/1.1" 200 0`,
			expected: map[string]any{
				"remote_host": "-",
				"rfc931":      "-",
				"authuser":    "-",
				"timestamp":   "10/Oct/2000:13:55:36 -0700",
				"request":     "GET / HTTP/1.1",
				"method":      "GET",
				"request_uri": "/",
				"protocol":    "HTTP/1.1",
				"status":      int64(200),
				"bytes":       int64(0),
			},
		},
		{
			name:  "bytes is dash (no content)",
			input: `192.168.1.1 - - [10/Oct/2000:13:55:36 -0700] "GET /redirect HTTP/1.1" 304 -`,
			expected: map[string]any{
				"remote_host": "192.168.1.1",
				"rfc931":      "-",
				"authuser":    "-",
				"timestamp":   "10/Oct/2000:13:55:36 -0700",
				"request":     "GET /redirect HTTP/1.1",
				"method":      "GET",
				"request_uri": "/redirect",
				"protocol":    "HTTP/1.1",
				"status":      int64(304),
			},
			absent: []string{"bytes"},
		},
		{
			name:  "IPv6 remote host",
			input: `2001:db8::1 - - [10/Oct/2000:13:55:36 -0700] "POST /api/v1/users HTTP/1.1" 201 512`,
			expected: map[string]any{
				"remote_host": "2001:db8::1",
				"rfc931":      "-",
				"authuser":    "-",
				"timestamp":   "10/Oct/2000:13:55:36 -0700",
				"request":     "POST /api/v1/users HTTP/1.1",
				"method":      "POST",
				"request_uri": "/api/v1/users",
				"protocol":    "HTTP/1.1",
				"status":      int64(201),
				"bytes":       int64(512),
			},
		},
		{
			name:  "hostname remote host with rfc931 user",
			input: `client.example.com bob alice [25/Dec/2023:00:00:00 +0000] "DELETE /resource HTTP/2.0" 204 0`,
			expected: map[string]any{
				"remote_host": "client.example.com",
				"rfc931":      "bob",
				"authuser":    "alice",
				"timestamp":   "25/Dec/2023:00:00:00 +0000",
				"request":     "DELETE /resource HTTP/2.0",
				"method":      "DELETE",
				"request_uri": "/resource",
				"protocol":    "HTTP/2.0",
				"status":      int64(204),
				"bytes":       int64(0),
			},
		},
		{
			name:  "request_uri with query string",
			input: `10.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "GET /search?q=hello+world&page=2 HTTP/1.1" 200 1024`,
			expected: map[string]any{
				"remote_host": "10.0.0.1",
				"rfc931":      "-",
				"authuser":    "-",
				"timestamp":   "10/Oct/2000:13:55:36 -0700",
				"request":     "GET /search?q=hello+world&page=2 HTTP/1.1",
				"method":      "GET",
				"request_uri": "/search?q=hello+world&page=2",
				"protocol":    "HTTP/1.1",
				"status":      int64(200),
				"bytes":       int64(1024),
			},
		},
		{
			name:  "leading and trailing whitespace tolerated",
			input: "   127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] \"GET / HTTP/1.1\" 200 42   ",
			expected: map[string]any{
				"remote_host": "127.0.0.1",
				"rfc931":      "-",
				"authuser":    "-",
				"timestamp":   "10/Oct/2000:13:55:36 -0700",
				"request":     "GET / HTTP/1.1",
				"method":      "GET",
				"request_uri": "/",
				"protocol":    "HTTP/1.1",
				"status":      int64(200),
				"bytes":       int64(42),
			},
		},
		{
			name:  "malformed request line is preserved but not split",
			input: `127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "MALFORMED" 400 0`,
			expected: map[string]any{
				"remote_host": "127.0.0.1",
				"rfc931":      "-",
				"authuser":    "-",
				"timestamp":   "10/Oct/2000:13:55:36 -0700",
				"request":     "MALFORMED",
				"status":      int64(400),
				"bytes":       int64(0),
			},
			absent: []string{"method", "request_uri", "protocol"},
		},
		{
			name:  "empty request line",
			input: `127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "" 408 0`,
			expected: map[string]any{
				"remote_host": "127.0.0.1",
				"rfc931":      "-",
				"authuser":    "-",
				"timestamp":   "10/Oct/2000:13:55:36 -0700",
				"request":     "",
				"status":      int64(408),
				"bytes":       int64(0),
			},
			absent: []string{"method", "request_uri", "protocol"},
		},
		{
			name:  "large byte counts",
			input: `127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "GET /download HTTP/1.1" 200 4294967296`,
			expected: map[string]any{
				"remote_host": "127.0.0.1",
				"rfc931":      "-",
				"authuser":    "-",
				"timestamp":   "10/Oct/2000:13:55:36 -0700",
				"request":     "GET /download HTTP/1.1",
				"method":      "GET",
				"request_uri": "/download",
				"protocol":    "HTTP/1.1",
				"status":      int64(200),
				"bytes":       int64(4294967296),
			},
		},
		{
			name:   "combined format with referer and user-agent",
			input:  `127.0.0.1 - frank [10/Oct/2000:13:55:36 -0700] "GET /apache_pb.gif HTTP/1.0" 200 2326 "http://www.example.com/start.html" "Mozilla/4.08 [en] (Win98; I ;Nav)"`,
			format: clfFormatCombined,
			expected: map[string]any{
				"remote_host": "127.0.0.1",
				"rfc931":      "-",
				"authuser":    "frank",
				"timestamp":   "10/Oct/2000:13:55:36 -0700",
				"request":     "GET /apache_pb.gif HTTP/1.0",
				"method":      "GET",
				"request_uri": "/apache_pb.gif",
				"protocol":    "HTTP/1.0",
				"status":      int64(200),
				"bytes":       int64(2326),
				"referer":     "http://www.example.com/start.html",
				"user_agent":  "Mozilla/4.08 [en] (Win98; I ;Nav)",
			},
		},
		{
			name:   "combined format with dash referer and empty user-agent",
			input:  `192.168.1.1 - - [10/Oct/2000:13:55:36 -0700] "GET / HTTP/1.1" 304 - "-" ""`,
			format: clfFormatCombined,
			expected: map[string]any{
				"remote_host": "192.168.1.1",
				"rfc931":      "-",
				"authuser":    "-",
				"timestamp":   "10/Oct/2000:13:55:36 -0700",
				"request":     "GET / HTTP/1.1",
				"method":      "GET",
				"request_uri": "/",
				"protocol":    "HTTP/1.1",
				"status":      int64(304),
				"referer":     "-",
				"user_agent":  "",
			},
			absent: []string{"bytes"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return tt.input, nil
				},
			}
			format := tt.format
			if format == "" {
				format = clfFormatCLF
			}
			exprFunc := parseCLF(target, format)
			result, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)

			resultMap, ok := result.(pcommon.Map)
			require.True(t, ok, "result should be pcommon.Map")

			assertCLFMap(t, resultMap, tt.expected)
			for _, k := range tt.absent {
				_, ok := resultMap.Get(k)
				assert.False(t, ok, "key %q should be absent", k)
			}
		})
	}
}

func Test_parseCLF_errors(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		format        string
		expectedError string
	}{
		{
			name:          "plain text",
			input:         "this is not a CLF message",
			expectedError: "does not match expected",
		},
		{
			name:          "missing brackets around date",
			input:         `127.0.0.1 - - 10/Oct/2000:13:55:36 -0700 "GET / HTTP/1.1" 200 42`,
			expectedError: "does not match expected",
		},
		{
			name:          "missing quotes around request",
			input:         `127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] GET / HTTP/1.1 200 42`,
			expectedError: "does not match expected",
		},
		{
			name:          "non-numeric status",
			input:         `127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "GET / HTTP/1.1" OK 42`,
			expectedError: "invalid status code",
		},
		{
			name:          "non-numeric bytes",
			input:         `127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "GET / HTTP/1.1" 200 lots`,
			expectedError: "invalid bytes value",
		},
		{
			name:          "too few fields",
			input:         `127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "GET / HTTP/1.1" 200`,
			expectedError: "does not match expected",
		},
		{
			name:          "combined line rejected by strict clf format",
			input:         `127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "GET / HTTP/1.1" 200 42 "-" "curl/8.0"`,
			expectedError: `does not match expected "clf" format`,
		},
		{
			name:          "plain clf line rejected by combined format",
			input:         `127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "GET / HTTP/1.1" 200 42`,
			format:        clfFormatCombined,
			expectedError: `does not match expected "combined" format`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return tt.input, nil
				},
			}
			format := tt.format
			if format == "" {
				format = clfFormatCLF
			}
			exprFunc := parseCLF(target, format)
			_, err := exprFunc(t.Context(), nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func Test_parseCLF_empty(t *testing.T) {
	target := ottl.StandardStringGetter[*ottllog.TransformContext]{
		Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
			return "", nil
		},
	}
	exprFunc := parseCLF(target, clfFormatCLF)
	_, err := exprFunc(t.Context(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot parse empty CLF message")
}

func Test_parseCLF_target_error(t *testing.T) {
	target := ottl.StandardStringGetter[*ottllog.TransformContext]{
		Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
			return nil, assert.AnError
		},
	}
	exprFunc := parseCLF(target, clfFormatCLF)
	_, err := exprFunc(t.Context(), nil)
	require.Error(t, err)
}

func Test_createParseCLFFunction(t *testing.T) {
	factory := NewParseCLFFactory()
	assert.Equal(t, "ParseCLF", factory.Name())

	args := &parseCLFArguments{
		Target: ottl.StandardStringGetter[*ottllog.TransformContext]{
			Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
				return `127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "GET / HTTP/1.1" 200 42`, nil
			},
		},
	}

	exprFunc, err := factory.CreateFunction(ottl.FunctionContext{}, args)
	require.NoError(t, err)

	result, err := exprFunc(t.Context(), nil)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func Test_createParseCLFFunction_combinedFormat(t *testing.T) {
	factory := NewParseCLFFactory()

	args := &parseCLFArguments{
		Target: ottl.StandardStringGetter[*ottllog.TransformContext]{
			Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
				return `127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "GET / HTTP/1.1" 200 42 "http://www.example.com/" "curl/8.0"`, nil
			},
		},
		Format: ottl.NewTestingOptional(clfFormatCombined),
	}

	exprFunc, err := factory.CreateFunction(ottl.FunctionContext{}, args)
	require.NoError(t, err)

	result, err := exprFunc(t.Context(), nil)
	require.NoError(t, err)

	resultMap, ok := result.(pcommon.Map)
	require.True(t, ok, "result should be pcommon.Map")
	referer, ok := resultMap.Get("referer")
	require.True(t, ok)
	assert.Equal(t, "http://www.example.com/", referer.Str())
	userAgent, ok := resultMap.Get("user_agent")
	require.True(t, ok)
	assert.Equal(t, "curl/8.0", userAgent.Str())
}

func Test_createParseCLFFunction_invalidFormat(t *testing.T) {
	factory := NewParseCLFFactory()

	args := &parseCLFArguments{
		Target: ottl.StandardStringGetter[*ottllog.TransformContext]{
			Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
				return "", nil
			},
		},
		Format: ottl.NewTestingOptional("common"),
	}

	_, err := factory.CreateFunction(ottl.FunctionContext{}, args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid format "common"`)
}

func Test_createParseCLFFunction_wrongArgs(t *testing.T) {
	factory := NewParseCLFFactory()

	_, err := factory.CreateFunction(ottl.FunctionContext{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parseCLFFactory args must be of type *parseCLFArguments")
}

func assertCLFMap(t *testing.T, m pcommon.Map, expected map[string]any) {
	t.Helper()

	assert.Equal(t, len(expected), m.Len(), "field count mismatch; got map %v", m.AsRaw())

	for k, v := range expected {
		val, ok := m.Get(k)
		require.True(t, ok, "key %q should exist", k)
		switch want := v.(type) {
		case string:
			assert.Equal(t, want, val.Str(), "value for key %q mismatch", k)
		case int64:
			assert.Equal(t, want, val.Int(), "value for key %q mismatch", k)
		default:
			t.Fatalf("unexpected expected-value type for key %q: %T", k, v)
		}
	}
}
