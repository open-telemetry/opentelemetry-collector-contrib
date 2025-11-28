// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yanggrpcreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
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
					MaxConnections:    1000,
					ConnectionTimeout: 30 * time.Second,
				},
				MaxConcurrentStreams: 100,
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
				MaxConcurrentStreams: 100,
			},
			expectError: true,
			errorMsg:    "requests_per_second must be positive",
		},
		{
			name: "Invalid max connections",
			config: &Config{
				Security: SecurityConfig{
					MaxConnections: -1,
				},
				MaxConcurrentStreams: 100,
			},
			expectError: true,
			errorMsg:    "max_connections must be non-negative",
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

func TestSecurityManager_IPAllowlist(t *testing.T) {
	securityConfig := &SecurityConfig{
		AllowedClients: []string{"192.168.1.1", "10.0.0.0/8"},
	}
	tlsConfig := &TLSConfig{}
	sm := NewSecurityManager(securityConfig, tlsConfig, zap.NewNop())

	tests := []struct {
		clientIP string
		allowed  bool
	}{
		{"192.168.1.1", true},    // Exact match
		{"192.168.1.2", false},   // Not in list
		{"10.0.0.1", true},       // In CIDR range
		{"10.255.255.255", true}, // In CIDR range
		{"11.0.0.1", false},      // Not in CIDR range
	}

	for _, tt := range tests {
		t.Run(tt.clientIP, func(t *testing.T) {
			allowed := sm.isIPAllowed(tt.clientIP)
			assert.Equal(t, tt.allowed, allowed)
		})
	}
}
