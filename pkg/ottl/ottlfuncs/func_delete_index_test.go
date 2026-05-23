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

func psliceSetter(_ context.Context, res, val any) error {
	rSlice := res.(pcommon.Slice)
	vSlice := val.(pcommon.Slice)
	return rSlice.FromRaw(vSlice.AsRaw())
}

func mockIntGetter(index int64) ottl.IntGetter[any] {
	return ottl.StandardIntGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return index, nil
		},
	}
}

func asPSlice(t *testing.T, expected []any) pcommon.Slice {
	expectedSlice := pcommon.NewSlice()
	err := expectedSlice.FromRaw(expected)
	assert.NoError(t, err)
	return expectedSlice
}

func asMultiSlice(t *testing.T, expected [][]any) pcommon.Slice {
	expectedSlice := pcommon.NewSlice()
	expectedSlice.EnsureCapacity(len(expected))
	for _, v := range expected {
		elem := expectedSlice.AppendEmpty()
		assert.NoError(t, elem.FromRaw(v))
	}
	return expectedSlice
}

func sliceGetSetter(getter func(_ context.Context, _ any) (any, error)) ottl.PSliceGetSetter[any] {
	sliceGetter := &ottl.StandardPSliceGetter[any]{
		Getter: getter,
	}
	return &ottl.StandardPSliceGetSetter[any]{
		Getter: sliceGetter.Get,
		Setter: psliceSetter,
	}
}

func TestDeleteIndex(t *testing.T) {
	testCases := []struct {
		name       string
		target     ottl.PSliceGetSetter[any]
		startIndex ottl.IntGetter[any]
		endIndex   ottl.Optional[ottl.IntGetter[any]]
		expected   pcommon.Slice
	}{
		{
			name: "delete single int element",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return asPSlice(t, []any{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}), nil
			}),
			startIndex: mockIntGetter(3),
			endIndex:   nilOptional,
			expected:   asPSlice(t, []any{0, 1, 2, 4, 5, 6, 7, 8, 9}),
		},
		{
			name: "delete multiple int elements",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return asPSlice(t, []any{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}), nil
			}),
			startIndex: mockIntGetter(2),
			endIndex:   ottl.NewTestingOptional(mockIntGetter(6)),
			expected:   asPSlice(t, []any{0, 1, 6, 7, 8, 9}),
		},
		{
			name: "delete single string element",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return asPSlice(t, []any{"a", "b", "c", "d", "e"}), nil
			}),
			startIndex: mockIntGetter(1),
			endIndex:   nilOptional,
			expected:   asPSlice(t, []any{"a", "c", "d", "e"}),
		},
		{
			name: "delete multiple string elements",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return asPSlice(t, []any{"a", "b", "c", "d", "e"}), nil
			}),
			startIndex: mockIntGetter(1),
			endIndex:   ottl.NewTestingOptional(mockIntGetter(4)),
			expected:   asPSlice(t, []any{"a", "e"}),
		},
		{
			name: "delete single bool element",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return asPSlice(t, []any{true, false, true, false}), nil
			}),
			startIndex: mockIntGetter(2),
			endIndex:   nilOptional,
			expected:   asPSlice(t, []any{true, false, false}),
		},
		{
			name: "delete multiple bool elements",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return asPSlice(t, []any{true, false, true, false, true}), nil
			}),
			startIndex: mockIntGetter(1),
			endIndex:   ottl.NewTestingOptional(mockIntGetter(4)),
			expected:   asPSlice(t, []any{true, true}),
		},
		{
			name: "delete single float element",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return asPSlice(t, []any{1.1, 2.2, 3.3, 4.4}), nil
			}),
			startIndex: mockIntGetter(0),
			endIndex:   nilOptional,
			expected:   asPSlice(t, []any{2.2, 3.3, 4.4}),
		},
		{
			name: "delete multiple float elements",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return asPSlice(t, []any{1.1, 2.2, 3.3, 4.4, 5.5}), nil
			}),
			startIndex: mockIntGetter(1),
			endIndex:   ottl.NewTestingOptional(mockIntGetter(3)),
			expected:   asPSlice(t, []any{1.1, 4.4, 5.5}),
		},
		{
			name: "delete element from multitype slice",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return asPSlice(t, []any{"a", 1, true, 2.2}), nil
			}),
			startIndex: mockIntGetter(2),
			endIndex:   nilOptional,
			expected:   asPSlice(t, []any{"a", 1, 2.2}),
		},
		{
			name: "delete from multi slice",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return asMultiSlice(t, [][]any{
					{"a", "b", "c"},
					{"a", 1, true},
					{int64(1), int64(2), int64(3)},
					{true, false, true},
					{1.1, 2.2, 3.3},
				}), nil
			}),
			startIndex: mockIntGetter(1),
			endIndex:   nilOptional,
			expected: asMultiSlice(t, [][]any{
				{"a", "b", "c"},
				{int64(1), int64(2), int64(3)},
				{true, false, true},
				{1.1, 2.2, 3.3},
			}),
		},
		{
			name: "delete all elements",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return asPSlice(t, []any{0, "a", 1.1, true, "hello"}), nil
			}),
			startIndex: mockIntGetter(0),
			endIndex:   ottl.NewTestingOptional(mockIntGetter(5)),
			expected:   asPSlice(t, []any{}),
		},
		{
			name: "target is pcommon.Value slice",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				val := pcommon.NewValueSlice()
				slice := val.Slice()
				slice.AppendEmpty().SetInt(10)
				slice.AppendEmpty().SetInt(20)
				slice.AppendEmpty().SetInt(30)
				slice.AppendEmpty().SetInt(40)
				return val, nil
			}),
			startIndex: mockIntGetter(2),
			endIndex:   nilOptional,
			expected:   asPSlice(t, []any{10, 20, 40}),
		},
		{
			name: "delete with endIndex equal to startIndex",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return asPSlice(t, []any{0, 1, 2, 3, 4}), nil
			}),
			startIndex: mockIntGetter(2),
			endIndex:   ottl.NewTestingOptional(mockIntGetter(2)),
			expected:   asPSlice(t, []any{0, 1, 2, 3, 4}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			exprFunc := deleteIndexFrom(tc.target, tc.startIndex, tc.endIndex)

			res := pcommon.NewSlice()
			result, err := exprFunc(t.Context(), res)
			assert.NoError(t, err)
			assert.Nil(t, result)
			assert.Equal(t, tc.expected, res)
		})
	}
}

func TestDeleteIndex_Errors(t *testing.T) {
	errorTestCases := []struct {
		name        string
		target      ottl.PSliceGetSetter[any]
		index       ottl.IntGetter[any]
		endIndex    ottl.Optional[ottl.IntGetter[any]]
		expectedErr string
	}{
		{
			name: "cannot get target",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return nil, errors.New("cannot get target")
			}),
			index:       mockIntGetter(0),
			endIndex:    nilOptional,
			expectedErr: "cannot get target",
		},
		{
			name: "index is not int",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return asPSlice(t, []any{0, 1, 2}), nil
			}),
			index: ottl.StandardIntGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "invalid_index", nil
				},
			},
			endIndex:    nilOptional,
			expectedErr: "expected int64 but got string",
		},
		{
			name: "endIndex is not int",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return asPSlice(t, []any{0, 1, 2}), nil
			}),
			index: mockIntGetter(0),
			endIndex: ottl.NewTestingOptional[ottl.IntGetter[any]](ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "invalid_endIndex", nil
				},
			}),
			expectedErr: "expected int64 but got string",
		},
		{
			name: "target is not a slice",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return "not_a_slice", nil
			}),
			index:       mockIntGetter(0),
			endIndex:    nilOptional,
			expectedErr: "expected pcommon.Slice but got string",
		},
		{
			name: "target is pcommon.Value int but not slice",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				val := pcommon.NewValueInt(42)
				return val, nil
			}),
			index:       mockIntGetter(0),
			endIndex:    nilOptional,
			expectedErr: "expected pcommon.Slice but got Int",
		},
		{
			name: "index out of bounds",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return asPSlice(t, []any{0, 1, 2}), nil
			}),
			index:       mockIntGetter(5),
			endIndex:    nilOptional,
			expectedErr: "startIndex 5 out of bounds",
		},
		{
			name: "endIndex less than startIndex",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return asPSlice(t, []any{0, 1, 2, 3}), nil
			}),
			index:       mockIntGetter(2),
			endIndex:    ottl.NewTestingOptional(mockIntGetter(1)),
			expectedErr: "endIndex 1 cannot be less than startIndex 2",
		},
		{
			name: "deletion range out of bounds",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return asPSlice(t, []any{0, 1, 2, 3}), nil
			}),
			index:       mockIntGetter(2),
			endIndex:    ottl.NewTestingOptional(mockIntGetter(7)),
			expectedErr: "deletion range [2:7] out of bounds",
		},
		{
			name: "negative startIndex",
			target: sliceGetSetter(func(_ context.Context, _ any) (any, error) {
				return asPSlice(t, []any{0, 1, 2, 3}), nil
			}),
			index:       mockIntGetter(-1),
			endIndex:    nilOptional,
			expectedErr: "startIndex -1 out of bounds",
		},
	}
	for _, etc := range errorTestCases {
		t.Run(etc.name, func(t *testing.T) {
			exprFunc := deleteIndexFrom(etc.target, etc.index, etc.endIndex)

			res := pcommon.NewSlice()
			_, err := exprFunc(t.Context(), res)
			assert.ErrorContains(t, err, etc.expectedErr)
		})
	}
}
