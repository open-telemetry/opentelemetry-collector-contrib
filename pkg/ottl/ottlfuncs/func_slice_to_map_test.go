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

func Test_SliceToMap(t *testing.T) {
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
			wantExecutionErr: "could not extract key from element: provided object does not contain the path [notfound]",
		},
		{
			name:      "flat object with key path and value path",
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
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutEmptyMap("value").PutStr("test", "x")

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)
				thing2.PutEmptyMap("value").PutStr("test", "y")

				return sl
			},
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
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutEmptyMap("value").PutStr("test", "x")

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)
				thing2.PutEmptyMap("value").PutStr("test", "y")

				return sl
			},
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
			name:      "nested object with value path not resolving to a value",
			keyPath:   []string{"value", "test"},
			valuePath: []string{"notfound"},
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutEmptyMap("value").PutStr("test", "x")

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)
				thing2.PutEmptyMap("value").PutStr("test", "y")

				return sl
			},
			wantExecutionErr: "could not extract value from element: provided object does not contain the path [notfound]",
		},
		{
			name:      "nested object with value path segment resolving to non-map value",
			keyPath:   []string{"value", "test"},
			valuePath: []string{"name", "nothing"},
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutEmptyMap("value").PutStr("test", "x")

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)
				thing2.PutEmptyMap("value").PutStr("test", "y")

				return sl
			},
			wantExecutionErr: "could not extract value from element: provided object does not contain the path [name nothing]",
		},
		{
			name:    "unsupported type",
			keyPath: []string{"name"},
			value: func() any {
				return pcommon.NewMap()
			},
			wantExecutionErr: "unsupported type provided to SliceToMap function: pcommon.Map",
		},
		{
			name:    "slice containing unsupported value type",
			keyPath: []string{"name"},
			value: func() any {
				sl := pcommon.NewSlice()
				sl.AppendEmpty().SetStr("unsupported")

				return sl
			},
			wantExecutionErr: "could not cast element 'unsupported' to map[string]any",
		},
		{
			name:    "empty key path",
			keyPath: []string{},
			value: func() any {
				return pcommon.NewMap()
			},
			wantConfigErr: "key path must contain at least one element",
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
			wantExecutionErr: "could not cast element 'nothingToSeeHere' to map[string]any",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valuePathOptional := ottl.Optional[[]string]{}

			if len(tt.valuePath) > 0 {
				valuePathOptional = ottl.NewTestingOptional(tt.valuePath)
			}
			associateFunc, err := sliceToMapFunction[any](ottl.FunctionContext{}, &SliceToMapArguments[any]{
				Target: &ottl.StandardGetSetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return tt.value(), nil
					},
				},
				KeyPath:   tt.keyPath,
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
			require.Equal(t, tt.want().AsRaw(), result.(pcommon.Map).AsRaw())
		})
	}
}
