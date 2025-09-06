// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChunkedEncoder_Encode(t *testing.T) {
	tests := []struct {
		name         string
		maxChunkSize int
		items        []any
		expected     []string // expected buffer contents
	}{
		{
			name:         "single chunk",
			maxChunkSize: 1024,
			items:        []any{map[string]string{"a": "b"}, map[string]string{"c": "d"}},
			expected:     []string{`{"a":"b"}` + "\n" + `{"c":"d"}` + "\n"},
		},
		{
			name:         "multiple chunks",
			maxChunkSize: 15, // small enough to force chunking
			items:        []any{map[string]string{"a": "b"}, map[string]string{"c": "d"}},
			expected:     []string{`{"a":"b"}` + "\n", `{"c":"d"}` + "\n"},
		},
		{
			name:         "complex types",
			maxChunkSize: 1024,
			items:        []any{map[string]any{"str": "v", "num": 1, "arr": []int{1, 2}}},
			expected:     []string{`{"arr":[1,2],"num":1,"str":"v"}` + "\n"},
		},
		{
			name:         "empty map",
			maxChunkSize: 1024,
			items:        []any{map[string]any{}},
			expected:     []string{"{}\n"},
		},
		{
			name:         "invalid json",
			maxChunkSize: 1024,
			items:        []any{make(chan int)},
			expected:     []string{}, // buffer remains empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoder := NewChunkedEncoder(tt.maxChunkSize)
			for _, item := range tt.items {
				_ = encoder.Encode(item)
			}
			buffers := encoder.Buffers()
			got := []string{}
			for _, b := range buffers {
				got = append(got, b.String())
			}
			assert.Equal(t, tt.expected, got)
		})
	}
}
