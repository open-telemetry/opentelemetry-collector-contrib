// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yanggrpcreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestSecurityConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid security config",
			config: &Config{
				Security: SecurityConfig{
					RateLimiting: RateLimitingConfig{
						Enabled:           true,
						RequestsPerSecond: 100.0,
						BurstSize:         10,
					},
					ConnectionTimeout: 30 * time.Second,
				},
			},
			expectError: false,
		},
		{
			name: "Invalid rate limiting - negative requests per second",
			config: &Config{
				Security: SecurityConfig{
					RateLimiting: RateLimitingConfig{
						Enabled:           true,
						RequestsPerSecond: -1.0,
						BurstSize:         10,
					},
				},
				ServerConfig: configgrpc.NewDefaultServerConfig(),
			},
			expectError: true,
			errorMsg:    "requests_per_second must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfigs(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	for _, test := range []struct {
		name string
	}{
		{
			"yanggrpc/production",
		},
		{
			"yanggrpc/default",
		},
	} {
		c, err := cm.Sub(test.name)
		require.NoError(t, err)
		var cfg Config
		require.NoError(t, c.Unmarshal(&cfg))
		require.NoError(t, cfg.Validate())
	}
}
