// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nooplookupextension

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestFactoryCreatesExtension(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig()
	require.IsType(t, &Config{}, cfg)

	settings := extensiontest.NewNopSettings(Type)

	ext, err := factory.Create(t.Context(), settings, cfg)
	require.NoError(t, err)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, ext.Shutdown(t.Context()))
}

func TestNewLookupExtensionBehaviour(t *testing.T) {
	settings := componenttest.NewNopTelemetrySettings()
	settings.MeterProvider = noop.NewMeterProvider()

	ext, err := NewLookupExtension(settings)
	require.NoError(t, err)

	ctx := t.Context()
	require.NoError(t, ext.Start(ctx, componenttest.NewNopHost()))
	val, found, err := ext.Lookup(ctx, "missing")
	require.NoError(t, err)
	require.False(t, found)
	require.Nil(t, val)
	require.NoError(t, ext.Shutdown(ctx))
}
