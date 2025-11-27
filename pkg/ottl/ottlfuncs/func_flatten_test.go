// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_flatten(t *testing.T) {
	tests := []struct {
		name     string
		target   map[string]any
		prefix   ottl.Optional[string]
		depth    ottl.Optional[int64]
		expected map[string]any
		conflict bool
	}{
		{
			name: "simple",
			target: map[string]any{
				"name": "test",
			},
			prefix: ottl.Optional[string]{},
			depth:  ottl.Optional[int64]{},
			expected: map[string]any{
				"name": "test",
			},
		},
		{
			name: "nested map",
			target: map[string]any{
				"address": map[string]any{
					"street": "first",
					"house":  int64(1234),
				},
			},
			prefix: ottl.Optional[string]{},
			depth:  ottl.Optional[int64]{},
			expected: map[string]any{
				"address.street": "first",
				"address.house":  int64(1234),
			},
		},
		{
			name: "nested slice",
			target: map[string]any{
				"occupants": []any{
					"user 1",
					"user 2",
				},
			},
			prefix: ottl.Optional[string]{},
			depth:  ottl.Optional[int64]{},
			expected: map[string]any{
				"occupants.0": "user 1",
				"occupants.1": "user 2",
			},
		},
		{
			name: "combination",
			target: map[string]any{
				"name": "test",
				"address": map[string]any{
					"street": "first",
					"house":  int64(1234),
				},
				"occupants": []any{
					"user 1",
					"user 2",
				},
			},
			prefix: ottl.Optional[string]{},
			depth:  ottl.Optional[int64]{},
			expected: map[string]any{
				"name":           "test",
				"address.street": "first",
				"address.house":  int64(1234),
				"occupants.0":    "user 1",
				"occupants.1":    "user 2",
			},
		},
		{
			name: "combination with mixed nested slices",
			target: map[string]any{
				"name": "test",
				"address": map[string]any{
					"street": "first",
					"house":  int64(1234),
				},
				"occupants": []any{
					"user 1",
					map[string]any{
						"name": "user 2",
					},
				},
			},
			prefix: ottl.Optional[string]{},
			depth:  ottl.Optional[int64]{},
			expected: map[string]any{
				"name":             "test",
				"address.street":   "first",
				"address.house":    int64(1234),
				"occupants.0":      "user 1",
				"occupants.1.name": "user 2",
			},
		},
		{
			name: "deep nesting",
			target: map[string]any{
				"1": map[string]any{
					"2": map[string]any{
						"3": map[string]any{
							"4": "5",
						},
					},
				},
			},
			prefix: ottl.Optional[string]{},
			depth:  ottl.Optional[int64]{},
			expected: map[string]any{
				"1.2.3.4": "5",
			},
		},
		{
			name: "use prefix",
			target: map[string]any{
				"name": "test",
				"address": map[string]any{
					"street": "first",
					"house":  int64(1234),
				},
				"occupants": []any{
					"user 1",
					"user 2",
				},
			},
			prefix: ottl.NewTestingOptional[string]("app"),
			depth:  ottl.Optional[int64]{},
			expected: map[string]any{
				"app.name":           "test",
				"app.address.street": "first",
				"app.address.house":  int64(1234),
				"app.occupants.0":    "user 1",
				"app.occupants.1":    "user 2",
			},
		},
		{
			name: "max depth",
			target: map[string]any{
				"0": map[string]any{
					"1": map[string]any{
						"2": map[string]any{
							"3": "value",
						},
					},
				},
			},
			prefix: ottl.Optional[string]{},
			depth:  ottl.NewTestingOptional[int64](2),
			expected: map[string]any{
				"0.1.2": map[string]any{
					"3": "value",
				},
			},
		},
		{
			name: "max depth with slice",
			target: map[string]any{
				"0": map[string]any{
					"1": map[string]any{
						"2": map[string]any{
							"3": "value",
						},
					},
				},
				"1": map[string]any{
					"1": []any{
						map[string]any{
							"1": "value",
						},
					},
				},
			},
			prefix: ottl.Optional[string]{},
			depth:  ottl.NewTestingOptional[int64](1),
			expected: map[string]any{
				"0.1": map[string]any{
					"2": map[string]any{
						"3": "value",
					},
				},
				"1.1": []any{
					map[string]any{
						"1": "value",
					},
				},
			},
		},
		{
			name: "simple - conflict on",
			target: map[string]any{
				"name": "test",
			},
			prefix: ottl.Optional[string]{},
			depth:  ottl.Optional[int64]{},
			expected: map[string]any{
				"name": "test",
			},
			conflict: true,
		},
		{
			name: "nested map - conflict on",
			target: map[string]any{
				"address": map[string]any{
					"street": "first",
					"house":  int64(1234),
				},
			},
			prefix: ottl.Optional[string]{},
			depth:  ottl.Optional[int64]{},
			expected: map[string]any{
				"address.street": "first",
				"address.house":  int64(1234),
			},
			conflict: true,
		},
		{
			name: "nested slice - conflict on",
			target: map[string]any{
				"occupants": []any{
					"user 1",
					"user 2",
				},
			},
			prefix: ottl.Optional[string]{},
			depth:  ottl.Optional[int64]{},
			expected: map[string]any{
				"occupants":   "user 1",
				"occupants.0": "user 2",
			},
			conflict: true,
		},
		{
			name: "combination - conflict on",
			target: map[string]any{
				"name": "test",
				"address": map[string]any{
					"street": "first",
					"house":  int64(1234),
				},
				"occupants": []any{
					"user 1",
					"user 2",
				},
			},
			prefix: ottl.Optional[string]{},
			depth:  ottl.Optional[int64]{},
			expected: map[string]any{
				"name":           "test",
				"address.street": "first",
				"address.house":  int64(1234),
				"occupants":      "user 1",
				"occupants.0":    "user 2",
			},
			conflict: true,
		},
		{
			name: "deep nesting - conflict on",
			target: map[string]any{
				"1": map[string]any{
					"2": map[string]any{
						"3": map[string]any{
							"4": "5",
						},
					},
				},
			},
			prefix: ottl.Optional[string]{},
			depth:  ottl.Optional[int64]{},
			expected: map[string]any{
				"1.2.3.4": "5",
			},
			conflict: true,
		},
		{
			name: "use prefix - conflict on",
			target: map[string]any{
				"name": "test",
				"address": map[string]any{
					"street": "first",
					"house":  int64(1234),
				},
				"occupants": []any{
					"user 1",
					"user 2",
				},
			},
			prefix: ottl.NewTestingOptional[string]("app"),
			depth:  ottl.Optional[int64]{},
			expected: map[string]any{
				"app.name":           "test",
				"app.address.street": "first",
				"app.address.house":  int64(1234),
				"app.occupants":      "user 1",
				"app.occupants.0":    "user 2",
			},
			conflict: true,
		},
		{
			name: "max depth - conflict on",
			target: map[string]any{
				"0": map[string]any{
					"1": map[string]any{
						"2": map[string]any{
							"3": "value",
						},
					},
				},
			},
			prefix: ottl.Optional[string]{},
			depth:  ottl.NewTestingOptional[int64](2),
			expected: map[string]any{
				"0.1.2": map[string]any{
					"3": "value",
				},
			},
			conflict: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pcommon.NewMap()
			err := m.FromRaw(tt.target)
			require.NoError(t, err)

			setterWasCalled := false
			target := ottl.StandardPMapGetSetter[any]{
				Getter: func(context.Context, any) (pcommon.Map, error) {
					return m, nil
				},
				Setter: func(_ context.Context, _, val any) error {
					setterWasCalled = true
					if v, ok := val.(pcommon.Map); ok {
						v.CopyTo(m)
						return nil
					}
					return errors.New("expected pcommon.Map")
				},
			}

			exprFunc, err := flatten[any](target, tt.prefix, tt.depth, ottl.NewTestingOptional[bool](tt.conflict))
			require.NoError(t, err)

			_, err = exprFunc(nil, nil)
			require.NoError(t, err)
			assert.True(t, setterWasCalled)

			assert.Equal(t, tt.expected, m.AsRaw())
		})
	}
}

func Test_flatten_undeterministic(t *testing.T) {
	tests := []struct {
		name           string
		target         map[string]any
		prefix         ottl.Optional[string]
		depth          ottl.Optional[int64]
		expectedKeys   []string
		expectedValues []any
		conflict       bool
	}{
		{
			name: "conflicting map - conflict on",
			target: map[string]any{
				"address": map[string]any{
					"street": map[string]any{
						"house": int64(1234),
					},
				},
				"address.street": map[string]any{
					"house": int64(1235),
				},
			},
			prefix: ottl.Optional[string]{},
			depth:  ottl.Optional[int64]{},
			expectedKeys: []string{
				"address.street.house",
				"address.street.house.0",
			},
			expectedValues: []any{
				int64(1234),
				int64(1235),
			},
			conflict: true,
		},
		{
			name: "conflicting slice - conflict on",
			target: map[string]any{
				"address": map[string]any{
					"street": []any{"first"},
					"house":  int64(1234),
				},
				"address.street": []any{"second"},
			},
			prefix: ottl.Optional[string]{},
			depth:  ottl.Optional[int64]{},
			expectedKeys: []string{
				"address.street",
				"address.house",
				"address.street.0",
			},
			expectedValues: []any{
				int64(1234),
				"second",
				"first",
			},
			conflict: true,
		},
		{
			name: "conflicting map with nested slice - conflict on",
			target: map[string]any{
				"address": map[string]any{
					"street": "first",
					"house":  int64(1234),
				},
				"address.street": "second",
				"occupants": []any{
					"user 1",
					"user 2",
				},
			},
			prefix: ottl.Optional[string]{},
			depth:  ottl.Optional[int64]{},
			expectedKeys: []string{
				"address.street",
				"address.house",
				"address.street.0",
				"occupants",
				"occupants.0",
			},
			expectedValues: []any{
				int64(1234),
				"second",
				"first",
				"user 1",
				"user 2",
			},
			conflict: true,
		},
		{
			name: "conflicting map with nested slice in conflicting item - conflict on",
			target: map[string]any{
				"address": map[string]any{
					"street": map[string]any{
						"number": "first",
					},
					"house": int64(1234),
				},
				"address.street": map[string]any{
					"number": []any{"second", "third"},
				},
				"address.street.number": "fourth",
				"occupants": []any{
					"user 1",
					"user 2",
				},
			},
			prefix: ottl.Optional[string]{},
			depth:  ottl.Optional[int64]{},
			expectedKeys: []string{
				"address.street.number",
				"address.house",
				"address.street.number.0",
				"address.street.number.1",
				"occupants",
				"occupants.0",
				"address.street.number.2",
			},
			expectedValues: []any{
				int64(1234),
				"second",
				"first",
				"third",
				"fourth",
				"user 1",
				"user 2",
			},
			conflict: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pcommon.NewMap()
			err := m.FromRaw(tt.target)
			require.NoError(t, err)

			setterWasCalled := false
			target := ottl.StandardPMapGetSetter[any]{
				Getter: func(context.Context, any) (pcommon.Map, error) {
					return m, nil
				},
				Setter: func(_ context.Context, _, val any) error {
					setterWasCalled = true
					if v, ok := val.(pcommon.Map); ok {
						v.CopyTo(m)
						return nil
					}
					return errors.New("expected pcommon.Map")
				},
			}

			exprFunc, err := flatten[any](target, tt.prefix, tt.depth, ottl.NewTestingOptional[bool](tt.conflict))
			require.NoError(t, err)

			_, err = exprFunc(nil, nil)
			require.NoError(t, err)
			assert.True(t, setterWasCalled)

			keys, val := extractKeysAndValues(m.AsRaw())

			assert.True(t, compareSlices(keys, tt.expectedKeys))
			assert.True(t, compareSlices(val, tt.expectedValues))
		})
	}
}

func Test_flatten_bad_depth(t *testing.T) {
	tests := []struct {
		name  string
		depth ottl.Optional[int64]
	}{
		{
			name:  "negative depth",
			depth: ottl.NewTestingOptional[int64](-1),
		},
		{
			name:  "zero depth",
			depth: ottl.NewTestingOptional[int64](0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := &ottl.StandardPMapGetSetter[any]{
				Getter: func(context.Context, any) (pcommon.Map, error) {
					return pcommon.NewMap(), nil
				},
			}
			_, err := flatten[any](target, ottl.Optional[string]{}, tt.depth, ottl.NewTestingOptional[bool](false))
			assert.Error(t, err)
		})
	}
}

func extractKeysAndValues(m map[string]any) ([]string, []any) {
	keys := make([]string, 0, len(m))
	values := make([]any, 0, len(m))
	for key, value := range m {
		keys = append(keys, key)
		values = append(values, value)
	}
	return keys, values
}

func compareSlices[K string | any](a, b []K) bool {
	if len(a) != len(b) {
		return false
	}

	aMap := make(map[any]int)
	bMap := make(map[any]int)

	for _, item := range a {
		aMap[item]++
	}

	for _, item := range b {
		bMap[item]++
	}

	return reflect.DeepEqual(aMap, bMap)
}
