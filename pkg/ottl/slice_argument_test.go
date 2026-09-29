// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/slicegetter"
)

func TestSliceGetter_set(t *testing.T) {
	t.Run("literal slice", func(t *testing.T) {
		var sg slicegetter.SliceGetter[any, string]
		require.NoError(t, slicegetter.SetReflectValue(&sg, reflect.ValueOf([]string{"a", "b"})))
		length, ok := sg.Len()
		require.True(t, ok)
		require.Equal(t, 2, length)
	})

	t.Run("invalid value type", func(t *testing.T) {
		var sg slicegetter.SliceGetter[any, string]
		require.Error(t, slicegetter.SetReflectValue(&sg, reflect.ValueOf(123)))
	})

	t.Run("not a slice getter", func(t *testing.T) {
		var s []string
		require.Error(t, slicegetter.SetReflectValue(&s, reflect.ValueOf([]string{"a"})))
		_, ok := slicegetter.ReflectTypeParam(&s)
		require.False(t, ok)
	})

	t.Run("item type", func(t *testing.T) {
		var sg slicegetter.SliceGetter[any, StringGetter[any]]
		itemType, ok := slicegetter.ReflectTypeParam(&sg)
		require.True(t, ok)
		require.Equal(t, reflect.TypeFor[StringGetter[any]](), itemType)
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
					{String: new("a")},
					{String: new("b")},
				},
			},
		}
		result, err := buildSliceGetterValue[any](
			val,
			reflect.TypeFor[string](),
			false,
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
			false,
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
			true,
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
		var sg slicegetter.SliceGetter[any, StringGetter[any]]
		require.NoError(t, slicegetter.SetReflectValue(&sg, reflect.ValueOf(result)))
		vals, err := sg.Get(t.Context(), nil)
		require.NoError(t, err)
		require.Len(t, vals, 1)
		got, err := vals[0].Get(t.Context(), nil)
		require.NoError(t, err)
		require.Equal(t, "dynamic", got)
	})

	t.Run("buildGetter error", func(t *testing.T) {
		_, err := buildSliceGetterValue[any](
			value{},
			reflect.TypeFor[StringGetter[any]](),
			true,
			nil,
			pc.buildStandardGetSetter,
			func(value) (Getter[any], error) {
				return nil, errors.New("getter failed")
			},
		)
		require.EqualError(t, err, "getter failed")
	})

	t.Run("dynamic slices disabled", func(t *testing.T) {
		_, err := buildSliceGetterValue[any](
			value{},
			reflect.TypeFor[StringGetter[any]](),
			false,
			nil,
			pc.buildStandardGetSetter,
			func(value) (Getter[any], error) {
				t.Fatal("buildGetter should not be called")
				return nil, nil
			},
		)
		require.ErrorIs(t, err, errDynamicSliceArgumentsDisabled)
	})
}

func TestGetScalarLiteralValues(t *testing.T) {
	t.Run("empty literal slice", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any](true, []string{})
		vals, ok := slicegetter.GetScalarLiteralValues(sg)
		require.True(t, ok)
		require.Empty(t, vals)
	})

	t.Run("string literals", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any](true, []string{"a", "b"})
		vals, ok := slicegetter.GetScalarLiteralValues(sg)
		require.True(t, ok)
		require.Equal(t, []string{"a", "b"}, vals)
	})

	t.Run("byte literals", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any](true, []byte{1, 2})
		vals, ok := slicegetter.GetScalarLiteralValues(sg)
		require.True(t, ok)
		require.Equal(t, []byte{1, 2}, vals)
	})

	t.Run("int64 literals", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any](true, []int64{-1, 2})
		vals, ok := slicegetter.GetScalarLiteralValues(sg)
		require.True(t, ok)
		require.Equal(t, []int64{-1, 2}, vals)
	})

	t.Run("float64 literals", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any](true, []float64{1.5, 2.5})
		vals, ok := slicegetter.GetScalarLiteralValues(sg)
		require.True(t, ok)
		require.Equal(t, []float64{1.5, 2.5}, vals)
	})

	t.Run("runtime slice", func(t *testing.T) {
		slice := newTestSliceGetterWithRuntimeSource[any, int64](
			newTestRuntimeSliceSource[any, int64](&exprGetter[any]{expr: Expr[any]{
				exprFunc: func(context.Context, any) (any, error) {
					t.Error("runtime slice getter shouldn't be called")
					return int64(0), nil
				},
			}}),
		)
		vals, ok := slicegetter.GetScalarLiteralValues(slice)
		require.False(t, ok)
		require.Nil(t, vals)
	})
}

func TestGetLiteralValues(t *testing.T) {
	t.Run("static typed getter literals", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any](true, []StringGetter[any]{
			newLiteral[any, string]("one"),
			newLiteral[any, string]("two"),
		})
		vals, ok := getLiteralValues[any, string](sg)
		require.True(t, ok)
		require.Equal(t, []string{"one", "two"}, vals)
	})

	t.Run("literal runtime slice of typed getters", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any, StringGetter[any]](true, []StringGetter[any]{
			newLiteral[any, string]("10.0.0.0/8"),
			newLiteral[any, string]("172.16.0.0/12"),
		})
		vals, ok := getLiteralValues[any, string](sg)
		require.True(t, ok)
		require.Equal(t, []string{"10.0.0.0/8", "172.16.0.0/12"}, vals)
	})

	t.Run("literal runtime slice of untyped getters", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any, Getter[any]](true, []Getter[any]{
			newLiteral[any, any]("first"),
			newLiteral[any, any]("second"),
		})
		vals, ok := getLiteralValues[any, any](sg)
		require.True(t, ok)
		require.Equal(t, []any{"first", "second"}, vals)
	})

	t.Run("dynamic non-literal getters", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any, StringGetter[any]](false, []StringGetter[any]{
			nonLiteralStringGetter[any]{v: "dynamic"},
		})
		vals, ok := getLiteralValues[any, string](sg)
		require.False(t, ok)
		require.Nil(t, vals)
	})

	t.Run("runtime slice", func(t *testing.T) {
		slice := newTestSliceGetterWithRuntimeSource[any, StringGetter[any]](
			newTestRuntimeSliceSource[any, StringGetter[any]](&exprGetter[any]{expr: Expr[any]{
				exprFunc: func(context.Context, any) (any, error) {
					t.Error("runtime slice getter shouldn't be called")
					return nil, nil
				},
			}}),
		)
		vals, ok := getLiteralValues[any, string](slice)
		require.False(t, ok)
		require.Nil(t, vals)
	})

	t.Run("mixed literal and non-literal literal values", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any](true, []StringGetter[any]{
			newLiteral[any, string]("literal"),
			nonLiteralStringGetter[any]{v: "dynamic"},
		})
		vals, ok := getLiteralValues[any, string](sg)
		require.False(t, ok)
		require.Nil(t, vals)
	})
}

func TestSliceGetter_Get(t *testing.T) {
	t.Run("literal values", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any](true, []string{"a", "b"})
		vals, err := sg.Get(t.Context(), nil)
		require.NoError(t, err)
		require.Equal(t, []string{"a", "b"}, vals)
	})

	t.Run("empty literal values", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any](true, []string{})
		vals, err := sg.Get(t.Context(), nil)
		require.NoError(t, err)
		require.Empty(t, vals)
	})

	t.Run("dynamic literal slice direct []V", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any, string](true, []string{"x", "y"})
		vals, err := sg.Get(t.Context(), nil)
		require.NoError(t, err)
		require.Equal(t, []string{"x", "y"}, vals)
	})

	t.Run("dynamic non-literal []V", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any, Getter[any]](false, []Getter[any]{
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

func TestSliceGetter_DistinguishesNilAndEmptySlices(t *testing.T) {
	for _, tt := range []struct {
		name       string
		value      any
		wantNonNil bool
	}{
		{name: "nil", wantNonNil: false},
		{name: "typed nil", value: []StringGetter[any](nil), wantNonNil: false},
		{name: "coerced nil", value: []string(nil), wantNonNil: false},
		{name: "empty", value: []StringGetter[any]{}, wantNonNil: true},
		{name: "coerced empty", value: []string{}, wantNonNil: true},
		{name: "pdata empty", value: pcommon.NewSlice(), wantNonNil: true},
		{name: "pdata value empty", value: pcommon.NewValueSlice(), wantNonNil: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, isLiteral := range []bool{true, false} {
				var name string
				var getter Getter[any]
				if isLiteral {
					name = "literal"
					getter = newLiteral[any, any](tt.value)
				} else {
					name = "runtime"
					getter = &exprGetter[any]{expr: Expr[any]{
						exprFunc: func(context.Context, any) (any, error) {
							return tt.value, nil
						},
					}}
				}
				t.Run(name, func(t *testing.T) {
					var sg slicegetter.SliceGetter[any, StringGetter[any]]
					source := newTestRuntimeSliceSource[any, StringGetter[any]](getter)
					require.NoError(t, slicegetter.SetReflectValue(&sg, reflect.ValueOf(source.value())))
					nonNil, err := sg.Range(t.Context(), nil, func(StringGetter[any]) bool {
						t.Fatal("nil or empty slice must not invoke yield")
						return false
					})
					require.NoError(t, err)
					require.Equal(t, tt.wantNonNil, nonNil)
					values, err := sg.Get(t.Context(), nil)
					require.NoError(t, err)
					require.Equal(t, tt.wantNonNil, values != nil)
				})
			}
		})
	}
}

func TestSliceGetter_RangeEvaluatesOnceAndStopsEarly(t *testing.T) {
	getCalls := 0
	sg := newTestSliceGetterWithRuntimeSource[any, StringGetter[any]](
		newTestRuntimeSliceSource[any, StringGetter[any]](&exprGetter[any]{expr: Expr[any]{
			exprFunc: func(context.Context, any) (any, error) {
				getCalls++
				return []string{"a", "b"}, nil
			},
		}}),
	)
	calls := 0
	nonNil, err := sg.Range(t.Context(), nil, func(StringGetter[any]) bool {
		calls++
		return false
	})
	require.NoError(t, err)
	require.True(t, nonNil)
	require.Equal(t, 1, calls)
	require.Equal(t, 1, getCalls)
}

func TestSliceGetter_Range(t *testing.T) {
	t.Run("static values", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any](true, []int{1, 2, 3})
		var collected []int
		nonNil, err := sg.Range(t.Context(), nil, func(v int) bool {
			collected = append(collected, v)
			return true
		})
		require.NoError(t, err)
		require.True(t, nonNil)
		require.Equal(t, []int{1, 2, 3}, collected)
	})

	t.Run("early stop", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any](true, []int{1, 2, 3})
		var collected []int
		nonNil, err := sg.Range(t.Context(), nil, func(v int) bool {
			collected = append(collected, v)
			return v < 2
		})
		require.NoError(t, err)
		require.True(t, nonNil)
		require.Equal(t, []int{1, 2}, collected)
	})

	t.Run("dynamic []V fast path", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any, int](false, []int{4, 5, 6})
		var collected []int
		nonNil, err := sg.Range(t.Context(), nil, func(v int) bool {
			collected = append(collected, v)
			return true
		})
		require.NoError(t, err)
		require.True(t, nonNil)
		require.Equal(t, []int{4, 5, 6}, collected)
	})

	t.Run("dynamic coerced values", func(t *testing.T) {
		sg := newTestSliceGetterWithRuntimeSource[any, StringGetter[any]](
			newTestRuntimeSliceSource[any, StringGetter[any]](newLiteral[any, any]([]any{"a", "b"})),
		)
		count := 0
		nonNil, err := sg.Range(t.Context(), nil, func(_ StringGetter[any]) bool {
			count++
			return true
		})
		require.NoError(t, err)
		require.True(t, nonNil)
		require.Equal(t, 2, count)
	})

	t.Run("getter error", func(t *testing.T) {
		sg := newTestSliceGetterWithRuntimeSource[any, string](
			newTestRuntimeSliceSource[any, string](errSliceGetter{err: errors.New("range get failed")}),
		)
		nonNil, err := sg.Range(t.Context(), nil, func(_ string) bool { return true })
		require.Error(t, err)
		require.False(t, nonNil)
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
		nonNil, err := sg.Range(t.Context(), nil, func(_ StringGetter[any]) bool { return true })
		require.Error(t, err)
		require.False(t, nonNil)
		var typeErr TypeError
		require.ErrorAs(t, err, &typeErr)
	})

	t.Run("rangeSlice error", func(t *testing.T) {
		sg := newTestSliceGetterWithRuntimeSource[any, string](
			newTestRuntimeSliceSource[any, string](newLiteral[any, any]("not-a-slice")),
		)
		nonNil, err := sg.Range(t.Context(), nil, func(_ string) bool { return true })
		require.Error(t, err)
		require.False(t, nonNil)
		require.Contains(t, err.Error(), "expected a slice")
	})
}

func BenchmarkSliceGetter(b *testing.B) {
	strings := []string{"one", "two", "three"}
	stringGetters := make([]StringGetter[any], 0, len(strings))
	for _, val := range strings {
		getter, err := NewTestingLiteralGetter[any, string](true, StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return val, nil
			},
		})
		if err != nil {
			b.Fatal(err)
		}
		stringGetters = append(stringGetters, getter)
	}

	literalScalarSliceGetter := slicegetter.NewTestingSliceGetter[any, string](true, strings)
	runtimeScalarSliceGetter := slicegetter.NewTestingSliceGetter[any, string](false, strings)
	literalGetterSliceGetter := slicegetter.NewTestingSliceGetter[any, StringGetter[any]](true, stringGetters)
	runtimeGetterSliceGetter := slicegetter.NewTestingSliceGetter[any, StringGetter[any]](false, stringGetters)

	b.Run("string/bare", func(b *testing.B) {
		b.ReportAllocs()
		total := 0
		for b.Loop() {
			for _, val := range strings {
				total += len(val)
			}
		}
	})

	benchmarkScalarGet := func(b *testing.B, sliceGetter *slicegetter.SliceGetter[any, string]) {
		ctx := b.Context()
		b.ReportAllocs()
		total := 0
		for b.Loop() {
			vals, err := sliceGetter.Get(ctx, nil)
			if err != nil {
				b.Fatal(err)
			}
			for _, val := range vals {
				total += len(val)
			}
		}
	}
	b.Run("string/get/literal", func(b *testing.B) {
		benchmarkScalarGet(b, literalScalarSliceGetter)
	})
	b.Run("string/get/runtime", func(b *testing.B) {
		benchmarkScalarGet(b, runtimeScalarSliceGetter)
	})

	benchmarkScalarRange := func(b *testing.B, sliceGetter *slicegetter.SliceGetter[any, string]) {
		ctx := b.Context()
		b.ReportAllocs()
		total := 0
		for b.Loop() {
			_, err := sliceGetter.Range(ctx, nil, func(val string) bool {
				total += len(val)
				return true
			})
			if err != nil {
				b.Fatal(err)
			}
		}
	}
	b.Run("string/range/literal", func(b *testing.B) {
		benchmarkScalarRange(b, literalScalarSliceGetter)
	})
	b.Run("string/range/runtime", func(b *testing.B) {
		benchmarkScalarRange(b, runtimeScalarSliceGetter)
	})

	b.Run("StringGetter/bare", func(b *testing.B) {
		ctx := b.Context()
		b.ReportAllocs()
		total := 0
		for b.Loop() {
			for _, getter := range stringGetters {
				val, err := getter.Get(ctx, nil)
				if err != nil {
					b.Fatal(err)
				}
				total += len(val)
			}
		}
	})

	benchmarkGetterGet := func(b *testing.B, sliceGetter *slicegetter.SliceGetter[any, StringGetter[any]]) {
		ctx := b.Context()
		b.ReportAllocs()
		total := 0
		for b.Loop() {
			getters, err := sliceGetter.Get(ctx, nil)
			if err != nil {
				b.Fatal(err)
			}
			for _, getter := range getters {
				val, err := getter.Get(ctx, nil)
				if err != nil {
					b.Fatal(err)
				}
				total += len(val)
			}
		}
	}
	b.Run("StringGetter/get/literal", func(b *testing.B) {
		benchmarkGetterGet(b, literalGetterSliceGetter)
	})
	b.Run("StringGetter/get/runtime", func(b *testing.B) {
		benchmarkGetterGet(b, runtimeGetterSliceGetter)
	})

	benchmarkGetterRange := func(b *testing.B, sliceGetter *slicegetter.SliceGetter[any, StringGetter[any]]) {
		ctx := b.Context()
		b.ReportAllocs()
		total := 0
		for b.Loop() {
			var getErr error
			_, err := sliceGetter.Range(ctx, nil, func(getter StringGetter[any]) bool {
				val, err := getter.Get(ctx, nil)
				if err != nil {
					getErr = err
					return false
				}
				total += len(val)
				return true
			})
			if err != nil {
				b.Fatal(err)
			}
			if getErr != nil {
				b.Fatal(getErr)
			}
		}
	}
	b.Run("StringGetter/range/literal", func(b *testing.B) {
		benchmarkGetterRange(b, literalGetterSliceGetter)
	})
	b.Run("StringGetter/range/runtime", func(b *testing.B) {
		benchmarkGetterRange(b, runtimeGetterSliceGetter)
	})
}

func TestSliceGetter_Len(t *testing.T) {
	t.Run("static values", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any](true, []string{"a", "b", "c"})
		length, ok := sg.Len()
		require.True(t, ok)
		require.Equal(t, 3, length)
	})

	t.Run("empty static values", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any](true, []string{})
		length, ok := sg.Len()
		require.True(t, ok)
		require.Equal(t, 0, length)
	})

	t.Run("runtime slice", func(t *testing.T) {
		sg := slicegetter.NewTestingSliceGetter[any, string](false, []string{"dynamic"})
		length, ok := sg.Len()
		require.False(t, ok)
		require.Equal(t, 0, length)
	})

	t.Run("unset getter", func(t *testing.T) {
		var sg slicegetter.SliceGetter[any, string]
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
		nonNil, err := sg.Range(ctx, nil, func(_ string) bool {
			calls++
			return true
		})
		require.NoError(t, err)
		require.False(t, nonNil)
		require.Equal(t, 0, calls)
	})
}

func TestSliceGetter_nilLiteralSlice(t *testing.T) {
	ctx := t.Context()
	var sg slicegetter.SliceGetter[any, string]
	src := newTestRuntimeSliceSource[any, string](newLiteral[any, any](nil))
	require.NoError(t, slicegetter.SetReflectValue(&sg, reflect.ValueOf(src.value())))

	t.Run("Get", func(t *testing.T) {
		vals, err := sg.Get(ctx, nil)
		require.NoError(t, err)
		require.Nil(t, vals)
	})

	t.Run("Range", func(t *testing.T) {
		calls := 0
		nonNil, err := sg.Range(ctx, nil, func(_ string) bool {
			calls++
			return true
		})
		require.NoError(t, err)
		require.False(t, nonNil)
		require.Zero(t, calls)
	})
}

func TestSliceGetter_foldsLiteralSources(t *testing.T) {
	t.Run("non-literal getter", func(t *testing.T) {
		source := newTestRuntimeSliceSource[any, string](mockedGetter[any]{value: []string{"a"}})
		vals, ok := foldLiterals[string](t, source)
		require.False(t, ok)
		require.Nil(t, vals)
	})

	t.Run("literal getter with direct []V", func(t *testing.T) {
		source := newTestRuntimeSliceSource[any, int](newLiteral[any, any]([]int{1, 2}))
		vals, ok := foldLiterals[int](t, source)
		require.True(t, ok)
		require.Equal(t, []int{1, 2}, vals)
	})

	t.Run("literal getter with coercible []any", func(t *testing.T) {
		source := newTestRuntimeSliceSource[any, StringGetter[any]](newLiteral[any, any]([]any{"a", "b"}))
		vals, ok := foldLiterals[StringGetter[any]](t, source)
		require.True(t, ok)
		require.Len(t, vals, 2)
	})

	t.Run("literal getter with pcommon.Slice", func(t *testing.T) {
		pSlice := pcommon.NewSlice()
		pSlice.AppendEmpty().SetStr("a")
		pSlice.AppendEmpty().SetStr("b")
		dyn := newTestRuntimeSliceSource[any, StringGetter[any]](newLiteral[any, any](pSlice))
		vals, ok := foldLiterals[StringGetter[any]](t, dyn)
		require.True(t, ok)
		require.Len(t, vals, 2)
	})

	t.Run("getter error", func(t *testing.T) {
		source := newTestRuntimeSliceSource[any, string](errSliceGetter{err: errors.New("literal get failed")})
		vals, ok := foldLiterals[string](t, source)
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
		vals, ok := foldLiterals[string](t, source)
		require.False(t, ok)
		require.Nil(t, vals)
	})

	t.Run("rangeSlice error", func(t *testing.T) {
		source := newTestRuntimeSliceSource[any, string](newLiteral[any, any]("not-a-slice"))
		vals, ok := foldLiterals[string](t, source)
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
		vals, ok := foldLiterals[string](t, source)
		require.False(t, ok)
		require.Nil(t, vals)
	})
}

// foldLiterals reports whether setting source folds it into literal values, and returns them.
func foldLiterals[V any](t *testing.T, source *testRuntimeSliceSource[any]) ([]V, bool) {
	var sg slicegetter.SliceGetter[any, V]
	require.NoError(t, slicegetter.SetReflectValue(&sg, reflect.ValueOf(source.value())))
	if _, ok := sg.Len(); !ok {
		return nil, false
	}
	vals, err := sg.Get(t.Context(), nil)
	require.NoError(t, err)
	return vals, true
}

type errSliceGetter struct {
	err error
}

func (g errSliceGetter) Get(_ context.Context, _ any) (any, error) {
	return nil, g.err
}

type testSliceElementCoercer[K any] struct {
	sliceItemType        reflect.Type
	buildSliceItemGetter func(string, Getter[K]) (any, error)
}

type testRuntimeSliceSource[K any] struct {
	getter Getter[K]
	*testSliceElementCoercer[K]
}

func (s *testRuntimeSliceSource[K]) value() any {
	return newRuntimeSliceSource(s.getter, isLiteralGetter(s.getter), s.sliceItemType, s.buildSliceItemGetter)
}

func newTestSliceElementCoercer[K, V any]() *testSliceElementCoercer[K] {
	pc := parseContext[K]{}
	return newTestSliceElementCoercerWithBuilder[K](reflect.TypeFor[V](), pc.buildStandardGetSetter)
}

func newTestSliceElementCoercerWithBuilder[K any](
	sliceItemType reflect.Type,
	buildSliceItemGetter func(string, Getter[K]) (any, error),
) *testSliceElementCoercer[K] {
	return &testSliceElementCoercer[K]{sliceItemType: sliceItemType, buildSliceItemGetter: buildSliceItemGetter}
}

func newTestRuntimeSliceSource[K, V any](getter Getter[K]) *testRuntimeSliceSource[K] {
	return &testRuntimeSliceSource[K]{
		getter:                  getter,
		testSliceElementCoercer: newTestSliceElementCoercer[K, V](),
	}
}

func newTestRuntimeSliceSourceWithCoercer[K any](getter Getter[K], coercer *testSliceElementCoercer[K]) *testRuntimeSliceSource[K] {
	return &testRuntimeSliceSource[K]{
		getter:                  getter,
		testSliceElementCoercer: coercer,
	}
}

// newTestSliceGetterWithRuntimeSource resolves src at runtime even when its getter is a
// literal, instead of folding it into literal values.
func newTestSliceGetterWithRuntimeSource[K, V any](src *testRuntimeSliceSource[K]) *slicegetter.SliceGetter[K, V] {
	var sg slicegetter.SliceGetter[K, V]
	err := slicegetter.SetReflectValue(&sg, reflect.ValueOf(newRuntimeSliceSource(src.getter, false, src.sliceItemType, src.buildSliceItemGetter)))
	if err != nil {
		panic(err)
	}
	return &sg
}

func getLiteralValues[K, V any, G TypedGetter[K, V]](slice *slicegetter.SliceGetter[K, G]) ([]V, bool) {
	return slicegetter.GetLiteralValues(slice, func(getter G) (V, bool) {
		return GetLiteralValue[K, V](getter)
	})
}
