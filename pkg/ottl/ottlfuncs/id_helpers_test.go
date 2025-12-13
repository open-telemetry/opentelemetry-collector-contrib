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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const fakeFuncName = "funkyfake"

func Test_newIDExprFunc_rawBytes(t *testing.T) {
	target, err := ottl.NewTestingLiteralGetter(true, makeIDGetter([]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	require.NoError(t, err)
	expr, err := newIDExprFunc(fakeFuncName, target, decodeHexToSpanID)
	require.NoError(t, err, "initialization should succeed for literal getters with valid data")

	result, err := expr(t.Context(), nil)
	require.NoError(t, err)
	assert.Equal(t, pcommon.SpanID{1, 2, 3, 4, 5, 6, 7, 8}, result)
}

func Test_newIDExprFunc_hexBytes(t *testing.T) {
	target, err := ottl.NewTestingLiteralGetter(true, makeIDGetter([]byte("0102030405060708090a0b0c0d0e0f10")))
	require.NoError(t, err)
	expr, err := newIDExprFunc(fakeFuncName, target, decodeHexToProfileID)
	require.NoError(t, err, "initialization should succeed for literal getters with valid data")

	result, err := expr(t.Context(), nil)
	require.NoError(t, err)
	assert.Equal(t, pprofile.ProfileID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, result)
}

func Test_newIDExprFunc_literalSuccess(t *testing.T) {
	target, err := ottl.NewTestingLiteralGetter(true, makeIDGetter([]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	require.NoError(t, err)
	expr, err := spanID[any](target)
	require.NoError(t, err, "initialization should succeed for literal getters with valid data")
	got, err := expr(t.Context(), nil)
	require.NoError(t, err)
	assert.Equal(t, pcommon.SpanID{1, 2, 3, 4, 5, 6, 7, 8}, got)
}

func Test_newIDExprFunc_literalInitErrors(t *testing.T) {
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
			target, err := ottl.NewTestingLiteralGetter(true, makeIDGetter(tt.value))
			require.NoError(t, err)

			expr, err := newIDExprFunc(fakeFuncName, target, decodeHexToSpanID)
			// For literal getters with invalid data, initialization should fail
			require.Error(t, err, "initialization should fail for literal getters with invalid data")
			assert.Nil(t, expr, "expression should be nil when initialization fails")
			assertErrorIsFuncDecode(t, err, fakeFuncName)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func Test_newIDExprFunc_literalStringInitErrors(t *testing.T) {
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
			target, err := ottl.NewTestingLiteralGetter(true, makeIDGetter([]byte(tt.value)))
			require.NoError(t, err)
			expr, err := newIDExprFunc(fakeFuncName, target, decodeHexToTraceID)

			// For literal getters with invalid data, initialization should fail
			require.Error(t, err, "initialization should fail for literal getters with invalid data")
			assert.Nil(t, expr, "expression should be nil when initialization fails")
			assertErrorIsFuncDecode(t, err, fakeFuncName)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

type nonLiteralByteGetter[R idByteArray] struct {
	value []byte
}

func (g *nonLiteralByteGetter[R]) Get(context.Context, any) ([]byte, error) {
	return g.value, nil
}

func Test_newIDExprFunc_dynamicSuccess(t *testing.T) {
	target := &nonLiteralByteGetter[pcommon.SpanID]{value: []byte{1, 2, 3, 4, 5, 6, 7, 8}}
	expr, err := spanID[any](target)
	require.NoError(t, err, "init should succeed for dynamic getters")
	result, err := expr(t.Context(), nil)
	require.NoError(t, err, "execution should succeed for dynamic getters with valid data")
	assert.Equal(t, pcommon.SpanID{1, 2, 3, 4, 5, 6, 7, 8}, result)
}

func Test_newIDExprFunc_dynamicErrors(t *testing.T) {
	target := &nonLiteralByteGetter[pcommon.SpanID]{value: []byte{1, 2, 3}}
	expr, err := spanID[any](target)
	require.NoError(t, err, "init should succeed for dynamic getters")
	result, err := expr(t.Context(), nil)
	assert.Nil(t, result)
	assertErrorIsFuncDecode(t, err, spanIDFuncName)
	assert.ErrorIs(t, err, errIDInvalidLength)
}
