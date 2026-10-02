// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_sanitizeUTF8_string(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		replacement ottl.Optional[string]
		want        string
		wantSet     bool
	}{
		{
			name:  "valid string is not touched",
			input: "héllo 世界",
			want:  "héllo 世界",
		},
		{
			name:    "invalid byte replaced with default replacement",
			input:   "a\xffb",
			want:    "a�b",
			wantSet: true,
		},
		{
			name:    "run of invalid bytes replaced once",
			input:   "a\xff\xfe\xfdb",
			want:    "a�b",
			wantSet: true,
		},
		{
			name:        "custom replacement",
			input:       "a\xffb",
			replacement: ottl.NewTestingOptional("?"),
			want:        "a?b",
			wantSet:     true,
		},
		{
			name:        "empty replacement drops invalid bytes",
			input:       "a\xffb",
			replacement: ottl.NewTestingOptional(""),
			want:        "ab",
			wantSet:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setterCalled := false
			got := tt.input
			target := &ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return tt.input, nil
				},
				Setter: func(_ context.Context, _, val any) error {
					setterCalled = true
					got = val.(string)
					return nil
				},
			}

			exprFunc, err := sanitizeUTF8[any](target, tt.replacement)
			require.NoError(t, err)
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			assert.Nil(t, result)
			assert.Equal(t, tt.wantSet, setterCalled)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_sanitizeUTF8_map(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("valid", "ok")
	input.PutStr("invalid_value", "a\xffb")
	input.PutStr("invalid_key\xff", "c\xffd")
	input.PutInt("int", 1)
	input.PutEmptyBytes("bytes").FromRaw([]byte{0xff})
	nested := input.PutEmptyMap("nested")
	nested.PutStr("k\xff", "v\xff")
	nested.PutEmptySlice("slice").AppendEmpty().SetStr("s\xff")
	slice := input.PutEmptySlice("slice")
	slice.AppendEmpty().SetStr("e\xff")
	slice.AppendEmpty().SetEmptyMap().PutStr("m\xff", "n\xff")

	expected := pcommon.NewMap()
	expected.PutStr("valid", "ok")
	expected.PutStr("invalid_value", "a�b")
	expected.PutInt("int", 1)
	expected.PutEmptyBytes("bytes").FromRaw([]byte{0xff})
	enested := expected.PutEmptyMap("nested")
	enested.PutStr("k�", "v�")
	enested.PutEmptySlice("slice").AppendEmpty().SetStr("s�")
	eslice := expected.PutEmptySlice("slice")
	eslice.AppendEmpty().SetStr("e�")
	eslice.AppendEmpty().SetEmptyMap().PutStr("m�", "n�")
	expected.PutStr("invalid_key�", "c�d")

	setterCalled := false
	target := &ottl.StandardGetSetter[pcommon.Map]{
		Getter: func(_ context.Context, tCtx pcommon.Map) (any, error) {
			return tCtx, nil
		},
		Setter: func(_ context.Context, tCtx pcommon.Map, val any) error {
			setterCalled = true
			m, ok := val.(pcommon.Map)
			require.True(t, ok)
			m.CopyTo(tCtx)
			return nil
		},
	}

	exprFunc, err := sanitizeUTF8[pcommon.Map](target, ottl.Optional[string]{})
	require.NoError(t, err)
	_, err = exprFunc(nil, input)
	require.NoError(t, err)
	assert.True(t, setterCalled)
	assert.Equal(t, expected.AsRaw(), input.AsRaw())
}

func Test_sanitizeUTF8_map_key_conflict(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("key�", "existing")
	input.PutStr("key\xff", "moved")

	target := &ottl.StandardGetSetter[pcommon.Map]{
		Getter: func(_ context.Context, tCtx pcommon.Map) (any, error) {
			return tCtx, nil
		},
		Setter: func(_ context.Context, _ pcommon.Map, _ any) error {
			return nil
		},
	}

	exprFunc, err := sanitizeUTF8[pcommon.Map](target, ottl.Optional[string]{})
	require.NoError(t, err)
	_, err = exprFunc(nil, input)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"key�": "moved"}, input.AsRaw())
}

func Test_sanitizeUTF8_value(t *testing.T) {
	tests := []struct {
		name  string
		input func() pcommon.Value
		want  func() pcommon.Value
	}{
		{
			name:  "string value",
			input: func() pcommon.Value { return pcommon.NewValueStr("a\xffb") },
			want:  func() pcommon.Value { return pcommon.NewValueStr("a�b") },
		},
		{
			name: "map value",
			input: func() pcommon.Value {
				v := pcommon.NewValueMap()
				v.Map().PutStr("k\xff", "v\xff")
				return v
			},
			want: func() pcommon.Value {
				v := pcommon.NewValueMap()
				v.Map().PutStr("k�", "v�")
				return v
			},
		},
		{
			name: "slice value",
			input: func() pcommon.Value {
				v := pcommon.NewValueSlice()
				v.Slice().AppendEmpty().SetStr("a\xff")
				v.Slice().AppendEmpty().SetInt(2)
				return v
			},
			want: func() pcommon.Value {
				v := pcommon.NewValueSlice()
				v.Slice().AppendEmpty().SetStr("a�")
				v.Slice().AppendEmpty().SetInt(2)
				return v
			},
		},
		{
			name:  "bytes value untouched",
			input: pcommon.NewValueBytes,
			want:  pcommon.NewValueBytes,
		},
		{
			name:  "empty value untouched",
			input: pcommon.NewValueEmpty,
			want:  pcommon.NewValueEmpty,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := tt.input()
			target := &ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(_ context.Context, tCtx pcommon.Value) (any, error) {
					return tCtx, nil
				},
				Setter: func(_ context.Context, tCtx pcommon.Value, val any) error {
					v, ok := val.(pcommon.Value)
					require.True(t, ok)
					v.CopyTo(tCtx)
					return nil
				},
			}

			exprFunc, err := sanitizeUTF8[pcommon.Value](target, ottl.Optional[string]{})
			require.NoError(t, err)
			_, err = exprFunc(nil, value)
			require.NoError(t, err)
			assert.Equal(t, tt.want(), value)
		})
	}
}

func Test_sanitizeUTF8_slice(t *testing.T) {
	input := pcommon.NewSlice()
	input.AppendEmpty().SetStr("a\xff")
	input.AppendEmpty().SetEmptyMap().PutStr("k\xff", "v\xff")

	expected := pcommon.NewSlice()
	expected.AppendEmpty().SetStr("a�")
	expected.AppendEmpty().SetEmptyMap().PutStr("k�", "v�")

	target := &ottl.StandardGetSetter[pcommon.Slice]{
		Getter: func(_ context.Context, tCtx pcommon.Slice) (any, error) {
			return tCtx, nil
		},
		Setter: func(_ context.Context, tCtx pcommon.Slice, val any) error {
			s, ok := val.(pcommon.Slice)
			require.True(t, ok)
			s.CopyTo(tCtx)
			return nil
		},
	}

	exprFunc, err := sanitizeUTF8[pcommon.Slice](target, ottl.Optional[string]{})
	require.NoError(t, err)
	_, err = exprFunc(nil, input)
	require.NoError(t, err)
	assert.Equal(t, expected.AsRaw(), input.AsRaw())
}

func Test_sanitizeUTF8_scalars_are_noop(t *testing.T) {
	for _, val := range []any{nil, true, int64(1), 1.5, []byte{0xff}} {
		t.Run(fmt.Sprintf("%T", val), func(t *testing.T) {
			target := &ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return val, nil
				},
				Setter: func(_ context.Context, _, _ any) error {
					return errors.New("setter must not be called")
				},
			}

			exprFunc, err := sanitizeUTF8[any](target, ottl.Optional[string]{})
			require.NoError(t, err)
			_, err = exprFunc(nil, nil)
			require.NoError(t, err)
		})
	}
}

func Test_sanitizeUTF8_unsupported_type(t *testing.T) {
	target := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return pcommon.NewTraceIDEmpty(), nil
		},
	}

	exprFunc, err := sanitizeUTF8[any](target, ottl.Optional[string]{})
	require.NoError(t, err)
	_, err = exprFunc(nil, nil)
	require.ErrorContains(t, err, "sanitize_utf8 target must be a string, map or slice")
}

func Test_sanitizeUTF8_getter_error(t *testing.T) {
	target := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return nil, errors.New("getter failed")
		},
	}

	exprFunc, err := sanitizeUTF8[any](target, ottl.Optional[string]{})
	require.NoError(t, err)
	_, err = exprFunc(nil, nil)
	require.EqualError(t, err, "getter failed")
}

func Test_sanitizeUTF8_setter_error(t *testing.T) {
	target := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "a\xff", nil
		},
		Setter: func(_ context.Context, _, _ any) error {
			return errors.New("setter failed")
		},
	}

	exprFunc, err := sanitizeUTF8[any](target, ottl.Optional[string]{})
	require.NoError(t, err)
	_, err = exprFunc(nil, nil)
	require.EqualError(t, err, "setter failed")
}

func Test_sanitizeUTF8_invalid_replacement(t *testing.T) {
	target := &ottl.StandardGetSetter[any]{}
	_, err := sanitizeUTF8[any](target, ottl.NewTestingOptional("\xff"))
	require.ErrorContains(t, err, "is not valid UTF-8")
}

func Test_sanitizeUTF8_factory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewSanitizeUTF8Factory[any]()
		assert.Equal(t, "sanitize_utf8", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewSanitizeUTF8Factory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &sanitizeUTF8Arguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target", "Replacement"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewSanitizeUTF8Factory[any]()
		args := factory.CreateDefaultArguments()
		sanitizeArgs, ok := args.(*sanitizeUTF8Arguments[any])
		require.True(t, ok)
		sanitizeArgs.Target = &ottl.StandardGetSetter[any]{
			Getter: func(_ context.Context, _ any) (any, error) { return "a\xff", nil },
			Setter: func(_ context.Context, _, val any) error {
				assert.Equal(t, "a\uFFFD", val)
				return nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		_, err = fn(t.Context(), nil)
		require.NoError(t, err)
	})

	t.Run("bad arguments", func(t *testing.T) {
		_, err := createSanitizeUTF8Function[any](ottl.FunctionContext{}, nil)
		require.Error(t, err)
	})
}
