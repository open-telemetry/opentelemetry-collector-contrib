// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasetexporter

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestConfigUnmarshalUnknownAttributes(t *testing.T) {
	f := NewFactory()
	config := f.CreateDefaultConfig().(*Config)
	configMap := confmap.NewFromStringMap(map[string]any{
		"dataset_url":       "https://example.com",
		"api_key":           "secret",
		"unknown_attribute": "some value",
	})
	err := config.Unmarshal(configMap)

	unmarshalErr := fmt.Errorf("1 error(s) decoding:\n\n* '' has invalid keys: unknown_attribute")
	expectedError := fmt.Errorf("cannot unmarshal config: %w", unmarshalErr)

	assert.Equal(t, expectedError.Error(), err.Error())
}

func TestConfigUseDefaults(t *testing.T) {
	f := NewFactory()
	config := f.CreateDefaultConfig().(*Config)
	configMap := confmap.NewFromStringMap(map[string]any{
		"dataset_url": "https://example.com",
		"api_key":     "secret",
	})
	err := config.Unmarshal(configMap)
	assert.Nil(t, err)

	assert.Equal(t, "https://example.com", config.DatasetURL)
	assert.Equal(t, "secret", string(config.APIKey))
	assert.Equal(t, bufferMaxLifetime, config.MaxLifetime)
	assert.Equal(t, logsExportResourceInfoDefault, config.LogsSettings.ExportResourceInfo)
	assert.Equal(t, logsExportResourcePrefixDefault, config.LogsSettings.ExportResourcePrefix)
	assert.Equal(t, logsExportScopeInfoDefault, config.LogsSettings.ExportScopeInfo)
	assert.Equal(t, logsExportScopePrefixDefault, config.LogsSettings.ExportScopePrefix)
	assert.Equal(t, logsDecomposeComplexMessageFieldDefault, config.LogsSettings.DecomposeComplexMessageField)
	assert.Equal(t, exportSeparatorDefault, config.LogsSettings.exportSettings.ExportSeparator)
	assert.Equal(t, exportDistinguishingSuffix, config.LogsSettings.exportSettings.ExportDistinguishingSuffix)
	assert.Equal(t, exportSeparatorDefault, config.TracesSettings.exportSettings.ExportSeparator)
	assert.Equal(t, exportDistinguishingSuffix, config.TracesSettings.exportSettings.ExportDistinguishingSuffix)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected error
	}{
		{
			name: "valid config",
			config: Config{
				DatasetURL: "https://example.com",
				APIKey:     "secret",
				BufferSettings: BufferSettings{
					MaxLifetime: 123 * time.Millisecond,
				},
			},
			expected: nil,
		},
		{
			name: "missing api_key",
			config: Config{
				DatasetURL: "https://example.com",
				BufferSettings: BufferSettings{
					MaxLifetime: bufferMaxLifetime,
				},
			},
			expected: fmt.Errorf("api_key is required"),
		},
		{
			name: "missing dataset_url",
			config: Config{
				APIKey: "1234",
				BufferSettings: BufferSettings{
					MaxLifetime: bufferMaxLifetime,
				},
			},
			expected: fmt.Errorf("dataset_url is required"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err == nil {
				assert.Nil(t, tt.expected, tt.name)
			} else {
				assert.Equal(t, tt.expected.Error(), err.Error(), tt.name)
			}
		})
	}
}

func TestConfigString(t *testing.T) {
	config := Config{
		DatasetURL: "https://example.com",
		APIKey:     "secret",
		Debug:      true,
		BufferSettings: BufferSettings{
			MaxLifetime: 123,
			GroupBy:     []string{"field1", "field2"},
		},
		TracesSettings: TracesSettings{
			exportSettings: exportSettings{
				ExportSeparator:            "TTT",
				ExportDistinguishingSuffix: "UUU",
			},
		},
		LogsSettings: LogsSettings{
			ExportResourceInfo:             true,
			ExportResourcePrefix:           "AAA",
			ExportScopeInfo:                true,
			ExportScopePrefix:              "BBB",
			DecomposeComplexMessageField:   true,
			DecomposedComplexMessagePrefix: "EEE",
			exportSettings: exportSettings{
				ExportSeparator:            "CCC",
				ExportDistinguishingSuffix: "DDD",
			},
		},
		ServerHostSettings: ServerHostSettings{
			ServerHost:  "foo-bar",
			UseHostName: false,
		},
		BackOffConfig:   configretry.NewDefaultBackOffConfig(),
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
	}

	assert.Equal(t,
		"DatasetURL: https://example.com; APIKey: [REDACTED] (6); Debug: true; BufferSettings: {MaxLifetime:123ns GroupBy:[field1 field2] RetryInitialInterval:0s RetryMaxInterval:0s RetryMaxElapsedTime:0s RetryShutdownTimeout:0s}; LogsSettings: {ExportResourceInfo:true ExportResourcePrefix:AAA ExportScopeInfo:true ExportScopePrefix:BBB DecomposeComplexMessageField:true DecomposedComplexMessagePrefix:EEE exportSettings:{ExportSeparator:CCC ExportDistinguishingSuffix:DDD}}; TracesSettings: {exportSettings:{ExportSeparator:TTT ExportDistinguishingSuffix:UUU}}; ServerHostSettings: {UseHostName:false ServerHost:foo-bar}; BackOffConfig: {Enabled:true InitialInterval:5s RandomizationFactor:0.5 Multiplier:1.5 MaxInterval:30s MaxElapsedTime:5m0s}; QueueSettings: {Enabled:true NumConsumers:10 QueueSize:1000 StorageID:<nil>}; TimeoutSettings: {Timeout:5s}",
		config.String(),
	)
}

func TestConfigUseProvidedExportResourceInfoValue(t *testing.T) {
	f := NewFactory()
	config := f.CreateDefaultConfig().(*Config)
	configMap := confmap.NewFromStringMap(map[string]any{
		"dataset_url": "https://example.com",
		"api_key":     "secret",
		"logs": map[string]any{
			"export_resource_info_on_event": true,
		},
	})
	err := config.Unmarshal(configMap)
	assert.Nil(t, err)
	assert.Equal(t, true, config.LogsSettings.ExportResourceInfo)
}

func TestConfigUseProvidedExportScopeInfoValue(t *testing.T) {
	f := NewFactory()
	config := f.CreateDefaultConfig().(*Config)
	configMap := confmap.NewFromStringMap(map[string]any{
		"dataset_url": "https://example.com",
		"api_key":     "secret",
		"logs": map[string]any{
			"export_scope_info_on_event": false,
		},
	})
	err := config.Unmarshal(configMap)
	assert.Nil(t, err)
	assert.Equal(t, false, config.LogsSettings.ExportScopeInfo)
}
