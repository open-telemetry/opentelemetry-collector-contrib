// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_toJSON(t *testing.T) {
	tests := []struct {
		name     string
		target   ottl.Getter[any]
		expected any
	}{
		{
			name: "simple map",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("key1", "value1")
					return m, nil
				},
			},
			expected: `{"key1":"value1"}`,
		},
		{
			name: "nested map",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("key1", "value1")
					nested := m.PutEmptyMap("key2")
					nested.PutStr("nested1", "nestedval1")
					return m, nil
				},
			},
			expected: `{"key1":"value1","key2":{"nested1":"nestedval1"}}`,
		},
		{
			name: "simple slice",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					s := pcommon.NewSlice()
					s.AppendEmpty().SetStr("a")
					s.AppendEmpty().SetStr("b")
					s.AppendEmpty().SetStr("c")
					return s, nil
				},
			},
			expected: `["a","b","c"]`,
		},
		{
			name: "slice of mixed types",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					s := pcommon.NewSlice()
					s.AppendEmpty().SetStr("hello")
					s.AppendEmpty().SetInt(42)
					s.AppendEmpty().SetBool(true)
					s.AppendEmpty().SetDouble(3.14)
					return s, nil
				},
			},
			expected: `["hello",42,true,3.14]`,
		},
		{
			name: "pcommon.Value string",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					v := pcommon.NewValueStr("hello world")
					return v, nil
				},
			},
			expected: `"hello world"`,
		},
		{
			name: "pcommon.Value int",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					v := pcommon.NewValueInt(42)
					return v, nil
				},
			},
			expected: `42`,
		},
		{
			name: "pcommon.Value double",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					v := pcommon.NewValueDouble(3.14)
					return v, nil
				},
			},
			expected: `3.14`,
		},
		{
			name: "pcommon.Value bool",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					v := pcommon.NewValueBool(true)
					return v, nil
				},
			},
			expected: `true`,
		},
		{
			name: "pcommon.Value bytes",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					v := pcommon.NewValueBytes()
					v.Bytes().FromRaw([]byte("hello"))
					return v, nil
				},
			},
			expected: `"aGVsbG8="`,
		},
		{
			name: "pcommon.Value empty (null)",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					v := pcommon.NewValueEmpty()
					return v, nil
				},
			},
			expected: `null`,
		},
		{
			name: "pcommon.Value map",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					v := pcommon.NewValueEmpty()
					m := v.SetEmptyMap()
					m.PutStr("key", "val")
					return v, nil
				},
			},
			expected: `{"key":"val"}`,
		},
		{
			name: "pcommon.Value slice",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					v := pcommon.NewValueEmpty()
					s := v.SetEmptySlice()
					s.AppendEmpty().SetStr("x")
					s.AppendEmpty().SetStr("y")
					return v, nil
				},
			},
			expected: `["x","y"]`,
		},
		{
			name: "raw Go map",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return map[string]any{
						"key1": "value1",
						"key2": float64(42),
					}, nil
				},
			},
			expected: `{"key1":"value1","key2":42}`,
		},
		{
			name: "raw Go slice",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []any{"a", "b", "c"}, nil
				},
			},
			expected: `["a","b","c"]`,
		},
		{
			name: "raw Go slice containing a pcommon.Map (list literal with a path element)",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("key", "val")
					return []any{m, "x"}, nil
				},
			},
			expected: `[{"key":"val"},"x"]`,
		},
		{
			name: "raw Go map containing a pcommon.Slice (map literal with a path element)",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					s := pcommon.NewSlice()
					s.AppendEmpty().SetStr("a")
					return map[string]any{"items": s}, nil
				},
			},
			expected: `{"items":["a"]}`,
		},
		{
			name: "raw Go string",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "hello", nil
				},
			},
			expected: `"hello"`,
		},
		{
			name: "raw Go int",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return 42, nil
				},
			},
			expected: `42`,
		},
		{
			name: "raw Go float64",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return float64(3.14), nil
				},
			},
			expected: `3.14`,
		},
		{
			name: "raw Go bool",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return true, nil
				},
			},
			expected: `true`,
		},
		{
			name: "nil value",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return nil, nil
				},
			},
			expected: nil,
		},
		{
			name: "empty map",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return pcommon.NewMap(), nil
				},
			},
			expected: `{}`,
		},
		{
			name: "empty slice",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return pcommon.NewSlice(), nil
				},
			},
			expected: `[]`,
		},
		{
			name: "complex nested structure",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("name", "test")
					m.PutInt("count", 5)
					m.PutBool("active", true)
					m.PutDouble("ratio", 0.75)
					m.PutEmpty("nothing")

					tags := m.PutEmptySlice("tags")
					tags.AppendEmpty().SetStr("tag1")
					tags.AppendEmpty().SetStr("tag2")

					nested := m.PutEmptyMap("metadata")
					nested.PutStr("source", "ottl")
					inner := nested.PutEmptyMap("details")
					inner.PutInt("level", 3)

					return m, nil
				},
			},
			expected: `{"active":true,"count":5,"metadata":{"details":{"level":3},"source":"ottl"},"name":"test","nothing":null,"ratio":0.75,"tags":["tag1","tag2"]}`,
		},
		{
			name: "map with special characters in values",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("msg", `value with "quotes" and \backslash`)
					m.PutStr("newline", "line1\nline2")
					return m, nil
				},
			},
			expected: `{"msg":"value with \"quotes\" and \\backslash","newline":"line1\nline2"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := toJSON[any](tt.target)
			result, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)

			if tt.expected == nil {
				assert.Nil(t, result)
				return
			}

			actual, ok := result.(string)
			require.True(t, ok)
			assert.JSONEq(t, tt.expected.(string), actual)
		})
	}
}

func Test_toJSON_Error(t *testing.T) {
	target := ottl.StandardGetSetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return nil, assert.AnError
		},
	}
	exprFunc := toJSON[any](target)
	_, err := exprFunc(t.Context(), nil)
	assert.Error(t, err)
}

func Test_toJSON_MarshalError(t *testing.T) {
	// Unlike Test_toJSON_Error, these cases exercise the json.Marshal failure
	// path itself (an unmarshalable value), not the upstream Getter error path.
	tests := []struct {
		name   string
		target ottl.Getter[any]
	}{
		{
			name: "NaN double",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return pcommon.NewValueDouble(math.NaN()), nil
				},
			},
		},
		{
			name: "positive infinity double",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return pcommon.NewValueDouble(math.Inf(1)), nil
				},
			},
		},
		{
			name: "negative infinity double nested in a slice",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []any{math.Inf(-1)}, nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := toJSON[any](tt.target)
			_, err := exprFunc(t.Context(), nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to marshal")
		})
	}
}

func Test_toJSON_roundtrip_with_ParseJSON(t *testing.T) {
	// Test that ToJSON is truly the inverse of ParseJSON.
	// ParseJSON(ToJSON(value)) should return the original structure.
	tests := []struct {
		name   string
		target ottl.Getter[any]
		verify func(t *testing.T, result any)
	}{
		{
			name: "roundtrip map",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("key1", "value1")
					m.PutBool("key2", true)
					m.PutDouble("key3", 42.5)
					return m, nil
				},
			},
			verify: func(t *testing.T, result any) {
				resultMap, ok := result.(pcommon.Map)
				require.True(t, ok)

				v1, ok := resultMap.Get("key1")
				require.True(t, ok)
				assert.Equal(t, "value1", v1.Str())

				v2, ok := resultMap.Get("key2")
				require.True(t, ok)
				assert.Equal(t, true, v2.Bool())

				v3, ok := resultMap.Get("key3")
				require.True(t, ok)
				assert.InDelta(t, 42.5, v3.Double(), 0.001)
			},
		},
		{
			name: "roundtrip slice",
			target: ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					s := pcommon.NewSlice()
					s.AppendEmpty().SetStr("a")
					s.AppendEmpty().SetStr("b")
					return s, nil
				},
			},
			verify: func(t *testing.T, result any) {
				resultSlice, ok := result.(pcommon.Slice)
				require.True(t, ok)
				assert.Equal(t, 2, resultSlice.Len())
				assert.Equal(t, "a", resultSlice.At(0).Str())
				assert.Equal(t, "b", resultSlice.At(1).Str())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Step 1: ToJSON
			toJSONFunc := toJSON[any](tt.target)
			jsonResult, err := toJSONFunc(t.Context(), nil)
			require.NoError(t, err)
			jsonStr, ok := jsonResult.(string)
			require.True(t, ok)

			// Step 2: ParseJSON (feed the JSON string back)
			parseTarget := ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return jsonStr, nil
				},
			}
			parseJSONFunc := parseJSON[any](parseTarget)
			parseResult, err := parseJSONFunc(t.Context(), nil)
			require.NoError(t, err)

			// Step 3: Verify the roundtrip preserved the structure
			tt.verify(t, parseResult)
		})
	}
}

func Test_createToJSONFunction(t *testing.T) {
	// Test that the factory function validates argument types correctly.
	factory := NewToJSONFactory[any]()
	assert.Equal(t, "ToJSON", factory.Name())
}

func BenchmarkToJSON(b *testing.B) {
	ctx := b.Context()
	b.ReportAllocs()

	target := ottl.StandardGetSetter[any]{
		Getter: func(context.Context, any) (any, error) {
			m := pcommon.NewMap()
			m.PutStr("_id", "667cb0db02f4dfc7648b0f6b")
			m.PutInt("index", 0)
			m.PutStr("guid", "2e419732-8214-4e36-a158-d3ced0217ab6")
			m.PutBool("isActive", true)
			m.PutStr("balance", "$1,105.05")
			m.PutInt("age", 22)
			m.PutStr("eyeColor", "blue")
			m.PutStr("name", "Vincent Knox")
			m.PutStr("gender", "male")
			m.PutStr("company", "ANIVET")
			m.PutStr("email", "vincentknox@anivet.com")
			m.PutStr("phone", "+1 (914) 599-2454")

			tags := m.PutEmptySlice("tags")
			tags.AppendEmpty().SetStr("pariatur")
			tags.AppendEmpty().SetStr("anim")
			tags.AppendEmpty().SetStr("id")
			tags.AppendEmpty().SetStr("duis")

			friends := m.PutEmptySlice("friends")
			f0 := friends.AppendEmpty().SetEmptyMap()
			f0.PutInt("id", 0)
			f0.PutStr("name", "Hester Bruce")
			f1 := friends.AppendEmpty().SetEmptyMap()
			f1.PutInt("id", 1)
			f1.PutStr("name", "Laurel Mcknight")

			return m, nil
		},
	}

	exprFunc := toJSON[any](target)

	for b.Loop() {
		_, err := exprFunc(ctx, nil)
		require.NoError(b, err)
	}
}
