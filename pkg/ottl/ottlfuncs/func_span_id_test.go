// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func Test_spanID(t *testing.T) {
	tests := []struct {
		name  string
		bytes []byte
		want  pcommon.SpanID
	}{
		{
			name:  "create span id",
			bytes: []byte{1, 2, 3, 4, 5, 6, 7, 8},
			want:  pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := spanID[any](tt.bytes)
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_spanID_validation(t *testing.T) {
	tests := []struct {
		name  string
		bytes []byte
	}{
		{
			name:  "byte slice less than 8",
			bytes: []byte{1, 2, 3, 4, 5, 6, 7},
		},
		{
			name:  "byte slice longer than 8",
			bytes: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := spanID[any](tt.bytes)
			require.Error(t, err)
			assert.ErrorContains(t, err, "span ids must be 8 bytes")
		})
	}
}
