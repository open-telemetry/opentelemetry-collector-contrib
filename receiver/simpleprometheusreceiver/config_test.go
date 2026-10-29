// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package simpleprometheusreceiver

import (
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	clientConfigPath := confighttp.NewDefaultClientConfig()
	clientConfigPath.Endpoint = "localhost:1234"
	clientConfigPath.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile:   "path",
			CertFile: "path",
			KeyFile:  "path",
		},
		InsecureSkipVerify: true,
	}

	clientConfigTLS := confighttp.NewDefaultClientConfig()
	clientConfigTLS.Endpoint = "localhost:1234"
	clientConfigTLS.TLS = configtls.ClientConfig{
		Insecure: true,
	}

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "localhost:1234"

	clientConfigREADME := confighttp.NewDefaultClientConfig()
	clientConfigREADME.Endpoint = "172.17.0.5:9153"
	clientConfigREADME.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile:   "/path/to/ca",
			CertFile: "/path/to/cert",
			KeyFile:  "/path/to/key",
		},
		Insecure: true,
	}

	clientConfigHeadersProxy := confighttp.NewDefaultClientConfig()
	clientConfigHeadersProxy.Endpoint = "localhost:1234"
	clientConfigHeadersProxy.TLS = configtls.ClientConfig{
		Insecure: true,
	}
	clientConfigHeadersProxy.Headers = configopaque.MapList{{Name: "X-Scope-OrgID", Value: "tenant-a"}}
	clientConfigHeadersProxy.ProxyURL = "http://proxy.local:3128"

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "all_settings"),
			expected: &Config{
				ClientConfig:       clientConfigPath,
				CollectionInterval: 30 * time.Second,
				MetricsPath:        "/v2/metrics",
				JobName:            "job123",
				Labels:             map[string]string{"env": "prod"},
				Params:             url.Values{"columns": []string{"name", "messages"}, "key": []string{"foo", "bar"}},
				UseServiceAccount:  true,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "partial_settings"),
			expected: &Config{
				ClientConfig:       clientConfigTLS,
				CollectionInterval: 30 * time.Second,
				MetricsPath:        "/metrics",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "partial_tls_settings"),
			expected: &Config{
				ClientConfig:       clientConfig,
				CollectionInterval: 30 * time.Second,
				MetricsPath:        "/metrics",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "readme_example"),
			expected: &Config{
				ClientConfig:       clientConfigREADME,
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
				UseServiceAccount:  true,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "headers_and_proxy"),
			expected: &Config{
				ClientConfig:       clientConfigHeadersProxy,
				CollectionInterval: 30 * time.Second,
				MetricsPath:        "/metrics",
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

			assert.NoError(t, confmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name:   "default config is valid",
			mutate: func(*Config) {},
		},
		{
			name:    "missing endpoint",
			mutate:  func(cfg *Config) { cfg.ClientConfig.Endpoint = "" },
			wantErr: "endpoint must be specified",
		},
		{
			name:    "zero collection_interval",
			mutate:  func(cfg *Config) { cfg.CollectionInterval = 0 },
			wantErr: "collection_interval must be positive",
		},
		{
			name:    "negative collection_interval",
			mutate:  func(cfg *Config) { cfg.CollectionInterval = -5 * time.Second },
			wantErr: "collection_interval must be positive",
		},
		{
			name: "non-reserved header",
			mutate: func(cfg *Config) {
				cfg.ClientConfig.Headers = configopaque.MapList{{Name: "X-Scope-OrgID", Value: "tenant-a"}}
			},
		},
		{
			name: "reserved header",
			mutate: func(cfg *Config) {
				cfg.ClientConfig.Headers = configopaque.MapList{{Name: "Authorization", Value: "Bearer token"}}
			},
			wantErr: `headers: header "Authorization" is reserved and cannot be set`,
		},
		{
			name: "reserved header in lowercase",
			mutate: func(cfg *Config) {
				cfg.ClientConfig.Headers = configopaque.MapList{{Name: "user-agent", Value: "custom"}}
			},
			wantErr: `headers: header "user-agent" is reserved and cannot be set`,
		},
		{
			name:   "proxy_url without credentials",
			mutate: func(cfg *Config) { cfg.ClientConfig.ProxyURL = "http://proxy.local:3128" },
		},
		{
			name:    "unparsable proxy_url",
			mutate:  func(cfg *Config) { cfg.ClientConfig.ProxyURL = "proxy.local" },
			wantErr: "proxy_url is not valid",
		},
		{
			name:    "proxy_url with credentials",
			mutate:  func(cfg *Config) { cfg.ClientConfig.ProxyURL = "http://user:pass@proxy.local:3128" },
			wantErr: "proxy_url must not contain credentials; see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/52081",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			tt.mutate(cfg)
			err := cfg.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
