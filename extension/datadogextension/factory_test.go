// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogextension

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/httpserver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/metadata"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	assert.Equal(t, metadata.Type, f.Type())
}

func TestFactory_CreateDefaultConfig(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()
	require.NotNil(t, cfg)

	extCfg, ok := cfg.(*Config)
	require.True(t, ok)

	assert.Equal(t, datadogconfig.DefaultSite, extCfg.API.Site)
	assert.True(t, extCfg.API.FailOnInvalidKey)
	assert.Equal(t, httpserver.DefaultServerEndpoint, extCfg.HTTPConfig.Endpoint)
	assert.Equal(t, "/metadata", extCfg.HTTPConfig.Path)
	assert.Equal(t, confighttp.NewDefaultClientConfig(), extCfg.ClientConfig)
}

func TestFactory_Create(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()
	// The API key is required for the config to be valid, but create doesn't validate.
	cfg.(*Config).API.Key = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	set := extensiontest.NewNopSettings(component.MustNewType("datadog"))

	t.Run("success", func(t *testing.T) {
		ext, err := f.Create(context.Background(), set, cfg)
		require.NoError(t, err)
		require.NotNil(t, ext)
	})

	t.Run("invalid config type", func(t *testing.T) {
		_, err := f.Create(context.Background(), set, &struct{}{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid config type")
	})

	// Note: Testing the error path from `f.SourceProvider` within `Create` is difficult
	// because it requires mocking the hostmetadata package, which performs network calls.
	// We test the error handling of the `SourceProvider` method directly instead.
}

func TestInternalFactory_SourceProvider(t *testing.T) {
	set := componenttest.NewNopTelemetrySettings()
	hostname := "test-host"
	timeout := 1 * time.Second

	t.Run("success and memoization", func(t *testing.T) {
		f := &factory{} // Test the internal factory struct directly

		// First call should create the provider
		sp, err := f.SourceProvider(set, hostname, timeout)
		require.NoError(t, err)
		require.NotNil(t, sp)
		require.NotNil(t, f.sourceProvider, "internal source provider should be set")

		// Second call should return the same provider instance due to sync.Once
		sp2, err := f.SourceProvider(set, hostname, timeout)
		require.NoError(t, err)
		assert.Same(t, sp, sp2)
		assert.Same(t, f.sourceProvider, sp2)
	})

	t.Run("error handling", func(t *testing.T) {
		f := &factory{
			providerErr: errors.New("provider creation failed"),
		}
		// Manually trigger the once.Do to simulate the error being stored
		f.onceProvider.Do(func() {})

		// Subsequent calls should return the stored error
		_, err := f.SourceProvider(set, hostname, timeout)
		require.Error(t, err)
		assert.Equal(t, "provider creation failed", err.Error())
	})
}

func TestFactory_CreateErrorPaths(t *testing.T) {
	f := NewFactory()

	t.Run("invalid config type", func(t *testing.T) {
		set := extensiontest.NewNopSettings(component.MustNewType("datadog"))
		_, err := f.Create(context.Background(), set, &struct{}{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid config type")
	})

	t.Run("source provider error", func(t *testing.T) {
		// Create a factory with pre-set error
		internalFactory := &factory{
			providerErr: errors.New("source provider creation failed"),
		}
		internalFactory.onceProvider.Do(func() {}) // Trigger the sync.Once to store the error

		cfg := &Config{
			API: datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Site: "datadoghq.com"},
		}
		set := extensiontest.NewNopSettings(component.MustNewType("datadog"))

		_, err := internalFactory.create(context.Background(), set, cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "source provider creation failed")
	})

	t.Run("newExtension error from hostProvider", func(t *testing.T) {
		// This test requires a factory that will fail in newExtension
		internalFactory := &factory{}

		// Mock the SourceProvider to return an error
		internalFactory.sourceProvider = &mockSourceProvider{
			err: errors.New("host provider error"),
		}
		internalFactory.onceProvider.Do(func() {}) // Ensure sync.Once is executed

		cfg := &Config{
			API: datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Site: "datadoghq.com"},
		}
		set := extensiontest.NewNopSettings(component.MustNewType("datadog"))

		_, err := internalFactory.create(context.Background(), set, cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "host provider error")
	})
}
