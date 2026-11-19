// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xottl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTestingLambdaExpression(t *testing.T) {
	expr := NewTestingLambdaExpression[any]([]string{"_", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
		return []any{resolveBinding("v"), resolveBinding("_"), resolveBinding("missing")}, nil
	})
	require.NoError(t, expr.ValidateArity(2))

	activation, err := expr.Activate(t.Context())
	require.NoError(t, err)
	defer activation.Close()

	assert.False(t, activation.IsArgBound(0))
	assert.True(t, activation.IsArgBound(1))
	require.NoError(t, activation.SetArg(0, "skip"))
	require.NoError(t, activation.SetArg(1, "value"))

	got, err := activation.Eval(nil)
	require.NoError(t, err)
	assert.Equal(t, []any{"value", nil, nil}, got)
}
