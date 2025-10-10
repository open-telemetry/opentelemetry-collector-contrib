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
				CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
				Path:                   "/",
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "missingendpoint"),
			expectedErr: errNoEndpointProvided,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalidthreshold"),
			expectedErr: errInvalidExporterFailureThresholdProvided,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalidpath"),
			expectedErr: errInvalidPath,
		},
		{
			id: component.NewIDWithName(metadata.Type, "response-body"),
			expected: func() component.Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.ResponseBody = &ResponseBodySettings{
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

func TestLoadConfigV2RequiresGate(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	cfg := NewFactory().CreateDefaultConfig()
	sub, err := cm.Sub("health_check/v2-http-only")
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	assert.NotNil(t, cfg.(*Config).HTTP)
	assert.True(t, cfg.(*Config).hasV2Settings)
	assert.ErrorIs(t, cfg.(*Config).Validate(), errV2ConfigWithoutGate)

	assert.ErrorIs(t, xconfmap.Validate(cfg), errV2ConfigWithoutGate)
}

func TestLoadConfigV2WithGate(t *testing.T) {
	prev := disableCompatibilityWrapperGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(disableCompatibilityWrapperGate.ID(), true))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(disableCompatibilityWrapperGate.ID(), prev))
	})

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	cfg := NewFactory().CreateDefaultConfig().(*Config)
	sub, err := cm.Sub("health_check/v2-both-protocols")
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.NoError(t, xconfmap.Validate(cfg))
	assert.Equal(t, &Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: "localhost:13133",
		},
		Path: "/",
		CheckCollectorPipeline: checkCollectorPipelineSettings{
			Enabled:                  false,
			Interval:                 "5m",
			ExporterFailureThreshold: 5,
		},
		HTTP: &healthcheck.HTTPConfig{
			ServerConfig: confighttp.ServerConfig{
				Endpoint: "localhost:13133",
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
		GRPC: &healthcheck.GRPCConfig{
			ServerConfig: configgrpc.ServerConfig{
				NetAddr: confignet.AddrConfig{
					Endpoint:  "localhost:13132",
					Transport: confignet.TransportTypeTCP,
				},
			},
		},
		ComponentHealth: &healthcheck.ComponentHealthConfig{
			IncludePermanent:   true,
			IncludeRecoverable: true,
			RecoveryDuration:   time.Minute,
		},
		hasV2Settings: true,
	}, cfg)
}
