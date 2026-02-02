// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_keys(t *testing.T) {
	tests := []struct {
		name     string
		target   map[string]any
		expected []any
	}{
		{
			name: "simple",
			target: map[string]any{
				"name":  "test",
				"value": "test2",
			},
			expected: []any{"name", "value"},
		},
		{
			name:     "empty",
			target:   map[string]any{},
			expected: []any{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pcommon.NewMap()
			err := m.FromRaw(tt.target)
			require.NoError(t, err)
			target := ottl.StandardPMapGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return m, nil
				},
			}
			expected := pcommon.NewSlice()
			err = expected.FromRaw(tt.expected)
			require.NoError(t, err)

			exprFunc := keys[any](target)
			rv, err := exprFunc(nil, nil)
			require.NoError(t, err)
			rvSlice := rv.(pcommon.Slice)
			raw := rvSlice.AsRaw()

			assert.True(t, compareSlices(tt.expected, raw))
		})
	}
}
