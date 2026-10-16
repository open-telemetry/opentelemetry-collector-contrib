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

func literalFormat(tb testing.TB, format string) ottl.StringGetter[any] {
	tb.Helper()
	g, err := ottl.NewTestingLiteralGetter[any, string](true, &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return format, nil
		},
	})
	require.NoError(tb, err)
	return g
}

func runtimeFormat(format string) ottl.StringGetter[any] {
	return &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return format, nil
		},
	}
}

func stringTarget(value any) ottl.StringGetter[any] {
	return &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return value, nil
		},
	}
}

func Test_sanitize(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		format      ottl.StringGetter[any]
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
		{
			name:   "format resolved at run time",
			input:  "a\xffb",
			format: runtimeFormat("utf-8"),
			want:   "a�b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			format := tt.format
			if format == nil {
				format = literalFormat(t, "utf-8")
			}
			exprFunc, err := sanitize[any](stringTarget(tt.input), format, tt.replacement)
			require.NoError(t, err)
			result, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_sanitize_unsupported_format(t *testing.T) {
	t.Run("literal format fails at parse time", func(t *testing.T) {
		_, err := sanitize[any](stringTarget("a"), literalFormat(t, "ascii"), ottl.Optional[string]{})
		require.ErrorContains(t, err, `unsupported format for Sanitize function: "ascii"`)
	})

	t.Run("utf8 without hyphen is not accepted", func(t *testing.T) {
		_, err := sanitize[any](stringTarget("a"), literalFormat(t, "utf8"), ottl.Optional[string]{})
		require.ErrorContains(t, err, `unsupported format for Sanitize function: "utf8"`)
	})

	t.Run("runtime format fails at execution time", func(t *testing.T) {
		exprFunc, err := sanitize[any](stringTarget("a"), runtimeFormat("ascii"), ottl.Optional[string]{})
		require.NoError(t, err)
		_, err = exprFunc(t.Context(), nil)
		require.ErrorContains(t, err, `unsupported format for Sanitize function: "ascii"`)
	})

	t.Run("runtime format getter error", func(t *testing.T) {
		format := &ottl.StandardStringGetter[any]{
			Getter: func(_ context.Context, _ any) (any, error) {
				return nil, errors.New("format failed")
			},
		}
		exprFunc, err := sanitize[any](stringTarget("a"), format, ottl.Optional[string]{})
		require.NoError(t, err)
		_, err = exprFunc(t.Context(), nil)
		require.ErrorContains(t, err, "format failed")
	})
}

func Test_sanitize_invalid_replacement(t *testing.T) {
	t.Run("literal format fails at parse time", func(t *testing.T) {
		_, err := sanitize[any](stringTarget("a"), literalFormat(t, "utf-8"), ottl.NewTestingOptional("\xff"))
		require.ErrorContains(t, err, "is not valid utf-8")
	})

	t.Run("runtime format fails at execution time", func(t *testing.T) {
		exprFunc, err := sanitize[any](stringTarget("a"), runtimeFormat("utf-8"), ottl.NewTestingOptional("\xff"))
		require.NoError(t, err)
		_, err = exprFunc(t.Context(), nil)
		require.ErrorContains(t, err, "is not valid utf-8")
	})
}

func Test_sanitize_target_errors(t *testing.T) {
	t.Run("getter error", func(t *testing.T) {
		target := &ottl.StandardStringGetter[any]{
			Getter: func(_ context.Context, _ any) (any, error) {
				return nil, errors.New("getter failed")
			},
		}
		exprFunc, err := sanitize[any](target, literalFormat(t, "utf-8"), ottl.Optional[string]{})
		require.NoError(t, err)
		_, err = exprFunc(t.Context(), nil)
		require.ErrorContains(t, err, "getter failed")
	})

	t.Run("non-string target", func(t *testing.T) {
		exprFunc, err := sanitize[any](stringTarget(int64(1)), literalFormat(t, "utf-8"), ottl.Optional[string]{})
		require.NoError(t, err)
		_, err = exprFunc(t.Context(), nil)
		require.Error(t, err)
	})
}

func Test_sanitize_factory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewSanitizeFactory[any]()
		assert.Equal(t, "Sanitize", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewSanitizeFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &sanitizeArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target", "Format", "Replacement"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewSanitizeFactory[any]()
		args := factory.CreateDefaultArguments()
		sanitizeArgs, ok := args.(*sanitizeArguments[any])
		require.True(t, ok)
		sanitizeArgs.Target = stringTarget("a\xff")
		sanitizeArgs.Format = literalFormat(t, "utf-8")

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		result, err := fn(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, "a�", result)
	})

	t.Run("bad arguments", func(t *testing.T) {
		_, err := createSanitizeFunction[any](ottl.FunctionContext{}, nil)
		require.Error(t, err)
	})
}
