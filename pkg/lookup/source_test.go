// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookup

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestNewSourceNoopBehaviour(t *testing.T) {
	src := NewSource(nil, nil, nil, nil)

	val, found, err := src.Lookup(t.Context(), "foo")
	require.NoError(t, err)
	require.False(t, found)
	require.Nil(t, val)

	require.Empty(t, src.Type())
	require.NoError(t, src.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, src.Shutdown(t.Context()))
}

func TestNewSourceFunctionalBehaviour(t *testing.T) {
	started := false
	stopped := false

	src := NewSource(
		LookupFunc(func(context.Context, string) (any, bool, error) {
			return "value", true, nil
		}),
		TypeFunc(func() string { return "custom" }),
		StartFunc(func(context.Context, component.Host) error {
			started = true
			return nil
		}),
		ShutdownFunc(func(context.Context) error {
			stopped = true
			return nil
		}),
	)

	val, found, err := src.Lookup(t.Context(), "foo")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "value", val)
	require.Equal(t, "custom", src.Type())

	require.NoError(t, src.Start(t.Context(), componenttest.NewNopHost()))
	require.True(t, started)
	require.NoError(t, src.Shutdown(t.Context()))
	require.True(t, stopped)
}

func TestNewLookupExtensionSealed(t *testing.T) {
	ext := NewLookupExtension(nil, TypeFunc(func() string { return "ext" }), nil, nil)

	require.Equal(t, "ext", ext.Type())
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, ext.Shutdown(t.Context()))
}

func TestBaseSourceHelpers(t *testing.T) {
	settings := componenttest.NewNopTelemetrySettings()
	base, err := NewBaseSource("helper", settings, nil)
	require.NoError(t, err)
	require.Equal(t, "helper", base.TypeFunc().Type())

	lookupFn := base.WrapLookup(func(context.Context, string) (any, bool, error) {
		return 42, true, nil
	})

	val, found, err := lookupFn.Lookup(t.Context(), "anything")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 42, val)
}
