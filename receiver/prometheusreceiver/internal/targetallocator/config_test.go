// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package targetallocator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/targetallocator"

import (
	"path/filepath"
	"testing"
	"time"

	promConfig "github.com/prometheus/common/config"
	promHTTP "github.com/prometheus/prometheus/discovery/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
)

func TestComponentConfigStruct(t *testing.T) {
	require.NoError(t, componenttest.CheckConfigStruct(Config{}))
}

func TestLoadTargetAllocatorConfig(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
		require.NoError(t, err)
		cfg := &Config{}

		sub, err := cm.Sub("target_allocator")
		require.NoError(t, err)
		require.NoError(t, sub.Unmarshal(cfg))
		require.NoError(t, xconfmap.Validate(cfg))

		assert.Equal(t, "http://localhost:8080", cfg.Endpoint)
		assert.Equal(t, 5*time.Second, cfg.Timeout)
		assert.Equal(t, "client.crt", cfg.TLS.CertFile)
		assert.Equal(t, 30*time.Second, cfg.Interval)
		assert.Equal(t, "collector-1", cfg.CollectorID)
	})

	t.Run("special characters in password", func(t *testing.T) {
		cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_with_special_chars.yaml"))
		require.NoError(t, err)

		var cfg Config
		sub, err := cm.Sub("target_allocator")
		require.NoError(t, err)
		require.NoError(t, sub.Unmarshal(&cfg))

		require.NotNil(t, cfg.HTTPSDConfig, "http_sd_config should be present")
		require.NotNil(t, cfg.HTTPScrapeConfig, "http_scrape_config should be present")
		require.NotNil(t, cfg.HTTPScrapeConfig.BasicAuth, "basic_auth should be present")
		assert.Equal(t, "testuser", cfg.HTTPScrapeConfig.BasicAuth.Username)
		assert.Equal(t, "%password-with-percent", string(cfg.HTTPScrapeConfig.BasicAuth.Password),
			"password with special YAML characters should be preserved")
	})
}

func TestPromHTTPClientConfigValidateAuthorization(t *testing.T) {
	cfg := PromHTTPClientConfig{}
	require.NoError(t, xconfmap.Validate(cfg))
	cfg.Authorization = &promConfig.Authorization{}
	require.NoError(t, xconfmap.Validate(cfg))
	cfg.Authorization.CredentialsFile = "none"
	require.Error(t, xconfmap.Validate(cfg))
	cfg.Authorization.CredentialsFile = filepath.Join("testdata", "dummy-tls-cert-file")
	require.NoError(t, xconfmap.Validate(cfg))
}

func TestPromHTTPClientConfigValidateTLSConfig(t *testing.T) {
	cfg := PromHTTPClientConfig{}
	require.NoError(t, xconfmap.Validate(cfg))
	cfg.TLSConfig.CertFile = "none"
	require.Error(t, xconfmap.Validate(cfg))
	cfg.TLSConfig.CertFile = filepath.Join("testdata", "dummy-tls-cert-file")
	cfg.TLSConfig.KeyFile = "none"
	require.Error(t, xconfmap.Validate(cfg))
	cfg.TLSConfig.KeyFile = filepath.Join("testdata", "dummy-tls-key-file")
	require.NoError(t, xconfmap.Validate(cfg))
}

func TestPromHTTPClientConfigValidateMain(t *testing.T) {
	cfg := PromHTTPClientConfig{}
	require.NoError(t, xconfmap.Validate(cfg))
	cfg.BearerToken = "foo"
	cfg.BearerTokenFile = filepath.Join("testdata", "dummy-tls-key-file")
	require.Error(t, xconfmap.Validate(cfg))
}

func TestConfigValidate_InvalidEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    string
		collectorID string
		expectError bool
	}{
		{
			name:        "valid config",
			endpoint:    "http://localhost:8080",
			collectorID: "collector-1",
			expectError: false,
		},
		{
			name:        "invalid endpoint - malformed",
			endpoint:    "://invalid",
			collectorID: "collector-1",
			expectError: true,
		},
		{
			name:        "invalid endpoint - empty",
			endpoint:    "",
			collectorID: "collector-1",
			expectError: true,
		},
		{
			name:        "invalid collectorID - empty",
			endpoint:    "http://localhost:8080",
			collectorID: "",
			expectError: true,
		},
		{
			name:        "invalid collectorID - contains variable",
			endpoint:    "http://localhost:8080",
			collectorID: "${POD_NAME}",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				CollectorID: tt.collectorID,
			}
			cfg.Endpoint = tt.endpoint
			err := xconfmap.Validate(cfg)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConvertTLSVersion(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		expectError bool
	}{
		{
			name:        "TLS 1.2",
			version:     "1.2",
			expectError: false,
		},
		{
			name:        "TLS 1.3",
			version:     "1.3",
			expectError: false,
		},
		{
			name:        "TLS 1.0",
			version:     "1.0",
			expectError: false,
		},
		{
			name:        "TLS 1.1",
			version:     "1.1",
			expectError: false,
		},
		{
			name:        "invalid version",
			version:     "2.0",
			expectError: true,
		},
		{
			name:        "invalid format",
			version:     "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := convertTLSVersion(tt.version)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckTLSConfig(t *testing.T) {
	tests := []struct {
		name        string
		certFile    string
		keyFile     string
		expectError bool
	}{
		{
			name:        "empty config",
			certFile:    "",
			keyFile:     "",
			expectError: false,
		},
		{
			name:        "valid cert and key",
			certFile:    filepath.Join("testdata", "dummy-tls-cert-file"),
			keyFile:     filepath.Join("testdata", "dummy-tls-key-file"),
			expectError: false,
		},
		{
			name:        "invalid cert file",
			certFile:    "nonexistent-cert.pem",
			keyFile:     filepath.Join("testdata", "dummy-tls-key-file"),
			expectError: true,
		},
		{
			name:        "invalid key file",
			certFile:    filepath.Join("testdata", "dummy-tls-cert-file"),
			keyFile:     "nonexistent-key.pem",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlsConfig := promConfig.TLSConfig{
				CertFile: tt.certFile,
				KeyFile:  tt.keyFile,
			}
			cfg := &Config{
				CollectorID: "collector-1",
				HTTPScrapeConfig: &PromHTTPClientConfig{
					TLSConfig: tlsConfig,
				},
			}
			cfg.Endpoint = "http://localhost:8080"
			err := xconfmap.Validate(cfg)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPromHTTPSDConfigUnmarshal(t *testing.T) {
	tests := []struct {
		name          string
		input         map[string]any
		errorContains string
		validate      func(*testing.T, *PromHTTPSDConfig)
	}{
		{
			name:     "empty config",
			input:    map[string]any{},
			validate: func(t *testing.T, cfg *PromHTTPSDConfig) { assert.NotNil(t, cfg) },
		},
		{
			name: "valid config with refresh interval",
			input: map[string]any{
				"refresh_interval": "30s",
			},
			validate: func(t *testing.T, cfg *PromHTTPSDConfig) { assert.NotNil(t, cfg) },
		},
		{
			name: "valid config with multiple fields",
			input: map[string]any{
				"refresh_interval": "1m",
				"proxy_url":        "http://proxy.example.com:8080",
			},
			validate: func(t *testing.T, cfg *PromHTTPSDConfig) { assert.NotNil(t, cfg) },
		},
		{
			name: "invalid config with bad refresh interval",
			input: map[string]any{
				"refresh_interval": "invalid",
			},
			errorContains: "not a valid duration string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &PromHTTPSDConfig{}
			conf := confmap.NewFromStringMap(tt.input)
			err := cfg.Unmarshal(conf)
			if tt.errorContains != "" {
				assert.ErrorContains(t, err, tt.errorContains)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, cfg)
				}
			}
		})
	}
}

func TestPromHTTPClientConfigUnmarshal(t *testing.T) {
	tests := []struct {
		name          string
		input         map[string]any
		errorContains string
		validate      func(*testing.T, *PromHTTPClientConfig)
	}{
		{
			name:     "empty config",
			input:    map[string]any{},
			validate: func(t *testing.T, cfg *PromHTTPClientConfig) { assert.NotNil(t, cfg) },
		},
		{
			name: "valid config with bearer token",
			input: map[string]any{
				"bearer_token": "test-token",
			},
			validate: func(t *testing.T, cfg *PromHTTPClientConfig) {
				assert.NotNil(t, cfg)
				// Note: bearer_token gets unmarshaled correctly but Secret type
				// redaction makes it hard to test the exact value
			},
		},
		{
			name: "valid config with TLS",
			input: map[string]any{
				"tls_config": map[string]any{
					"insecure_skip_verify": true,
				},
			},
			validate: func(t *testing.T, cfg *PromHTTPClientConfig) {
				assert.NotNil(t, cfg)
				assert.True(t, cfg.TLSConfig.InsecureSkipVerify)
			},
		},
		{
			name: "valid config with authorization",
			input: map[string]any{
				"authorization": map[string]any{
					"type": "Bearer",
				},
			},
			validate: func(t *testing.T, cfg *PromHTTPClientConfig) {
				assert.NotNil(t, cfg)
				assert.NotNil(t, cfg.Authorization)
			},
		},
		{
			name: "valid config with proxy URL",
			input: map[string]any{
				"proxy_url": "http://proxy.example.com:8080",
			},
			validate: func(t *testing.T, cfg *PromHTTPClientConfig) { assert.NotNil(t, cfg) },
		},
		{
			name: "valid config with follow redirects",
			input: map[string]any{
				"follow_redirects": true,
			},
			validate: func(t *testing.T, cfg *PromHTTPClientConfig) {
				assert.NotNil(t, cfg)
				assert.True(t, cfg.FollowRedirects)
			},
		},
		{
			name: "valid config with enable http2",
			input: map[string]any{
				"enable_http2": false,
			},
			validate: func(t *testing.T, cfg *PromHTTPClientConfig) { assert.NotNil(t, cfg) },
		},
		{
			name: "basic auth password starting with percent sign",
			input: map[string]any{
				"basic_auth": map[string]any{
					"username": "user",
					"password": "%password-with-percent",
				},
			},
			validate: func(t *testing.T, cfg *PromHTTPClientConfig) {
				assert.NotNil(t, cfg)
				assert.NotNil(t, cfg.BasicAuth)
				assert.Equal(t, "user", cfg.BasicAuth.Username)
				assert.Equal(t, "%password-with-percent", string(cfg.BasicAuth.Password))
			},
		},
		{
			name: "invalid config with bad tls version",
			input: map[string]any{
				"tls_config": map[string]any{
					"min_version": "TLS99",
				},
			},
			errorContains: "unknown TLS version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &PromHTTPClientConfig{}
			conf := confmap.NewFromStringMap(tt.input)
			err := cfg.Unmarshal(conf)
			if tt.errorContains != "" {
				assert.ErrorContains(t, err, tt.errorContains)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, cfg)
				}
			}
		})
	}
}

func TestConfigureSDHTTPClientConfigFromTA_Errors(t *testing.T) {
	tests := []struct {
		name          string
		setupConfig   func() *Config
		errorContains string
	}{
		{
			name: "invalid base64 in CAPem",
			setupConfig: func() *Config {
				cfg := &Config{}
				cfg.TLS.CAPem = configopaque.String("not-valid-base64!@#$%")
				return cfg
			},
			errorContains: "failed to decode CA",
		},
		{
			name: "invalid base64 in CertPem",
			setupConfig: func() *Config {
				cfg := &Config{}
				cfg.TLS.CertPem = configopaque.String("not-valid-base64!@#$%")
				return cfg
			},
			errorContains: "failed to decode Cert",
		},
		{
			name: "invalid base64 in KeyPem",
			setupConfig: func() *Config {
				cfg := &Config{}
				cfg.TLS.KeyPem = configopaque.String("not-valid-base64!@#$%")
				return cfg
			},
			errorContains: "failed to decode Key",
		},
		{
			name: "invalid TLS MinVersion",
			setupConfig: func() *Config {
				cfg := &Config{}
				cfg.TLS.MinVersion = "99.9"
				return cfg
			},
			errorContains: "unsupported TLS version",
		},
		{
			name: "invalid TLS MaxVersion",
			setupConfig: func() *Config {
				cfg := &Config{}
				cfg.TLS.MaxVersion = "invalid-version"
				return cfg
			},
			errorContains: "unsupported TLS version",
		},
		{
			name: "invalid ProxyURL",
			setupConfig: func() *Config {
				cfg := &Config{}
				cfg.ProxyURL = "://invalid-url-scheme"
				return cfg
			},
			errorContains: "missing protocol scheme",
		},
		{
			name: "valid config - no errors",
			setupConfig: func() *Config {
				cfg := &Config{}
				cfg.TLS = configtls.ClientConfig{
					InsecureSkipVerify: true,
					ServerName:         "test.example.com",
				}
				return cfg
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allocConf := tt.setupConfig()
			httpSD := &promHTTP.SDConfig{}

			err := configureSDHTTPClientConfigFromTA(httpSD, allocConf)

			if tt.errorContains != "" {
				assert.ErrorContains(t, err, tt.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUnmarshalConf(t *testing.T) {
	t.Run("empty config", func(t *testing.T) {
		var cfg promConfig.HTTPClientConfig
		err := unmarshalConf(confmap.NewFromStringMap(map[string]any{}), nil, &cfg)
		require.NoError(t, err)
		assert.Zero(t, cfg)
	})

	t.Run("special YAML characters preserved", func(t *testing.T) {
		var cfg promConfig.HTTPClientConfig
		input := map[string]any{
			"basic_auth": map[string]any{
				"username": "user",
				"password": "%password-with-percent",
			},
		}
		err := unmarshalConf(confmap.NewFromStringMap(input), nil, &cfg)
		require.NoError(t, err)
		require.NotNil(t, cfg.BasicAuth)
		assert.Equal(t, "%password-with-percent", string(cfg.BasicAuth.Password))
	})

	t.Run("callback mutates config", func(t *testing.T) {
		var cfg promConfig.HTTPClientConfig
		input := map[string]any{
			"basic_auth": map[string]any{
				"username": "original",
			},
		}
		cb := func(m map[string]any) {
			m["basic_auth"].(map[string]any)["username"] = "mutated"
		}
		require.NoError(t, unmarshalConf(confmap.NewFromStringMap(input), cb, &cfg))
		require.NotNil(t, cfg.BasicAuth)
		assert.Equal(t, "mutated", cfg.BasicAuth.Username)
	})

	t.Run("marshal error", func(t *testing.T) {
		var cfg promConfig.HTTPClientConfig
		input := map[string]any{
			"invalid": make(chan int), // channels can't be marshaled to YAML
		}
		err := unmarshalConf(confmap.NewFromStringMap(input), nil, &cfg)
		require.ErrorContains(t, err, "failed to marshal")
	})
}
