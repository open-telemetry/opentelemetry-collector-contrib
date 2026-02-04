// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension"

import (
	"errors"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/httpserver"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr error
	}{
		{
			name: "Valid configuration",
			config: Config{
				API: datadogconfig.APIConfig{
					Site: datadogconfig.DefaultSite,
					Key:  "1234567890abcdef1234567890abcdef",
				},
				HTTPConfig: &httpserver.Config{
					ServerConfig: confighttp.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Transport: confignet.TransportTypeTCP,
							Endpoint:  "localhost:8080",
						},
					},
					Path: "/metadata",
				},
			},
			wantErr: nil,
		},
		{
			name: "Empty site",
			config: Config{
				API: datadogconfig.APIConfig{
					Site: "",
					Key:  "1234567890abcdef1234567890abcdef",
				},
				HTTPConfig: &httpserver.Config{
					ServerConfig: confighttp.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Transport: confignet.TransportTypeTCP,
							Endpoint:  "localhost:8080",
						},
					},
					Path: "/metadata",
				},
			},
			wantErr: datadogconfig.ErrEmptyEndpoint,
		},
		{
			name: "Unset API key",
			config: Config{
				API: datadogconfig.APIConfig{
					Site: datadogconfig.DefaultSite,
					Key:  "",
				},
				HTTPConfig: &httpserver.Config{
					ServerConfig: confighttp.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Transport: confignet.TransportTypeTCP,
							Endpoint:  "localhost:8080",
						},
					},
					Path: "/metadata",
				},
			},
			wantErr: datadogconfig.ErrUnsetAPIKey,
		},
		{
			name: "Missing HTTP config",
			config: Config{
				API: datadogconfig.APIConfig{
					Site: datadogconfig.DefaultSite,
					Key:  "1234567890abcdef1234567890abcdef",
				},
			},
			wantErr: errors.New("http config is required"),
		},
		{
			name: "Valid configuration with gateway deployment type",
			config: Config{
				API: datadogconfig.APIConfig{
					Site: datadogconfig.DefaultSite,
					Key:  "1234567890abcdef1234567890abcdef",
				},
				HTTPConfig: &httpserver.Config{
					ServerConfig: confighttp.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Transport: confignet.TransportTypeTCP,
							Endpoint:  "localhost:8080",
						},
					},
					Path: "/metadata",
				},
				DeploymentType: "gateway",
			},
			wantErr: nil,
		},
		{
			name: "Valid configuration with daemonset deployment type",
			config: Config{
				API: datadogconfig.APIConfig{
					Site: datadogconfig.DefaultSite,
					Key:  "1234567890abcdef1234567890abcdef",
				},
				HTTPConfig: &httpserver.Config{
					ServerConfig: confighttp.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Transport: confignet.TransportTypeTCP,
							Endpoint:  "localhost:8080",
						},
					},
					Path: "/metadata",
				},
				DeploymentType: "daemonset",
			},
			wantErr: nil,
		},
		{
			name: "Valid configuration with unknown deployment type",
			config: Config{
				API: datadogconfig.APIConfig{
					Site: datadogconfig.DefaultSite,
					Key:  "1234567890abcdef1234567890abcdef",
				},
				HTTPConfig: &httpserver.Config{
					ServerConfig: confighttp.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Transport: confignet.TransportTypeTCP,
							Endpoint:  "localhost:8080",
						},
					},
					Path: "/metadata",
				},
				DeploymentType: "unknown",
			},
			wantErr: nil,
		},
		{
			name: "Invalid deployment type",
			config: Config{
				API: datadogconfig.APIConfig{
					Site: datadogconfig.DefaultSite,
					Key:  "1234567890abcdef1234567890abcdef",
				},
				HTTPConfig: &httpserver.Config{
					ServerConfig: confighttp.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Transport: confignet.TransportTypeTCP,
							Endpoint:  "localhost:8080",
						},
					},
					Path: "/metadata",
				},
				DeploymentType: "invalid-mode",
			},
			wantErr: errors.New("deployment_type must be one of: gateway, daemonset, or unknown"),
		},
		{
			name: "Empty deployment type defaults to unknown",
			config: Config{
				API: datadogconfig.APIConfig{
					Site: datadogconfig.DefaultSite,
					Key:  "1234567890abcdef1234567890abcdef",
				},
				HTTPConfig: &httpserver.Config{
					ServerConfig: confighttp.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Transport: confignet.TransportTypeTCP,
							Endpoint:  "localhost:8080",
						},
					},
					Path: "/metadata",
				},
				DeploymentType: "",
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.wantErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExtensionWithProxyConfig(t *testing.T) {
	// Create config with proxy settings
	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			ProxyURL: "http://proxy.example.com:8080",
			Timeout:  30 * time.Second,
			TLS: configtls.ClientConfig{
				InsecureSkipVerify: true,
			},
		},
		API: datadogconfig.APIConfig{
			Key:              "test-api-key-12345",
			Site:             "datadoghq.com",
			FailOnInvalidKey: true,
		},
		Hostname: "test-host",
	}

	// Create extension with proxy config
	set := extension.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger:   zap.NewNop(),
			Resource: pcommon.NewResource(),
		},
	}
	hostProvider := &mockSourceProvider{
		source: source.Source{
			Kind:       source.HostnameKind,
			Identifier: "test-host",
		},
	}
	uuidProvider := &mockUUIDProvider{
		mockUUID: "test-uuid",
	}

	ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
	require.NoError(t, err)
	require.NotNil(t, ext)

	// Verify the extension has a serializer
	serializer := ext.GetSerializer()
	require.NotNil(t, serializer)

	// Start and stop the extension to test lifecycle
	err = ext.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	err = ext.Shutdown(t.Context())
	require.NoError(t, err)
}

func TestConfig_DeploymentTypeDefault(t *testing.T) {
	cfg := Config{
		API: datadogconfig.APIConfig{
			Site: datadogconfig.DefaultSite,
			Key:  "1234567890abcdef1234567890abcdef",
		},
		HTTPConfig: &httpserver.Config{
			ServerConfig: confighttp.ServerConfig{
				NetAddr: confignet.AddrConfig{
					Transport: confignet.TransportTypeTCP,
					Endpoint:  "localhost:8080",
				},
			},
			Path: "/metadata",
		},
		DeploymentType: "",
	}

	err := cfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, "unknown", cfg.DeploymentType, "DeploymentType should default to 'unknown'")
}

func TestConfig_DeploymentTypeValidValues(t *testing.T) {
	tests := []struct {
		name           string
		deploymentType string
		expectError    bool
	}{
		{
			name:           "gateway is valid",
			deploymentType: "gateway",
			expectError:    false,
		},
		{
			name:           "daemonset is valid",
			deploymentType: "daemonset",
			expectError:    false,
		},
		{
			name:           "unknown is valid",
			deploymentType: "unknown",
			expectError:    false,
		},
		{
			name:           "invalid value fails",
			deploymentType: "invalid",
			expectError:    true,
		},
		{
			name:           "sidecar is invalid",
			deploymentType: "sidecar",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				API: datadogconfig.APIConfig{
					Site: datadogconfig.DefaultSite,
					Key:  "1234567890abcdef1234567890abcdef",
				},
				HTTPConfig: &httpserver.Config{
					ServerConfig: confighttp.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Transport: confignet.TransportTypeTCP,
							Endpoint:  "localhost:8080",
						},
					},
					Path: "/metadata",
				},
				DeploymentType: tt.deploymentType,
			}

			err := cfg.Validate()
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "deployment_type must be one of")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
