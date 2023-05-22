// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasetexporter

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestConfigUnmarshalUnknownAttributes(t *testing.T) {
	f := NewFactory()
	config := f.CreateDefaultConfig().(*Config)
	configMap := confmap.NewFromStringMap(map[string]interface{}{
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
	configMap := confmap.NewFromStringMap(map[string]interface{}{
		"dataset_url": "https://example.com",
		"api_key":     "secret",
	})
	err := config.Unmarshal(configMap)
	assert.Nil(t, err)

	assert.Equal(t, "https://example.com", config.DatasetURL)
	assert.Equal(t, "secret", string(config.APIKey))
	assert.Equal(t, bufferMaxLifetime, config.MaxLifetime)
	assert.Equal(t, tracesMaxWait, config.TracesSettings.MaxWait)
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
		BufferSettings: BufferSettings{
			MaxLifetime: 123,
			GroupBy:     []string{"field1", "field2"},
		},
		TracesSettings: TracesSettings{
			Aggregate: true,
			MaxWait:   45 * time.Second,
		},
		RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
	}

	assert.Equal(t,
		"DatasetURL: https://example.com; BufferSettings: {MaxLifetime:123ns GroupBy:[field1 field2] RetryInitialInterval:0s RetryMaxInterval:0s RetryMaxElapsedTime:0s}; TracesSettings: {Aggregate:true MaxWait:45s}; RetrySettings: {Enabled:true InitialInterval:5s RandomizationFactor:0.5 Multiplier:1.5 MaxInterval:30s MaxElapsedTime:5m0s}; QueueSettings: {Enabled:true NumConsumers:10 QueueSize:1000 StorageID:<nil>}; TimeoutSettings: {Timeout:5s}",
		config.String(),
	)
}
