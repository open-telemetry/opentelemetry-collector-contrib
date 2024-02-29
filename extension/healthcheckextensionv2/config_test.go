// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextensionv2 // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2"

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/grpc"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/http"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/metadata"
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
				LegacySettings: http.LegacySettings{
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: defaultHTTPEndpoint,
					},
					Path: "/",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "legacysettings"),
			expected: &Config{
				LegacySettings: http.LegacySettings{
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: "localhost:13",
						TLSSetting: &configtls.TLSServerSetting{
							TLSSetting: configtls.TLSSetting{
								CAFile:   "/path/to/ca",
								CertFile: "/path/to/cert",
								KeyFile:  "/path/to/key",
							},
						},
					},
					CheckCollectorPipeline: &http.CheckCollectorPipelineSettings{
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
				LegacySettings: http.LegacySettings{
					UseV2Settings: true,
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: defaultHTTPEndpoint,
					},
					Path: "/",
				},
				HTTPSettings: &http.Settings{
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: defaultHTTPEndpoint,
					},
					Status: http.PathSettings{
						Enabled: true,
						Path:    "/status",
					},
					Config: http.PathSettings{
						Enabled: false,
						Path:    "/config",
					},
				},
				GRPCSettings: &grpc.Settings{
					GRPCServerSettings: configgrpc.GRPCServerSettings{
						NetAddr: confignet.NetAddr{
							Endpoint:  defaultGRPCEndpoint,
							Transport: "tcp",
						},
					},
				},
				ComponentHealthSettings: &common.ComponentHealthSettings{
					IncludePermanent:   true,
					IncludeRecoverable: true,
					RecoveryDuration:   5 * time.Minute,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "v2httpcustomized"),
			expected: &Config{
				LegacySettings: http.LegacySettings{
					UseV2Settings: true,
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: defaultHTTPEndpoint,
					},
					Path: "/",
				},
				HTTPSettings: &http.Settings{
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: "localhost:13",
					},
					Status: http.PathSettings{
						Enabled: true,
						Path:    "/health",
					},
					Config: http.PathSettings{
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
				LegacySettings: http.LegacySettings{
					UseV2Settings: true,
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: defaultHTTPEndpoint,
					},
					Path: "/",
				},
				GRPCSettings: &grpc.Settings{
					GRPCServerSettings: configgrpc.GRPCServerSettings{
						NetAddr: confignet.NetAddr{
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
			require.NoError(t, component.UnmarshalConfig(sub, cfg))
			if tt.expectedErr != nil {
				assert.ErrorIs(t, component.ValidateConfig(cfg), tt.expectedErr)
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
