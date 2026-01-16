// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_toKeyValueString(t *testing.T) {
	tests := []struct {
		name          string
		target        ottl.PMapGetter[any]
		delimiter     ottl.Optional[string]
		pairDelimiter ottl.Optional[string]
		expected      string
	}{
		{
			name: "default delimiters with no nesting",
			target: ottl.StandardPMapGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return map[string]any{
						"key1": "value1",
						"key2": "value2",
					}, nil
				},
			},
			delimiter:     ottl.Optional[string]{},
			pairDelimiter: ottl.Optional[string]{},
			expected:      `key1=value1 key2=value2`,
		},
		{
			name: "custom delimiter with no nesting",
			target: ottl.StandardPMapGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return map[string]any{
						"key1": "value1",
						"key2": "value2",
					}, nil
				},
			},
			delimiter:     ottl.NewTestingOptional[string](":"),
			pairDelimiter: ottl.Optional[string]{},
			expected:      `key1:value1 key2:value2`,
		},
		{
			name: "custom pair delimiter with no nesting",
			target: ottl.StandardPMapGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return map[string]any{
						"key1": "value1",
						"key2": "value2",
					}, nil
				},
			},
			delimiter:     ottl.Optional[string]{},
			pairDelimiter: ottl.NewTestingOptional[string](","),
			expected:      `key1=value1,key2=value2`,
		},
		{
			name: "delimiters present in keys and values",
			target: ottl.StandardPMapGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return map[string]any{
						"key 1": "value 1",
						"key2=": "value2=",
					}, nil
				},
			},
			delimiter:     ottl.Optional[string]{},
			pairDelimiter: ottl.Optional[string]{},
			expected:      `"key 1"="value 1" "key2="="value2="`,
		},
		{
			name: "long delimiters present in keys and values",
			target: ottl.StandardPMapGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return map[string]any{
						"key1":    "value1",
						"key2,,,": "value2,,,,,,",
					}, nil
				},
			},
			delimiter:     ottl.NewTestingOptional[string](",,,"),
			pairDelimiter: ottl.Optional[string]{},
			expected:      `key1,,,value1 "key2,,,",,,"value2,,,,,,"`,
		},
		{
			name: "delimiters and quotes present in keys and values",
			target: ottl.StandardPMapGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return map[string]any{
						"key 1":   "value 1",
						"key2\"=": "value2\"=",
						"key\"3":  "value\"3",
					}, nil
				},
			},
			delimiter:     ottl.Optional[string]{},
			pairDelimiter: ottl.Optional[string]{},
			expected:      `"key 1"="value 1" key\"3=value\"3 "key2\"="="value2\"="`,
		},
		{
			name: "nested",
			target: ottl.StandardPMapGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return map[string]any{
						"key1": "value1",
						"key2": map[string]any{
							"key3": "value3",
							"key4": map[string]any{
								"key5": "value5",
								"key6": []any{"value6a", "value6b"},
							},
						},
						"key7": []any{"value7", []any{"value8a", map[string]any{"key8b": "value8b"}}},
					}, nil
				},
			},
			delimiter:     ottl.Optional[string]{},
			pairDelimiter: ottl.Optional[string]{},
			expected:      `key1=value1 key2={\"key3\":\"value3\",\"key4\":{\"key5\":\"value5\",\"key6\":[\"value6a\",\"value6b\"]}} key7=[\"value7\",[\"value8a\",{\"key8b\":\"value8b\"}]]`,
		},
		{
			name: "nested with delimiter present",
			target: ottl.StandardPMapGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return map[string]any{
						"key1": "value1",
						"key2": map[string]any{
							"key3": "value3",
							"key4": map[string]any{
								"key5": "value=5",
								"key6": []any{"value6a", "value6b"},
							},
						},
						"key7": []any{"value7", []any{"value8a", map[string]any{"key 8b": "value8b"}}},
					}, nil
				},
			},
			delimiter:     ottl.Optional[string]{},
			pairDelimiter: ottl.Optional[string]{},
			expected:      `key1=value1 key2="{\"key3\":\"value3\",\"key4\":{\"key5\":\"value=5\",\"key6\":[\"value6a\",\"value6b\"]}}" key7="[\"value7\",[\"value8a\",{\"key 8b\":\"value8b\"}]]"`,
		},
		{
			name: "nested with delimiter and quotes present",
			target: ottl.StandardPMapGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return map[string]any{
						"key1": "value1\"",
						"key2": map[string]any{
							"key3": "value3",
							"key4": map[string]any{
								"key5": "value=5\"",
								"key6": []any{"value6a", "value6b"},
							},
						},
						"key7": []any{"value7", []any{"value8a", map[string]any{"key 8b\"": "value8b"}}},
					}, nil
				},
			},
			delimiter:     ottl.Optional[string]{},
			pairDelimiter: ottl.Optional[string]{},
			expected:      `key1=value1\" key2="{\"key3\":\"value3\",\"key4\":{\"key5\":\"value=5\\"\",\"key6\":[\"value6a\",\"value6b\"]}}" key7="[\"value7\",[\"value8a\",{\"key 8b\\"\":\"value8b\"}]]"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := toKeyValueString[any](tt.target, tt.delimiter, tt.pairDelimiter, ottl.NewTestingOptional[bool](true))
			require.NoError(t, err)

			result, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)

			actual, ok := result.(string)
			assert.True(t, ok)

			assert.Equal(t, tt.expected, actual)
		})
	}
}

func Test_toKeyValueString_equal_delimiters(t *testing.T) {
	target := ottl.StandardPMapGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return map[string]any{
				"key1": "value1",
				"key2": "value2",
			}, nil
		},
	}
	delimiter := ottl.NewTestingOptional[string]("=")
	pairDelimiter := ottl.NewTestingOptional[string]("=")
	_, err := toKeyValueString[any](target, delimiter, pairDelimiter, ottl.NewTestingOptional[bool](false))
	assert.Error(t, err)

	delimiter = ottl.NewTestingOptional[string](" ")
	_, err = toKeyValueString[any](target, delimiter, ottl.Optional[string]{}, ottl.NewTestingOptional[bool](false))
	assert.Error(t, err)
}

func Test_toKeyValueString_bad_target(t *testing.T) {
	target := ottl.StandardPMapGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return 1, nil
		},
	}
	delimiter := ottl.NewTestingOptional[string]("=")
	pairDelimiter := ottl.NewTestingOptional[string]("!")
	exprFunc, err := toKeyValueString[any](target, delimiter, pairDelimiter, ottl.NewTestingOptional[bool](false))
	require.NoError(t, err)
	_, err = exprFunc(t.Context(), nil)
	assert.Error(t, err)
}

func Test_toKeyValueString_empty_target(t *testing.T) {
	target := ottl.StandardPMapGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "", nil
		},
	}
	delimiter := ottl.NewTestingOptional[string]("=")
	pairDelimiter := ottl.NewTestingOptional[string]("!")
	exprFunc, err := toKeyValueString[any](target, delimiter, pairDelimiter, ottl.NewTestingOptional[bool](false))
	require.NoError(t, err)
	_, err = exprFunc(t.Context(), nil)
	assert.Error(t, err)
}

func Test_toKeyValueString_empty_delimiters(t *testing.T) {
	target := ottl.StandardPMapGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "a=b c=d", nil
		},
	}
	delimiter := ottl.NewTestingOptional[string]("")

	_, err := toKeyValueString[any](target, delimiter, ottl.Optional[string]{}, ottl.NewTestingOptional[bool](false))
	assert.ErrorContains(t, err, "delimiter cannot be set to an empty string")

	_, err = toKeyValueString[any](target, ottl.Optional[string]{}, delimiter, ottl.NewTestingOptional[bool](false))
	assert.ErrorContains(t, err, "pair delimiter cannot be set to an empty string")
}
