// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
)

func TestKeyValueSet(t *testing.T) {
	tests := []struct {
		flag     string
		expected KeyValue
		err      error
	}{
		{
			flag:     "key=\"value\"",
			expected: KeyValue(map[string]any{"key": "value"}),
		},
		{
			flag:     "key=\" value \"",
			expected: KeyValue(map[string]any{"key": " value "}),
		},
		{
			flag:     "key=\"\"",
			expected: KeyValue(map[string]any{"key": ""}),
		},
		{
			flag: "key=\"",
			err:  errDoubleQuotesOTLPAttributes,
		},
		{
			flag: "key=value",
			err:  errDoubleQuotesOTLPAttributes,
		},
		{
			flag: "key",
			err:  errFormatOTLPAttributes,
		},
		{
			flag:     "key=true",
			expected: KeyValue(map[string]any{"key": true}),
		},
		{
			flag:     "key=false",
			expected: KeyValue(map[string]any{"key": false}),
		},
		{
			flag:     "key=123",
			expected: KeyValue(map[string]any{"key": 123}),
		},
		{
			flag:     "key=-456",
			expected: KeyValue(map[string]any{"key": -456}),
		},
		{
			flag:     "key=0",
			expected: KeyValue(map[string]any{"key": 0}),
		},
		{
			flag:     "key=[\"value1\"]",
			expected: KeyValue(map[string]any{"key": []string{"value1"}}),
		},
		{
			flag:     "key=[\"value1\", \"value2\"]",
			expected: KeyValue(map[string]any{"key": []string{"value1", "value2"}}),
		},
		{
			flag:     "key=[1]",
			expected: KeyValue(map[string]any{"key": []int{1}}),
		},
		{
			flag:     "key=[1, 2]",
			expected: KeyValue(map[string]any{"key": []int{1, 2}}),
		},
		{
			flag:     "key=[true]",
			expected: KeyValue(map[string]any{"key": []bool{true}}),
		},
		{
			flag:     "key=[true, false]",
			expected: KeyValue(map[string]any{"key": []bool{true, false}}),
		},
		{
			flag: "key=[\"value1\", 2]",
			err:  errMixedTypeSlice,
		},
		{
			flag: "key=[true, \"value2\"]",
			err:  errMixedTypeSlice,
		},
		{
			flag: "key=[]",
			err:  errEmptySlice,
		},
		{
			flag:     "key=[\"\"]",
			expected: KeyValue(map[string]any{"key": []string{""}}),
		},
		{
			flag:     "key=[\" value \"]",
			expected: KeyValue(map[string]any{"key": []string{" value "}}),
		},
		{
			flag:     "key=[true,]",
			expected: KeyValue(map[string]any{"key": []bool{true}}),
		},
		{
			flag:     "key=[\"value1\",\"value-with,comma\"]",
			expected: KeyValue(map[string]any{"key": []string{"value1", "value-with,comma"}}),
		},
		{
			flag:     "key=[2,3,,1]",
			expected: KeyValue(map[string]any{"key": []int{2, 3, 1}}),
		},
		{
			flag: "key=12.34",
			err:  errDoubleQuotesOTLPAttributes,
		},
	}

	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			kv := KeyValue(make(map[string]any))
			err := kv.Set(tt.flag)
			if err != nil || tt.err != nil {
				assert.Equal(t, err, tt.err)
			} else {
				assert.Equal(t, tt.expected, kv)
			}
		})
	}
}

func TestEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		http     bool
		expected string
	}{
		{
			"default-no-http",
			"",
			false,
			defaultGRPCEndpoint,
		},
		{
			"default-with-http",
			"",
			true,
			defaultHTTPEndpoint,
		},
		{
			"custom-endpoint-no-http",
			"collector:4317",
			false,
			"collector:4317",
		},
		{
			"custom-endpoint-with-http",
			"collector:4317",
			true,
			"collector:4317",
		},
		{
			"wrong-custom-endpoint-with-http",
			defaultGRPCEndpoint,
			true,
			defaultGRPCEndpoint,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				CustomEndpoint: tc.endpoint,
				UseHTTP:        tc.http,
			}

			assert.Equal(t, tc.expected, cfg.Endpoint())
		})
	}
}

func TestGetAttributes(t *testing.T) {
	tests := []struct {
		name                        string
		cfg                         *Config
		expectedAttributes          []attribute.KeyValue
		expectedTelemetryAttributes []attribute.KeyValue
	}{
		{
			name: "String Attributes",
			cfg: &Config{
				ServiceName:         "test",
				ResourceAttributes:  KeyValue(map[string]any{"resourceKey": "value1"}),
				TelemetryAttributes: KeyValue(map[string]any{"telemetryKey": "value2"}),
			},
			expectedAttributes: []attribute.KeyValue{
				attribute.String("service.name", "test"),
				attribute.String("resourceKey", "value1"),
			},
			expectedTelemetryAttributes: []attribute.KeyValue{
				attribute.String("telemetryKey", "value2"),
			},
		},
		{
			name: "String Slice Attributes",
			cfg: &Config{
				ServiceName:         "test",
				ResourceAttributes:  KeyValue(map[string]any{"resourceKey": []string{"value1", "value2"}}),
				TelemetryAttributes: KeyValue(map[string]any{"telemetryKey": []string{"value3", "value4"}}),
			},
			expectedAttributes: []attribute.KeyValue{
				attribute.String("service.name", "test"),
				attribute.StringSlice("resourceKey", []string{"value1", "value2"}),
			},
			expectedTelemetryAttributes: []attribute.KeyValue{
				attribute.StringSlice("telemetryKey", []string{"value3", "value4"}),
			},
		},
		{
			name: "Int Attributes",
			cfg: &Config{
				ServiceName:         "test",
				ResourceAttributes:  KeyValue(map[string]any{"resourceKey": 123}),
				TelemetryAttributes: KeyValue(map[string]any{"telemetryKey": 456}),
			},
			expectedAttributes: []attribute.KeyValue{
				attribute.String("service.name", "test"),
				attribute.Int("resourceKey", 123),
			},
			expectedTelemetryAttributes: []attribute.KeyValue{
				attribute.Int("telemetryKey", 456),
			},
		},
		{
			name: "Int Slice Attributes",
			cfg: &Config{
				ServiceName:         "test",
				ResourceAttributes:  KeyValue(map[string]any{"resourceKey": []int{123, 456}}),
				TelemetryAttributes: KeyValue(map[string]any{"telemetryKey": []int{789, 101}}),
			},
			expectedAttributes: []attribute.KeyValue{
				attribute.String("service.name", "test"),
				attribute.IntSlice("resourceKey", []int{123, 456}),
			},
			expectedTelemetryAttributes: []attribute.KeyValue{
				attribute.IntSlice("telemetryKey", []int{789, 101}),
			},
		},
		{
			name: "Bool Attributes",
			cfg: &Config{
				ServiceName:         "test",
				ResourceAttributes:  KeyValue(map[string]any{"resourceKey": true}),
				TelemetryAttributes: KeyValue(map[string]any{"telemetryKey": false}),
			},
			expectedAttributes: []attribute.KeyValue{
				attribute.String("service.name", "test"),
				attribute.Bool("resourceKey", true),
			},
			expectedTelemetryAttributes: []attribute.KeyValue{
				attribute.Bool("telemetryKey", false),
			},
		},
		{
			name: "Bool Slice Attributes",
			cfg: &Config{
				ServiceName:         "test",
				ResourceAttributes:  KeyValue(map[string]any{"resourceKey": []bool{true, false}}),
				TelemetryAttributes: KeyValue(map[string]any{"telemetryKey": []bool{false, true, false}}),
			},
			expectedAttributes: []attribute.KeyValue{
				attribute.String("service.name", "test"),
				attribute.BoolSlice("resourceKey", []bool{true, false}),
			},
			expectedTelemetryAttributes: []attribute.KeyValue{
				attribute.BoolSlice("telemetryKey", []bool{false, true, false}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedAttributes, tt.cfg.GetAttributes())
			assert.Equal(t, tt.expectedTelemetryAttributes, tt.cfg.GetTelemetryAttributes())
		})
	}
}
