// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_CompileMap(t *testing.T) {
	map1 := pcommon.NewMap()
	map1.PutStr("pattern", "item")

	map2 := pcommon.NewMap()
	map2.PutStr("pattern", "item")
	map2.PutStr("x", "x")

	tests := []struct {
		name         string
		objectGetter func(context.Context, any) (any, error)
		pattern      any
		expected     any
		err          bool
	}{
		{
			name: "map",
			objectGetter: func(context.Context, any) (any, error) {
				return map[string]any{
					"pattern": "item",
					"x":       "y",
				}, nil
			},
			pattern:  "pattern",
			expected: map1,
			err:      false,
		},
		{
			name: "common map",
			objectGetter: func(context.Context, any) (any, error) {
				return map2, nil
			},
			pattern:  "pattern",
			expected: map1,
			err:      false,
		},
		{
			name: "error",
			objectGetter: func(context.Context, any) (any, error) {
				return nil, fmt.Errorf("err")
			},
			pattern:  "pattern",
			expected: nil,
			err:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := compileMap[any](
				&ottl.StandardPMapGetter[any]{
					Getter: tt.objectGetter,
				},
				&ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) {
						return tt.pattern, nil
					},
				},
			)
			result, err := exprFunc(nil, nil)
			if tt.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_CompileTarget(t *testing.T) {
	tests := []struct {
		name     string
		object   map[string]any
		pattern  string
		expected map[string]any
	}{
		{
			name: "simple map",
			object: map[string]any{
				"pattern": "item",
				"x":       "y",
			},
			pattern: "pattern",
			expected: map[string]any{
				"pattern": "item",
			},
		},
		{
			name: "non matching map",
			object: map[string]any{
				"x": "y",
				"z": map[string]any{
					"z": "z",
				},
			},
			pattern:  "pattern",
			expected: map[string]any{},
		},
		{
			name: "complex map",
			object: map[string]any{
				"pattern-1": "item",
				"x":         "y",
				"z": map[string]any{
					"z": "z",
				},
				"pattern-2": map[string]any{
					"z": "z",
				},
				"pattern-3": map[string]any{
					"pattern-2": map[string]any{
						"z":         "z",
						"pattern-2": "x",
						"pattern-4": map[string]any{
							"pattern-5": "x",
						},
					},
				},
			},
			pattern: "pattern-*",
			expected: map[string]any{
				"pattern-1": "item",
				"pattern-2": map[string]any{},
				"pattern-3": map[string]any{
					"pattern-2": map[string]any{
						"pattern-2": "x",
						"pattern-4": map[string]any{
							"pattern-5": "x",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := compileTarget(tt.object, tt.pattern)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
