// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckv2extension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension"

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	defaultLegacyServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	defaultLegacyServerConfig.WriteTimeout = 0
	defaultLegacyServerConfig.ReadHeaderTimeout = 0
	defaultLegacyServerConfig.IdleTimeout = 0
	defaultLegacyServerConfig.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.EndpointForPort(healthcheck.DefaultHTTPPort),
	}
	defaultLegacyServerConfig.KeepAlivesEnabled = true

	legacyConfigServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	legacyConfigServerConfig.WriteTimeout = 0
	legacyConfigServerConfig.ReadHeaderTimeout = 0
	legacyConfigServerConfig.IdleTimeout = 0
	legacyConfigServerConfig.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  "localhost:13",
	}
	legacyConfigServerConfig.TLS = configoptional.Some(configtls.ServerConfig{
		Config: configtls.Config{
			CAFile:   "/path/to/ca",
			CertFile: "/path/to/cert",
			KeyFile:  "/path/to/key",
		},
	})
	legacyConfigServerConfig.KeepAlivesEnabled = true

	v2allLegacyServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	v2allLegacyServerConfig.WriteTimeout = 0
	v2allLegacyServerConfig.ReadHeaderTimeout = 0
	v2allLegacyServerConfig.IdleTimeout = 0
	v2allLegacyServerConfig.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.EndpointForPort(healthcheck.DefaultHTTPPort),
	}
	v2allLegacyServerConfig.KeepAlivesEnabled = true

	v2allHTTPServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	v2allHTTPServerConfig.WriteTimeout = 0
	v2allHTTPServerConfig.ReadHeaderTimeout = 0
	v2allHTTPServerConfig.IdleTimeout = 0
	v2allHTTPServerConfig.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.EndpointForPort(healthcheck.DefaultHTTPPort),
	}
	v2allHTTPServerConfig.KeepAlivesEnabled = true

	v2httpCustomizedLegacyServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	v2httpCustomizedLegacyServerConfig.WriteTimeout = 0
	v2httpCustomizedLegacyServerConfig.ReadHeaderTimeout = 0
	v2httpCustomizedLegacyServerConfig.IdleTimeout = 0
	v2httpCustomizedLegacyServerConfig.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.EndpointForPort(healthcheck.DefaultHTTPPort),
	}
	v2httpCustomizedLegacyServerConfig.KeepAlivesEnabled = true

	v2httpCustomizedHTTPServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	v2httpCustomizedHTTPServerConfig.WriteTimeout = 0
	v2httpCustomizedHTTPServerConfig.ReadHeaderTimeout = 0
	v2httpCustomizedHTTPServerConfig.IdleTimeout = 0
	v2httpCustomizedHTTPServerConfig.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  "localhost:13",
	}
	v2httpCustomizedHTTPServerConfig.KeepAlivesEnabled = true

	v2grpcCustomizedLegacyServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	v2grpcCustomizedLegacyServerConfig.WriteTimeout = 0
	v2grpcCustomizedLegacyServerConfig.ReadHeaderTimeout = 0
	v2grpcCustomizedLegacyServerConfig.IdleTimeout = 0
	v2grpcCustomizedLegacyServerConfig.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.EndpointForPort(healthcheck.DefaultHTTPPort),
	}
	v2grpcCustomizedLegacyServerConfig.KeepAlivesEnabled = true

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr error
	}{
		{
			id: component.NewID(metadata.Type),
			expected: &Config{
				LegacyConfig: healthcheck.HTTPLegacyConfig{
					ServerConfig: defaultLegacyServerConfig,
					Path:         "/",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "legacyconfig"),
			expected: &Config{
				LegacyConfig: healthcheck.HTTPLegacyConfig{
					ServerConfig: legacyConfigServerConfig,
					CheckCollectorPipeline: &healthcheck.CheckCollectorPipelineConfig{
						Enabled:                  false,
						Interval:                 "5m",
						ExporterFailureThreshold: 5,
					},
					Path:         "/",
					ResponseBody: nil,
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
			id: component.NewIDWithName(metadata.Type, "v2all"),
			expected: &Config{
				LegacyConfig: healthcheck.HTTPLegacyConfig{
					UseV2:        true,
					ServerConfig: v2allLegacyServerConfig,
					Path:         "/",
				},
				HTTPConfig: &healthcheck.HTTPConfig{
					ServerConfig: v2allHTTPServerConfig,
					Status: healthcheck.PathConfig{
						Enabled: true,
						Path:    "/status",
					},
					Config: healthcheck.PathConfig{
						Enabled: false,
						Path:    "/config",
					},
				},
				GRPCConfig: &healthcheck.GRPCConfig{
					ServerConfig: configgrpc.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Endpoint:  testutil.EndpointForPort(healthcheck.DefaultGRPCPort),
							Transport: "tcp",
						},
						Keepalive: configoptional.Some(configgrpc.NewDefaultKeepaliveServerConfig()),
					},
				},
				ComponentHealthConfig: &healthcheck.ComponentHealthConfig{
					IncludePermanent:   true,
					IncludeRecoverable: true,
					RecoveryDuration:   5 * time.Minute,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "v2httpcustomized"),
			expected: &Config{
				LegacyConfig: healthcheck.HTTPLegacyConfig{
					UseV2:        true,
					ServerConfig: v2httpCustomizedLegacyServerConfig,
					Path:         "/",
				},
				HTTPConfig: &healthcheck.HTTPConfig{
					ServerConfig: v2httpCustomizedHTTPServerConfig,
					Status: healthcheck.PathConfig{
						Enabled: true,
						Path:    "/health",
					},
					Config: healthcheck.PathConfig{
						Enabled: true,
						Path:    "/conf",
					},
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "v2httpmissingendpoint"),
			expectedErr: healthcheck.ErrHTTPEndpointRequired,
		},
		{
			id: component.NewIDWithName(metadata.Type, "v2grpccustomized"),
			expected: &Config{
				LegacyConfig: healthcheck.HTTPLegacyConfig{
					UseV2:        true,
					ServerConfig: v2grpcCustomizedLegacyServerConfig,
					Path:         "/",
				},
				GRPCConfig: &healthcheck.GRPCConfig{
					ServerConfig: configgrpc.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Endpoint:  "localhost:13",
							Transport: "tcp",
						},
						Keepalive: configoptional.Some(configgrpc.NewDefaultKeepaliveServerConfig()),
					},
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "v2grpcmissingendpoint"),
			expectedErr: healthcheck.ErrGRPCEndpointRequired,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "v2noprotocols"),
			expectedErr: healthcheck.ErrMissingProtocol,
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("../../internal/healthcheck/testdata", "config.yaml"))
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
