// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookup

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestBaseSourceTypeFunc(t *testing.T) {
	settings := componenttest.NewNopTelemetrySettings()
	base, err := NewBaseSource("typed", settings, nil)
	require.NoError(t, err)
	require.Equal(t, "typed", base.TypeFunc().Type())
}

func TestBaseSourceWrapLookup(t *testing.T) {
	settings := componenttest.NewNopTelemetrySettings()
	base, err := NewBaseSource("typed", settings, nil)
	require.NoError(t, err)

	lookupFn := base.WrapLookup(func(context.Context, string) (any, bool, error) {
		return "value", true, nil
	})

	val, found, err := lookupFn.Lookup(t.Context(), "key")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "value", val)
}
