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
			name: "invalid config - missing all connection info",
			config: Config{},
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
