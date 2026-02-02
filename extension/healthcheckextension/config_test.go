// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextension

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck"
)

func TestLoadConfigLegacy(t *testing.T) {
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
			expected: &Config{
				Config: healthcheck.Config{
					LegacyConfig: healthcheck.HTTPLegacyConfig{
						ServerConfig: confighttp.ServerConfig{
							NetAddr: confignet.AddrConfig{
								Transport: "tcp",
								Endpoint:  "localhost:13",
							},
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
					},
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "missingendpoint"),
			expectedErr: healthcheck.ErrHTTPEndpointRequired,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalidpath"),
			expectedErr: healthcheck.ErrInvalidPath,
		},
		{
			id: component.NewIDWithName(metadata.Type, "response-body"),
			expected: func() component.Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
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
			cfg := NewFactory().CreateDefaultConfig()
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

func TestLoadConfigV2WithoutGate(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	cfg := NewFactory().CreateDefaultConfig()
	sub, err := cm.Sub("health_check/v2-http-only")
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	assert.NotNil(t, cfg.(*Config).HTTPConfig)

	// Without the feature gate, v2 config should cause a validation error.
	// This makes it immediately obvious to users that the configuration is invalid.
	err = xconfmap.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "v2 healthcheck configuration")
	assert.Contains(t, err.Error(), "feature gate is disabled")
}

func TestLoadConfigV2WithGate(t *testing.T) {
	prev := useComponentStatusGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(useComponentStatusGate.ID(), true))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(useComponentStatusGate.ID(), prev))
	})

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	cfg := NewFactory().CreateDefaultConfig().(*Config)
	sub, err := cm.Sub("health_check/v2-both-protocols")
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.NoError(t, xconfmap.Validate(cfg))
	assert.Equal(t, &Config{
		Config: healthcheck.Config{
			LegacyConfig: healthcheck.HTTPLegacyConfig{
				ServerConfig: confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Transport: "tcp",
						Endpoint:  "localhost:13133",
					},
				},
				Path: "/",
				CheckCollectorPipeline: &healthcheck.CheckCollectorPipelineConfig{
					Enabled:                  false,
					Interval:                 "5m",
					ExporterFailureThreshold: 5,
				},
			},
			HTTPConfig: &healthcheck.HTTPConfig{
				ServerConfig: confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Transport: "tcp",
						Endpoint:  "localhost:13133",
					},
				},
				Status: healthcheck.PathConfig{
					Enabled: true,
					Path:    "/status",
				},
				Config: healthcheck.PathConfig{
					Enabled: true,
					Path:    "/config",
				},
			},
			GRPCConfig: &healthcheck.GRPCConfig{
				ServerConfig: configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "localhost:13132",
						Transport: confignet.TransportTypeTCP,
					},
				},
			},
			ComponentHealthConfig: &healthcheck.ComponentHealthConfig{
				IncludePermanent:   true,
				IncludeRecoverable: true,
				RecoveryDuration:   time.Minute,
			},
		},
	}, cfg)
}
