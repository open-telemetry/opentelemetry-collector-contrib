// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faroexporter

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub("exporters")
	require.NoError(t, err)
	sub, err = sub.Sub("faro")
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.Equal(t, "https://faro.example.com/collect", cfg.(*Config).Endpoint)
}

func TestValidateConfig(t *testing.T) {
	emptyEndpointClientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	emptyEndpointClientConfig.MaxIdleConns = 0
	emptyEndpointClientConfig.IdleConnTimeout = 0
	emptyEndpointClientConfig.ForceAttemptHTTP2 = false
	emptyEndpointClientConfig.Endpoint = ""
	cfg := &Config{
		ClientConfig: emptyEndpointClientConfig,
	}
	assert.Error(t, cfg.Validate())

	validEndpointClientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	validEndpointClientConfig.MaxIdleConns = 0
	validEndpointClientConfig.IdleConnTimeout = 0
	validEndpointClientConfig.ForceAttemptHTTP2 = false
	validEndpointClientConfig.Endpoint = "https://faro.example.com/collect"
	cfg = &Config{
		ClientConfig: validEndpointClientConfig,
	}
	assert.NoError(t, cfg.Validate())
}
