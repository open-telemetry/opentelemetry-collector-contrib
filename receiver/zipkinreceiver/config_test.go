// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	customNameServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	customNameServerConfig.WriteTimeout = 0
	customNameServerConfig.ReadHeaderTimeout = 0
	customNameServerConfig.IdleTimeout = 0
	customNameServerConfig.KeepAlivesEnabled = false
	customNameServerConfig.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  "localhost:8765",
	}

	parseStringsServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	parseStringsServerConfig.WriteTimeout = 0
	parseStringsServerConfig.ReadHeaderTimeout = 0
	parseStringsServerConfig.IdleTimeout = 0
	parseStringsServerConfig.KeepAlivesEnabled = false
	parseStringsServerConfig.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  defaultHTTPEndpoint,
	}

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewID(metadata.Type),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "customname"),
			expected: &Config{
				ServerConfig:    customNameServerConfig,
				ParseStringTags: false,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "parse_strings"),
			expected: &Config{
				ServerConfig:    parseStringsServerConfig,
				ParseStringTags: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
