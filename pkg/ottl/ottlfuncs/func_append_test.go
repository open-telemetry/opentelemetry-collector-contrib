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

	var res pcommon.Slice

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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.(pcommon.Slice)
					return nil
				},
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
			res = pcommon.NewSlice()
			exprFunc, err := appendTo[any](tc.Target, tc.Value, tc.Values)
			assert.NoError(t, err)

			result, err := exprFunc(context.Background(), res)
			assert.NoError(t, err)
			assert.Nil(t, result)
			assert.NotNil(t, res)

			expectedSlice := pcommon.NewSlice()
			tc.Want(expectedSlice)
			assert.EqualValues(t, expectedSlice, res)
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
