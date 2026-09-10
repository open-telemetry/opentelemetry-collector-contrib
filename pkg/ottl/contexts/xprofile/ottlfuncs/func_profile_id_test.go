// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func makeIDGetter(bytes []byte) ottl.ByteSliceLikeGetter[any] {
	return ottl.StandardByteSliceLikeGetter[any]{Getter: func(context.Context, any) (any, error) {
		return bytes, nil
	}}
}

func Test_profileID(t *testing.T) {
	tests := []struct {
		name  string
		value []byte
		want  any
	}{
		{
			name:  "create profile id from 16 bytes",
			value: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			want:  pprofile.ProfileID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}),
		},
		{
			name:  "create profile id from 32 hex chars",
			value: []byte("0102030405060708090a0b0c0d0e0f10"),
			want:  pprofile.ProfileID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := profileID[any](makeIDGetter(tt.value))
			require.NoError(t, err)
			result, err := expr(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_profileID_validation(t *testing.T) {
	tests := []struct {
		name  string
		value []byte
		err   error
	}{
		{
			name:  "nil profile id",
			value: nil,
			err:   errProfileIDLength,
		},
		{
			name:  "byte slice less than 16 (15)",
			value: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			err:   errProfileIDLength,
		},
		{
			name:  "byte slice longer than 16 (17)",
			value: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17},
			err:   errProfileIDLength,
		},
		{
			name:  "invalid hex string",
			value: []byte("ZZ02030405060708090a0b0c0d0e0f10"),
			err:   errProfileIDHexDecode,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := profileID[any](makeIDGetter(tt.value))
			require.NoError(t, err)
			result, err := expr(t.Context(), nil)
			assert.Nil(t, result)
			assert.ErrorIs(t, err, errDecodeProfileID)
			assert.ErrorIs(t, err, tt.err)
			assert.ErrorContains(t, err, profileIDFuncName)
		})
	}
}

func Test_ProfileIDFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewProfileIDFactory[any]()
		assert.Equal(t, "ProfileID", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewProfileIDFactory[any]()
		args := factory.CreateDefaultArguments()
		assert.IsType(t, &ProfileIDArguments[any]{}, args)
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewProfileIDFactory[any]()
		args := factory.CreateDefaultArguments()
		profileIDArgs, ok := args.(*ProfileIDArguments[any])
		require.True(t, ok)
		profileIDArgs.Target = ottl.StandardByteSliceLikeGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return []byte("0102030405060708090a0b0c0d0e0f10"), nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createProfileIDFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "ProfileIDFactory args must be of type *ProfileIDArguments[K]")
	})
}
