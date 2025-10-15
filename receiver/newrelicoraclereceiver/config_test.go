// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicoraclereceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		expectErr bool
	}{
		{
			name: "valid config with datasource",
			config: Config{
				DataSource: "oracle://user:password@localhost:1521/XE",
			},
			expectErr: false,
		},
		{
			name: "valid config with endpoint components",
			config: Config{
				Endpoint: "localhost:1521",
				Username: "user",
				Password: "password",
				Service:  "XE",
			},
			expectErr: false,
		},
		{
			name:      "invalid config - missing all connection info",
			config:    Config{},
			expectErr: true,
		},
		{
			name: "invalid config - missing username",
			config: Config{
				Endpoint: "localhost:1521",
				Password: "password",
				Service:  "XE",
			},
			expectErr: true,
		},
		{
			name: "invalid config - missing password",
			config: Config{
				Endpoint: "localhost:1521",
				Username: "user",
				Service:  "XE",
			},
			expectErr: true,
		},
		{
			name: "invalid config - missing service",
			config: Config{
				Endpoint: "localhost:1521",
				Username: "user",
				Password: "password",
			},
			expectErr: true,
		},
		{
			name: "invalid config - bad endpoint format",
			config: Config{
				Endpoint: "localhost",
				Username: "user",
				Password: "password",
				Service:  "XE",
			},
			expectErr: true,
		},
		{
			name: "valid config with connection pool settings",
			config: Config{
				Endpoint:              "localhost:1521",
				Username:              "user",
				Password:              "password",
				Service:               "XE",
				MaxOpenConnections:    10,
				DisableConnectionPool: false,
			},
			expectErr: false,
		},
		{
			name: "valid config with disabled connection pool",
			config: Config{
				Endpoint:              "localhost:1521",
				Username:              "user",
				Password:              "password",
				Service:               "XE",
				MaxOpenConnections:    1,
				DisableConnectionPool: true,
			},
			expectErr: false,
		},
		{
			name: "invalid config - zero max_open_connections",
			config: Config{
				Endpoint:           "localhost:1521",
				Username:           "user",
				Password:           "password",
				Service:            "XE",
				MaxOpenConnections: 0,
			},
			expectErr: true,
		},
		{
			name: "invalid config - negative max_open_connections",
			config: Config{
				Endpoint:           "localhost:1521",
				Username:           "user",
				Password:           "password",
				Service:            "XE",
				MaxOpenConnections: -1,
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_SetDefaults(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	assert.Equal(t, defaultMaxOpenConnections, config.MaxOpenConnections)
	assert.Equal(t, defaultCollectionInterval, config.ControllerConfig.CollectionInterval)
}

func TestConfig_ValidateEnhanced(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		expectErr bool
		errorMsg  string
	}{
		{
			name: "invalid config - max connections too high",
			config: Config{
				Endpoint:           "localhost:1521",
				Username:           "user",
				Password:           "password",
				Service:            "XE",
				MaxOpenConnections: 2000,
			},
			expectErr: true,
			errorMsg:  "max_open_connections must be between 1 and 1000",
		},
		{
			name: "invalid config - username with dangerous characters",
			config: Config{
				Endpoint:           "localhost:1521",
				Username:           "user';DROP TABLE--",
				Password:           "password",
				Service:            "XE",
				MaxOpenConnections: 5,
			},
			expectErr: true,
			errorMsg:  "username cannot contain special characters",
		},
		{
			name: "invalid config - service with invalid characters",
			config: Config{
				Endpoint:           "localhost:1521",
				Username:           "user",
				Password:           "password",
				Service:            "XE; DROP DATABASE",
				MaxOpenConnections: 5,
			},
			expectErr: true,
			errorMsg:  "service name cannot contain special characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.SetDefaults()
			err := tt.config.Validate()
			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
