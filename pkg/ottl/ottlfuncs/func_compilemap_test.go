package ottlfuncs

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func Test_CompileMap(t *testing.T) {
	map1 := pcommon.NewMap()
	map1.PutStr("pattern", "item")

	map2 := pcommon.NewMap()
	map2.PutStr("pattern", "item")
	map2.PutStr("x", "x")

	tests := []struct {
		name     string
		object   any
		pattern  any
		expected pcommon.Map
		err      bool
	}{
		{
			name: "map",
			object: map[string]any{
				"pattern": "item",
				"x":       "y",
			},
			pattern:  "pattern",
			expected: map1,
			err:      false,
		},
		{
			name:     "common map",
			object:   map2,
			pattern:  "pattern",
			expected: map1,
			err:      false,
		},
		{
			name:     "unsupported datatype",
			object:   "str",
			pattern:  "pattern",
			expected: pcommon.NewMap(),
			err:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := compileMap[any](
				&ottl.StandardGetSetter[any]{
					Getter: func(context.Context, any) (any, error) {
						return tt.object, nil
					},
				},
				&ottl.StandardStringGetter[any]{
					Getter: func(ctx context.Context, tCtx any) (any, error) {
						return tt.pattern, nil
					},
				},
			)
			assert.NoError(t, err)
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
