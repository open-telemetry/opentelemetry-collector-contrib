// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package targetallocator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/targetallocator"

import (
	"path/filepath"
	"testing"
	"time"

	promConfig "github.com/prometheus/common/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
)

func TestComponentConfigStruct(t *testing.T) {
	require.NoError(t, componenttest.CheckConfigStruct(Config{}))
}

func TestLoadTargetAllocatorConfig(t *testing.T) {
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
