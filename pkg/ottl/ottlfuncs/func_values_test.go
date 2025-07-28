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

func Test_values(t *testing.T) {
	tests := []struct {
		name    string
		target  map[string]any
		wantRaw []any
	}{
		{
			name: "simple",
			target: map[string]any{
				"ke1":  "test",
				"key2": "test2",
				"key3": 4,
				"key4": true,
				"key5": []any{1, 2, 3},
				"key6": 3.14,
				"key7": map[string]any{"subkey": "subvalue"},
			},
			wantRaw: []any{"test", "test2", int64(4), true, []any{int64(1), int64(2), int64(3)}, 3.14, map[string]any{"subkey": "subvalue"}},
		},
		{
			name:    "empty input map",
			target:  map[string]any{},
			wantRaw: []any{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pcommon.NewMap()
			err := m.FromRaw(tt.target)
			require.NoError(t, err)
			target := ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return m, nil
				},
			}

			exprFunc := values[any](target)
			gotSlice, err := exprFunc(nil, nil)
			require.NoError(t, err)
			gotRaw := gotSlice.(pcommon.Slice).AsRaw()

			assert.ElementsMatch(t, gotRaw, tt.wantRaw)
		})
	}
}
