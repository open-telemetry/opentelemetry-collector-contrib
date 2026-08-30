// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestHex(t *testing.T) {
	type args struct {
		target ottl.ByteSliceLikeGetter[any]
	}
	type testCase struct {
		name     string
		args     args
		wantFunc func() any
		wantErr  error
	}
	tests := []testCase{
		{
			name: "int64",
			args: args{
				target: &ottl.StandardByteSliceLikeGetter[any]{
					Getter: func(context.Context, any) (any, error) {
						return int64(12), nil
					},
				},
			},
			wantFunc: func() any {
				return "000000000000000c"
			},
		},
		{
			name: "nil",
			args: args{
				target: &ottl.StandardByteSliceLikeGetter[any]{
					Getter: func(context.Context, any) (any, error) {
						return nil, nil
					},
				},
			},
			wantFunc: func() any {
				return ""
			},
			wantErr: nil,
		},
		{
			name: "error",
			args: args{
				target: &ottl.StandardByteSliceLikeGetter[any]{
					Getter: func(context.Context, any) (any, error) {
						return map[string]string{"hi": "hi"}, nil
					},
				},
			},
			wantFunc: func() any {
				return nil
			},
			wantErr: ottl.TypeError("unsupported type: map[string]string"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressionFunc, _ := Hex(tt.args.target)
			got, err := expressionFunc(t.Context(), tt.args)
			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.wantFunc(), got)
		})
	}
}

func Test_HexFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewHexFactory[any]()
		assert.Equal(t, "Hex", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewHexFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &HexArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewHexFactory[any]()
		args := factory.CreateDefaultArguments()
		hexArgs, ok := args.(*HexArguments[any])
		require.True(t, ok)
		hexArgs.Target = &ottl.StandardByteSliceLikeGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return []byte("hello"), nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createHexFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "HexFactory args must be of type *HexArguments[K]")
	})
}
