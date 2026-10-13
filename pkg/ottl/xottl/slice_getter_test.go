// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xottl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestNewTestingSliceGetter(t *testing.T) {
	for _, literal := range []bool{true, false} {
		sg := NewTestingSliceGetter[any](literal, []string{"a", "b"})

		vals, err := sg.Get(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, vals)

		var ranged []string
		nonNil, err := sg.Range(t.Context(), nil, func(v string) bool {
			ranged = append(ranged, v)
			return true
		})
		require.NoError(t, err)
		assert.True(t, nonNil)
		assert.Equal(t, []string{"a", "b"}, ranged)

		length, ok := sg.Len()
		assert.Equal(t, literal, ok)
		if literal {
			assert.Equal(t, 2, length)
		}
	}
}

func TestGetScalarLiteralValues(t *testing.T) {
	vals, ok := GetScalarLiteralValues(NewTestingSliceGetter[any](true, []int64{1, 2}))
	require.True(t, ok)
	assert.Equal(t, []int64{1, 2}, vals)

	vals, ok = GetScalarLiteralValues(NewTestingSliceGetter[any](false, []int64{1, 2}))
	require.False(t, ok)
	assert.Nil(t, vals)
}

func TestGetLiteralValues(t *testing.T) {
	literalGetter := func(v string, literal bool) ottl.StringGetter[any] {
		g, err := ottl.NewTestingLiteralGetter[any, string](literal, ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return v, nil
			},
		})
		require.NoError(t, err)
		return g
	}

	vals, ok := GetLiteralValues[any, string](NewTestingSliceGetter[any](true, []ottl.StringGetter[any]{
		literalGetter("a", true),
		literalGetter("b", true),
	}))
	require.True(t, ok)
	assert.Equal(t, []string{"a", "b"}, vals)

	vals, ok = GetLiteralValues[any, string](NewTestingSliceGetter[any](true, []ottl.StringGetter[any]{
		literalGetter("a", true),
		literalGetter("b", false),
	}))
	require.False(t, ok)
	assert.Nil(t, vals)

	vals, ok = GetLiteralValues[any, string](NewTestingSliceGetter[any](false, []ottl.StringGetter[any]{
		literalGetter("a", true),
	}))
	require.False(t, ok)
	assert.Nil(t, vals)
}
