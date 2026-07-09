// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logicmonitorexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter/internal/metadata"
)

// Test that the factory creates the default configuration
func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	clientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfig.MaxIdleConns = 0
	clientConfig.IdleConnTimeout = 0
	clientConfig.ForceAttemptHTTP2 = false
	assert.Equal(t, &Config{
		ClientConfig:  clientConfig,
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
	}, cfg, "failed to create default config")

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateLogs(t *testing.T) {
	clientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfig.MaxIdleConns = 0
	clientConfig.IdleConnTimeout = 0
	clientConfig.ForceAttemptHTTP2 = false
	clientConfig.Endpoint = "http://example.logicmonitor.com/rest"

	tests := []struct {
		name         string
		config       Config
		shouldError  bool
		errorMessage string
	}{
		{
			name: "valid config",
			config: Config{
				ClientConfig: clientConfig,
			},
			shouldError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			set := exportertest.NewNopSettings(metadata.Type)
			oexp, err := factory.CreateLogs(t.Context(), set, cfg)
			if (err != nil) != tt.shouldError {
				t.Errorf("CreateLogs() error = %v, shouldError %v", err, tt.shouldError)
				return
			}
			if tt.shouldError {
				assert.Error(t, err)
				if tt.errorMessage != "" {
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
	clientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfig.MaxIdleConns = 0
	clientConfig.IdleConnTimeout = 0
	clientConfig.ForceAttemptHTTP2 = false
	clientConfig.Endpoint = "http://example.logicmonitor.com/rest"

	tests := []struct {
		name         string
		config       Config
		shouldError  bool
		errorMessage string
	}{
		{
			name: "valid config",
			config: Config{
				ClientConfig: clientConfig,
			},
			shouldError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			set := exportertest.NewNopSettings(metadata.Type)
			oexp, err := factory.CreateTraces(t.Context(), set, cfg)
			if (err != nil) != tt.shouldError {
				t.Errorf("CreateTraces() error = %v, shouldError %v", err, tt.shouldError)
				return
			}
			if tt.shouldError {
				assert.Error(t, err)
				if tt.errorMessage != "" {
					assert.Equal(t, tt.errorMessage, err.Error())
				}
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, oexp)
		})
	}
}
