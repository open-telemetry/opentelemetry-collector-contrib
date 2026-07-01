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

func Test_parseCEF(t *testing.T) {
	tests := []struct {
		name     string
		target   ottl.StringGetter[*ottllog.TransformContext]
		expected map[string]any
	}{
		{
			name: "simple CEF 0 message",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "CEF:0|Security|threatmanager|1.0|100|worm successfully stopped|10|src=10.0.0.1 dst=2.1.2.2 spt=1232", nil
				},
			},
			expected: map[string]any{
				"version":               "0",
				"device_vendor":         "Security",
				"device_product":        "threatmanager",
				"device_version":        "1.0",
				"device_event_class_id": "100",
				"name":                  "worm successfully stopped",
				"severity":              "10",
				"extensions": map[string]any{
					"src": "10.0.0.1",
					"dst": "2.1.2.2",
					"spt": "1232",
				},
			},
		},
		{
			name: "CEF 1 message",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "CEF:1|ArcSight|ArcSight|2.4.1|machine:20|New alert|Low|src=10.0.0.1", nil
				},
			},
			expected: map[string]any{
				"version":               "1",
				"device_vendor":         "ArcSight",
				"device_product":        "ArcSight",
				"device_version":        "2.4.1",
				"device_event_class_id": "machine:20",
				"name":                  "New alert",
				"severity":              "Low",
				"extensions": map[string]any{
					"src": "10.0.0.1",
				},
			},
		},
		{
			name: "no extension and no trailing pipe",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "CEF:0|Vendor|Product|1.0|EventID|EventName|5", nil
				},
			},
			expected: map[string]any{
				"version":               "0",
				"device_vendor":         "Vendor",
				"device_product":        "Product",
				"device_version":        "1.0",
				"device_event_class_id": "EventID",
				"name":                  "EventName",
				"severity":              "5",
				"extensions":            map[string]any{},
			},
		},
		{
			name: "header with trailing pipe but empty extension",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "CEF:0|Vendor|Product|1.0|EventID|EventName|5|", nil
				},
			},
			expected: map[string]any{
				"version":               "0",
				"device_vendor":         "Vendor",
				"device_product":        "Product",
				"device_version":        "1.0",
				"device_event_class_id": "EventID",
				"name":                  "EventName",
				"severity":              "5",
				"extensions":            map[string]any{},
			},
		},
		{
			name: "escaped pipe in header",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return `CEF:0|Security|threatmanager|1.0|100|detected a \| in name|10|src=10.0.0.1`, nil
				},
			},
			expected: map[string]any{
				"version":               "0",
				"device_vendor":         "Security",
				"device_product":        "threatmanager",
				"device_version":        "1.0",
				"device_event_class_id": "100",
				"name":                  "detected a | in name",
				"severity":              "10",
				"extensions": map[string]any{
					"src": "10.0.0.1",
				},
			},
		},
		{
			name: "escaped backslash in header",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return `CEF:0|Security|threatmanager|1.0|100|detected a \\ in name|10|src=10.0.0.1`, nil
				},
			},
			expected: map[string]any{
				"version":               "0",
				"device_vendor":         "Security",
				"device_product":        "threatmanager",
				"device_version":        "1.0",
				"device_event_class_id": "100",
				"name":                  `detected a \ in name`,
				"severity":              "10",
				"extensions": map[string]any{
					"src": "10.0.0.1",
				},
			},
		},
		{
			name: "extension value with spaces",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "CEF:0|Vendor|Product|1.0|100|Event|5|src=10.0.0.1 msg=this is a message with spaces dst=1.2.3.4", nil
				},
			},
			expected: map[string]any{
				"version":               "0",
				"device_vendor":         "Vendor",
				"device_product":        "Product",
				"device_version":        "1.0",
				"device_event_class_id": "100",
				"name":                  "Event",
				"severity":              "5",
				"extensions": map[string]any{
					"src": "10.0.0.1",
					"msg": "this is a message with spaces",
					"dst": "1.2.3.4",
				},
			},
		},
		{
			name: "extension value with escaped equals",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return `CEF:0|Vendor|Product|1.0|100|Event|5|src=10.0.0.1 cs1=value with \= equals dst=1.2.3.4`, nil
				},
			},
			expected: map[string]any{
				"version":               "0",
				"device_vendor":         "Vendor",
				"device_product":        "Product",
				"device_version":        "1.0",
				"device_event_class_id": "100",
				"name":                  "Event",
				"severity":              "5",
				"extensions": map[string]any{
					"src": "10.0.0.1",
					"cs1": "value with = equals",
					"dst": "1.2.3.4",
				},
			},
		},
		{
			name: "extension value with escaped backslash",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return `CEF:0|Vendor|Product|1.0|100|Event|5|fname=C:\\Windows\\System32\\cmd.exe`, nil
				},
			},
			expected: map[string]any{
				"version":               "0",
				"device_vendor":         "Vendor",
				"device_product":        "Product",
				"device_version":        "1.0",
				"device_event_class_id": "100",
				"name":                  "Event",
				"severity":              "5",
				"extensions": map[string]any{
					"fname": `C:\Windows\System32\cmd.exe`,
				},
			},
		},
		{
			name: "extension value with escaped newline",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return `CEF:0|Vendor|Product|1.0|100|Event|5|msg=line one\nline two`, nil
				},
			},
			expected: map[string]any{
				"version":               "0",
				"device_vendor":         "Vendor",
				"device_product":        "Product",
				"device_version":        "1.0",
				"device_event_class_id": "100",
				"name":                  "Event",
				"severity":              "5",
				"extensions": map[string]any{
					"msg": "line one\nline two",
				},
			},
		},
		{
			name: "extension value with escaped carriage return",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return `CEF:0|Vendor|Product|1.0|100|Event|5|msg=line one\rline two`, nil
				},
			},
			expected: map[string]any{
				"version":               "0",
				"device_vendor":         "Vendor",
				"device_product":        "Product",
				"device_version":        "1.0",
				"device_event_class_id": "100",
				"name":                  "Event",
				"severity":              "5",
				"extensions": map[string]any{
					"msg": "line one\rline two",
				},
			},
		},
		{
			name: "syslog RFC 3164 prefix",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "<134>Sep 19 08:26:10 host CEF:0|Security|threatmanager|1.0|100|worm successfully stopped|10|src=10.0.0.1 dst=2.1.2.2", nil
				},
			},
			expected: map[string]any{
				"version":               "0",
				"device_vendor":         "Security",
				"device_product":        "threatmanager",
				"device_version":        "1.0",
				"device_event_class_id": "100",
				"name":                  "worm successfully stopped",
				"severity":              "10",
				"extensions": map[string]any{
					"src": "10.0.0.1",
					"dst": "2.1.2.2",
				},
			},
		},
		{
			name: "syslog RFC 5424 prefix",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "<134>1 2024-03-01T10:15:00.000Z host app - - - CEF:0|Vendor|Product|1.0|EventID|EventName|5|src=10.0.0.1", nil
				},
			},
			expected: map[string]any{
				"version":               "0",
				"device_vendor":         "Vendor",
				"device_product":        "Product",
				"device_version":        "1.0",
				"device_event_class_id": "EventID",
				"name":                  "EventName",
				"severity":              "5",
				"extensions": map[string]any{
					"src": "10.0.0.1",
				},
			},
		},
		{
			name: "custom string label fields",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "CEF:0|Vendor|Product|1.0|100|Event|5|cs1Label=Username cs1=jdoe cs2Label=Role cs2=admin", nil
				},
			},
			expected: map[string]any{
				"version":               "0",
				"device_vendor":         "Vendor",
				"device_product":        "Product",
				"device_version":        "1.0",
				"device_event_class_id": "100",
				"name":                  "Event",
				"severity":              "5",
				"extensions": map[string]any{
					"cs1Label": "Username",
					"cs1":      "jdoe",
					"cs2Label": "Role",
					"cs2":      "admin",
				},
			},
		},
		{
			name: "severity as text",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "CEF:0|Vendor|Product|1.0|100|Event|Very-High|src=10.0.0.1", nil
				},
			},
			expected: map[string]any{
				"version":               "0",
				"device_vendor":         "Vendor",
				"device_product":        "Product",
				"device_version":        "1.0",
				"device_event_class_id": "100",
				"name":                  "Event",
				"severity":              "Very-High",
				"extensions": map[string]any{
					"src": "10.0.0.1",
				},
			},
		},
		{
			name: "single extension key only",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "CEF:0|Vendor|Product|1.0|100|Event|5|src=10.0.0.1", nil
				},
			},
			expected: map[string]any{
				"version":               "0",
				"device_vendor":         "Vendor",
				"device_product":        "Product",
				"device_version":        "1.0",
				"device_event_class_id": "100",
				"name":                  "Event",
				"severity":              "5",
				"extensions": map[string]any{
					"src": "10.0.0.1",
				},
			},
		},
		{
			name: "extension value containing equals embedded in URL",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "CEF:0|Vendor|Product|1.0|100|Event|5|request=http://example.com?foo=bar&baz=qux src=10.0.0.1", nil
				},
			},
			expected: map[string]any{
				"version":               "0",
				"device_vendor":         "Vendor",
				"device_product":        "Product",
				"device_version":        "1.0",
				"device_event_class_id": "100",
				"name":                  "Event",
				"severity":              "5",
				"extensions": map[string]any{
					"request": "http://example.com?foo=bar&baz=qux",
					"src":     "10.0.0.1",
				},
			},
		},
		{
			name: "real-world firewall event",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "CEF:0|Cisco|ASA|9.8|FirewallDeny|Connection denied|7|src=10.1.1.1 dst=192.168.1.1 spt=12345 dpt=443 proto=TCP act=blocked", nil
				},
			},
			expected: map[string]any{
				"version":               "0",
				"device_vendor":         "Cisco",
				"device_product":        "ASA",
				"device_version":        "9.8",
				"device_event_class_id": "FirewallDeny",
				"name":                  "Connection denied",
				"severity":              "7",
				"extensions": map[string]any{
					"src":   "10.1.1.1",
					"dst":   "192.168.1.1",
					"spt":   "12345",
					"dpt":   "443",
					"proto": "TCP",
					"act":   "blocked",
				},
			},
		},
		{
			name: "header field with escaped pipe and backslash combined",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return `CEF:0|Vendor\\Co|Product\|X|1.0|100|Event|5|src=10.0.0.1`, nil
				},
			},
			expected: map[string]any{
				"version":               "0",
				"device_vendor":         `Vendor\Co`,
				"device_product":        "Product|X",
				"device_version":        "1.0",
				"device_event_class_id": "100",
				"name":                  "Event",
				"severity":              "5",
				"extensions": map[string]any{
					"src": "10.0.0.1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := parseCEF(tt.target)
			result, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)

			resultMap, ok := result.(pcommon.Map)
			require.True(t, ok, "result should be pcommon.Map")

			assertMapValue(t, resultMap, "cef.version", tt.expected["version"])
			assertMapValue(t, resultMap, "cef.device_vendor", tt.expected["device_vendor"])
			assertMapValue(t, resultMap, "cef.device_product", tt.expected["device_product"])
			assertMapValue(t, resultMap, "cef.device_version", tt.expected["device_version"])
			assertMapValue(t, resultMap, "cef.device_event_class_id", tt.expected["device_event_class_id"])
			assertMapValue(t, resultMap, "cef.name", tt.expected["name"])
			assertMapValue(t, resultMap, "cef.severity", tt.expected["severity"])

			expectedExt := tt.expected["extensions"].(map[string]any)
			extVal, ok := resultMap.Get("cef.extensions")
			require.True(t, ok, "extensions field should exist")
			extMap := extVal.Map()
			assert.Equal(t, len(expectedExt), extMap.Len(), "extensions count mismatch")
			for k, v := range expectedExt {
				attrVal, ok := extMap.Get(k)
				assert.True(t, ok, "extension %q should exist", k)
				assert.Equal(t, v, attrVal.Str(), "extension %q value mismatch", k)
			}
		})
	}
}

func Test_parseCEF_error(t *testing.T) {
	tests := []struct {
		name          string
		target        ottl.StringGetter[*ottllog.TransformContext]
		expectedError string
	}{
		{
			name: "empty input",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "", nil
				},
			},
			expectedError: "cannot parse empty CEF message",
		},
		{
			name: "not a CEF message",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "LEEF:1.0|Vendor|Product|1.0|EventID|src=1.2.3.4", nil
				},
			},
			expectedError: "'CEF:' not found",
		},
		{
			name: "plain text not CEF",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "This is just plain text log message", nil
				},
			},
			expectedError: "'CEF:' not found",
		},
		{
			name: "too few header fields",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "CEF:0|Vendor|Product|1.0|100|Event", nil
				},
			},
			expectedError: "expected at least 7 pipe-delimited fields",
		},
		{
			name: "missing version",
			target: ottl.StandardStringGetter[*ottllog.TransformContext]{
				Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
					return "CEF:|Vendor|Product|1.0|100|Event|5|src=1.2.3.4", nil
				},
			},
			expectedError: "missing version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := parseCEF(tt.target)
			_, err := exprFunc(t.Context(), nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func Test_parseCEF_target_error(t *testing.T) {
	target := ottl.StandardStringGetter[*ottllog.TransformContext]{
		Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
			return nil, assert.AnError
		},
	}
	exprFunc := parseCEF(target)
	_, err := exprFunc(t.Context(), nil)
	require.Error(t, err)
}

func Test_createParseCEFFunction(t *testing.T) {
	factory := NewParseCEFFactory()
	assert.Equal(t, "ParseCEF", factory.Name())

	args := &parseCEFArguments{
		Target: ottl.StandardStringGetter[*ottllog.TransformContext]{
			Getter: func(context.Context, *ottllog.TransformContext) (any, error) {
				return "CEF:0|Vendor|Product|1.0|100|Event|5|src=1.2.3.4", nil
			},
		},
	}

	exprFunc, err := factory.CreateFunction(ottl.FunctionContext{}, args)
	require.NoError(t, err)

	result, err := exprFunc(t.Context(), nil)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func Test_createParseCEFFunction_wrongArgs(t *testing.T) {
	factory := NewParseCEFFactory()

	_, err := factory.CreateFunction(ottl.FunctionContext{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parseCEFFactory args must be of type *parseCEFArguments")
}

func Test_unescapeCEFHeader(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "no escapes", input: "plain text", expected: "plain text"},
		{name: "escaped pipe", input: `a\|b`, expected: "a|b"},
		{name: "escaped backslash", input: `a\\b`, expected: `a\b`},
		{name: "multiple escapes", input: `a\\b\|c`, expected: `a\b|c`},
		{name: "trailing lone backslash preserved", input: `abc\`, expected: `abc\`},
		{name: "non-escape sequence preserved", input: `a\nb`, expected: `a\nb`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, unescapeCEFHeader(tt.input))
		})
	}
}

func Test_unescapeCEFValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "no escapes", input: "plain text", expected: "plain text"},
		{name: "escaped backslash", input: `a\\b`, expected: `a\b`},
		{name: "escaped equals", input: `a\=b`, expected: "a=b"},
		{name: "escaped newline", input: `a\nb`, expected: "a\nb"},
		{name: "escaped carriage return", input: `a\rb`, expected: "a\rb"},
		{name: "all escapes combined", input: `a\\b\=c\nd\re`, expected: "a\\b=c\nd\re"},
		{name: "non-escape sequence preserved", input: `a\xb`, expected: `a\xb`},
		{name: "trailing lone backslash preserved", input: `abc\`, expected: `abc\`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, unescapeCEFValue(tt.input))
		})
	}
}

func Test_parseCEFExtensions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]any
	}{
		{
			name:     "empty input",
			input:    "",
			expected: map[string]any{},
		},
		{
			name:  "single pair",
			input: "src=10.0.0.1",
			expected: map[string]any{
				"src": "10.0.0.1",
			},
		},
		{
			name:  "multiple pairs",
			input: "src=10.0.0.1 dst=10.0.0.2 spt=443",
			expected: map[string]any{
				"src": "10.0.0.1",
				"dst": "10.0.0.2",
				"spt": "443",
			},
		},
		{
			name:  "value with spaces",
			input: "src=10.0.0.1 msg=hello world foo bar dst=10.0.0.2",
			expected: map[string]any{
				"src": "10.0.0.1",
				"msg": "hello world foo bar",
				"dst": "10.0.0.2",
			},
		},
		{
			name:  "value with escaped equals",
			input: `cs1=a\=b src=10.0.0.1`,
			expected: map[string]any{
				"cs1": "a=b",
				"src": "10.0.0.1",
			},
		},
		{
			name:  "trailing space trimmed",
			input: "src=10.0.0.1 ",
			expected: map[string]any{
				"src": "10.0.0.1",
			},
		},
		{
			name:     "no recognizable keys",
			input:    "this is not a valid extension",
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseCEFExtensions(tt.input))
		})
	}
}
