// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
)

const fakeFuncName = "funkyfake"

func Test_newIDExprFunc_rawBytes(t *testing.T) {
	target := &literalByteGetter[pcommon.SpanID]{value: []byte{1, 2, 3, 4, 5, 6, 7, 8}}
	expr := newIDExprFunc[any, pcommon.SpanID](fakeFuncName, target)

	result, err := expr(t.Context(), nil)
	require.NoError(t, err)
	assert.Equal(t, pcommon.SpanID{1, 2, 3, 4, 5, 6, 7, 8}, result)
}

func Test_newIDExprFunc_hexBytes(t *testing.T) {
	target := &literalByteGetter[pprofile.ProfileID]{value: []byte("0102030405060708090a0b0c0d0e0f10")}
	expr := newIDExprFunc[any, pprofile.ProfileID](fakeFuncName, target)

	result, err := expr(t.Context(), nil)
	require.NoError(t, err)
	assert.Equal(t, pprofile.ProfileID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, result)
}

func Test_newIDExprFunc_errors(t *testing.T) {
	tests := []struct {
		name    string
		value   []byte
		wantErr error
	}{
		{
			name:    "invalid length",
			value:   []byte{1, 2, 3},
			wantErr: errIDInvalidLength,
		},
		{
			name:    "invalid hex",
			value:   []byte("zzzzzzzzzzzzzzzz"),
			wantErr: errIDHexDecode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := &literalByteGetter[pcommon.SpanID]{value: tt.value}
			expr := newIDExprFunc[any, pcommon.SpanID](fakeFuncName, target)

			result, err := expr(t.Context(), nil)

			assertErrorIsForFunction(t, err, fakeFuncName)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

type literalByteGetter[R idByteArray] struct {
	value []byte
}

func (g *literalByteGetter[R]) Get(context.Context, any) ([]byte, error) {
	return g.value, nil
}

func Test_newIDExprFunc_stringInput(t *testing.T) {
	target := &literalStringGetter[pcommon.TraceID]{value: "0102030405060708090a0b0c0d0e0f10"}
	expr := newIDExprFunc[any, pcommon.TraceID](fakeFuncName, target)

	result, err := expr(t.Context(), nil)
	require.NoError(t, err)
	assert.Equal(t, pcommon.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, result)
}

func Test_newIDExprFunc_stringErrors(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr error
	}{
		{
			name:    "invalid length string",
			value:   "010203",
			wantErr: errIDInvalidLength,
		},
		{
			name:    "invalid hex string",
			value:   "ZZ02030405060708090a0b0c0d0e0f10",
			wantErr: errIDHexDecode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := &literalStringGetter[pcommon.TraceID]{value: tt.value}
			expr := newIDExprFunc[any, pcommon.TraceID](fakeFuncName, target)

			result, err := expr(t.Context(), nil)

			assertErrorIsForFunction(t, err, fakeFuncName)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

type literalStringGetter[R idByteArray] struct {
	value string
}

func (g *literalStringGetter[R]) Get(context.Context, any) ([]byte, error) {
	return []byte(g.value), nil
}
