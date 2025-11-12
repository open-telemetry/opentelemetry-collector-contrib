// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// nonLiteralStringGetter implements typedGetter but NOT literalGetter.
// Used to verify GetLiteralValue returns false when literalGetter isn't implemented.
type nonLiteralStringGetter[K any] struct{ v string }

func (g nonLiteralStringGetter[K]) Get(_ context.Context, _ K) (string, error) { return g.v, nil }

func TestGetLiteralValue(t *testing.T) {
	t.Run("getter does not implement literalGetter", func(t *testing.T) {
		val, ok := GetLiteralValue[any, string](nonLiteralStringGetter[any]{v: "val"})
		require.False(t, ok)
		require.Empty(t, val)
	})

	t.Run("getter does not contain literal", func(t *testing.T) {
		g := mockedGetter[any]{value: "value"}
		val, ok := GetLiteralValue[any, any](g)
		require.False(t, ok)
		require.Nil(t, val)
	})
	t.Run("getter contains literal", func(t *testing.T) {
		g := newLiteral[any, any]("value")
		val, ok := GetLiteralValue[any, any](g)
		require.True(t, ok)
		require.Equal(t, "value", val)
	})
	t.Run("getter returns error", func(t *testing.T) {
		g := newErrLiteral(errors.New("err"))
		val, ok := GetLiteralValue[any, any](g)
		require.False(t, ok)
		require.Nil(t, val)
	})
}
