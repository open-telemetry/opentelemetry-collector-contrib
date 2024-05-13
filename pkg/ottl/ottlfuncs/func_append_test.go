// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_Append(t *testing.T) {
	var nilOptional ottl.Optional[string]
	var nilSliceOptional ottl.Optional[[]string]

	var res []string

	testCases := []struct {
		Name     string
		Target   ottl.GetSetter[any]
		Value    ottl.Optional[string]
		Values   ottl.Optional[[]string]
		Expected []string
	}{
		{
			"Single: standard []string target - empty",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []string{}, nil
				},
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.([]string)
					return nil
				},
			},
			ottl.NewTestingOptional("a"),
			nilSliceOptional,
			[]string{"a"},
		},
		{
			"Slice: standard []string target - empty",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []string{}, nil
				},
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.([]string)
					return nil
				},
			},
			nilOptional,
			ottl.NewTestingOptional[[]string]([]string{"a", "b"}),
			[]string{"a", "b"},
		},

		{
			"Single: standard []string target",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []string{"5", "6"}, nil
				},
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.([]string)
					return nil
				},
			},
			ottl.NewTestingOptional("a"),
			nilSliceOptional,
			[]string{"5", "6", "a"},
		},
		{
			"Slice: standard []string target",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []string{"5", "6"}, nil
				},
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.([]string)
					return nil
				},
			},
			nilOptional,
			ottl.NewTestingOptional[[]string]([]string{"a", "b"}),
			[]string{"5", "6", "a", "b"},
		},

		{
			"Single: Slice target",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					ps := pcommon.NewSlice()
					ps.FromRaw([]any{"5", "6"})
					return ps, nil
				},
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.([]string)
					return nil
				},
			},
			ottl.NewTestingOptional("a"),
			nilSliceOptional,
			[]string{"5", "6", "a"},
		},
		{
			"Slice: Slice target",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					ps := pcommon.NewSlice()
					ps.FromRaw([]any{"5", "6"})
					return ps, nil
				},
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.([]string)
					return nil
				},
			},
			nilOptional,
			ottl.NewTestingOptional[[]string]([]string{"a", "b"}),
			[]string{"5", "6", "a", "b"},
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
					res = val.([]string)
					return nil
				},
			},
			ottl.NewTestingOptional("a"),
			nilSliceOptional,
			[]string{"5", "6", "a"},
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
					res = val.([]string)
					return nil
				},
			},
			nilOptional,
			ottl.NewTestingOptional[[]string]([]string{"a", "b"}),
			[]string{"5", "6", "a", "b"},
		},

		{
			"Single: []any target",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []any{5, 6}, nil
				},
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.([]string)
					return nil
				},
			},
			ottl.NewTestingOptional("a"),
			nilSliceOptional,
			[]string{"5", "6", "a"},
		},
		{
			"Slice: []any target",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []any{5, 6}, nil
				},
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.([]string)
					return nil
				},
			},
			nilOptional,
			ottl.NewTestingOptional[[]string]([]string{"a", "b"}),
			[]string{"5", "6", "a", "b"},
		},

		{
			"Single: pcommon.Value - string",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueStr("5")
					return v, nil
				},
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.([]string)
					return nil
				},
			},
			ottl.NewTestingOptional("a"),
			nilSliceOptional,
			[]string{"5", "a"},
		},
		{
			"Slice: pcommon.Value - string",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueStr("5")
					return v, nil
				},
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.([]string)
					return nil
				},
			},
			nilOptional,
			ottl.NewTestingOptional[[]string]([]string{"a", "b"}),
			[]string{"5", "a", "b"},
		},

		{
			"Single: pcommon.Value - slice",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueSlice()
					v.FromRaw([]any{"5", "6"})
					return v, nil
				},
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.([]string)
					return nil
				},
			},
			ottl.NewTestingOptional("a"),
			nilSliceOptional,
			[]string{"5", "6", "a"},
		},
		{
			"Slice: pcommon.Value - slice",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueSlice()
					v.FromRaw([]any{"5", "6"})
					return v, nil
				},
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.([]string)
					return nil
				},
			},
			nilOptional,
			ottl.NewTestingOptional[[]string]([]string{"a", "b"}),
			[]string{"5", "6", "a", "b"},
		},

		{
			"Single: scalar target string",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "5", nil
				},
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.([]string)
					return nil
				},
			},
			ottl.NewTestingOptional("a"),
			nilSliceOptional,
			[]string{"5", "a"},
		},
		{
			"Slice: scalar target string",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "5", nil
				},
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.([]string)
					return nil
				},
			},
			nilOptional,
			ottl.NewTestingOptional[[]string]([]string{"a", "b"}),
			[]string{"5", "a", "b"},
		},

		{
			"Single: scalar target any",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 5, nil
				},
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.([]string)
					return nil
				},
			},
			ottl.NewTestingOptional("a"),
			nilSliceOptional,
			[]string{"5", "a"},
		},
		{
			"Slice: scalar target any",
			&ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 5, nil
				},
				Setter: func(_ context.Context, _ any, val any) error {
					res = val.([]string)
					return nil
				},
			},
			nilOptional,
			ottl.NewTestingOptional[[]string]([]string{"a", "b"}),
			[]string{"5", "a", "b"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			res = nil
			exprFunc, err := Append[any](tc.Target, tc.Value, tc.Values)
			require.NoError(t, err)

			_, err = exprFunc(context.Background(), nil)
			require.NoError(t, err)

			require.NotNil(t, res)
			require.EqualValues(t, tc.Expected, res)
		})
	}
}

func Test_ArgumentsArePresent(t *testing.T) {
	var nilOptional ottl.Optional[string]
	var nilSliceOptional ottl.Optional[[]string]
	testCases := []struct {
		Name            string
		Value           ottl.Optional[string]
		Values          ottl.Optional[[]string]
		IsErrorExpected bool
	}{
		{"providedBoth", ottl.NewTestingOptional("val"), ottl.NewTestingOptional([]string{"val1", "val2"}), false},
		{"provided values", nilOptional, ottl.NewTestingOptional([]string{"val1", "val2"}), false},
		{"provided value", ottl.NewTestingOptional("val"), nilSliceOptional, false},
		{"nothing provided", nilOptional, nilSliceOptional, true},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			_, err := Append[any](nil, tc.Value, tc.Values)
			require.Equal(t, tc.IsErrorExpected, err != nil)
		})
	}
}
