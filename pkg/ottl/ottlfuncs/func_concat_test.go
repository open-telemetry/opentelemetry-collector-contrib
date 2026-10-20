// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/slicegetter"
)

func Test_concat(t *testing.T) {
	tests := []struct {
		name      string
		vals      []ottl.StandardGetSetter[any]
		delimiter ottl.StringGetter[any]
		expected  string
		wantErr   string
	}{
		{
			name: "concat strings",
			vals: []ottl.StandardGetSetter[any]{
				{
					Getter: func(context.Context, any) (any, error) {
						return "hello", nil
					},
				},
				{
					Getter: func(context.Context, any) (any, error) {
						return "world", nil
					},
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return " ", nil }},
			expected:  "hello world",
		},
		{
			name: "nil",
			vals: []ottl.StandardGetSetter[any]{
				{
					Getter: func(context.Context, any) (any, error) {
						return "hello", nil
					},
				},
				{
					Getter: func(context.Context, any) (any, error) {
						return nil, nil
					},
				},
				{
					Getter: func(context.Context, any) (any, error) {
						return "world", nil
					},
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "", nil }},
			expected:  "hello<nil>world",
		},
		{
			name: "integers",
			vals: []ottl.StandardGetSetter[any]{
				{
					Getter: func(context.Context, any) (any, error) {
						return "hello", nil
					},
				},
				{
					Getter: func(context.Context, any) (any, error) {
						return int64(1), nil
					},
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "", nil }},
			expected:  "hello1",
		},
		{
			name: "floats",
			vals: []ottl.StandardGetSetter[any]{
				{
					Getter: func(context.Context, any) (any, error) {
						return "hello", nil
					},
				},
				{
					Getter: func(context.Context, any) (any, error) {
						return 3.14159, nil
					},
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "", nil }},
			expected:  "hello3.14159",
		},
		{
			name: "booleans",
			vals: []ottl.StandardGetSetter[any]{
				{
					Getter: func(context.Context, any) (any, error) {
						return "hello", nil
					},
				},
				{
					Getter: func(context.Context, any) (any, error) {
						return true, nil
					},
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return " ", nil }},
			expected:  "hello true",
		},
		{
			name: "byte slices",
			vals: []ottl.StandardGetSetter[any]{
				{
					Getter: func(context.Context, any) (any, error) {
						return []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0e, 0xd2, 0xe6, 0x3c, 0xbe, 0x71, 0xf5, 0xa8}, nil
					},
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "", nil }},
			expected:  "00000000000000000ed2e63cbe71f5a8",
		},
		{
			name: "pcommon.Slice",
			vals: []ottl.StandardGetSetter[any]{
				{
					Getter: func(context.Context, any) (any, error) {
						s := pcommon.NewSlice()
						_ = s.FromRaw([]any{1, 2})
						return s, nil
					},
				},
				{
					Getter: func(context.Context, any) (any, error) {
						s := pcommon.NewSlice()
						_ = s.FromRaw([]any{3, 4})
						return s, nil
					},
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return ",", nil }},
			wantErr:   "Concat: cannot concatenate a map or list, got pcommon.Slice",
		},
		{
			name: "maps",
			vals: []ottl.StandardGetSetter[any]{
				{
					Getter: func(context.Context, any) (any, error) {
						m := pcommon.NewMap()
						m.PutStr("a", "b")
						return m, nil
					},
				},
				{
					Getter: func(context.Context, any) (any, error) {
						m := pcommon.NewMap()
						m.PutStr("c", "d")
						return m, nil
					},
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return ",", nil }},
			wantErr:   "Concat: cannot concatenate a map or list, got pcommon.Map",
		},
		{
			name: "pcommon.Value map",
			vals: []ottl.StandardGetSetter[any]{
				{
					Getter: func(context.Context, any) (any, error) {
						return pcommon.NewValueMap(), nil
					},
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return ",", nil }},
			wantErr:   "Concat: cannot concatenate a map or list, got pcommon.Value",
		},
		{
			name: "pcommon.Value slice",
			vals: []ottl.StandardGetSetter[any]{
				{
					Getter: func(context.Context, any) (any, error) {
						return pcommon.NewValueSlice(), nil
					},
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return ",", nil }},
			wantErr:   "Concat: cannot concatenate a map or list, got pcommon.Value",
		},
		{
			name: "pcommon.Value bytes",
			vals: []ottl.StandardGetSetter[any]{
				{
					Getter: func(context.Context, any) (any, error) {
						return pcommon.NewValueBytes(), nil
					},
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return ",", nil }},
			expected:  "",
		},
		{
			name: "map[string]any",
			vals: []ottl.StandardGetSetter[any]{
				{
					Getter: func(context.Context, any) (any, error) {
						return map[string]any{"a": "b"}, nil
					},
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return ",", nil }},
			wantErr:   "Concat: cannot concatenate a map or list, got map[string]interface {}",
		},
		{
			name: "[]any",
			vals: []ottl.StandardGetSetter[any]{
				{
					Getter: func(context.Context, any) (any, error) {
						return []any{"a", "b"}, nil
					},
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return ",", nil }},
			wantErr:   "Concat: cannot concatenate a map or list, got []interface {}",
		},
		{
			name: "[]string",
			vals: []ottl.StandardGetSetter[any]{
				{
					Getter: func(context.Context, any) (any, error) {
						return []string{"a", "b"}, nil
					},
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return ",", nil }},
			wantErr:   "Concat: cannot concatenate a map or list, got []string",
		},
		{
			name: "empty string values",
			vals: []ottl.StandardGetSetter[any]{
				{
					Getter: func(context.Context, any) (any, error) {
						return "", nil
					},
				},
				{
					Getter: func(context.Context, any) (any, error) {
						return "", nil
					},
				},
				{
					Getter: func(context.Context, any) (any, error) {
						return "", nil
					},
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "__", nil }},
			expected:  "____",
		},
		{
			name: "single argument",
			vals: []ottl.StandardGetSetter[any]{
				{
					Getter: func(context.Context, any) (any, error) {
						return "hello", nil
					},
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "-", nil }},
			expected:  "hello",
		},
		{
			name:      "no arguments",
			vals:      []ottl.StandardGetSetter[any]{},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "-", nil }},
			expected:  "",
		},
		{
			name:      "no arguments with an empty delimiter",
			vals:      []ottl.StandardGetSetter[any]{},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "", nil }},
			expected:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getters := make([]ottl.Getter[any], len(tt.vals))

			for i, val := range tt.vals {
				getters[i] = val
			}

			vals := slicegetter.NewTestingSliceGetter[any, ottl.Getter[any]](true, getters)
			exprFunc := concat(vals, tt.delimiter)
			result, err := exprFunc(t.Context(), nil)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				var typeErr ottl.TypeError
				require.ErrorAs(t, err, &typeErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_concat_runtime_slice(t *testing.T) {
	t.Parallel()

	getters := []ottl.Getter[any]{
		&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) { return "hello", nil }},
		&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) { return "world", nil }},
	}
	vals := slicegetter.NewTestingSliceGetter[any, ottl.Getter[any]](false, getters)
	delimiter := &ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return " ", nil }}

	exprFunc := concat(vals, delimiter)
	result, err := exprFunc(t.Context(), nil)
	require.NoError(t, err)
	assert.Equal(t, "hello world", result)
}

func Test_concat_runtime_slice_map_item(t *testing.T) {
	t.Parallel()

	getters := []ottl.Getter[any]{
		&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) { return "hello", nil }},
		&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) { return pcommon.NewValueMap(), nil }},
	}
	vals := slicegetter.NewTestingSliceGetter[any, ottl.Getter[any]](false, getters)
	delimiter := &ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return " ", nil }}

	exprFunc := concat(vals, delimiter)
	_, err := exprFunc(t.Context(), nil)
	assert.ErrorContains(t, err, "Concat: cannot concatenate a map or list")
}

func Test_createConcatFunction_literal_map_or_list(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{name: "pcommon.Map", value: pcommon.NewMap()},
		{name: "pcommon.Slice", value: pcommon.NewSlice()},
		{name: "map[string]any", value: map[string]any{"a": "b"}},
		{name: "[]any", value: []any{"a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, err := ottl.NewTestingLiteralGetter(true, &ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) { return "a", nil },
			})
			require.NoError(t, err)
			second, err := ottl.NewTestingLiteralGetter(true, &ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) { return tt.value, nil },
			})
			require.NoError(t, err)
			args := &concatArguments[any]{
				Vals:      *slicegetter.NewTestingSliceGetter[any, ottl.Getter[any]](true, []ottl.Getter[any]{first, second}),
				Delimiter: &ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "", nil }},
			}
			_, err = createConcatFunction[any](ottl.FunctionContext{}, args)
			assert.ErrorContains(t, err, "Concat: value at index 1 is a map or list")
		})
	}
}

func Test_concat_error(t *testing.T) {
	target := &ottl.StandardGetSetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return make(chan int), nil
		},
	}
	delimiter := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "test", nil
		},
	}
	vals := slicegetter.NewTestingSliceGetter[any](true, []ottl.Getter[any]{target})
	exprFunc := concat(vals, delimiter)
	_, err := exprFunc(t.Context(), nil)
	assert.Error(t, err)
}

func Test_concat_error_delimiter(t *testing.T) {
	target := &ottl.StandardGetSetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return make(chan int), nil
		},
	}
	delimiter := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return 3, nil
		},
	}
	vals := slicegetter.NewTestingSliceGetter[any](true, []ottl.Getter[any]{target})
	exprFunc := concat(vals, delimiter)
	_, err := exprFunc(t.Context(), nil)
	assert.Error(t, err)
}

func Test_ConcatFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewConcatFactory[any]()
		assert.Equal(t, "Concat", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewConcatFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &concatArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Vals", "Delimiter"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewConcatFactory[any]()
		args := factory.CreateDefaultArguments()
		concatArgs, ok := args.(*concatArguments[any])
		require.True(t, ok)
		concatArgs.Vals = *slicegetter.NewTestingSliceGetter[any](true, []ottl.Getter[any]{
			&ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "hello", nil
				},
			},
		})
		concatArgs.Delimiter = &ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "-", nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createConcatFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "ConcatFactory args must be of type *concatArguments[K]")
	})
}

func BenchmarkConcat(b *testing.B) {
	getters := []ottl.Getter[any]{
		&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) { return "hello", nil }},
		&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) { return "world", nil }},
		&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) { return int64(42), nil }},
	}
	vals := slicegetter.NewTestingSliceGetter[any](true, getters)
	delimiter := &ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return " ", nil }}

	exprFunc := concat(vals, delimiter)
	ctx := b.Context()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := exprFunc(ctx, nil); err != nil {
			b.Fatal(err)
		}
	}
}
