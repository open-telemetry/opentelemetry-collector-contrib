// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectedErr string
	}{
		{
			name: "valid config with password auth",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{Host: "localhost:22", Username: "admin", Password: "password"},
				},
				Scrapers: ScrapersConfig{
					BGP: true,
				},
			},
			expectedErr: "",
		},
		{
			name: "valid config with key file auth",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{Host: "localhost:22", Username: "admin", KeyFile: "/path/to/key"},
				},
				Scrapers: ScrapersConfig{
					Facts: true,
				},
			},
			expectedErr: "",
		},
		{
			name: "no devices",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{},
				Scrapers: ScrapersConfig{
					BGP: true,
				},
			},
			expectedErr: "at least one device must be configured",
		},
		{
			name: "empty host",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{Host: "", Username: "admin", Password: "password"},
				},
				Scrapers: ScrapersConfig{
					BGP: true,
				},
			},
			expectedErr: "device host cannot be empty",
		},
		{
			name: "missing username for password auth",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{Host: "localhost:22", Username: "", Password: "password"},
				},
				Scrapers: ScrapersConfig{
					BGP: true,
				},
			},
			expectedErr: "device username cannot be empty",
		},
		{
			name: "missing password for password auth",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{Host: "localhost:22", Username: "admin", Password: ""},
				},
				Scrapers: ScrapersConfig{
					BGP: true,
				},
			},
			expectedErr: "device password cannot be empty",
		},
		{
			name: "no scrapers enabled",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{Host: "localhost:22", Username: "admin", Password: "password"},
				},
				Scrapers: ScrapersConfig{
					BGP:         false,
					Environment: false,
					Facts:       false,
					Interfaces:  false,
					Optics:      false,
				},
			},
			expectedErr: "at least one scraper must be enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}
