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

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr error
	}{
		{
			id: component.NewID(metadata.Type),
			expected: &Config{
				LegacyConfig: healthcheck.HTTPLegacyConfig{
					ServerConfig: confighttp.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Transport: "tcp",
							Endpoint:  testutil.EndpointForPort(healthcheck.DefaultHTTPPort),
						},
					},
					Path: "/",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "legacyconfig"),
			expected: &Config{
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
					UseV2: true,
					ServerConfig: confighttp.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Transport: "tcp",
							Endpoint:  testutil.EndpointForPort(healthcheck.DefaultHTTPPort),
						},
					},
					Path: "/",
				},
				HTTPConfig: &healthcheck.HTTPConfig{
					ServerConfig: confighttp.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Transport: "tcp",
							Endpoint:  testutil.EndpointForPort(healthcheck.DefaultHTTPPort),
						},
					},
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
					UseV2: true,
					ServerConfig: confighttp.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Transport: "tcp",
							Endpoint:  testutil.EndpointForPort(healthcheck.DefaultHTTPPort),
						},
					},
					Path: "/",
				},
				HTTPConfig: &healthcheck.HTTPConfig{
					ServerConfig: confighttp.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Transport: "tcp",
							Endpoint:  "localhost:13",
						},
					},
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
					UseV2: true,
					ServerConfig: confighttp.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Transport: "tcp",
							Endpoint:  testutil.EndpointForPort(healthcheck.DefaultHTTPPort),
						},
					},
					Path: "/",
				},
				GRPCConfig: &healthcheck.GRPCConfig{
					ServerConfig: configgrpc.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Endpoint:  "localhost:13",
							Transport: "tcp",
						},
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
