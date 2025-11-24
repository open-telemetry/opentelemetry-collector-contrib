// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func Test_traceID(t *testing.T) {
	tests := []struct {
		name  string
		bytes []byte
		want  pcommon.TraceID
	}{
		{
			name:  "create trace id from 16 bytes",
			bytes: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			want:  pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}),
		},
		{
			name:  "create trace id from 32 hex chars",
			bytes: []byte("0102030405060708090a0b0c0d0e0f10"),
			want:  pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := traceID[any](makeTraceIDGetter(tt.bytes))
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_traceID_validation(t *testing.T) {
	tests := []struct {
		name  string
		bytes []byte
	}{
		{
			name:  "byte slice less than 16 (15)",
			bytes: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		},
		{
			name:  "byte slice longer than 16 (17)",
			bytes: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17},
		},
		{
			name:  "byte slice longer than 32 (33)",
			bytes: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := traceID(makeTraceIDGetter(tt.bytes))

			result, err := exprFunc(nil, nil)
			assert.Error(t, err)
			assert.Nil(t, result)
			assert.ErrorIs(t, err, errTraceIDInvalidLength)
		})
	}
}

func makeTraceIDGetter(bytes []byte) *traceIDGetter {
	return &traceIDGetter{bytes: bytes}
}

type traceIDGetter struct {
	bytes []byte
}

func (t *traceIDGetter) Get(_ context.Context, _ any) ([]byte, error) {
	return t.bytes, nil
}
