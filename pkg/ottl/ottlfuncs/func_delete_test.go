// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

var nilOptional ottl.Optional[ottl.IntGetter[any]]

func setter(_ context.Context, res, val any) error {
	rSlice := res.(pcommon.Slice)
	vSlice := val.(pcommon.Slice)
	return rSlice.FromRaw(vSlice.AsRaw())
}

func getIntGetter(index int64) ottl.IntGetter[any] {
	return ottl.StandardIntGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return index, nil
		},
	}
}

func getSlice(t *testing.T, expected []any) pcommon.Slice {
	expectedSlice := pcommon.NewSlice()
	for _, v := range expected {
		elem := expectedSlice.AppendEmpty()
		switch val := v.(type) {
		case string:
			elem.SetStr(val)
		case int64:
			elem.SetInt(val)
		case int:
			elem.SetInt(int64(val))
		case bool:
			elem.SetBool(val)
		case float64:
			elem.SetDouble(val)
		default:
			t.Errorf("unsupported type in test case: %T", v)
		}
	}
	return expectedSlice
}

func getMultiSlice(t *testing.T, expected []any) pcommon.Slice {
	expectedSlice := pcommon.NewSlice()
	for _, v := range expected {
		elem := expectedSlice.AppendEmpty()
		switch val := v.(type) {
		case []string:
			arr := elem.SetEmptySlice()
			for _, s := range val {
				arr.AppendEmpty().SetStr(s)
			}
		case []any:
			arr := elem.SetEmptySlice()
			for _, v := range val {
				subElem := arr.AppendEmpty()
				switch val := v.(type) {
				case string:
					subElem.SetStr(val)
				case int64:
					subElem.SetInt(val)
				case int:
					subElem.SetInt(int64(val))
				case bool:
					subElem.SetBool(val)
				case float64:
					subElem.SetDouble(val)
				default:
					t.Errorf("unsupported type in test case: %T", v)
				}
			}
		case []int64:
			arr := elem.SetEmptySlice()
			for _, i := range val {
				arr.AppendEmpty().SetInt(i)
			}
		case []int:
			arr := elem.SetEmptySlice()
			for _, i := range val {
				arr.AppendEmpty().SetInt(int64(i))
			}
		case []bool:
			arr := elem.SetEmptySlice()
			for _, b := range val {
				arr.AppendEmpty().SetBool(b)
			}
		case []float64:
			arr := elem.SetEmptySlice()
			for _, f := range val {
				arr.AppendEmpty().SetDouble(f)
			}
		default:
			t.Errorf("unsupported type in test case: %T", v)
		}
	}
	return expectedSlice
}

func TestDelete(t *testing.T) {
	testCases := []struct {
		name     string
		target   ottl.GetSetter[any]
		index    ottl.IntGetter[any]
		length   ottl.Optional[ottl.IntGetter[any]]
		expected pcommon.Slice
	}{
		{
			name: "delete single int element",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getSlice(t, []any{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}), nil
				},
				Setter: setter,
			},
			index:    getIntGetter(3),
			length:   nilOptional,
			expected: getSlice(t, []any{0, 1, 2, 4, 5, 6, 7, 8, 9}),
		},
		{
			name: "delete multiple int elements",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getSlice(t, []any{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}), nil
				},
				Setter: setter,
			},
			index:    getIntGetter(2),
			length:   ottl.NewTestingOptional(getIntGetter(4)),
			expected: getSlice(t, []any{0, 1, 6, 7, 8, 9}),
		},
		{
			name: "delete single string element",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getSlice(t, []any{"a", "b", "c", "d", "e"}), nil
				},
				Setter: setter,
			},
			index:    getIntGetter(1),
			length:   nilOptional,
			expected: getSlice(t, []any{"a", "c", "d", "e"}),
		},
		{
			name: "delete multiple string elements",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getSlice(t, []any{"a", "b", "c", "d", "e"}), nil
				},
				Setter: setter,
			},
			index:    getIntGetter(1),
			length:   ottl.NewTestingOptional(getIntGetter(3)),
			expected: getSlice(t, []any{"a", "e"}),
		},
		{
			name: "delete single bool element",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getSlice(t, []any{true, false, true, false}), nil
				},
				Setter: setter,
			},
			index:    getIntGetter(2),
			length:   nilOptional,
			expected: getSlice(t, []any{true, false, false}),
		},
		{
			name: "delete multiple bool elements",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getSlice(t, []any{true, false, true, false, true}), nil
				},
				Setter: setter,
			},
			index:    getIntGetter(1),
			length:   ottl.NewTestingOptional(getIntGetter(3)),
			expected: getSlice(t, []any{true, true}),
		},
		{
			name: "delete single float element",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getSlice(t, []any{1.1, 2.2, 3.3, 4.4}), nil
				},
				Setter: setter,
			},
			index:    getIntGetter(0),
			length:   nilOptional,
			expected: getSlice(t, []any{2.2, 3.3, 4.4}),
		},
		{
			name: "delete multiple float elements",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getSlice(t, []any{1.1, 2.2, 3.3, 4.4, 5.5}), nil
				},
				Setter: setter,
			},
			index:    getIntGetter(1),
			length:   ottl.NewTestingOptional(getIntGetter(2)),
			expected: getSlice(t, []any{1.1, 4.4, 5.5}),
		},
		{
			name: "delete element from multitype slice",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getSlice(t, []any{"a", 1, true, 2.2}), nil
				},
				Setter: setter,
			},
			index:    getIntGetter(2),
			length:   nilOptional,
			expected: getSlice(t, []any{"a", 1, 2.2}),
		},
		{
			name: "delete from multi slice",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getMultiSlice(t, []any{
						[]string{"a", "b", "c"},
						[]any{"a", 1, true},
						[]int64{1, 2, 3},
						[]bool{true, false, true},
						[]float64{1.1, 2.2, 3.3},
					}), nil
				},
				Setter: setter,
			},
			index:  getIntGetter(1),
			length: nilOptional,
			expected: getMultiSlice(t, []any{
				[]string{"a", "b", "c"},
				[]int64{1, 2, 3},
				[]bool{true, false, true},
				[]float64{1.1, 2.2, 3.3},
			}),
		},
		{
			name: "delete all elements",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getSlice(t, []any{0, "a", 1.1, true, "hello"}), nil
				},
				Setter: setter,
			},
			index:    getIntGetter(0),
			length:   ottl.NewTestingOptional(getIntGetter(5)),
			expected: getSlice(t, []any{}),
		},
		{
			name: "target is pcommon.Value slice",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					val := pcommon.NewValueSlice()
					slice := val.Slice()
					slice.AppendEmpty().SetInt(10)
					slice.AppendEmpty().SetInt(20)
					slice.AppendEmpty().SetInt(30)
					slice.AppendEmpty().SetInt(40)
					return val, nil
				},
				Setter: setter,
			},
			index:    getIntGetter(2),
			length:   nilOptional,
			expected: getSlice(t, []any{10, 20, 40}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			exprFunc := deleteFrom(tc.target, tc.index, tc.length)

			res := pcommon.NewSlice()
			result, err := exprFunc(t.Context(), res)
			assert.NoError(t, err)
			assert.Nil(t, result)
			assert.Equal(t, tc.expected, res)
		})
	}
}

func TestDelete_Errors(t *testing.T) {
	errorTestCases := []struct {
		name        string
		target      ottl.GetSetter[any]
		index       ottl.IntGetter[any]
		length      ottl.Optional[ottl.IntGetter[any]]
		expectedErr string
	}{
		{
			name: "cannot get target",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return nil, errors.New("cannot get target")
				},
				Setter: setter,
			},
			index:       getIntGetter(0),
			length:      nilOptional,
			expectedErr: "cannot get target",
		},
		{
			name: "index is not int",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getSlice(t, []any{0, 1, 2}), nil
				},
				Setter: setter,
			},
			index: ottl.StandardIntGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "invalid_index", nil
				},
			},
			length:      nilOptional,
			expectedErr: "expected int64 but got string",
		},
		{
			name: "length is not int",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getSlice(t, []any{0, 1, 2}), nil
				},
				Setter: setter,
			},
			index: getIntGetter(0),
			length: ottl.NewTestingOptional[ottl.IntGetter[any]](ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "invalid_length", nil
				},
			}),
			expectedErr: "expected int64 but got string",
		},
		{
			name: "target is not a slice",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "not_a_slice", nil
				},
				Setter: setter,
			},
			index:       getIntGetter(0),
			length:      nilOptional,
			expectedErr: "target must be a slice type, got string",
		},
		{
			name: "target is pcommon.Value but not slice",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					val := pcommon.NewValueInt(42)
					return val, nil
				},
				Setter: setter,
			},
			index:       getIntGetter(0),
			length:      nilOptional,
			expectedErr: "target must be a slice type, got pcommon.Value",
		},
		{
			name: "index out of bounds",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getSlice(t, []any{0, 1, 2}), nil
				},
				Setter: setter,
			},
			index:       getIntGetter(5),
			length:      nilOptional,
			expectedErr: "index 5 out of bounds",
		},
		{
			name: "length is non-positive",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getSlice(t, []any{0, 1, 2, 3}), nil
				},
				Setter: setter,
			},
			index:       getIntGetter(1),
			length:      ottl.NewTestingOptional(getIntGetter(0)),
			expectedErr: "length must be positive, got 0",
		},
		{
			name: "deletion range out of bounds",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getSlice(t, []any{0, 1, 2, 3}), nil
				},
				Setter: setter,
			},
			index:       getIntGetter(2),
			length:      ottl.NewTestingOptional(getIntGetter(5)),
			expectedErr: "deletion range [2:7] out of bounds",
		},
		{
			name: "negative index",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getSlice(t, []any{0, 1, 2, 3}), nil
				},
				Setter: setter,
			},
			index:       getIntGetter(-1),
			length:      nilOptional,
			expectedErr: "index -1 out of bounds",
		},
	}
	for _, etc := range errorTestCases {
		t.Run(etc.name, func(t *testing.T) {
			exprFunc := deleteFrom(etc.target, etc.index, etc.length)

			res := pcommon.NewSlice()
			_, err := exprFunc(t.Context(), res)
			assert.ErrorContains(t, err, etc.expectedErr)
		})
	}
}
