// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_Append(t *testing.T) {
	setter := func(_ context.Context, res any, val any) error {
		rSlice := res.(pcommon.Slice)
		vSlice := val.(pcommon.Slice)
		assert.NoError(t, rSlice.FromRaw(vSlice.AsRaw()))

		return nil
	}

	var nilOptional ottl.Optional[ottl.Getter[any]]
	var nilSliceOptional ottl.Optional[[]ottl.Getter[any]]

	singleGetter := ottl.NewTestingOptional[ottl.Getter[any]](ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "a", nil
		},
	})

	singleIntGetter := ottl.NewTestingOptional[ottl.Getter[any]](ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return 66, nil
		},
	})

	multiGetter := ottl.NewTestingOptional[[]ottl.Getter[any]](
		[]ottl.Getter[any]{
			ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "a", nil
				},
			},
			ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "b", nil
				},
			},
		},
	)

	testCases := []struct {
		Name   string
		Target ottl.GetSetter[any]
		Value  ottl.Optional[ottl.Getter[any]]
		Values ottl.Optional[[]ottl.Getter[any]]
		Want   func(pcommon.Slice)
	}{
		{
			"Single: non existing target",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return nil, nil
				},
				Setter: setter,
			},
			singleGetter,
			nilSliceOptional,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("a")
			},
		},
		{
			"Single: non existing target - non string value",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return nil, nil
				},
				Setter: setter,
			},
			singleIntGetter,
			nilSliceOptional,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetInt(66)
			},
		},
		{
			"Single: standard []string target - empty",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []string{}, nil
				},
				Setter: setter,
			},
			singleGetter,
			nilSliceOptional,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("a")
			},
		},
		{
			"Slice: standard []string target - empty",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []string{}, nil
				},
				Setter: setter,
			},
			nilOptional,
			multiGetter,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("a")
				expectedValue.AppendEmpty().SetStr("b")
			},
		},

		{
			"Single: standard []string target",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []any{"5", "6"}, nil
				},
				Setter: setter,
			},
			singleGetter,
			nilSliceOptional,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("5")
				expectedValue.AppendEmpty().SetStr("6")
				expectedValue.AppendEmpty().SetStr("a")
			},
		},
		{
			"Slice: standard []string target",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []string{"5", "6"}, nil
				},
				Setter: setter,
			},
			nilOptional,
			multiGetter,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("5")
				expectedValue.AppendEmpty().SetStr("6")
				expectedValue.AppendEmpty().SetStr("a")
				expectedValue.AppendEmpty().SetStr("b")
			},
		},

		{
			"Single: Slice target",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					ps := pcommon.NewSlice()
					if err := ps.FromRaw([]any{"5", "6"}); err != nil {
						return nil, err
					}
					return ps, nil
				},
				Setter: setter,
			},
			singleGetter,
			nilSliceOptional,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("5")
				expectedValue.AppendEmpty().SetStr("6")
				expectedValue.AppendEmpty().SetStr("a")
			},
		},
		{
			"Slice: Slice target",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					ps := pcommon.NewSlice()
					if err := ps.FromRaw([]any{"5", "6"}); err != nil {
						return nil, err
					}
					return ps, nil
				},
				Setter: setter,
			},
			nilOptional,
			multiGetter,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("5")
				expectedValue.AppendEmpty().SetStr("6")
				expectedValue.AppendEmpty().SetStr("a")
				expectedValue.AppendEmpty().SetStr("b")
			},
		},

		{
			"Single: Slice target of string values in pcommon.value",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					ps := pcommon.NewSlice()
					ps.AppendEmpty().SetStr("5")
					ps.AppendEmpty().SetStr("6")
					return ps, nil
				},
				Setter: setter,
			},
			singleGetter,
			nilSliceOptional,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("5")
				expectedValue.AppendEmpty().SetStr("6")
				expectedValue.AppendEmpty().SetStr("a")
			},
		},
		{
			"Slice: Slice target of string values in pcommon.value",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					ps := pcommon.NewSlice()
					ps.AppendEmpty().SetStr("5")
					ps.AppendEmpty().SetStr("6")
					return ps, nil
				},
				Setter: setter,
			},
			nilOptional,
			multiGetter,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("5")
				expectedValue.AppendEmpty().SetStr("6")
				expectedValue.AppendEmpty().SetStr("a")
				expectedValue.AppendEmpty().SetStr("b")
			},
		},

		{
			"Single: []any target",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []any{5, 6}, nil
				},
				Setter: setter,
			},
			singleGetter,
			nilSliceOptional,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetInt(5)
				expectedValue.AppendEmpty().SetInt(6)
				expectedValue.AppendEmpty().SetStr("a")
			},
		},
		{
			"Slice: []any target",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []any{5, 6}, nil
				},
				Setter: setter,
			},
			nilOptional,
			multiGetter,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetInt(5)
				expectedValue.AppendEmpty().SetInt(6)
				expectedValue.AppendEmpty().SetStr("a")
				expectedValue.AppendEmpty().SetStr("b")
			},
		},

		{
			"Single: pcommon.Value - string",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := "5"
					return v, nil
				},
				Setter: setter,
			},
			singleGetter,
			nilSliceOptional,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("5")
				expectedValue.AppendEmpty().SetStr("a")
			},
		},
		{
			"Slice: pcommon.Value - string",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := "5"
					return v, nil
				},
				Setter: setter,
			},
			nilOptional,
			multiGetter,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("5")
				expectedValue.AppendEmpty().SetStr("a")
				expectedValue.AppendEmpty().SetStr("b")
			},
		},

		{
			"Single: pcommon.Value - slice",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueSlice()
					if err := v.FromRaw([]any{"5", "6"}); err != nil {
						return nil, err
					}
					return v, nil
				},
				Setter: setter,
			},
			singleGetter,
			nilSliceOptional,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("5")
				expectedValue.AppendEmpty().SetStr("6")
				expectedValue.AppendEmpty().SetStr("a")
			},
		},
		{
			"Slice: pcommon.Value - slice",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueSlice()
					if err := v.FromRaw([]any{"5", "6"}); err != nil {
						return nil, err
					}
					return v, nil
				},
				Setter: setter,
			},
			nilOptional,
			multiGetter,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("5")
				expectedValue.AppendEmpty().SetStr("6")
				expectedValue.AppendEmpty().SetStr("a")
				expectedValue.AppendEmpty().SetStr("b")
			},
		},

		{
			"Single: scalar target string",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "5", nil
				},
				Setter: setter,
			},
			singleGetter,
			nilSliceOptional,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("5")
				expectedValue.AppendEmpty().SetStr("a")
			},
		},
		{
			"Slice: scalar target string",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "5", nil
				},
				Setter: setter,
			},
			nilOptional,
			multiGetter,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("5")
				expectedValue.AppendEmpty().SetStr("a")
				expectedValue.AppendEmpty().SetStr("b")
			},
		},

		{
			"Single: scalar target any",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 5, nil
				},
				Setter: setter,
			},
			singleGetter,
			nilSliceOptional,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetInt(5)
				expectedValue.AppendEmpty().SetStr("a")
			},
		},
		{
			"Slice: scalar target any",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 5, nil
				},
				Setter: setter,
			},
			nilOptional,
			multiGetter,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetInt(5)
				expectedValue.AppendEmpty().SetStr("a")
				expectedValue.AppendEmpty().SetStr("b")
			},
		},

		{
			"Single: scalar target any append int",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 5, nil
				},
				Setter: setter,
			},
			singleIntGetter,
			nilSliceOptional,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetInt(5)
				expectedValue.AppendEmpty().SetInt(66)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			exprFunc, err := appendTo[any](tc.Target, tc.Value, tc.Values)
			assert.NoError(t, err)

			res := pcommon.NewSlice()
			result, err := exprFunc(context.Background(), res)
			assert.NoError(t, err)
			assert.Nil(t, result)
			assert.NotNil(t, res)

			expectedSlice := pcommon.NewSlice()
			tc.Want(expectedSlice)
			assert.Equal(t, expectedSlice, res)
		})
	}
}

func TestTargetType(t *testing.T) {
	expectedInt := 5
	expectedSlice := pcommon.NewValueSlice()
	assert.NoError(t, expectedSlice.Slice().FromRaw([]any{"a"}))
	singleIntGetter := ottl.NewTestingOptional[ottl.Getter[any]](ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return expectedInt, nil
		},
	})

	testCases := []struct {
		Name          string
		TargetValue   any
		Want          func(pcommon.Slice)
		expectedError bool
	}{
		{
			"pcommon.Slice",
			expectedSlice.Slice(),
			func(expectedValue pcommon.Slice) {
				expectedSlice.Slice().MoveAndAppendTo(expectedValue)
				expectedValue.AppendEmpty().SetInt(int64(expectedInt))
			},
			false,
		},
		{
			"pcommon.ValueTypeEmpty",
			pcommon.NewValueEmpty(),
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("")
				expectedValue.AppendEmpty().SetInt(int64(expectedInt))
			},
			false,
		},
		{
			"pcommon.ValueTypeStr",
			pcommon.NewValueStr("expected string"),
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("expected string")
				expectedValue.AppendEmpty().SetInt(int64(expectedInt))
			},
			false,
		},
		{
			"pcommon.ValueTypeInt",
			pcommon.NewValueInt(4),
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetInt(4)
				expectedValue.AppendEmpty().SetInt(int64(expectedInt))
			},
			false,
		},
		{
			"pcommon.ValueTypeDouble",
			pcommon.NewValueDouble(2.5),
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetDouble(2.5)
				expectedValue.AppendEmpty().SetInt(int64(expectedInt))
			},
			false,
		},
		{
			"pcommon.ValueTypeBool",
			pcommon.NewValueBool(true),
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetBool(true)
				expectedValue.AppendEmpty().SetInt(int64(expectedInt))
			},
			false,
		},
		{
			"pcommon.ValueTypeSlice",
			expectedSlice,
			func(expectedValue pcommon.Slice) {
				expectedSlice.Slice().MoveAndAppendTo(expectedValue)
				expectedValue.AppendEmpty().SetInt(int64(expectedInt))
			},
			false,
		},
		{
			"pcommon.ValueTypeMap",
			pcommon.NewValueMap(),
			func(_ pcommon.Slice) {
			},
			true,
		},

		{
			"string array",
			[]string{"a", "b"},
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("a")
				expectedValue.AppendEmpty().SetStr("b")
				expectedValue.AppendEmpty().SetInt(int64(expectedInt))
			},
			false,
		},
		{
			"any array",
			[]any{"a", "b"},
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("a")
				expectedValue.AppendEmpty().SetStr("b")
				expectedValue.AppendEmpty().SetInt(int64(expectedInt))
			},
			false,
		},
		{
			"int64 array",
			[]int64{5, 6},
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetInt(5)
				expectedValue.AppendEmpty().SetInt(6)
				expectedValue.AppendEmpty().SetInt(int64(expectedInt))
			},
			false,
		},
		{
			"bool array",
			[]bool{false, true},
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetBool(false)
				expectedValue.AppendEmpty().SetBool(true)
				expectedValue.AppendEmpty().SetInt(int64(expectedInt))
			},
			false,
		},
		{
			"float64 array",
			[]float64{1.5, 2.5},
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetDouble(1.5)
				expectedValue.AppendEmpty().SetDouble(2.5)
				expectedValue.AppendEmpty().SetInt(int64(expectedInt))
			},
			false,
		},

		{
			"string",
			"a",
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetStr("a")
				expectedValue.AppendEmpty().SetInt(int64(expectedInt))
			},
			false,
		},
		{
			"int64 ",
			5,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetInt(5)
				expectedValue.AppendEmpty().SetInt(int64(expectedInt))
			},
			false,
		},
		{
			"bool",
			true,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetBool(true)
				expectedValue.AppendEmpty().SetInt(int64(expectedInt))
			},
			false,
		},
		{
			"float64 ",
			2.5,
			func(expectedValue pcommon.Slice) {
				expectedValue.AppendEmpty().SetDouble(2.5)
				expectedValue.AppendEmpty().SetInt(int64(expectedInt))
			},
			false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			target := &ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return tc.TargetValue, nil
				},
				Setter: func(_ context.Context, res any, val any) error {
					rSlice := res.(pcommon.Slice)
					vSlice := val.(pcommon.Slice)
					assert.NoError(t, rSlice.FromRaw(vSlice.AsRaw()))

					return nil
				},
			}

			var nilSlice ottl.Optional[[]ottl.Getter[any]]
			exprFunc, err := appendTo[any](target, singleIntGetter, nilSlice)
			assert.NoError(t, err)

			res := pcommon.NewSlice()
			result, err := exprFunc(context.Background(), res)

			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Nil(t, result)
				assert.NotNil(t, res)

				expectedSlice := pcommon.NewSlice()
				tc.Want(expectedSlice)
				assert.Equal(t, expectedSlice, res)
			}
		})
	}
}

func Test_ArgumentsArePresent(t *testing.T) {
	var nilOptional ottl.Optional[ottl.Getter[any]]
	var nilSliceOptional ottl.Optional[[]ottl.Getter[any]]
	singleGetter := ottl.NewTestingOptional[ottl.Getter[any]](ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "val", nil
		},
	})

	multiGetter := ottl.NewTestingOptional[[]ottl.Getter[any]](
		[]ottl.Getter[any]{
			ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "val1", nil
				},
			},
			ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "val2", nil
				},
			},
		},
	)
	testCases := []struct {
		Name            string
		Value           ottl.Optional[ottl.Getter[any]]
		Values          ottl.Optional[[]ottl.Getter[any]]
		IsErrorExpected bool
	}{
		{"providedBoth", singleGetter, multiGetter, false},
		{"provided values", nilOptional, multiGetter, false},
		{"provided value", singleGetter, nilSliceOptional, false},
		{"nothing provided", nilOptional, nilSliceOptional, true},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			_, err := appendTo[any](nil, tc.Value, tc.Values)
			assert.Equal(t, tc.IsErrorExpected, err != nil)
		})
	}
}
