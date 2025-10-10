// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
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
					{
						Device: DeviceInfo{
							Host: HostInfo{
								Name: "test-device",
								IP:   "192.168.1.1",
								Port: 22,
							},
						},
						Auth: AuthConfig{
							Username: "admin",
							Password: configopaque.String("password"),
						},
					},
				},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("system"): nil,
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
					{
						Device: DeviceInfo{
							Host: HostInfo{
								Name: "test-device",
								IP:   "192.168.1.1",
								Port: 22,
							},
						},
						Auth: AuthConfig{
							Username: "admin",
							KeyFile:  "/path/to/key",
						},
					},
				},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("interfaces"): nil,
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
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("system"): nil,
				},
			},
			expectedErr: "at least one device must be configured",
		},
		{
			name: "empty host IP",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{
						Device: DeviceInfo{
							Host: HostInfo{
								Name: "test-device",
								IP:   "",
								Port: 22,
							},
						},
						Auth: AuthConfig{
							Username: "admin",
							Password: configopaque.String("password"),
						},
					},
				},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("system"): nil,
				},
			},
			expectedErr: "host.ip cannot be empty",
		},
		{
			name: "missing port",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{
						Device: DeviceInfo{
							Host: HostInfo{
								Name: "test-device",
								IP:   "192.168.1.1",
								Port: 0,
							},
						},
						Auth: AuthConfig{
							Username: "admin",
							Password: configopaque.String("password"),
						},
					},
				},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("system"): nil,
				},
			},
			expectedErr: "host.port cannot be empty",
		},
		{
			name: "missing username",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{
						Device: DeviceInfo{
							Host: HostInfo{
								Name: "test-device",
								IP:   "192.168.1.1",
								Port: 22,
							},
						},
						Auth: AuthConfig{
							Username: "",
							Password: configopaque.String("password"),
						},
					},
				},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("system"): nil,
				},
			},
			expectedErr: "auth.username cannot be empty",
		},
		{
			name: "missing password and key file",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{
						Device: DeviceInfo{
							Host: HostInfo{
								Name: "test-device",
								IP:   "192.168.1.1",
								Port: 22,
							},
						},
						Auth: AuthConfig{
							Username: "admin",
						},
					},
				},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("system"): nil,
				},
			},
			expectedErr: "auth.password or auth.key_file must be provided",
		},
		{
			name: "no scrapers configured",
			config: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					Timeout:            30 * time.Second,
					CollectionInterval: 60 * time.Second,
				},
				Devices: []DeviceConfig{
					{
						Device: DeviceInfo{
							Host: HostInfo{
								Name: "test-device",
								IP:   "192.168.1.1",
								Port: 22,
							},
						},
						Auth: AuthConfig{
							Username: "admin",
							Password: configopaque.String("password"),
						},
					},
				},
				Scrapers: map[component.Type]component.Config{},
			},
			expectedErr: "must specify at least one scraper",
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
