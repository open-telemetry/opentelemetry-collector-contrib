// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
			err:  errFormatOTLPAttributes,
		},
		{
			flag: "key=[true, \"value2\"]",
			err:  errFormatOTLPAttributes,
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
