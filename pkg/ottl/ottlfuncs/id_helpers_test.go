package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_newIDExprFunc_rawBytes(t *testing.T) {
	target := &literalByteGetter[[8]byte]{value: []byte{1, 2, 3, 4, 5, 6, 7, 8}}
	expr := newIDExprFunc[any, [8]byte](target)

	result, err := expr(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, [8]byte{1, 2, 3, 4, 5, 6, 7, 8}, result)
}

func Test_newIDExprFunc_hexBytes(t *testing.T) {
	target := &literalByteGetter[[16]byte]{value: []byte("0102030405060708090a0b0c0d0e0f10")}
	expr := newIDExprFunc[any, [16]byte](target)

	result, err := expr(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, result)
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
			target := &literalByteGetter[[8]byte]{value: tt.value}
			expr := newIDExprFunc[any, [8]byte](target)

			result, err := expr(context.Background(), nil)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

type literalByteGetter[R IDByteArray] struct {
	value []byte
}

func (g *literalByteGetter[R]) Get(context.Context, any) ([]byte, error) {
	return g.value, nil
}

func Test_newIDExprFunc_stringInput(t *testing.T) {
	target := &literalStringGetter[[16]byte]{value: "0102030405060708090a0b0c0d0e0f10"}
	expr := newIDExprFunc[any, [16]byte](target)

	result, err := expr(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, result)
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
			target := &literalStringGetter[[8]byte]{value: tt.value}
			expr := newIDExprFunc[any, [8]byte](target)

			result, err := expr(context.Background(), nil)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, result)
		})
	}
}

type literalStringGetter[R IDByteArray] struct {
	value string
}

func (g *literalStringGetter[R]) Get(context.Context, any) ([]byte, error) {
	return []byte(g.value), nil
}
