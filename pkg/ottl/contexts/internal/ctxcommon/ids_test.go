// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxcommon_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxcommon"
)

func TestParseSpanIDError(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "incorrect size",
			input:   "0123456789abcde",
			wantErr: "span ids must be 16 hex characters",
		},
		{
			name:    "incorrect characters",
			input:   "0123456789Xbcdef",
			wantErr: "encoding/hex: invalid byte: U+0058 'X'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ctxcommon.ParseSpanID(tt.input)
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestParseTraceIDError(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "incorrect size",
			input:   "0123456789abcdef0123456789abcde",
			wantErr: "trace ids must be 32 hex characters",
		},
		{
			name:    "incorrect characters",
			input:   "0123456789Xbcdef0123456789abcdef",
			wantErr: "encoding/hex: invalid byte: U+0058 'X'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ctxcommon.ParseTraceID(tt.input)
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestParseProfileIDError(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "incorrect size",
			input:   "0123456789abcdef0123456789abcde",
			wantErr: "profile ids must be 32 hex characters",
		},
		{
			name:    "incorrect characters",
			input:   "0123456789Xbcdef0123456789abcdef",
			wantErr: "encoding/hex: invalid byte: U+0058 'X'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ctxcommon.ParseProfileID(tt.input)
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}
