// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHexToBytes(t *testing.T) {
	testCases := []struct {
		hexStr    string
		want      []byte
		wantError bool
	}{
		{
			hexStr: "616263",
			want:   []byte{'a', 'b', 'c'},
		},
		{
			hexStr: "",
			want:   []byte{},
		},
		{
			hexStr: "123456",
			want:   []byte{0x12, 0x34, 0x56},
		},
		{
			hexStr:    "gfds",
			want:      nil,
			wantError: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.hexStr, func(t *testing.T) {
			f, err := HexToBytes[any](tt.hexStr)
			assert.NoError(t, err)
			assert.NotNil(t, f)

			got, err := f(context.Background(), nil)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
