// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package slicegetter

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type testGetter struct {
	value any
	err   error
	gets  int
}

func (g *testGetter) Get(context.Context, any) (any, error) {
	g.gets++
	return g.value, g.err
}

func newTestLiteral(value any) Getter[any] {
	return &testGetter{value: value}
}

func buildGetterItem(_ string, getter Getter[any]) (any, error) {
	return getter, nil
}

func newTestSliceElementCoercer[V any](buildSliceItemGetter func(string, Getter[any]) (any, error)) *sliceElementCoercer[any] {
	return newSliceElementCoercer[any](reflect.TypeFor[V](), buildSliceItemGetter, newTestLiteral)
}

func newTestSliceGetter[V any](getter Getter[any], isLiteral bool) *SliceGetter[any, V] {
	var sg SliceGetter[any, V]
	source := NewRuntimeSliceSource[any](getter, isLiteral, reflect.TypeFor[V](), buildGetterItem, newTestLiteral)
	if err := SetReflectValue(&sg, reflect.ValueOf(source)); err != nil {
		panic(err)
	}
	return &sg
}

func TestReflectTypeParamAndSetReflectValue(t *testing.T) {
	var sg SliceGetter[any, string]
	itemType, ok := ReflectTypeParam(&sg)
	require.True(t, ok)
	assert.Equal(t, reflect.TypeFor[string](), itemType)

	require.NoError(t, SetReflectValue(&sg, reflect.ValueOf([]string{"a"})))
	vals, err := sg.Get(t.Context(), nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"a"}, vals)

	require.ErrorContains(t, SetReflectValue(&sg, reflect.ValueOf(1)), "cannot set value of type int")

	var notSliceGetter []string
	_, ok = ReflectTypeParam(&notSliceGetter)
	assert.False(t, ok)
	require.Error(t, SetReflectValue(&notSliceGetter, reflect.ValueOf([]string{"a"})))
}

func TestSetReflectValue_foldsLiteralSources(t *testing.T) {
	tests := []struct {
		name       string
		getter     *testGetter
		wantFolded bool
		wantLen    int
	}{
		{name: "coerced items", getter: &testGetter{value: []any{"a", "b"}}, wantFolded: true, wantLen: 2},
		{name: "typed items", getter: &testGetter{value: []Getter[any]{newTestLiteral("a")}}, wantFolded: true, wantLen: 1},
		{name: "nil", getter: &testGetter{}},
		{name: "not a slice", getter: &testGetter{value: "a"}},
		{name: "get error", getter: &testGetter{err: errors.New("boom")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sg := newTestSliceGetter[Getter[any]](tt.getter, true)
			length, ok := sg.Len()
			assert.Equal(t, tt.wantFolded, ok)
			assert.Equal(t, tt.wantLen, length)
		})
	}
}

func TestSliceGetter_runtimeSource(t *testing.T) {
	getter := &testGetter{value: []any{"a", "b"}}
	sg := newTestSliceGetter[Getter[any]](getter, false)
	assert.Zero(t, getter.gets)

	vals, err := sg.Get(t.Context(), nil)
	require.NoError(t, err)
	require.Len(t, vals, 2)

	calls := 0
	nonNil, err := sg.Range(t.Context(), nil, func(Getter[any]) bool {
		calls++
		return false
	})
	require.NoError(t, err)
	assert.True(t, nonNil)
	assert.Equal(t, 1, calls)
	assert.Equal(t, 2, getter.gets)

	_, ok := GetLiteralValues(sg, func(g Getter[any]) (Getter[any], bool) { return g, true })
	assert.False(t, ok)
	assert.Equal(t, 2, getter.gets)

	getter.value = nil
	vals, err = sg.Get(t.Context(), nil)
	require.NoError(t, err)
	assert.Nil(t, vals)
	nonNil, err = sg.Range(t.Context(), nil, func(Getter[any]) bool { return true })
	require.NoError(t, err)
	assert.False(t, nonNil)
}

func TestSliceGetter_itemTypeMismatch(t *testing.T) {
	var sg SliceGetter[any, string]
	source := NewRuntimeSliceSource[any](&testGetter{value: []any{1}}, false, reflect.TypeFor[string](), buildGetterItem, newTestLiteral)
	require.NoError(t, SetReflectValue(&sg, reflect.ValueOf(source)))

	_, err := sg.Get(t.Context(), nil)
	require.ErrorContains(t, err, "expected slice item of type string")
	_, err = sg.Range(t.Context(), nil, func(string) bool { return true })
	require.ErrorContains(t, err, "expected slice item of type string")
}

func TestGetLiteralValues(t *testing.T) {
	sg := NewTestingSliceGetter[any](true, []string{"a", "b"})
	vals, ok := GetScalarLiteralValues(sg)
	require.True(t, ok)
	assert.Equal(t, []string{"a", "b"}, vals)

	vals, ok = GetLiteralValues(sg, func(v string) (string, bool) { return v, true })
	require.True(t, ok)
	assert.Equal(t, []string{"a", "b"}, vals)

	vals, ok = GetLiteralValues(sg, func(v string) (string, bool) { return v, v == "a" })
	require.False(t, ok)
	assert.Nil(t, vals)

	vals, ok = GetScalarLiteralValues(NewTestingSliceGetter[any](false, []string{"a"}))
	require.False(t, ok)
	assert.Nil(t, vals)
}

func TestNewTestingSliceGetter(t *testing.T) {
	for _, literal := range []bool{true, false} {
		sg := NewTestingSliceGetter[any](literal, []int64{1, 2})
		vals, err := sg.Get(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, []int64{1, 2}, vals)
		_, ok := sg.Len()
		assert.Equal(t, literal, ok)
	}

	nonNil, err := NewTestingSliceGetter[any, int64](false, nil).Range(t.Context(), nil, func(int64) bool { return true })
	require.NoError(t, err)
	assert.False(t, nonNil)
}

func Test_sliceElementCoercer_sliceLen(t *testing.T) {
	coercer := newTestSliceElementCoercer[string](buildGetterItem)

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
		coercer := newTestSliceElementCoercer[string](func(string, Getter[any]) (any, error) {
			return nil, errors.New("build failed")
		})
		_, err := coercer.rangeSlice([]any{"a"}, func(_ any) bool { return true })
		require.EqualError(t, err, "build failed")
	})

	t.Run("yield stops early", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[string](buildGetterItem)
		calls := 0
		_, err := coercer.rangeSlice([]string{"a", "b", "c"}, func(_ any) bool {
			calls++
			return calls < 2
		})
		require.NoError(t, err)
		require.Equal(t, 2, calls)
	})

	t.Run("reuses existing Getter elements", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[Getter[any]](buildGetterItem)
		original := newTestLiteral("kept")
		var seen Getter[any]
		_, err := coercer.rangeSlice([]Getter[any]{original}, func(val any) bool {
			seen = val.(Getter[any])
			return true
		})
		require.NoError(t, err)
		assert.Same(t, original, seen)
	})

	t.Run("passes Getter items to the builder", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[string](func(_ string, getter Getter[any]) (any, error) {
			return getter.Get(t.Context(), nil)
		})
		var collected []any
		_, err := coercer.rangeSlice([]any{newTestLiteral("from getter"), "raw"}, func(val any) bool {
			collected = append(collected, val)
			return true
		})
		require.NoError(t, err)
		require.Equal(t, []any{"from getter", "raw"}, collected)
	})

	t.Run("coerces pcommon.Slice items", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[Getter[any]](buildGetterItem)
		pSlice := pcommon.NewSlice()
		pSlice.AppendEmpty().SetStr("coerced")

		var got any
		_, err := coercer.rangeSlice(pSlice, func(val any) bool {
			v, err := val.(Getter[any]).Get(t.Context(), nil)
			require.NoError(t, err)
			got = v
			return true
		})
		require.NoError(t, err)
		require.Equal(t, pcommon.NewValueStr("coerced"), got)
	})

	t.Run("pcommon.Slice buildSliceItemGetter error", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[string](func(string, Getter[any]) (any, error) {
			return nil, errors.New("pcommon build failed")
		})
		pSlice := pcommon.NewSlice()
		pSlice.AppendEmpty().SetStr("item")

		_, err := coercer.rangeSlice(pSlice, func(_ any) bool { return true })
		require.EqualError(t, err, "pcommon build failed")
	})

	t.Run("pcommon.Slice yield stops early", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[Getter[any]](buildGetterItem)
		pSlice := pcommon.NewSlice()
		pSlice.AppendEmpty().SetStr("a")
		pSlice.AppendEmpty().SetStr("b")

		calls := 0
		_, err := coercer.rangeSlice(pSlice, func(_ any) bool {
			calls++
			return calls < 2
		})
		require.NoError(t, err)
		require.Equal(t, 2, calls)
	})

	t.Run("coerces pcommon.Value slice wrapper", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[Getter[any]](buildGetterItem)
		pVal := pcommon.NewValueSlice()
		pVal.Slice().AppendEmpty().SetStr("wrapped")

		count := 0
		_, err := coercer.rangeSlice(pVal, func(val any) bool {
			count++
			_, ok := val.(Getter[any])
			require.True(t, ok)
			return true
		})
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	t.Run("pcommon.Value non-slice error", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[string](buildGetterItem)
		_, err := coercer.rangeSlice(pcommon.NewValueStr("text"), func(_ any) bool { return true })
		require.ErrorContains(t, err, "expected a slice")
	})

	t.Run("reflect slice with matching element type", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[string](buildGetterItem)
		var collected []string
		_, err := coercer.rangeSlice([]string{"direct"}, func(val any) bool {
			collected = append(collected, val.(string))
			return true
		})
		require.NoError(t, err)
		require.Equal(t, []string{"direct"}, collected)
	})

	t.Run("reflect slice coerced yield stops early", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[Getter[any]](buildGetterItem)
		calls := 0
		_, err := coercer.rangeSlice([]any{"a", "b"}, func(_ any) bool {
			calls++
			return calls < 2
		})
		require.NoError(t, err)
		require.Equal(t, 2, calls)
	})

	t.Run("reflect nil slice", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[string](buildGetterItem)
		nonNil, err := coercer.rangeSlice([]any(nil), func(_ any) bool { return true })
		require.NoError(t, err)
		require.False(t, nonNil)
	})

	t.Run("reflect non-slice error", func(t *testing.T) {
		coercer := newTestSliceElementCoercer[string](buildGetterItem)
		_, err := coercer.rangeSlice(123, func(_ any) bool { return true })
		require.ErrorContains(t, err, "expected a slice")
	})
}
