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

func Test_set(t *testing.T) {
	const missingValue = "<nil>"
	target := &ottl.StandardGetSetter[pcommon.Value]{
		Setter: func(_ context.Context, tCtx pcommon.Value, val any) error {
			tCtx.SetStr(val.(string))
			return nil
		},
		Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
			if tCtx.Str() == missingValue {
				return nil, nil
			}
			return tCtx.Str(), nil
		},
	}

	tests := []struct {
		name          string
		initialValue  string
		setter        ottl.GetSetter[pcommon.Value]
		getter        ottl.Getter[pcommon.Value]
		strategy      ottl.Optional[string]
		want          func(pcommon.Value)
		expectedError error
	}{
		{
			name:         "set name",
			setter:       target,
			initialValue: "original name",
			strategy:     ottl.Optional[string]{},
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(_ context.Context, _ pcommon.Value) (any, error) {
					return "new name", nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("new name")
			},
		},
		{
			name:         "set nil value",
			setter:       target,
			initialValue: "original name",
			strategy:     ottl.Optional[string]{},
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(_ context.Context, _ pcommon.Value) (any, error) {
					return nil, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("original name")
			},
		},
		{
			name:         "set value existing - IGNORE",
			initialValue: "some value",
			setter:       target,
			strategy:     ottl.NewTestingOptional[string](setConflictInsert),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return "new name", nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("some value")
			},
		},
		{
			name:         "set value existing - FAIL",
			initialValue: "some value",
			setter:       target,
			strategy:     ottl.NewTestingOptional[string](setConflictFail),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return "new name", nil
				},
			},
			expectedError: ErrValueAlreadyPresent,
		},
		{
			name:         "set value existing - REPLACE",
			initialValue: "some value",
			setter:       target,
			strategy:     ottl.NewTestingOptional[string](setConflictUpsert),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return "new name", nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("new name")
			},
		},
		{
			name:         "set value existing - UPDATE",
			initialValue: "some value",
			setter:       target,
			strategy:     ottl.NewTestingOptional[string](setConflictUpdate),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return "new name", nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("new name")
			},
		},

		{
			name:         "set value missing - UPDATE",
			initialValue: missingValue,
			setter:       target,
			strategy:     ottl.NewTestingOptional[string](setConflictUpdate),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return "new name", nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr(missingValue)
			},
		},
		{
			name:         "set value existing - default",
			initialValue: "some value",
			setter:       target,
			strategy:     ottl.Optional[string]{},
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return "new name", nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("new name")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioValue := pcommon.NewValueStr(tt.initialValue)
			exprFunc, err := set(tt.setter, tt.getter, tt.strategy)
			assert.NoError(t, err)

			result, err := exprFunc(nil, scenarioValue)
			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err)
				return
			}

			assert.NoError(t, err)
			assert.Nil(t, result)

			expected := pcommon.NewValueStr("")
			tt.want(expected)

			assert.Equal(t, expected, scenarioValue)
		})
	}
}

func Test_set_default_bool(t *testing.T) {
	defaultValue := false
	newValue := true

	target := &ottl.StandardGetSetter[pcommon.Value]{
		Setter: func(ctx context.Context, tCtx pcommon.Value, val any) error {
			tCtx.SetBool(val.(bool))
			return nil
		},
		Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
			return tCtx.Bool(), nil
		},
	}

	tests := []struct {
		name          string
		setter        ottl.GetSetter[pcommon.Value]
		getter        ottl.Getter[pcommon.Value]
		strategy      ottl.Optional[string]
		want          func(pcommon.Value)
		expectedError error
	}{
		{
			name:     "set field with default value - upsert",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictUpsert),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetBool(newValue)
			},
		},
		{
			name:     "set field with default value - update",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictUpdate),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetBool(defaultValue)
			},
		},
		{
			name:     "set field with default value - insert",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictInsert),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetBool(newValue)
			},
		},
		{
			name:     "set field with default value - fail",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictFail),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetBool(newValue)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioValue := pcommon.NewValueBool(defaultValue)
			exprFunc, err := set(tt.setter, tt.getter, tt.strategy)
			assert.NoError(t, err)

			result, err := exprFunc(nil, scenarioValue)
			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err)
				return
			}

			assert.NoError(t, err)
			assert.Nil(t, result)

			expected := pcommon.NewValueBool(defaultValue)
			tt.want(expected)

			assert.Equal(t, expected, scenarioValue)
		})
	}
}

func Test_set_default_string(t *testing.T) {
	defaultValue := ""
	newValue := "new value"

	target := &ottl.StandardGetSetter[pcommon.Value]{
		Setter: func(ctx context.Context, tCtx pcommon.Value, val any) error {
			tCtx.SetStr(val.(string))
			return nil
		},
		Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
			return tCtx.Str(), nil
		},
	}

	tests := []struct {
		name          string
		setter        ottl.GetSetter[pcommon.Value]
		getter        ottl.Getter[pcommon.Value]
		strategy      ottl.Optional[string]
		want          func(pcommon.Value)
		expectedError error
	}{
		{
			name:     "set field with default value - upsert",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictUpsert),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr(newValue)
			},
		},
		{
			name:     "set field with default value - update",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictUpdate),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr(defaultValue)
			},
		},
		{
			name:     "set field with default value - insert",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictInsert),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr(newValue)
			},
		},
		{
			name:     "set field with default value - fail",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictFail),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr(newValue)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioValue := pcommon.NewValueStr(defaultValue)
			exprFunc, err := set(tt.setter, tt.getter, tt.strategy)
			assert.NoError(t, err)

			result, err := exprFunc(nil, scenarioValue)
			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err)
				return
			}

			assert.NoError(t, err)
			assert.Nil(t, result)

			expected := pcommon.NewValueStr(defaultValue)
			tt.want(expected)

			assert.Equal(t, expected, scenarioValue)
		})
	}
}

func Test_set_default_int64(t *testing.T) {
	defaultValue := int64(0)
	newValue := int64(1)

	target := &ottl.StandardGetSetter[pcommon.Value]{
		Setter: func(ctx context.Context, tCtx pcommon.Value, val any) error {
			tCtx.SetInt(val.(int64))
			return nil
		},
		Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
			return tCtx.Int(), nil
		},
	}

	tests := []struct {
		name          string
		setter        ottl.GetSetter[pcommon.Value]
		getter        ottl.Getter[pcommon.Value]
		strategy      ottl.Optional[string]
		want          func(pcommon.Value)
		expectedError error
	}{
		{
			name:     "set field with default value - upsert",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictUpsert),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetInt(newValue)
			},
		},
		{
			name:     "set field with default value - update",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictUpdate),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetInt(defaultValue)
			},
		},
		{
			name:     "set field with default value - insert",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictInsert),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetInt(newValue)
			},
		},
		{
			name:     "set field with default value - fail",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictFail),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetInt(newValue)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioValue := pcommon.NewValueInt(defaultValue)
			exprFunc, err := set(tt.setter, tt.getter, tt.strategy)
			assert.NoError(t, err)

			result, err := exprFunc(nil, scenarioValue)
			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err)
				return
			}

			assert.NoError(t, err)
			assert.Nil(t, result)

			expected := pcommon.NewValueInt(defaultValue)
			tt.want(expected)

			assert.Equal(t, expected, scenarioValue)
		})
	}
}

func Test_set_default_float(t *testing.T) {
	defaultValue := float64(0)
	newValue := float64(1)

	target := &ottl.StandardGetSetter[pcommon.Value]{
		Setter: func(ctx context.Context, tCtx pcommon.Value, val any) error {
			tCtx.SetDouble(val.(float64))
			return nil
		},
		Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
			return tCtx.Double(), nil
		},
	}

	tests := []struct {
		name          string
		setter        ottl.GetSetter[pcommon.Value]
		getter        ottl.Getter[pcommon.Value]
		strategy      ottl.Optional[string]
		want          func(pcommon.Value)
		expectedError error
	}{
		{
			name:     "set field with default value - upsert",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictUpsert),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetDouble(newValue)
			},
		},
		{
			name:     "set field with default value - update",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictUpdate),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetDouble(defaultValue)
			},
		},
		{
			name:     "set field with default value - insert",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictInsert),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetDouble(newValue)
			},
		},
		{
			name:     "set field with default value - fail",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictFail),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetDouble(newValue)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioValue := pcommon.NewValueDouble(defaultValue)
			exprFunc, err := set(tt.setter, tt.getter, tt.strategy)
			assert.NoError(t, err)

			result, err := exprFunc(nil, scenarioValue)
			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err)
				return
			}

			assert.NoError(t, err)
			assert.Nil(t, result)

			expected := pcommon.NewValueDouble(defaultValue)
			tt.want(expected)

			assert.Equal(t, expected, scenarioValue)
		})
	}
}

func Test_set_default_byte_array(t *testing.T) {
	var defaultValue []byte
	newValue := []byte("new value")

	target := &ottl.StandardGetSetter[pcommon.Value]{
		Setter: func(ctx context.Context, tCtx pcommon.Value, val any) error {
			slice := tCtx.SetEmptyBytes()
			slice.Append(val.([]byte)...)
			return nil
		},
		Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
			return tCtx.Bytes(), nil
		},
	}

	tests := []struct {
		name          string
		setter        ottl.GetSetter[pcommon.Value]
		getter        ottl.Getter[pcommon.Value]
		strategy      ottl.Optional[string]
		want          func(pcommon.Value)
		expectedError error
	}{
		{
			name:     "set field with default value - upsert",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictUpsert),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				slice := expectedValue.SetEmptyBytes()
				slice.Append(newValue...)
			},
		},
		{
			name:     "set field with default value - update",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictUpdate),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				slice := expectedValue.SetEmptyBytes()
				slice.Append(defaultValue...)
			},
		},
		{
			name:     "set field with default value - insert",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictInsert),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				slice := expectedValue.SetEmptyBytes()
				slice.Append(newValue...)
			},
		},
		{
			name:     "set field with default value - fail",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictFail),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return newValue, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				slice := expectedValue.SetEmptyBytes()
				slice.Append(newValue...)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioValue := pcommon.NewValueBytes()
			exprFunc, err := set(tt.setter, tt.getter, tt.strategy)
			assert.NoError(t, err)

			result, err := exprFunc(nil, scenarioValue)
			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err)
				return
			}

			assert.NoError(t, err)
			assert.Nil(t, result)

			expected := pcommon.NewValueBytes()
			tt.want(expected)

			assert.Equal(t, expected.Bytes(), scenarioValue.Bytes())
		})
	}
}

func Test_set_get_nil(t *testing.T) {
	setter := &ottl.StandardGetSetter[any]{
		Setter: func(_ context.Context, _ any, _ any) error {
			t.Errorf("nothing should be set in this scenario")
			return nil
		},
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
	}

	getter := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
	}

	exprFunc, err := set[any](setter, getter, ottl.Optional[string]{})
	assert.NoError(t, err)

	result, err := exprFunc(nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}
