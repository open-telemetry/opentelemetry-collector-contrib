// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextension

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr error
	}{
		{
			id:       component.NewID(metadata.Type),
			expected: NewFactory().CreateDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "1"),
			expected: &healthcheck.Config{
				LegacyConfig: healthcheck.HTTPLegacyConfig{
					ServerConfig: confighttp.ServerConfig{
						Endpoint: "localhost:13",
						TLS: configoptional.Some(configtls.ServerConfig{
							Config: configtls.Config{
								CAFile:   "/path/to/ca",
								CertFile: "/path/to/cert",
								KeyFile:  "/path/to/key",
							},
						}),
					},
					Path: "/",
					CheckCollectorPipeline: &healthcheck.CheckCollectorPipelineConfig{
						Enabled:                  false,
						Interval:                 "5m",
						ExporterFailureThreshold: 5,
					},
					UseV2: false,
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "missingendpoint"),
			expectedErr: healthcheck.ErrHTTPEndpointRequired,
		},
		{
			id: component.NewIDWithName(metadata.Type, "invalidthreshold"),
			expected: &healthcheck.Config{
				LegacyConfig: healthcheck.HTTPLegacyConfig{
					ServerConfig: confighttp.ServerConfig{
						Endpoint: "localhost:13",
					},
					Path: "/",
					CheckCollectorPipeline: &healthcheck.CheckCollectorPipelineConfig{
						Enabled:                  false,
						Interval:                 "5m",
						ExporterFailureThreshold: -1,
					},
					UseV2: false,
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalidpath"),
			expectedErr: healthcheck.ErrInvalidPath,
		},
		{
			id: component.NewIDWithName(metadata.Type, "response-body"),
			expected: func() component.Config {
				cfg := NewFactory().CreateDefaultConfig().(*healthcheck.Config)
				cfg.ResponseBody = &healthcheck.ResponseBodyConfig{
					Healthy:   "I'm OK",
					Unhealthy: "I'm not well",
				}
				return cfg
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			if tt.expectedErr != nil {
				assert.ErrorIs(t, xconfmap.Validate(cfg), tt.expectedErr)
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
