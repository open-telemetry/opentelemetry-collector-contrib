// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func TestSliceGetter_setReflectValue(t *testing.T) {
	t.Run("literal slice", func(t *testing.T) {
		var sg SliceGetter[any, string]
		err := sg.setReflectValue(reflect.ValueOf([]string{"a", "b"}))
		require.NoError(t, err)
		require.Equal(t, []string{"a", "b"}, sg.typedValues)
		require.Nil(t, sg.runtimeSlice)
	})

	t.Run("literal runtime source folds to literal slice", func(t *testing.T) {
		var sg SliceGetter[any, string]
		src := newTestRuntimeSliceSource[any, string](newLiteral[any, any]([]string{"x", "y"}))
		err := sg.setReflectValue(reflect.ValueOf(*src))
		require.NoError(t, err)
		require.Equal(t, []string{"x", "y"}, sg.typedValues)
		require.Nil(t, sg.runtimeSlice)
	})

	t.Run("non-literal runtime source stays runtime", func(t *testing.T) {
		var sg SliceGetter[any, StringGetter[any]]
		src := newTestRuntimeSliceSource[any, StringGetter[any]](mockedGetter[any]{value: []StringGetter[any]{
			nonLiteralStringGetter[any]{v: "runtime"},
		}})
		err := sg.setReflectValue(reflect.ValueOf(*src))
		require.NoError(t, err)
		require.Nil(t, sg.typedValues)
		require.NotNil(t, sg.runtimeSlice)
	})

	t.Run("invalid value type", func(t *testing.T) {
		var sg SliceGetter[any, string]
		err := sg.setReflectValue(reflect.ValueOf(123))
		require.Error(t, err)
		var typeErr TypeError
		require.ErrorAs(t, err, &typeErr)
	})
}

func Test_isLiteralSliceElementType(t *testing.T) {
	require.True(t, isLiteralSliceElementType(reflect.TypeFor[string]()))
	require.True(t, isLiteralSliceElementType(reflect.TypeFor[uint8]()))
	require.True(t, isLiteralSliceElementType(reflect.TypeFor[float64]()))
	require.True(t, isLiteralSliceElementType(reflect.TypeFor[int64]()))
	require.False(t, isLiteralSliceElementType(reflect.TypeFor[StringGetter[any]]()))
	require.False(t, isLiteralSliceElementType(reflect.TypeFor[int]()))
}

func Test_buildSliceGetterValue(t *testing.T) {
	pc := parseContext[any]{}

	t.Run("list value delegates to buildSliceArg", func(t *testing.T) {
		val := value{
			List: &list{
				Values: []value{
					{String: ottltest.Strp("a")},
					{String: ottltest.Strp("b")},
				},
			},
		}
		result, err := buildSliceGetterValue[any](
			val,
			reflect.TypeFor[string](),
			pc.buildSliceArg,
			pc.buildStandardGetSetter,
			pc.newGetter,
		)
		require.NoError(t, err)
		require.Equal(t, []string{"a", "b"}, result)
	})

	t.Run("byte slice literal without list", func(t *testing.T) {
		raw := byteSlice{0x01, 0x02}
		val := value{Bytes: &raw}
		result, err := buildSliceGetterValue[any](
			val,
			reflect.TypeFor[uint8](),
			pc.buildSliceArg,
			pc.buildStandardGetSetter,
			pc.newGetter,
		)
		require.NoError(t, err)
		require.Equal(t, []byte{0x01, 0x02}, result)
	})

	t.Run("runtime getter for typed slice elements", func(t *testing.T) {
		expectedGetter := newLiteral[any, any]([]StringGetter[any]{
			newLiteral[any, string]("dynamic"),
		})
		result, err := buildSliceGetterValue[any](
			value{},
			reflect.TypeFor[StringGetter[any]](),
			func(value, reflect.Type) (any, error) {
				t.Fatal("buildSliceArg should not be called")
				return nil, nil
			},
			pc.buildStandardGetSetter,
			func(value) (Getter[any], error) {
				return expectedGetter, nil
			},
		)
		require.NoError(t, err)
		src, ok := result.(runtimeSliceSource[any])
		require.True(t, ok)
		assert.Same(t, expectedGetter, src.Getter)
		require.NotNil(t, src.sliceElementCoercer)
	})

	t.Run("buildGetter error", func(t *testing.T) {
		_, err := buildSliceGetterValue[any](
			value{},
			reflect.TypeFor[StringGetter[any]](),
			nil,
			pc.buildStandardGetSetter,
			func(value) (Getter[any], error) {
				return nil, errors.New("getter failed")
			},
		)
		require.EqualError(t, err, "getter failed")
	})
}

func TestGetSliceLiteralValues(t *testing.T) {
	t.Run("empty literal slice", func(t *testing.T) {
		sg := SliceGetter[any, string]{typedValues: []string{}}
		vals, ok := GetSliceLiteralValues[any, string, string](&sg)
		require.True(t, ok)
		require.Empty(t, vals)
	})

	t.Run("static scalar literals", func(t *testing.T) {
		sg := SliceGetter[any, string]{typedValues: []string{"a", "b"}}
		vals, ok := GetSliceLiteralValues[any, string, string](&sg)
		require.True(t, ok)
		require.Equal(t, []string{"a", "b"}, vals)
	})

	t.Run("static typed getter literals", func(t *testing.T) {
		sg := SliceGetter[any, StringGetter[any]]{
			typedValues: []StringGetter[any]{
				newLiteral[any, string]("one"),
				newLiteral[any, string]("two"),
			},
		}
		vals, ok := GetSliceLiteralValues[any, string, StringGetter[any]](&sg)
		require.True(t, ok)
		require.Equal(t, []string{"one", "two"}, vals)
	})

	t.Run("literal runtime slice of typed getters", func(t *testing.T) {
		sg := NewTestingSliceGetter[any, StringGetter[any]](true, []StringGetter[any]{
			newLiteral[any, string]("10.0.0.0/8"),
			newLiteral[any, string]("172.16.0.0/12"),
		})
		vals, ok := GetSliceLiteralValues[any, string, StringGetter[any]](sg)
		require.True(t, ok)
		require.Equal(t, []string{"10.0.0.0/8", "172.16.0.0/12"}, vals)
	})

	t.Run("literal runtime slice of untyped getters", func(t *testing.T) {
		sg := NewTestingSliceGetter[any, Getter[any]](true, []Getter[any]{
			newLiteral[any, any]("first"),
			newLiteral[any, any]("second"),
		})
		vals, ok := GetSliceLiteralValues[any, any, Getter[any]](sg)
		require.True(t, ok)
		require.Equal(t, []any{"first", "second"}, vals)
	})

	t.Run("dynamic non-literal getters", func(t *testing.T) {
		sg := NewTestingSliceGetter[any, StringGetter[any]](false, []StringGetter[any]{
			nonLiteralStringGetter[any]{v: "dynamic"},
		})
		vals, ok := GetSliceLiteralValues[any, string, StringGetter[any]](sg)
		require.False(t, ok)
		require.Nil(t, vals)
	})

	t.Run("mixed literal and non-literal literal values", func(t *testing.T) {
		sg := SliceGetter[any, StringGetter[any]]{
			typedValues: []StringGetter[any]{
				newLiteral[any, string]("literal"),
				nonLiteralStringGetter[any]{v: "dynamic"},
			},
		}
		vals, ok := GetSliceLiteralValues[any, string, StringGetter[any]](&sg)
		require.False(t, ok)
		require.Nil(t, vals)
	})

	t.Run("range error returns false", func(t *testing.T) {
		sg := newTestSliceGetterWithRuntimeSource[any, StringGetter[any]](
			newTestRuntimeSliceSource[any, StringGetter[any]](errSliceGetter{err: errors.New("range failed")}),
		)
		vals, ok := GetSliceLiteralValues[any, string, StringGetter[any]](&sg)
		require.False(t, ok)
		require.Nil(t, vals)
	})
}

func TestSliceGetter_Get(t *testing.T) {
	t.Run("literal values", func(t *testing.T) {
		sg := SliceGetter[any, string]{typedValues: []string{"a", "b"}}
		vals, err := sg.Get(t.Context(), nil)
		require.NoError(t, err)
		require.Equal(t, []string{"a", "b"}, vals)
	})

	t.Run("empty literal values", func(t *testing.T) {
		sg := SliceGetter[any, string]{typedValues: []string{}}
		vals, err := sg.Get(t.Context(), nil)
		require.NoError(t, err)
		require.Empty(t, vals)
	})

	t.Run("dynamic literal slice direct []V", func(t *testing.T) {
		sg := NewTestingSliceGetter[any, string](true, []string{"x", "y"})
		vals, err := sg.Get(t.Context(), nil)
		require.NoError(t, err)
		require.Equal(t, []string{"x", "y"}, vals)
	})

	t.Run("dynamic non-literal []V", func(t *testing.T) {
		sg := NewTestingSliceGetter[any, Getter[any]](false, []Getter[any]{
			newLiteral[any, any]("v1"),
			newLiteral[any, any]("v2"),
		})
		vals, err := sg.Get(t.Context(), nil)
		require.NoError(t, err)
		require.Len(t, vals, 2)
	})

	t.Run("dynamic []any coerced to StringGetter", func(t *testing.T) {
		sg := newTestSliceGetterWithRuntimeSource[any, StringGetter[any]](
			newTestRuntimeSliceSource[any, StringGetter[any]](newLiteral[any, any]([]any{"alpha", "beta"})),
		)
		vals, err := sg.Get(t.Context(), nil)
		require.NoError(t, err)
		require.Len(t, vals, 2)
		first, err := vals[0].Get(t.Context(), nil)
		require.NoError(t, err)
		require.Equal(t, "alpha", first)
	})

	t.Run("dynamic pcommon.Slice coerced to StringGetter", func(t *testing.T) {
		pSlice := pcommon.NewSlice()
		pSlice.AppendEmpty().SetStr("one")
		pSlice.AppendEmpty().SetStr("two")

		sg := newTestSliceGetterWithRuntimeSource[any, StringGetter[any]](
			newTestRuntimeSliceSource[any, StringGetter[any]](newLiteral[any, any](pSlice)),
		)
		vals, err := sg.Get(t.Context(), nil)
		require.NoError(t, err)
		require.Len(t, vals, 2)
		second, err := vals[1].Get(t.Context(), nil)
		require.NoError(t, err)
		require.Equal(t, "two", second)
	})

	t.Run("dynamic pcommon.Value slice wrapper", func(t *testing.T) {
		pVal := pcommon.NewValueSlice()
		pVal.Slice().AppendEmpty().SetStr("wrapped")

		sg := newTestSliceGetterWithRuntimeSource[any, StringGetter[any]](
			newTestRuntimeSliceSource[any, StringGetter[any]](newLiteral[any, any](pVal)),
		)
		vals, err := sg.Get(t.Context(), nil)
		require.NoError(t, err)
		require.Len(t, vals, 1)
	})

	t.Run("getter error", func(t *testing.T) {
		sg := newTestSliceGetterWithRuntimeSource[any, string](
			newTestRuntimeSliceSource[any, string](errSliceGetter{err: errors.New("get failed")}),
		)
		vals, err := sg.Get(t.Context(), nil)
		require.Error(t, err)
		require.Nil(t, vals)
		require.EqualError(t, err, "get failed")
	})

	t.Run("not a slice", func(t *testing.T) {
		sg := newTestSliceGetterWithRuntimeSource[any, string](
			newTestRuntimeSliceSource[any, string](newLiteral[any, any]("not-a-slice")),
		)
		vals, err := sg.Get(t.Context(), nil)
		require.ErrorContains(t, err, "expected a slice")
		require.Nil(t, vals)
	})

	t.Run("pcommon.Value not slice type", func(t *testing.T) {
		pVal := pcommon.NewValueStr("not-slice")
		sg := newTestSliceGetterWithRuntimeSource[any, string](
			newTestRuntimeSliceSource[any, string](newLiteral[any, any](pVal)),
		)
		vals, err := sg.Get(t.Context(), nil)
		require.ErrorContains(t, err, "expected a slice")
		require.Nil(t, vals)
	})

	t.Run("type mismatch returns TypeError", func(t *testing.T) {
		coercer := newTestSliceElementCoercerWithBuilder[any](
			reflect.TypeFor[StringGetter[any]](),
			func(_ string, _ Getter[any]) (any, error) {
				return 123, nil
			},
		)
		sg := newTestSliceGetterWithRuntimeSource[any, StringGetter[any]](
			newTestRuntimeSliceSourceWithCoercer[any](newLiteral[any, any]([]string{"only-strings"}), coercer),
		)
		vals, err := sg.Get(t.Context(), nil)
		require.Error(t, err)
		require.Nil(t, vals)
		var typeErr TypeError
		require.ErrorAs(t, err, &typeErr)
	})
}

func TestSliceGetter_Range(t *testing.T) {
	t.Run("static values", func(t *testing.T) {
		sg := SliceGetter[any, int]{typedValues: []int{1, 2, 3}}
		var collected []int
		err := sg.Range(t.Context(), nil, func(v int) bool {
			collected = append(collected, v)
			return true
		})
		require.NoError(t, err)
		require.Equal(t, []int{1, 2, 3}, collected)
	})

	t.Run("early stop", func(t *testing.T) {
		sg := SliceGetter[any, int]{typedValues: []int{1, 2, 3}}
		var collected []int
		err := sg.Range(t.Context(), nil, func(v int) bool {
			collected = append(collected, v)
			return v < 2
		})
		require.NoError(t, err)
		require.Equal(t, []int{1, 2}, collected)
	})

	t.Run("dynamic []V fast path", func(t *testing.T) {
		sg := NewTestingSliceGetter[any, int](false, []int{4, 5, 6})
		var collected []int
		err := sg.Range(t.Context(), nil, func(v int) bool {
			collected = append(collected, v)
			return true
		})
		require.NoError(t, err)
		require.Equal(t, []int{4, 5, 6}, collected)
	})

	t.Run("dynamic coerced values", func(t *testing.T) {
		sg := newTestSliceGetterWithRuntimeSource[any, StringGetter[any]](
			newTestRuntimeSliceSource[any, StringGetter[any]](newLiteral[any, any]([]any{"a", "b"})),
		)
		count := 0
		err := sg.Range(t.Context(), nil, func(_ StringGetter[any]) bool {
			count++
			return true
		})
		require.NoError(t, err)
		require.Equal(t, 2, count)
	})

	t.Run("getter error", func(t *testing.T) {
		sg := newTestSliceGetterWithRuntimeSource[any, string](
			newTestRuntimeSliceSource[any, string](errSliceGetter{err: errors.New("range get failed")}),
		)
		err := sg.Range(t.Context(), nil, func(_ string) bool { return true })
		require.Error(t, err)
		require.EqualError(t, err, "range get failed")
	})

	t.Run("type mismatch returns TypeError", func(t *testing.T) {
		coercer := newTestSliceElementCoercerWithBuilder[any](
			reflect.TypeFor[StringGetter[any]](),
			func(_ string, _ Getter[any]) (any, error) {
				return struct{}{}, nil
			},
		)
		sg := newTestSliceGetterWithRuntimeSource[any, StringGetter[any]](
			newTestRuntimeSliceSourceWithCoercer[any](newLiteral[any, any]([]string{"x"}), coercer),
		)
		err := sg.Range(t.Context(), nil, func(_ StringGetter[any]) bool { return true })
		require.Error(t, err)
		var typeErr TypeError
		require.ErrorAs(t, err, &typeErr)
	})

	t.Run("rangeSlice error", func(t *testing.T) {
		sg := newTestSliceGetterWithRuntimeSource[any, string](
			newTestRuntimeSliceSource[any, string](newLiteral[any, any]("not-a-slice")),
		)
		err := sg.Range(t.Context(), nil, func(_ string) bool { return true })
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected a slice")
	})
}

func TestSliceGetter_Len(t *testing.T) {
	t.Run("static values", func(t *testing.T) {
		sg := SliceGetter[any, string]{typedValues: []string{"a", "b", "c"}}
		length, ok := sg.Len()
		require.True(t, ok)
		require.Equal(t, 3, length)
	})

	t.Run("empty static values", func(t *testing.T) {
		sg := SliceGetter[any, string]{typedValues: []string{}}
		length, ok := sg.Len()
		require.True(t, ok)
		require.Equal(t, 0, length)
	})

	t.Run("runtime slice", func(t *testing.T) {
		sg := NewTestingSliceGetter[any, string](false, []string{"dynamic"})
		length, ok := sg.Len()
		require.False(t, ok)
		require.Equal(t, 0, length)
	})

	t.Run("unset getter", func(t *testing.T) {
		var sg SliceGetter[any, string]
		length, ok := sg.Len()
		require.False(t, ok)
		require.Equal(t, 0, length)
	})
}

func TestSliceGetter_nilRuntimeSlice(t *testing.T) {
	ctx := t.Context()
	sg := newTestSliceGetterWithRuntimeSource[any, string](
		newTestRuntimeSliceSource[any, string](newLiteral[any, any](nil)),
	)

	t.Run("Get", func(t *testing.T) {
		vals, err := sg.Get(ctx, nil)
		require.NoError(t, err)
		require.Nil(t, vals)
	})

	t.Run("Range", func(t *testing.T) {
		calls := 0
		err := sg.Range(ctx, nil, func(_ string) bool {
			calls++
			return true
		})
		require.NoError(t, err)
		require.Equal(t, 0, calls)
	})
}

func Test_sliceElementCoercer_sliceLen(t *testing.T) {
	coercer := newTestSliceElementCoercer[any, string]()

	t.Run("pcommon.Slice", func(t *testing.T) {
		s := pcommon.NewSlice()
		s.AppendEmpty().SetStr("a")
		lenVal, ok := coercer.sliceLen(s)
		require.True(t, ok)
		require.Equal(t, 1, lenVal)
	})

	t.Run("pcommon.Value slice", func(t *testing.T) {
		v := pcommon.NewValueSlice()
		v.Slice().AppendEmpty().SetStr("a")
		lenVal, ok := coercer.sliceLen(v)
		require.True(t, ok)
		require.Equal(t, 1, lenVal)
	})

	t.Run("pcommon.Value non-slice", func(t *testing.T) {
		v := pcommon.NewValueStr("text")
		lenVal, ok := coercer.sliceLen(v)
		require.False(t, ok)
		require.Equal(t, 0, lenVal)
	})

	t.Run("reflect slice", func(t *testing.T) {
		lenVal, ok := coercer.sliceLen([]string{"a", "b"})
		require.True(t, ok)
		require.Equal(t, 2, lenVal)
	})

	t.Run("not a slice", func(t *testing.T) {
		lenVal, ok := coercer.sliceLen(map[string]any{})
		require.False(t, ok)
		require.Equal(t, 0, lenVal)
	})
}

func Test_sliceElementCoercer_rangeSlice(t *testing.T) {
	t.Run("buildSliceItemGetter error", func(t *testing.T) {
		coercer := newTestSliceElementCoercerWithBuilder[any](
			reflect.TypeFor[string](),
			func(_ string, _ Getter[any]) (any, error) {
				return nil, errors.New("build failed")
			},
		)
		err := coercer.rangeSlice([]any{"a"}, func(_ any) bool { return true })
		require.Error(t, err)
		require.EqualError(t, err, "build failed")
	})

	t.Run("yield stops early", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[any, string]()
		calls := 0
		err := coercer.rangeSlice([]string{"a", "b", "c"}, func(_ any) bool {
			calls++
			return calls < 2
		})
		require.NoError(t, err)
		require.Equal(t, 2, calls)
	})

	t.Run("reuses existing Getter elements", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[any, Getter[any]]()
		original := newLiteral[any, any]("kept")
		var seen Getter[any]
		err := coercer.rangeSlice([]Getter[any]{original}, func(val any) bool {
			seen = val.(Getter[any])
			return true
		})
		require.NoError(t, err)
		assert.Same(t, original, seen)
	})

	t.Run("coerces pcommon.Slice items", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[any, StringGetter[any]]()
		pSlice := pcommon.NewSlice()
		pSlice.AppendEmpty().SetStr("coerced")

		var got string
		err := coercer.rangeSlice(pSlice, func(val any) bool {
			sg := val.(StringGetter[any])
			str, err := sg.Get(t.Context(), nil)
			require.NoError(t, err)
			got = str
			return true
		})
		require.NoError(t, err)
		require.Equal(t, "coerced", got)
	})

	t.Run("pcommon.Slice buildSliceItemGetter error", func(t *testing.T) {
		coercer := newTestSliceElementCoercerWithBuilder[any](
			reflect.TypeFor[string](),
			func(_ string, _ Getter[any]) (any, error) {
				return nil, errors.New("pcommon build failed")
			},
		)
		pSlice := pcommon.NewSlice()
		pSlice.AppendEmpty().SetStr("item")

		err := coercer.rangeSlice(pSlice, func(_ any) bool { return true })
		require.Error(t, err)
		require.EqualError(t, err, "pcommon build failed")
	})

	t.Run("pcommon.Slice yield stops early", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[any, StringGetter[any]]()
		pSlice := pcommon.NewSlice()
		pSlice.AppendEmpty().SetStr("a")
		pSlice.AppendEmpty().SetStr("b")

		calls := 0
		err := coercer.rangeSlice(pSlice, func(_ any) bool {
			calls++
			return calls < 2
		})
		require.NoError(t, err)
		require.Equal(t, 2, calls)
	})

	t.Run("coerces pcommon.Value slice wrapper", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[any, StringGetter[any]]()
		pVal := pcommon.NewValueSlice()
		pVal.Slice().AppendEmpty().SetStr("wrapped")

		count := 0
		err := coercer.rangeSlice(pVal, func(val any) bool {
			count++
			_, ok := val.(StringGetter[any])
			require.True(t, ok)
			return true
		})
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	t.Run("pcommon.Value non-slice error", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[any, string]()
		err := coercer.rangeSlice(pcommon.NewValueStr("text"), func(_ any) bool { return true })
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected a slice")
	})

	t.Run("reflect slice with matching element type", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[any, string]()
		var collected []string
		err := coercer.rangeSlice([]string{"direct"}, func(val any) bool {
			collected = append(collected, val.(string))
			return true
		})
		require.NoError(t, err)
		require.Equal(t, []string{"direct"}, collected)
	})

	t.Run("reflect slice matching type yield stops early", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[any, string]()
		calls := 0
		err := coercer.rangeSlice([]string{"a", "b", "c"}, func(_ any) bool {
			calls++
			return calls < 2
		})
		require.NoError(t, err)
		require.Equal(t, 2, calls)
	})

	t.Run("reflect slice coerced yield stops early", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[any, StringGetter[any]]()
		calls := 0
		err := coercer.rangeSlice([]any{"a", "b"}, func(_ any) bool {
			calls++
			return calls < 2
		})
		require.NoError(t, err)
		require.Equal(t, 2, calls)
	})

	t.Run("reflect slice reuses getter yield stops early", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[any, Getter[any]]()
		calls := 0
		err := coercer.rangeSlice([]Getter[any]{
			newLiteral[any, any]("a"),
			newLiteral[any, any]("b"),
		}, func(_ any) bool {
			calls++
			return calls < 2
		})
		require.NoError(t, err)
		require.Equal(t, 2, calls)
	})

	t.Run("reflect non-slice error", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[any, string]()
		err := coercer.rangeSlice(123, func(_ any) bool { return true })
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected a slice")
	})
}

func Test_getSliceElementLiteralValue(t *testing.T) {
	t.Run("typed literal getter", func(t *testing.T) {
		val, ok := getSliceElementLiteralValue[any, string, StringGetter[any]](newLiteral[any, string]("lit"))
		require.True(t, ok)
		require.Equal(t, "lit", val)
	})

	t.Run("non-literal typed getter", func(t *testing.T) {
		val, ok := getSliceElementLiteralValue[any, string, StringGetter[any]](nonLiteralStringGetter[any]{v: "dyn"})
		require.False(t, ok)
		require.Empty(t, val)
	})

	t.Run("bare scalar value", func(t *testing.T) {
		val, ok := getSliceElementLiteralValue[any, string, string]("scalar")
		require.True(t, ok)
		require.Equal(t, "scalar", val)
	})

	t.Run("unsupported value type", func(t *testing.T) {
		val, ok := getSliceElementLiteralValue[any, string, struct{}](struct{}{})
		require.False(t, ok)
		require.Empty(t, val)
	})
}

func Test_getRuntimeSliceLiterals(t *testing.T) {
	t.Run("non-literal getter", func(t *testing.T) {
		source := newTestRuntimeSliceSource[any, string](mockedGetter[any]{value: []string{"a"}})
		vals, ok := getRuntimeSliceLiterals[any, string](source)
		require.False(t, ok)
		require.Nil(t, vals)
	})

	t.Run("literal getter with direct []V", func(t *testing.T) {
		source := newTestRuntimeSliceSource[any, int](newLiteral[any, any]([]int{1, 2}))
		vals, ok := getRuntimeSliceLiterals[any, int](source)
		require.True(t, ok)
		require.Equal(t, []int{1, 2}, vals)
	})

	t.Run("literal getter with coercible []any", func(t *testing.T) {
		source := newTestRuntimeSliceSource[any, StringGetter[any]](newLiteral[any, any]([]any{"a", "b"}))
		vals, ok := getRuntimeSliceLiterals[any, StringGetter[any]](source)
		require.True(t, ok)
		require.Len(t, vals, 2)
	})

	t.Run("literal getter with pcommon.Slice", func(t *testing.T) {
		pSlice := pcommon.NewSlice()
		pSlice.AppendEmpty().SetStr("a")
		pSlice.AppendEmpty().SetStr("b")
		dyn := newTestRuntimeSliceSource[any, StringGetter[any]](newLiteral[any, any](pSlice))
		vals, ok := getRuntimeSliceLiterals[any, StringGetter[any]](dyn)
		require.True(t, ok)
		require.Len(t, vals, 2)
	})

	t.Run("getter error", func(t *testing.T) {
		source := newTestRuntimeSliceSource[any, string](errSliceGetter{err: errors.New("literal get failed")})
		vals, ok := getRuntimeSliceLiterals[any, string](source)
		require.False(t, ok)
		require.Nil(t, vals)
	})

	t.Run("type mismatch during coercion", func(t *testing.T) {
		coercer := newTestSliceElementCoercerWithBuilder[any](
			reflect.TypeFor[string](),
			func(_ string, _ Getter[any]) (any, error) {
				return struct{}{}, nil
			},
		)
		source := newTestRuntimeSliceSourceWithCoercer[any](newLiteral[any, any]([]any{"x"}), coercer)
		vals, ok := getRuntimeSliceLiterals[any, string](source)
		require.False(t, ok)
		require.Nil(t, vals)
	})

	t.Run("rangeSlice error", func(t *testing.T) {
		source := newTestRuntimeSliceSource[any, string](newLiteral[any, any]("not-a-slice"))
		vals, ok := getRuntimeSliceLiterals[any, string](source)
		require.False(t, ok)
		require.Nil(t, vals)
	})

	t.Run("coercion buildSliceItemGetter error", func(t *testing.T) {
		coercer := newTestSliceElementCoercerWithBuilder[any](
			reflect.TypeFor[string](),
			func(_ string, _ Getter[any]) (any, error) {
				return nil, errors.New("build during literals")
			},
		)
		source := newTestRuntimeSliceSourceWithCoercer[any](newLiteral[any, any]([]any{"x"}), coercer)
		vals, ok := getRuntimeSliceLiterals[any, string](source)
		require.False(t, ok)
		require.Nil(t, vals)
	})
}

type errSliceGetter struct {
	err error
}

func (g errSliceGetter) Get(_ context.Context, _ any) (any, error) {
	return nil, g.err
}

func newTestSliceElementCoercer[K, V any]() *sliceElementCoercer[K] {
	pc := parseContext[K]{}
	return newSliceElementCoercer[K](reflect.TypeFor[V](), pc.buildStandardGetSetter)
}

func newTestSliceElementCoercerWithBuilder[K any](
	sliceItemType reflect.Type,
	buildSliceItemGetter func(string, Getter[K]) (any, error),
) *sliceElementCoercer[K] {
	return newSliceElementCoercer[K](sliceItemType, buildSliceItemGetter)
}

func newTestRuntimeSliceSource[K, V any](getter Getter[K]) *runtimeSliceSource[K] {
	return &runtimeSliceSource[K]{
		Getter:              getter,
		sliceElementCoercer: newTestSliceElementCoercer[K, V](),
	}
}

func newTestRuntimeSliceSourceWithCoercer[K any](getter Getter[K], coercer *sliceElementCoercer[K]) *runtimeSliceSource[K] {
	return &runtimeSliceSource[K]{
		Getter:              getter,
		sliceElementCoercer: coercer,
	}
}

func newTestSliceGetterWithRuntimeSource[K, V any](src *runtimeSliceSource[K]) SliceGetter[K, V] {
	return SliceGetter[K, V]{runtimeSlice: src}
}
