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
	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "",
		},
	}
	assert.Error(t, cfg.Validate())

	cfg = &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "https://faro.example.com/collect",
		},
	}
	assert.NoError(t, cfg.Validate())
}
