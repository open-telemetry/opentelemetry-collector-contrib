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
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/grpc"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/http"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
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
				LegacyConfig: http.LegacyConfig{
					ServerConfig: confighttp.ServerConfig{
						Endpoint: testutil.EndpointForPort(defaultHTTPPort),
					},
					Path: "/",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "legacyconfig"),
			expected: &Config{
				LegacyConfig: http.LegacyConfig{
					ServerConfig: confighttp.ServerConfig{
						Endpoint: "localhost:13",
						TLS: &configtls.ServerConfig{
							Config: configtls.Config{
								CAFile:   "/path/to/ca",
								CertFile: "/path/to/cert",
								KeyFile:  "/path/to/key",
							},
						},
					},
					CheckCollectorPipeline: &http.CheckCollectorPipelineConfig{
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
			expectedErr: errHTTPEndpointRequired,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalidpath"),
			expectedErr: errInvalidPath,
		},
		{
			id: component.NewIDWithName(metadata.Type, "v2all"),
			expected: &Config{
				LegacyConfig: http.LegacyConfig{
					UseV2: true,
					ServerConfig: confighttp.ServerConfig{
						Endpoint: testutil.EndpointForPort(defaultHTTPPort),
					},
					Path: "/",
				},
				HTTPConfig: &http.Config{
					ServerConfig: confighttp.ServerConfig{
						Endpoint: testutil.EndpointForPort(defaultHTTPPort),
					},
					Status: http.PathConfig{
						Enabled: true,
						Path:    "/status",
					},
					Config: http.PathConfig{
						Enabled: false,
						Path:    "/config",
					},
				},
				GRPCConfig: &grpc.Config{
					ServerConfig: configgrpc.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Endpoint:  testutil.EndpointForPort(defaultGRPCPort),
							Transport: "tcp",
						},
					},
				},
				ComponentHealthConfig: &common.ComponentHealthConfig{
					IncludePermanent:   true,
					IncludeRecoverable: true,
					RecoveryDuration:   5 * time.Minute,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "v2httpcustomized"),
			expected: &Config{
				LegacyConfig: http.LegacyConfig{
					UseV2: true,
					ServerConfig: confighttp.ServerConfig{
						Endpoint: testutil.EndpointForPort(defaultHTTPPort),
					},
					Path: "/",
				},
				HTTPConfig: &http.Config{
					ServerConfig: confighttp.ServerConfig{
						Endpoint: "localhost:13",
					},
					Status: http.PathConfig{
						Enabled: true,
						Path:    "/health",
					},
					Config: http.PathConfig{
						Enabled: true,
						Path:    "/conf",
					},
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "v2httpmissingendpoint"),
			expectedErr: errHTTPEndpointRequired,
		},
		{
			id: component.NewIDWithName(metadata.Type, "v2grpccustomized"),
			expected: &Config{
				LegacyConfig: http.LegacyConfig{
					UseV2: true,
					ServerConfig: confighttp.ServerConfig{
						Endpoint: testutil.EndpointForPort(defaultHTTPPort),
					},
					Path: "/",
				},
				GRPCConfig: &grpc.Config{
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
			expectedErr: errGRPCEndpointRequired,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "v2noprotocols"),
			expectedErr: errMissingProtocol,
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
