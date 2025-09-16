// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
		wantErr  bool
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				CollectionInterval: 60 * time.Second,
				Timeout:            30 * time.Second,
				Collectors: CollectorsConfig{
					BGP:         true,
					Environment: true,
					Facts:       true,
					Interfaces:  true,
					Optics:      true,
				},
				Devices: []DeviceConfig{
					{
						Host:     "192.168.1.1:22",
						Username: "admin",
						Password: "password",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "custom"),
			expected: &Config{
				CollectionInterval: 30 * time.Second,
				Timeout:            15 * time.Second,
				Collectors: CollectorsConfig{
					BGP:         true,
					Environment: false,
					Facts:       true,
					Interfaces:  true,
					Optics:      false,
				},
				Devices: []DeviceConfig{
					{
						Host:     "192.168.1.1:22",
						Username: "admin",
						Password: "secret",
					},
					{
						Host:     "192.168.1.2:22",
						Username: "operator",
						Password: "password",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.wantErr {
				assert.Error(t, cfg.(*Config).Validate())
			} else {
				assert.NoError(t, cfg.(*Config).Validate())
				assert.Equal(t, tt.expected, cfg)
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid_config",
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
					},
				},
			},
			wantErr: false,
		},
		{
			name: "no_devices",
			config: &Config{
				CollectionInterval: 60 * time.Second,
				Timeout:            30 * time.Second,
				Collectors: CollectorsConfig{
					BGP: true,
				},
				Devices: []DeviceConfig{},
			},
			wantErr: true,
			errMsg:  "at least one device must be configured",
		},
		{
			name: "zero_timeout",
			config: &Config{
				CollectionInterval: 60 * time.Second,
				Timeout:            0,
				Collectors: CollectorsConfig{
					BGP: true,
				},
				Devices: []DeviceConfig{
					{
						Host:     "192.168.1.1:22",
						Username: "admin",
						Password: "password",
					},
				},
			},
			wantErr: true,
			errMsg:  "timeout must be greater than 0",
		},
		{
			name: "zero_collection_interval",
			config: &Config{
				CollectionInterval: 0,
				Timeout:            30 * time.Second,
				Collectors: CollectorsConfig{
					BGP: true,
				},
				Devices: []DeviceConfig{
					{
						Host:     "192.168.1.1:22",
						Username: "admin",
						Password: "password",
					},
				},
			},
			wantErr: true,
			errMsg:  "collection_interval must be greater than 0",
		},
		{
			name: "no_collectors_enabled",
			config: &Config{
				CollectionInterval: 60 * time.Second,
				Timeout:            30 * time.Second,
				Collectors: CollectorsConfig{
					BGP:         false,
					Environment: false,
					Facts:       false,
					Interfaces:  false,
					Optics:      false,
				},
				Devices: []DeviceConfig{
					{
						Host:     "192.168.1.1:22",
						Username: "admin",
						Password: "password",
					},
				},
			},
			wantErr: true,
			errMsg:  "at least one collector must be enabled",
		},
		{
			name: "device_missing_host",
			config: &Config{
				CollectionInterval: 60 * time.Second,
				Timeout:            30 * time.Second,
				Collectors: CollectorsConfig{
					BGP: true,
				},
				Devices: []DeviceConfig{
					{
						Host:     "",
						Username: "admin",
						Password: "password",
					},
				},
			},
			wantErr: true,
			errMsg:  "device host cannot be empty",
		},
		{
			name: "device_missing_username",
			config: &Config{
				CollectionInterval: 60 * time.Second,
				Timeout:            30 * time.Second,
				Collectors: CollectorsConfig{
					BGP: true,
				},
				Devices: []DeviceConfig{
					{
						Host:     "192.168.1.1:22",
						Username: "",
						Password: "password",
					},
				},
			},
			wantErr: true,
			errMsg:  "device username cannot be empty",
		},
		{
			name: "device_missing_password",
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
						Password: "",
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
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
