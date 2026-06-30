// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension"

import (
	"os"
	"path"
	"testing"

	"github.com/cenkalti/backoff/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension/internal/credentials"
)

func TestFactory_CreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	homePath, err := os.UserHomeDir()
	require.NoError(t, err)
	defaultCredsPath := path.Join(homePath, credentials.DefaultCollectorDataDirectory)
	clientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfig.MaxIdleConns = 0
	clientConfig.IdleConnTimeout = 0
	clientConfig.ForceAttemptHTTP2 = false
	assert.Equal(t, &Config{
		ClientConfig:                  clientConfig,
		HeartBeatInterval:             DefaultHeartbeatInterval,
		APIBaseURL:                    DefaultAPIBaseURL,
		CollectorCredentialsDirectory: defaultCredsPath,
		DiscoverCollectorTags:         true,
		UpdateMetadata:                true,
		BackOff: backOffConfig{
			InitialInterval: backoff.DefaultInitialInterval,
			MaxInterval:     backoff.DefaultMaxInterval,
			MaxElapsedTime:  backoff.DefaultMaxElapsedTime,
		},
	}, cfg)

	assert.NoError(t, xconfmap.Validate(cfg))

	ccfg := cfg.(*Config)
	ccfg.CollectorName = "test_collector"
	ccfg.Credentials.InstallationToken = "dummy_install_token"

	ext, err := createExtension(t.Context(),
		extension.Settings{
			TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		},
		cfg,
	)
	require.NoError(t, err)
	require.NotNil(t, ext)
}

func TestFactory_Create(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.CollectorName = "test_collector"
	cfg.Credentials.InstallationToken = "dummy_install_token"

	ext, err := createExtension(t.Context(),
		extension.Settings{
			TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		},
		cfg,
	)
	require.NoError(t, err)
	require.NotNil(t, ext)
}
