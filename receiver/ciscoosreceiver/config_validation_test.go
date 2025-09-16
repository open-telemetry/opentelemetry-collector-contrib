// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigValidation_AuthenticationLogic(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid_key_file_auth",
			config: &Config{
				CollectionInterval: 60 * time.Second,
				Timeout:            30 * time.Second,
				Collectors: CollectorsConfig{
					BGP: true,
				},
				Devices: []DeviceConfig{
					{
						Host:     "192.168.1.1:22",
						Username: "admin",
						KeyFile:  "/path/to/key",
						// Password is optional with key file
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid_key_file_auth_with_password",
			config: &Config{
				CollectionInterval: 60 * time.Second,
				Timeout:            30 * time.Second,
				Collectors: CollectorsConfig{
					BGP: true,
				},
				Devices: []DeviceConfig{
					{
						Host:     "192.168.1.1:22",
						Username: "admin",
						KeyFile:  "/path/to/key",
						Password: "password", // Password allowed but not required with key file
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid_password_auth",
			config: &Config{
				CollectionInterval: 60 * time.Second,
				Timeout:            30 * time.Second,
				Collectors: CollectorsConfig{
					BGP: true,
				},
				Devices: []DeviceConfig{
					{
						Host:     "192.168.1.1:22",
						Username: "admin",
						Password: "password",
						// No key file
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid_key_file_missing_username",
			config: &Config{
				CollectionInterval: 60 * time.Second,
				Timeout:            30 * time.Second,
				Collectors: CollectorsConfig{
					BGP: true,
				},
				Devices: []DeviceConfig{
					{
						Host:    "192.168.1.1:22",
						KeyFile: "/path/to/key",
						// Missing username
					},
				},
			},
			wantErr: true,
			errMsg:  "device username cannot be empty",
		},
		{
			name: "invalid_password_auth_missing_username",
			config: &Config{
				CollectionInterval: 60 * time.Second,
				Timeout:            30 * time.Second,
				Collectors: CollectorsConfig{
					BGP: true,
				},
				Devices: []DeviceConfig{
					{
						Host:     "192.168.1.1:22",
						Password: "password",
						// Missing username, no key file
					},
				},
			},
			wantErr: true,
			errMsg:  "device username cannot be empty",
		},
		{
			name: "invalid_password_auth_missing_password",
			config: &Config{
				CollectionInterval: 60 * time.Second,
				Timeout:            30 * time.Second,
				Collectors: CollectorsConfig{
					BGP: true,
				},
				Devices: []DeviceConfig{
					{
						Host:     "192.168.1.1:22",
						Username: "admin",
						// Missing password, no key file
					},
				},
			},
			wantErr: true,
			errMsg:  "device password cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
