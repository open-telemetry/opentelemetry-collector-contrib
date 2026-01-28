// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetLiteralValue(t *testing.T) {
	t.Run("getter does not implement literalGetter", func(t *testing.T) {
		val, ok := GetLiteralValue[any, StringValue](newValueGetter[any](NewStringValue("val")))
		require.False(t, ok)
		require.Empty(t, val)
	})
	t.Run("getter contains literal", func(t *testing.T) {
		g := newLiteralGetter[any](NewStringValue("value"))
		val, ok := GetLiteralValue[any, StringValue](g)
		require.True(t, ok)
		require.Equal(t, NewStringValue("value"), val)
	})
}
