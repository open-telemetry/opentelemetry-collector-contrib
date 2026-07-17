// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupsource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

type testConfig struct {
	Value string
}

func (*testConfig) Validate() error {
	return nil
}

func TestNewSourceFactory(t *testing.T) {
	factory := NewSourceFactory(
		"test",
		func() SourceConfig { return &testConfig{Value: "default"} },
		func(_ context.Context, _ CreateSettings, cfg SourceConfig) (Source, error) {
			tc := cfg.(*testConfig)
			return NewSource(
				func(_ context.Context, _ string) (any, bool, error) {
					return tc.Value, true, nil
				},
				func() string { return "test" },
				nil,
				nil,
			), nil
		},
	)

	require.NotNil(t, factory)
	assert.Equal(t, "test", factory.Type())

	// Test default config
	defaultCfg := factory.CreateDefaultConfig()
	require.NotNil(t, defaultCfg)
	assert.Equal(t, "default", defaultCfg.(*testConfig).Value)

	// Test source creation
	settings := CreateSettings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}
	source, err := factory.CreateSource(t.Context(), settings, defaultCfg)
	require.NoError(t, err)
	require.NotNil(t, source)
	assert.Equal(t, "test", source.Type())

	// Verify lookup works
	val, found, err := source.Lookup(t.Context(), "key")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "default", val)
}

func TestSourceFactoryNilFunctions(t *testing.T) {
	factory := NewSourceFactory("empty", nil, nil)

	assert.Equal(t, "empty", factory.Type())
	assert.Nil(t, factory.CreateDefaultConfig())

	settings := CreateSettings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}
	source, err := factory.CreateSource(t.Context(), settings, nil)
	require.NoError(t, err)
	assert.Nil(t, source)
}
