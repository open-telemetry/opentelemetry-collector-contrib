// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_SliceToMap(t *testing.T) {
	nestedObj := func() any {
		sl := pcommon.NewSlice()
		thing1 := sl.AppendEmpty().SetEmptyMap()
		thing1.PutStr("name", "foo")
		thing1.PutEmptyMap("value").PutStr("test", "x")

		thing2 := sl.AppendEmpty().SetEmptyMap()
		thing2.PutStr("name", "bar")
		thing2.PutInt("value", 5)
		thing2.PutEmptyMap("value").PutStr("test", "y")

		return sl
	}

	type testCase struct {
		name             string
		value            func() any
		keyPath          []string
		valuePath        []string
		want             func() pcommon.Map
		wantExecutionErr string
		wantConfigErr    string
	}
	tests := []testCase{
		{
			name:    "flat object with key path only",
			keyPath: []string{"name"},
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutInt("value", 2)

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)

				return sl
			},
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				thing1 := m.PutEmptyMap("foo")
				thing1.PutStr("name", "foo")
				thing1.PutInt("value", 2)

				thing2 := m.PutEmptyMap("bar")
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)

				return m
			},
		},
		{
			name:    "flat object with missing key value",
			keyPath: []string{"notfound"},
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutInt("value", 2)

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)

				return sl
			},
			wantExecutionErr: "could not extract key from element 0: provided object does not contain the path [notfound]",
		},
		{
			name:      "flat object with both key and value path",
			keyPath:   []string{"name"},
			valuePath: []string{"value"},
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutInt("value", 2)

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)

				return sl
			},
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutInt("foo", 2)
				m.PutInt("bar", 5)

				return m
			},
		},
		{
			name:    "nested object with key path only",
			keyPath: []string{"value", "test"},
			value:   nestedObj,
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				thing1 := m.PutEmptyMap("x")
				thing1.PutStr("name", "foo")
				thing1.PutEmptyMap("value").PutStr("test", "x")

				thing2 := m.PutEmptyMap("y")
				thing2.PutStr("name", "bar")
				thing2.PutEmptyMap("value").PutStr("test", "y")

				return m
			},
		},
		{
			name:      "nested object with key path and value path",
			keyPath:   []string{"value", "test"},
			valuePath: []string{"name"},
			value:     nestedObj,
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("x", "foo")
				m.PutStr("y", "bar")

				return m
			},
		},
		{
			name:    "flat object with key path resolving to non-string",
			keyPath: []string{"value"},
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutInt("value", 2)

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)

				return sl
			},
			wantExecutionErr: "extracted key attribute is not of type string",
		},
		{
			name:             "nested object with value path not resolving to a value",
			keyPath:          []string{"value", "test"},
			valuePath:        []string{"notfound"},
			value:            nestedObj,
			wantExecutionErr: "could not extract value from element 0: provided object does not contain the path [notfound]",
		},
		{
			name:             "nested object with value path segment resolving to non-map value",
			keyPath:          []string{"value", "test"},
			valuePath:        []string{"name", "nothing"},
			value:            nestedObj,
			wantExecutionErr: "could not extract value from element 0: provided object does not contain the path [name nothing]",
		},
		{
			name:    "unsupported type",
			keyPath: []string{"name"},
			value: func() any {
				return pcommon.NewMap()
			},
			wantExecutionErr: "error getting value in ottl.StandardPSliceGetter[interface {}]: expected pcommon.Slice, got pcommon.Map",
		},
		{
			name:    "slice containing unsupported value type",
			keyPath: []string{"name"},
			value: func() any {
				sl := pcommon.NewSlice()
				sl.AppendEmpty().SetStr("unsupported")

				return sl
			},
			wantExecutionErr: "could not cast element of type `Str` to a map",
		},
		{
			name:      "mixed data types with invalid element",
			keyPath:   []string{"name"},
			valuePath: []string{"value"},
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutInt("value", 2)

				sl.AppendEmpty().SetStr("nothingToSeeHere")

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)

				return sl
			},
			wantExecutionErr: "could not cast element of type `Str` to a map",
		},
		{
			name:      "nested with different value data types",
			keyPath:   []string{"name"},
			valuePath: []string{"value"},
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutEmptyMap("value").PutStr("test", "value")

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)

				return sl
			},
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutEmptyMap("foo").PutStr("test", "value")
				m.PutInt("bar", 5)

				return m
			},
		},
		{
			name: "flat object with no key path and value path", // (SliceToMap([{"one": 1}, {"two": 2}])
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutInt("value", 2)

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)

				return sl
			},
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				thing1 := m.PutEmptyMap("0")
				thing1.PutStr("name", "foo")
				thing1.PutInt("value", 2)

				thing2 := m.PutEmptyMap("1")
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)

				return m
			},
		},
		{
			name:      "flat object with only value path",
			valuePath: []string{"value"},
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutInt("value", 2)

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)

				return sl
			},
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutInt("0", 2)
				m.PutInt("1", 5)

				return m
			},
		},
		{
			name: "nested object with no key path and value path",
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutEmptyMap("value").PutStr("test", "x")

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutEmptyMap("value").PutStr("test", "y")

				return sl
			},
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				thing1 := m.PutEmptyMap("0")
				thing1.PutStr("name", "foo")
				thing1.PutEmptyMap("value").PutStr("test", "x")

				thing2 := m.PutEmptyMap("1")
				thing2.PutStr("name", "bar")
				thing2.PutEmptyMap("value").PutStr("test", "y")

				return m
			},
		},
		{
			name:      "nested object with only value path",
			valuePath: []string{"value"},
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutEmptyMap("value").PutStr("test", "x")

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutEmptyMap("value").PutStr("test", "y")

				return sl
			},
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				thing1 := m.PutEmptyMap("0")
				thing1.PutStr("test", "x")
				thing2 := m.PutEmptyMap("1")
				thing2.PutStr("test", "y")

				return m
			},
		},

		{
			name: "literal slice without key/value path",
			value: func() any {
				sl := pcommon.NewSlice()
				sl.AppendEmpty().SetStr("a")
				sl.AppendEmpty().SetStr("b")
				return sl
			},
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("0", "a")
				m.PutStr("1", "b")
				return m
			},
		},

		{
			name: "pcommon.Value slice without key/value path",
			value: func() any {
				sl := pcommon.NewSlice()
				sl.AppendEmpty().SetStr("x")
				sl.AppendEmpty().SetInt(42)
				return sl
			},
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("0", "x")
				m.PutInt("1", 42)
				return m
			},
		},
		{
			name: "mixed types without key/value path",
			value: func() any {
				sl := pcommon.NewSlice()
				sl.AppendEmpty().SetStr("a")
				sl.AppendEmpty().SetInt(123)
				sl.AppendEmpty().SetBool(true)
				sl.AppendEmpty().SetStr("val")
				return sl
			},
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("0", "a")
				m.PutInt("1", 123)
				m.PutBool("2", true)
				m.PutStr("3", "val")
				return m
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyPathOptional := ottl.Optional[[]string]{}
			if len(tt.keyPath) > 0 {
				keyPathOptional = ottl.NewTestingOptional(tt.keyPath)
			}

			valuePathOptional := ottl.Optional[[]string]{}

			if len(tt.valuePath) > 0 {
				valuePathOptional = ottl.NewTestingOptional(tt.valuePath)
			}

			associateFunc, err := sliceToMapFunction[any](ottl.FunctionContext{}, &SliceToMapArguments[any]{
				Target: ottl.StandardPSliceGetter[any]{
					Getter: func(context.Context, any) (any, error) {
						val := tt.value()
						slice, ok := val.(pcommon.Slice)
						if !ok {
							return nil, fmt.Errorf("expected pcommon.Slice, got %T", val)
						}
						return slice, nil
					},
				},
				KeyPath:   keyPathOptional,
				ValuePath: valuePathOptional,
			})

			if tt.wantConfigErr != "" {
				require.ErrorContains(t, err, tt.wantConfigErr)
				return
			}
			require.NoError(t, err)

			result, err := associateFunc(nil, nil)
			if tt.wantExecutionErr != "" {
				require.ErrorContains(t, err, tt.wantExecutionErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want(), result.(pcommon.Map))
		})
	}
}
