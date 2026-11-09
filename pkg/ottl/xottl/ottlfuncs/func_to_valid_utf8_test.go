// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func stringTarget(value any) ottl.StringGetter[any] {
	return &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return value, nil
		},
	}
}

func Test_toValidUTF8(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		replacement ottl.Optional[string]
		want        string
	}{
		{
			name:  "valid string is returned unchanged",
			input: "héllo 世界",
			want:  "héllo 世界",
		},
		{
			name:  "invalid byte replaced with default replacement",
			input: "a\xffb",
			want:  "a�b",
		},
		{
			name:  "run of invalid bytes replaced once",
			input: "a\xff\xfe\xfdb",
			want:  "a�b",
		},
		{
			name:  "truncated multibyte sequence",
			input: "a\xe4\xb8",
			want:  "a�",
		},
		{
			name:        "custom replacement",
			input:       "a\xffb",
			replacement: ottl.NewTestingOptional("?"),
			want:        "a?b",
		},
		{
			name:        "empty replacement drops invalid bytes",
			input:       "a\xffb",
			replacement: ottl.NewTestingOptional(""),
			want:        "ab",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := toValidUTF8[any](stringTarget(tt.input), tt.replacement)
			require.NoError(t, err)
			result, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_toValidUTF8_invalid_replacement(t *testing.T) {
	_, err := toValidUTF8[any](stringTarget("a"), ottl.NewTestingOptional("\xff"))
	require.ErrorContains(t, err, "is not valid UTF-8")
}

func Test_toValidUTF8_target_errors(t *testing.T) {
	t.Run("getter error", func(t *testing.T) {
		target := &ottl.StandardStringGetter[any]{
			Getter: func(_ context.Context, _ any) (any, error) {
				return nil, errors.New("getter failed")
			},
		}
		exprFunc, err := toValidUTF8[any](target, ottl.Optional[string]{})
		require.NoError(t, err)
		_, err = exprFunc(t.Context(), nil)
		require.ErrorContains(t, err, "getter failed")
	})

	t.Run("non-string target", func(t *testing.T) {
		exprFunc, err := toValidUTF8[any](stringTarget(int64(1)), ottl.Optional[string]{})
		require.NoError(t, err)
		_, err = exprFunc(t.Context(), nil)
		require.Error(t, err)
	})
}

func Test_toValidUTF8_factory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewToValidUTF8Factory[any]()
		assert.Equal(t, "ToValidUTF8", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewToValidUTF8Factory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &toValidUTF8Arguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target", "Replacement"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewToValidUTF8Factory[any]()
		args := factory.CreateDefaultArguments()
		toValidUTF8Args, ok := args.(*toValidUTF8Arguments[any])
		require.True(t, ok)
		toValidUTF8Args.Target = stringTarget("a\xff")

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		result, err := fn(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, "a�", result)
	})

	t.Run("bad arguments", func(t *testing.T) {
		_, err := createToValidUTF8Function[any](ottl.FunctionContext{}, nil)
		require.Error(t, err)
	})
}
