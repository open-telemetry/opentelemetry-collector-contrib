// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))

	defaultConfig := cfg.(*Config)
	assert.Equal(t, 60*time.Second, defaultConfig.CollectionInterval)
	assert.Equal(t, 30*time.Second, defaultConfig.Timeout)
	assert.True(t, defaultConfig.Collectors.BGP)
	assert.True(t, defaultConfig.Collectors.Environment)
	assert.True(t, defaultConfig.Collectors.Facts)
	assert.True(t, defaultConfig.Collectors.Interfaces)
	assert.True(t, defaultConfig.Collectors.Optics)
	assert.Empty(t, defaultConfig.Devices)
}

func TestCreateMetricsReceiver(t *testing.T) {
	factory := NewFactory()

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid_config",
			config: &Config{
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
			wantErr: false,
		},
		{
			name: "invalid_config_no_devices",
			config: &Config{
				CollectionInterval: 60 * time.Second,
				Timeout:            30 * time.Second,
				Collectors: CollectorsConfig{
					BGP: true,
				},
				Devices: []DeviceConfig{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumer := consumertest.NewNop()
			settings := receivertest.NewNopSettings(metadata.Type)

			receiver, err := factory.CreateMetrics(
				context.Background(),
				settings,
				tt.config,
				consumer,
			)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, receiver)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, receiver)
			}
		})
	}
}

func TestFactoryType(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, metadata.Type, factory.Type())
}

func TestCreateTracesReceiver(t *testing.T) {
	factory := NewFactory()

	// Should return error since this receiver doesn't support traces
	receiver, err := factory.CreateTraces(
		context.Background(),
		receivertest.NewNopSettings(metadata.Type),
		factory.CreateDefaultConfig(),
		consumertest.NewNop(),
	)

	assert.Error(t, err)
	assert.Nil(t, receiver)
}

func TestCreateLogsReceiver(t *testing.T) {
	factory := NewFactory()

	// Should return error since this receiver doesn't support logs
	receiver, err := factory.CreateLogs(
		context.Background(),
		receivertest.NewNopSettings(metadata.Type),
		factory.CreateDefaultConfig(),
		consumertest.NewNop(),
	)

	assert.Error(t, err)
	assert.Nil(t, receiver)
}
