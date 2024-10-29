// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logicmonitorexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

// Test that the factory creates the default configuration
func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, &Config{
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: exporterhelper.NewDefaultQueueConfig(),
	}, cfg, "failed to create default config")

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateLogs(t *testing.T) {
	tests := []struct {
		name         string
		config       Config
		shouldError  bool
		errorMessage string
	}{
		{
			name: "valid config",
			config: Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "http://example.logicmonitor.com/rest",
				},
			},
			shouldError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			set := exportertest.NewNopSettings()
			oexp, err := factory.CreateLogs(context.Background(), set, cfg)
			if (err != nil) != tt.shouldError {
				t.Errorf("CreateLogs() error = %v, shouldError %v", err, tt.shouldError)
				return
			}
			if tt.shouldError {
				assert.Error(t, err)
				if len(tt.errorMessage) != 0 {
					assert.Equal(t, tt.errorMessage, err.Error())
				}
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, oexp)
		})
	}
}

func TestCreateTraces(t *testing.T) {
	tests := []struct {
		name         string
		config       Config
		shouldError  bool
		errorMessage string
	}{
		{
			name: "valid config",
			config: Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "http://example.logicmonitor.com/rest",
				},
			},
			shouldError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			set := exportertest.NewNopSettings()
			oexp, err := factory.CreateTraces(context.Background(), set, cfg)
			if (err != nil) != tt.shouldError {
				t.Errorf("CreateTraces() error = %v, shouldError %v", err, tt.shouldError)
				return
			}
			if tt.shouldError {
				assert.Error(t, err)
				if len(tt.errorMessage) != 0 {
					assert.Equal(t, tt.errorMessage, err.Error())
				}
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, oexp)
		})
	}
}
