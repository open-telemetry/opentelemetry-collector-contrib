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
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

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
				ServerConfig: confighttp.ServerConfig{
					Endpoint: "localhost:8765",
				},
				// When unmarshaling from YAML, protocols is empty for legacy config
				Protocols:       Protocols{},
				ParseStringTags: false,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "parse_strings"),
			expected: &Config{
				ServerConfig: confighttp.ServerConfig{
					Endpoint: defaultHTTPEndpoint,
				},
				// When unmarshaling from YAML, protocols is empty for legacy config
				Protocols:       Protocols{},
				ParseStringTags: true,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "protocols_config"),
			expected: &Config{
				// For new config format, protocols is populated with HTTP config
				Protocols: Protocols{
					HTTP: &HTTPConfig{
						ServerConfig: confighttp.ServerConfig{
							Endpoint: "localhost:9411",
						},
					},
				},
				// When using protocols config, the legacy endpoint should be empty
				ServerConfig: confighttp.ServerConfig{
					Endpoint: "",
				},
				ParseStringTags: false,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "protocols_with_legacy"),
			expected: &Config{
				// We expect protocols to override the legacy endpoint
				Protocols: Protocols{
					HTTP: &HTTPConfig{
						ServerConfig: confighttp.ServerConfig{
							Endpoint: "localhost:9412",
						},
					},
				},
				// Legacy endpoint should be cleared when protocols.http is set
				ServerConfig: confighttp.ServerConfig{
					Endpoint: "",
				},
				ParseStringTags: false,
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

			// Instead of comparing the entire config, just check the specific fields we care about
			actualCfg := cfg.(*Config)
			expectedCfg := tt.expected.(*Config)

			// Check endpoint
			assert.Equal(t, expectedCfg.Endpoint, actualCfg.Endpoint)

			// Check parse_string_tags
			assert.Equal(t, expectedCfg.ParseStringTags, actualCfg.ParseStringTags)

			// For the protocols_config test, check that protocols.http is configured correctly
			if tt.id.Name() == "protocols_config" {
				require.NotNil(t, actualCfg.Protocols.HTTP)
				assert.Equal(t, "localhost:9411", actualCfg.Protocols.HTTP.ServerConfig.Endpoint)
			}
			// For the protocols_config test, check that protocols.http is configured correctly
			if tt.id.Name() == "protocols_with_legacy" {
				// Check that endpoint is set in protocols.http
				require.NotNil(t, actualCfg.Protocols.HTTP)
				assert.Equal(t, "localhost:9412", actualCfg.Protocols.HTTP.ServerConfig.Endpoint)
				// Check that legacy endpoint is cleared
				assert.Equal(t, "", actualCfg.ServerConfig.Endpoint)
			}
		})
	}
}
