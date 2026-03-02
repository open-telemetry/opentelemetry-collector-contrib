// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package noop

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)
	assert.Equal(t, "noop", factory.Type())
}

func TestConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg)

	// Config should validate successfully
	require.NoError(t, cfg.Validate())
}

func TestCreateSource(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	settings := lookupsource.CreateSettings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}

	source, err := factory.CreateSource(t.Context(), settings, cfg)
	require.NoError(t, err)
	require.NotNil(t, source)

	assert.Equal(t, "noop", source.Type())
}

func TestNoopSourceLookup(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	settings := lookupsource.CreateSettings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}

	source, err := factory.CreateSource(t.Context(), settings, cfg)
	require.NoError(t, err)

	// Noop source always returns not found
	val, found, err := source.Lookup(t.Context(), "any-key")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, val)

	// Try different keys - all should return not found
	for _, key := range []string{"", "key1", "user001", "test"} {
		val, found, err = source.Lookup(t.Context(), key)
		require.NoError(t, err)
		assert.False(t, found, "key %q should not be found", key)
		assert.Nil(t, val)
	}
}

func TestNoopSourceStartShutdown(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	settings := lookupsource.CreateSettings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}

	source, err := factory.CreateSource(t.Context(), settings, cfg)
	require.NoError(t, err)

	host := componenttest.NewNopHost()

	// Start should succeed
	require.NoError(t, source.Start(t.Context(), host))

	// Shutdown should succeed
	require.NoError(t, source.Shutdown(t.Context()))
}
