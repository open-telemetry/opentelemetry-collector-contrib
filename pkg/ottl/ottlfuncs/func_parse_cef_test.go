// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_ParseCEF(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]any
		wantErr  bool
	}{
		{
			name:  "basic CEF message",
			input: "CEF:0|Vendor|Product|1.0|100|Event Name|High|src=10.0.0.1 dst=10.0.0.2",
			expected: map[string]any{
				"version":            "0",
				"deviceVendor":       "Vendor",
				"deviceProduct":      "Product",
				"deviceVersion":      "1.0",
				"deviceEventClassId": "100",
				"name":               "Event Name",
				"severity":           "High",
				"extensions": map[string]any{
					"src": "10.0.0.1",
					"dst": "10.0.0.2",
				},
			},
		},
		{
			name:  "CEF with escaped pipes in header",
			input: `CEF:0|Vendor\|Corp|Product\|Name|1.0|100|Event\|Name|High|src=10.0.0.1`,
			expected: map[string]any{
				"version":            "0",
				"deviceVendor":       "Vendor|Corp",
				"deviceProduct":      "Product|Name",
				"deviceVersion":      "1.0",
				"deviceEventClassId": "100",
				"name":               "Event|Name",
				"severity":           "High",
				"extensions": map[string]any{
					"src": "10.0.0.1",
				},
			},
		},
		{
			name:  "CEF with escaped equals in extensions",
			input: `CEF:0|Vendor|Product|1.0|100|Event|High|key1\=special=value1 key2=value\=2`,
			expected: map[string]any{
				"version":            "0",
				"deviceVendor":       "Vendor",
				"deviceProduct":      "Product",
				"deviceVersion":      "1.0",
				"deviceEventClassId": "100",
				"name":               "Event",
				"severity":           "High",
				"extensions": map[string]any{
					"key1=special": "value1",
					"key2":         "value=2",
				},
			},
		},
		{
			name:  "CEF with literal backslash sequences",
			input: `CEF:0|Vendor|Product|1.0|100|Event\nName|High|msg=Line1\nLine2\tTabbed rt=\r\nCRLF`,
			expected: map[string]any{
				"version":            "0",
				"deviceVendor":       "Vendor",
				"deviceProduct":      "Product",
				"deviceVersion":      "1.0",
				"deviceEventClassId": "100",
				"name":               `Event\nName`,
				"severity":           "High",
				"extensions": map[string]any{
					"msg": `Line1\nLine2\tTabbed`,
					"rt":  `\r\nCRLF`,
				},
			},
		},
		{
			name:  "CEF with quotes as literal characters",
			input: `CEF:0|Vendor|Product|1.0|100|Event|High|msg="quoted value" key="has spaces"`,
			expected: map[string]any{
				"version":            "0",
				"deviceVendor":       "Vendor",
				"deviceProduct":      "Product",
				"deviceVersion":      "1.0",
				"deviceEventClassId": "100",
				"name":               "Event",
				"severity":           "High",
				"extensions": map[string]any{
					"msg": `"quoted value"`,
					"key": `"has spaces"`,
				},
			},
		},
		{
			name:  "CEF with empty extensions",
			input: "CEF:0|Vendor|Product|1.0|100|Event|High|",
			expected: map[string]any{
				"version":            "0",
				"deviceVendor":       "Vendor",
				"deviceProduct":      "Product",
				"deviceVersion":      "1.0",
				"deviceEventClassId": "100",
				"name":               "Event",
				"severity":           "High",
				"extensions":         map[string]any{},
			},
		},
		{
			name:  "CEF without extensions",
			input: "CEF:0|Vendor|Product|1.0|100|Event|High",
			expected: map[string]any{
				"version":            "0",
				"deviceVendor":       "Vendor",
				"deviceProduct":      "Product",
				"deviceVersion":      "1.0",
				"deviceEventClassId": "100",
				"name":               "Event",
				"severity":           "High",
				"extensions":         map[string]any{},
			},
		},
		{
			name:  "CEF with complex extensions",
			input: `CEF:0|ArcSight|Logger|6.0.0.0.0|3000000000|Logger Internal Event|Unknown|eventId=1 categoryBehavior=/Logger/Node/Application categoryDeviceGroup=/Application categoryObject=/Application/Database categoryOutcome=/Success categorySignificance=/Important deviceCustomDate1=Jan 18 2011 21:00:18 deviceCustomDate1Label=StartTime deviceDirection=1`,
			expected: map[string]any{
				"version":            "0",
				"deviceVendor":       "ArcSight",
				"deviceProduct":      "Logger",
				"deviceVersion":      "6.0.0.0.0",
				"deviceEventClassId": "3000000000",
				"name":               "Logger Internal Event",
				"severity":           "Unknown",
				"extensions": map[string]any{
					"eventId":                "1",
					"categoryBehavior":       "/Logger/Node/Application",
					"categoryDeviceGroup":    "/Application",
					"categoryObject":         "/Application/Database",
					"categoryOutcome":        "/Success",
					"categorySignificance":   "/Important",
					"deviceCustomDate1":      "Jan 18 2011 21:00:18",
					"deviceCustomDate1Label": "StartTime",
					"deviceDirection":        "1",
				},
			},
		},
		{
			name:    "invalid CEF - missing prefix",
			input:   "0|Vendor|Product|1.0|100|Event|High|src=10.0.0.1",
			wantErr: true,
		},
		{
			name:    "invalid CEF - too few fields",
			input:   "CEF:0|Vendor|Product|1.0",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.input, nil
				},
			}

			exprFunc, err := createParseCEFFunction[any](ottl.FunctionContext{}, &ParseCEFArguments[any]{
				Target: target,
			})
			require.NoError(t, err)

			result, err := exprFunc(context.Background(), nil)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			resultMap, ok := result.(pcommon.Map)
			require.True(t, ok)

			// Convert to comparable format
			actualMap := resultMap.AsRaw()
			assert.Equal(t, tt.expected, actualMap)
		})
	}
}

func Test_ParseCEF_InvalidArguments(t *testing.T) {
	_, err := createParseCEFFunction[any](ottl.FunctionContext{}, "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ParseCEFFactory args must be of type")
}

func Test_ParseCEF_TargetError(t *testing.T) {
	target := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return nil, assert.AnError
		},
	}

	exprFunc, err := createParseCEFFunction[any](ottl.FunctionContext{}, &ParseCEFArguments[any]{
		Target: target,
	})
	require.NoError(t, err)

	_, err = exprFunc(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error getting value for target in ParseCEF")
}

func Test_unescapeCEFValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "pipe escape",
			input:    `value\|with\|pipes`,
			expected: "value|with|pipes",
		},
		{
			name:     "equals escape",
			input:    `key\=value`,
			expected: "key=value",
		},
		{
			name:     "backslash escape",
			input:    `path\\to\\file`,
			expected: `path\to\file`,
		},
		{
			name:     "literal backslash sequences (not escaped)",
			input:    `line1\nline2`,
			expected: `line1\nline2`,
		},
		{
			name:     "literal carriage return sequence",
			input:    `line1\rline2`,
			expected: `line1\rline2`,
		},
		{
			name:     "literal tab sequence",
			input:    `col1\tcol2`,
			expected: `col1\tcol2`,
		},
		{
			name:     "mixed escapes with literals",
			input:    `msg\=test\|value\nwith\ttabs`,
			expected: `msg=test|value\nwith\ttabs`,
		},
		{
			name:     "no escapes",
			input:    "regular text",
			expected: "regular text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := unescapeCEFValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_parseCEFHeader(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		expectedHeader    []string
		expectedExtension string
		wantErr           bool
	}{
		{
			name:              "complete header with extension",
			input:             "0|Vendor|Product|1.0|100|Event|High|src=10.0.0.1",
			expectedHeader:    []string{"0", "Vendor", "Product", "1.0", "100", "Event", "High"},
			expectedExtension: "src=10.0.0.1",
		},
		{
			name:              "header without extension",
			input:             "0|Vendor|Product|1.0|100|Event|High",
			expectedHeader:    []string{"0", "Vendor", "Product", "1.0", "100", "Event", "High"},
			expectedExtension: "",
		},
		{
			name:              "header with escaped pipes",
			input:             `0|Vendor\|Corp|Product|1.0|100|Event|High|ext=value`,
			expectedHeader:    []string{"0", `Vendor\|Corp`, "Product", "1.0", "100", "Event", "High"},
			expectedExtension: "ext=value",
		},
		{
			name:    "too few fields",
			input:   "0|Vendor|Product",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header, extension, err := parseCEFHeader(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedHeader, header)
			assert.Equal(t, tt.expectedExtension, extension)
		})
	}
}
