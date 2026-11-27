// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package canonicaljson

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckUTF8(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		what    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid UTF-8",
			input:   "Hello, 世界",
			what:    "test string",
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			what:    "empty",
			wantErr: false,
		},
		{
			name:    "ASCII only",
			input:   "Hello World",
			what:    "ascii string",
			wantErr: false,
		},
		{
			name:    "invalid UTF-8",
			input:   "Hello\xffWorld",
			what:    "test string",
			wantErr: true,
			errMsg:  "test string contains invalid UTF-8",
		},
		{
			name:    "invalid UTF-8 in middle",
			input:   "Hello\x80\x80World",
			what:    "attribute key",
			wantErr: true,
			errMsg:  "attribute key contains invalid UTF-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkUTF8(tt.input, tt.what)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
