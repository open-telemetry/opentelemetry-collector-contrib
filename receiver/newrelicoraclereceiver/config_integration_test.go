// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicoraclereceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigOracleCompatibility(t *testing.T) {
	// Test that our config follows Oracle database connection best practices
	tests := []struct {
		name   string
		config Config
		valid  bool
	}{
		{
			name: "standard Oracle configuration",
			config: Config{
				Endpoint:              "localhost:1521",
				Username:              "user",
				Password:              "password",
				Service:               "XE",
				MaxOpenConnections:    5,     // Standard default
				DisableConnectionPool: false, // Connection pooling enabled
			},
			valid: true,
		},
		{
			name: "custom Oracle configuration",
			config: Config{
				Endpoint:              "localhost:1521",
				Username:              "user",
				Password:              "password",
				Service:               "XE",
				MaxOpenConnections:    10,   // Custom value
				DisableConnectionPool: true, // Disabled
			},
			valid: true,
		},
		{
			name: "minimum valid configuration",
			config: Config{
				Endpoint:           "localhost:1521",
				Username:           "user",
				Password:           "password",
				Service:            "XE",
				MaxOpenConnections: 1, // Minimum allowed
			},
			valid: true,
		},
		{
			name: "invalid - zero connections",
			config: Config{
				Endpoint:           "localhost:1521",
				Username:           "user",
				Password:           "password",
				Service:            "XE",
				MaxOpenConnections: 0, // Invalid
			},
			valid: false,
		},
		{
			name: "invalid - negative connections",
			config: Config{
				Endpoint:           "localhost:1521",
				Username:           "user",
				Password:           "password",
				Service:            "XE",
				MaxOpenConnections: -1, // Invalid
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.valid {
				assert.NoError(t, err, "Expected configuration to be valid")
			} else {
				assert.Error(t, err, "Expected configuration to be invalid")
			}
		})
	}
}

func TestConfigDataSourcePriority(t *testing.T) {
	// Test that DataSource takes precedence over individual connection parameters
	config := Config{
		DataSource: "oracle://test:test@localhost:1521/TEST", // Should be used
		// These should be ignored when DataSource is set
		Endpoint: "ignored:1521",
		Username: "ignored",
		Password: "ignored",
		Service:  "ignored",
	}

	err := config.Validate()
	assert.NoError(t, err, "DataSource configuration should be valid")
}

func TestConfigEndpointValidation(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		valid    bool
	}{
		{
			name:     "valid host and port",
			endpoint: "localhost:1521",
			valid:    true,
		},
		{
			name:     "valid IP and port",
			endpoint: "192.168.1.100:1521",
			valid:    true,
		},
		{
			name:     "valid IPv6 and port",
			endpoint: "[::1]:1521",
			valid:    true,
		},
		{
			name:     "invalid - missing port",
			endpoint: "localhost",
			valid:    false,
		},
		{
			name:     "invalid - port out of range",
			endpoint: "localhost:65536",
			valid:    false,
		},
		{
			name:     "invalid - negative port",
			endpoint: "localhost:-1",
			valid:    false,
		},
		{
			name:     "invalid - non-numeric port",
			endpoint: "localhost:abc",
			valid:    false,
		},
		{
			name:     "invalid - empty host",
			endpoint: ":1521",
			valid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Endpoint:           tt.endpoint,
				Username:           "user",
				Password:           "password",
				Service:            "XE",
				MaxOpenConnections: 5,
			}

			err := config.Validate()
			if tt.valid {
				assert.NoError(t, err, "Expected endpoint to be valid")
			} else {
				assert.Error(t, err, "Expected endpoint to be invalid")
			}
		})
	}
}

func TestConfigRequiredFields(t *testing.T) {
	baseConfig := Config{
		Endpoint:           "localhost:1521",
		Username:           "user",
		Password:           "password",
		Service:            "XE",
		MaxOpenConnections: 5,
	}

	tests := []struct {
		name     string
		modifier func(*Config)
		valid    bool
	}{
		{
			name:     "all required fields present",
			modifier: func(c *Config) {}, // No changes
			valid:    true,
		},
		{
			name: "missing username",
			modifier: func(c *Config) {
				c.Username = ""
			},
			valid: false,
		},
		{
			name: "missing password",
			modifier: func(c *Config) {
				c.Password = ""
			},
			valid: false,
		},
		{
			name: "missing service",
			modifier: func(c *Config) {
				c.Service = ""
			},
			valid: false,
		},
		{
			name: "missing endpoint",
			modifier: func(c *Config) {
				c.Endpoint = ""
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := baseConfig // Copy
			tt.modifier(&config)

			err := config.Validate()
			if tt.valid {
				assert.NoError(t, err, "Expected configuration to be valid")
			} else {
				assert.Error(t, err, "Expected configuration to be invalid")
			}
		})
	}
}

func TestConfigConnectionPoolDefaults(t *testing.T) {
	// Test that default values follow Oracle database best practices
	factory := NewFactory()
	defaultConfig := factory.CreateDefaultConfig().(*Config)

	// Verify Oracle-compatible defaults
	assert.Equal(t, 5, defaultConfig.MaxOpenConnections,
		"Default MaxOpenConnections should be 5 for optimal Oracle performance")
	assert.False(t, defaultConfig.DisableConnectionPool,
		"Default DisableConnectionPool should be false for better resource management")
}

func TestConfigValidationErrors(t *testing.T) {
	// Test specific error messages for better debugging
	config := Config{
		Endpoint:           "invalid-endpoint",
		Username:           "",
		Password:           "",
		Service:            "",
		MaxOpenConnections: 0,
	}

	err := config.Validate()
	require.Error(t, err, "Expected validation to fail")

	errorStr := err.Error()
	assert.Contains(t, errorStr, "endpoint must be specified as host:port")
	assert.Contains(t, errorStr, "username must be set")
	assert.Contains(t, errorStr, "password must be set")
	assert.Contains(t, errorStr, "service must be specified")
	assert.Contains(t, errorStr, "max_open_connections must be at least 1")
}

func TestDataSourceValidation(t *testing.T) {
	tests := []struct {
		name       string
		dataSource string
		valid      bool
	}{
		{
			name:       "valid oracle data source",
			dataSource: "oracle://user:password@localhost:1521/XE",
			valid:      true,
		},
		{
			name:       "valid data source with parameters",
			dataSource: "oracle://user:password@localhost:1521/XE?timeout=30s",
			valid:      true,
		},
		{
			name:       "invalid data source - malformed URL",
			dataSource: "not-a-valid-url",
			valid:      false,
		},
		{
			name:       "invalid data source - missing scheme",
			dataSource: "user:password@localhost:1521/XE",
			valid:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				DataSource:         tt.dataSource,
				MaxOpenConnections: 5,
			}

			err := config.Validate()
			if tt.valid {
				assert.NoError(t, err, "Expected data source to be valid")
			} else {
				assert.Error(t, err, "Expected data source to be invalid")
			}
		})
	}
}
