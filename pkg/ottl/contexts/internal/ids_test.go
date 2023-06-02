// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
			_, err := ParseSpanID(tt.input)
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
			_, err := ParseTraceID(tt.input)
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}
