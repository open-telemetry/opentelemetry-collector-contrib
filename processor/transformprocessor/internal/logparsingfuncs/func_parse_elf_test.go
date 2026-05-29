// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logparsingfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

func Test_parseELF(t *testing.T) {
	tests := []struct {
		name     string
		target   ottl.StringGetter[*ottllog.TransformContext]
		expected map[string]any
		wantErr  bool
	}{
		{
			name: "basic W3C ELF block",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "#Version: 1.0\n#Date: 12-Jan-1996 00:00:00\n#Fields: time cs-method cs-uri\n00:34:23 GET /foo/bar.html\n12:21:16 GET /baz/index.html", nil
				},
			},
			expected: map[string]any{
				"version": "1.0",
				"date":    "12-Jan-1996 00:00:00",
				"fields":  []any{"time", "cs-method", "cs-uri"},
				"entries": []any{
					map[string]any{"time": "00:34:23", "cs-method": "GET", "cs-uri": "/foo/bar.html"},
					map[string]any{"time": "12:21:16", "cs-method": "GET", "cs-uri": "/baz/index.html"},
				},
			},
		},
		{
			name: "IIS W3C extended log with all header directives",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "#Software: Microsoft Internet Information Services 6.0\n#Version: 1.0\n#Date: 2002-05-24 20:18:01\n#Fields: date time c-ip cs-username s-ip cs-method cs-uri-stem cs-uri-query sc-status\n2002-05-24 20:18:01 172.224.24.114 - 206.73.118.24 GET /Default.htm - 200", nil
				},
			},
			expected: map[string]any{
				"version":  "1.0",
				"software": "Microsoft Internet Information Services 6.0",
				"date":     "2002-05-24 20:18:01",
				"fields":   []any{"date", "time", "c-ip", "cs-username", "s-ip", "cs-method", "cs-uri-stem", "cs-uri-query", "sc-status"},
				"entries": []any{
					map[string]any{
						"date":          "2002-05-24",
						"time":          "20:18:01",
						"c-ip":          "172.224.24.114",
						"cs-username":   "-",
						"s-ip":          "206.73.118.24",
						"cs-method":     "GET",
						"cs-uri-stem":   "/Default.htm",
						"cs-uri-query":  "-",
						"sc-status":     "200",
					},
				},
			},
		},
		{
			name: "missing values filled with dash",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "#Version: 1.0\n#Fields: a b c d\nval1 val2", nil
				},
			},
			expected: map[string]any{
				"version": "1.0",
				"fields":  []any{"a", "b", "c", "d"},
				"entries": []any{
					map[string]any{"a": "val1", "b": "val2", "c": "-", "d": "-"},
				},
			},
		},
		{
			name: "quoted field values",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "#Version: 1.0\n#Fields: cs-method cs-uri cs(User-Agent)\nGET /page.html \"Mozilla/5.0 (Windows NT 10.0)\"", nil
				},
			},
			expected: map[string]any{
				"version": "1.0",
				"fields":  []any{"cs-method", "cs-uri", "cs(User-Agent)"},
				"entries": []any{
					map[string]any{
						"cs-method":      "GET",
						"cs-uri":         "/page.html",
						"cs(User-Agent)": "Mozilla/5.0 (Windows NT 10.0)",
					},
				},
			},
		},
		{
			name: "multiple #Fields directives",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "#Version: 1.0\n#Fields: time cs-method\n10:00:00 GET\n#Fields: time sc-status\n10:01:00 200", nil
				},
			},
			expected: map[string]any{
				"version": "1.0",
				"fields":  []any{"time", "sc-status"}, // last #Fields
				"entries": []any{
					map[string]any{"time": "10:00:00", "cs-method": "GET"},
					map[string]any{"time": "10:01:00", "sc-status": "200"},
				},
			},
		},
		{
			name: "start_date and end_date directives",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "#Version: 1.0\n#Start-Date: 2024-01-01 00:00:00\n#End-Date: 2024-01-01 23:59:59\n#Fields: time\n12:00:00", nil
				},
			},
			expected: map[string]any{
				"version":    "1.0",
				"start_date": "2024-01-01 00:00:00",
				"end_date":   "2024-01-01 23:59:59",
				"fields":     []any{"time"},
				"entries":    []any{map[string]any{"time": "12:00:00"}},
			},
		},
		{
			name: "remark directive",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "#Version: 1.0\n#Remark: test log file\n#Fields: time\n00:00:01", nil
				},
			},
			expected: map[string]any{
				"version": "1.0",
				"remark":  "test log file",
				"fields":  []any{"time"},
				"entries": []any{map[string]any{"time": "00:00:01"}},
			},
		},
		{
			name: "blank lines and CRLF line endings",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "#Version: 1.0\r\n\r\n#Fields: time\r\n00:00:01\r\n", nil
				},
			},
			expected: map[string]any{
				"version": "1.0",
				"fields":  []any{"time"},
				"entries": []any{map[string]any{"time": "00:00:01"}},
			},
		},
		{
			name: "no data entries, only directives",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "#Version: 1.0\n#Fields: time cs-method", nil
				},
			},
			expected: map[string]any{
				"version": "1.0",
				"fields":  []any{"time", "cs-method"},
				"entries": []any{},
			},
		},
		{
			name: "empty input",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "", nil
				},
			},
			wantErr: true,
		},
		{
			name: "missing #Version directive",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "#Fields: time\n00:00:01", nil
				},
			},
			wantErr: true,
		},
		{
			name: "data line before #Fields directive",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "#Version: 1.0\n00:00:01 GET /foo", nil
				},
			},
			wantErr: true,
		},
		{
			name: "unterminated quoted value",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "#Version: 1.0\n#Fields: method uri\nGET \"/unterminated", nil
				},
			},
			wantErr: true,
		},
		{
			name: "malformed #Fields directive",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					// #Fields with no colon is a hard error — it would poison subsequent data lines.
					return "#Version: 1.0\n#Fields\n00:00:01", nil
				},
			},
			wantErr: true,
		},
		{
			name: "tab-separated data line",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "#Version: 1.0\n#Fields: time cs-method cs-uri\n00:34:23\tGET\t/foo/bar.html", nil
				},
			},
			expected: map[string]any{
				"version": "1.0",
				"fields":  []any{"time", "cs-method", "cs-uri"},
				"entries": []any{
					map[string]any{"time": "00:34:23", "cs-method": "GET", "cs-uri": "/foo/bar.html"},
				},
			},
		},
		{
			name: "multiple #Version directives, last one wins",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "#Version: 1.0\n#Version: 1.1\n#Fields: time\n12:00:00", nil
				},
			},
			expected: map[string]any{
				"version": "1.1",
				"fields":  []any{"time"},
				"entries": []any{
					map[string]any{"time": "12:00:00"},
				},
			},
		},
		{
			name: "extra values beyond field count are dropped",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "#Version: 1.0\n#Fields: a b\nval1 val2 val3 val4", nil
				},
			},
			expected: map[string]any{
				"version": "1.0",
				"fields":  []any{"a", "b"},
				"entries": []any{
					map[string]any{"a": "val1", "b": "val2"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := parseELF(tt.target, zap.NewNop())
			result, err := exprFunc(context.Background(), &ottllog.TransformContext{})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			m, ok := result.(pcommon.Map)
			require.True(t, ok, "result must be pcommon.Map")
			assertELFMapEqual(t, tt.expected, m)
		})
	}
}

// assertELFMapEqual compares a pcommon.Map against an expected map[string]any,
// supporting nested maps and slices of strings / maps.
func assertELFMapEqual(t *testing.T, expected map[string]any, actual pcommon.Map) {
	t.Helper()
	assert.Equal(t, len(expected), actual.Len(), "map length mismatch")
	for k, wantRaw := range expected {
		v, ok := actual.Get(k)
		require.True(t, ok, "key %q missing from result", k)

		switch want := wantRaw.(type) {
		case string:
			assert.Equal(t, want, v.Str(), "key %q", k)
		case []any:
			s := v.Slice()
			require.Equal(t, len(want), s.Len(), "slice length for key %q", k)
			for i, elem := range want {
				sv := s.At(i)
				switch e := elem.(type) {
				case string:
					assert.Equal(t, e, sv.Str(), "key %q index %d", k, i)
				case map[string]any:
					assertELFMapEqual(t, e, sv.Map())
				default:
					t.Errorf("assertELFMapEqual: unsupported slice element type %T at key %q index %d", elem, k, i)
				}
			}
		default:
			t.Errorf("assertELFMapEqual: unsupported expected value type %T for key %q", wantRaw, k)
		}
	}
}

func Test_parseELFDataLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
		wantErr  bool
	}{
		{
			name:     "simple tokens",
			input:    "GET /foo.html 200",
			expected: []string{"GET", "/foo.html", "200"},
		},
		{
			name:     "quoted string with spaces",
			input:    `GET /foo.html "Mozilla/5.0 (Windows NT)"`,
			expected: []string{"GET", "/foo.html", "Mozilla/5.0 (Windows NT)"},
		},
		{
			name:     "dash placeholder",
			input:    "GET /foo.html - 200",
			expected: []string{"GET", "/foo.html", "-", "200"},
		},
		{
			name:     "leading and trailing spaces",
			input:    "  GET /foo.html  ",
			expected: []string{"GET", "/foo.html"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "doubled double-quote escape inside quoted value",
			input:    `GET /page.html "He said ""hi"" today"`,
			expected: []string{"GET", "/page.html", `He said "hi" today`},
		},
		{
			name:     "tab-separated tokens",
			input:    "GET\t/foo.html\t200",
			expected: []string{"GET", "/foo.html", "200"},
		},
		{
			name:     "mixed tab and space separation",
			input:    "GET /foo.html\t200",
			expected: []string{"GET", "/foo.html", "200"},
		},
		{
			name:    "unterminated quoted value",
			input:   `GET "/unterminated`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseELFDataLine(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_parseELFDirective(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantKey   string
		wantValue string
		wantErr   bool
	}{
		{
			name:      "version directive",
			input:     "#Version: 1.0",
			wantKey:   "Version",
			wantValue: "1.0",
		},
		{
			name:      "fields directive with multiple fields",
			input:     "#Fields: date time c-ip",
			wantKey:   "Fields",
			wantValue: "date time c-ip",
		},
		{
			name:      "software with colon in value",
			input:     "#Software: IIS/6.0: Logging",
			wantKey:   "Software",
			wantValue: "IIS/6.0: Logging",
		},
		{
			name:    "no colon separator",
			input:   "#Remark",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, value, err := parseELFDirective(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantKey, key)
			assert.Equal(t, tt.wantValue, value)
		})
	}
}

func Test_NewParseELFFactory(t *testing.T) {
	factory := NewParseELFFactory()
	assert.Equal(t, "ParseELF", string(factory.Name()))
}
