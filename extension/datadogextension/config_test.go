// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension"

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/extension"
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
						Endpoint: "http://localhost:8080",
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
						Endpoint: "http://localhost:8080",
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
						Endpoint: "http://localhost:8080",
					},
					Path: "/metadata",
				},
			},
			wantErr: datadogconfig.ErrUnsetAPIKey,
		},
		{
			name: "Invalid API key characters",
			config: Config{
				API: datadogconfig.APIConfig{
					Site: datadogconfig.DefaultSite,
					Key:  "1234567890abcdef1234567890abcdeg",
				},
				HTTPConfig: &httpserver.Config{
					ServerConfig: confighttp.ServerConfig{
						Endpoint: "http://localhost:8080",
					},
					Path: "/metadata",
				},
			},
			wantErr: fmt.Errorf("%w: invalid characters: %s", datadogconfig.ErrAPIKeyFormat, "g"),
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
			Logger: zap.NewNop(),
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

	ext, err := newExtension(context.Background(), cfg, set, hostProvider, uuidProvider)
	require.NoError(t, err)
	require.NotNil(t, ext)

	// Verify the extension has a serializer
	serializer := ext.GetSerializer()
	require.NotNil(t, serializer)

	// Start and stop the extension to test lifecycle
	err = ext.Start(context.Background(), nil)
	require.NoError(t, err)

	err = ext.Shutdown(context.Background())
	require.NoError(t, err)
}
