// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasetexporter

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datasetexporter/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, &Config{
		BufferSettings:     newDefaultBufferSettings(),
		TracesSettings:     newDefaultTracesSettings(),
		LogsSettings:       newDefaultLogsSettings(),
		ServerHostSettings: newDefaultServerHostSettings(),
		BackOffConfig:      configretry.NewDefaultBackOffConfig(),
		QueueSettings:      exporterhelper.NewDefaultQueueConfig(),
		TimeoutSettings:    exporterhelper.NewDefaultTimeoutConfig(),
	}, cfg, "failed to create default config")

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	assert.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, "minimal"),
			expected: &Config{
				DatasetURL:         "https://app.scalyr.com",
				APIKey:             "key-minimal",
				BufferSettings:     newDefaultBufferSettings(),
				TracesSettings:     newDefaultTracesSettings(),
				LogsSettings:       newDefaultLogsSettings(),
				ServerHostSettings: newDefaultServerHostSettings(),
				BackOffConfig:      configretry.NewDefaultBackOffConfig(),
				QueueSettings:      exporterhelper.NewDefaultQueueConfig(),
				TimeoutSettings:    exporterhelper.NewDefaultTimeoutConfig(),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "lib"),
			expected: &Config{
				DatasetURL: "https://app.eu.scalyr.com",
				APIKey:     "key-lib",
				BufferSettings: BufferSettings{
					MaxLifetime:          345 * time.Millisecond,
					PurgeOlderThan:       bufferPurgeOlderThan,
					GroupBy:              []string{"attributes.container_id", "attributes.log.file.path"},
					RetryInitialInterval: bufferRetryInitialInterval,
					RetryMaxInterval:     bufferRetryMaxInterval,
					RetryMaxElapsedTime:  bufferRetryMaxElapsedTime,
					RetryShutdownTimeout: bufferRetryShutdownTimeout,
					MaxParallelOutgoing:  bufferMaxParallelOutgoing,
				},
				TracesSettings:     newDefaultTracesSettings(),
				LogsSettings:       newDefaultLogsSettings(),
				ServerHostSettings: newDefaultServerHostSettings(),
				BackOffConfig:      configretry.NewDefaultBackOffConfig(),
				QueueSettings:      exporterhelper.NewDefaultQueueConfig(),
				TimeoutSettings:    exporterhelper.NewDefaultTimeoutConfig(),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "full"),
			expected: &Config{
				DatasetURL: "https://app.scalyr.com",
				APIKey:     "key-full",
				Debug:      true,
				BufferSettings: BufferSettings{
					MaxLifetime:          3456 * time.Millisecond,
					PurgeOlderThan:       78 * time.Second,
					GroupBy:              []string{"body.map.kubernetes.pod_id", "body.map.kubernetes.docker_id", "body.map.stream"},
					RetryInitialInterval: 21 * time.Second,
					RetryMaxInterval:     22 * time.Second,
					RetryMaxElapsedTime:  23 * time.Second,
					RetryShutdownTimeout: 24 * time.Second,
					MaxParallelOutgoing:  25,
				},
				TracesSettings: TracesSettings{
					exportSettings: exportSettings{
						ExportSeparator:            "_Y_",
						ExportDistinguishingSuffix: "_T_",
					},
				},
				LogsSettings: LogsSettings{
					ExportResourceInfo:             true,
					ExportResourcePrefix:           "_resource_",
					ExportScopeInfo:                true,
					ExportScopePrefix:              "_scope_",
					DecomposeComplexMessageField:   true,
					DecomposedComplexMessagePrefix: "_body_",
					exportSettings: exportSettings{
						ExportSeparator:            "_X_",
						ExportDistinguishingSuffix: "_L_",
					},
				},
				ServerHostSettings: ServerHostSettings{
					UseHostName: false,
					ServerHost:  "server-host",
				},
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     11 * time.Nanosecond,
					RandomizationFactor: 0.113,
					Multiplier:          11.6,
					MaxInterval:         12 * time.Nanosecond,
					MaxElapsedTime:      13 * time.Nanosecond,
				},
				QueueSettings: exporterhelper.QueueBatchConfig{
					Enabled:      true,
					NumConsumers: 14,
					QueueSize:    15,
					Sizer:        exporterhelper.RequestSizerTypeRequests,
				},
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 16 * time.Nanosecond,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.Name(), func(*testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			if assert.NoError(t, xconfmap.Validate(cfg)) {
				assert.Equal(t, tt.expected, cfg)
			}
		})
	}
}

func TestValidateConfigs(t *testing.T) {
	tests := createExporterTests()

	for _, tt := range tests {
		t.Run(tt.name, func(*testing.T) {
			err := xconfmap.Validate(tt.config)
			if tt.expectedError != nil {
				assert.ErrorContains(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

type CreateTest struct {
	name          string
	config        component.Config
	expectedError error
}

func createExporterTests() []CreateTest {
	factory := NewFactory()
	defaultCfg := factory.CreateDefaultConfig().(*Config)
	defaultCfg.APIKey = "default-api-key"
	defaultCfg.DatasetURL = "https://app.eu.scalyr.com"

	return []CreateTest{
		{
			name:          "broken",
			config:        &Config{},
			expectedError: fmt.Errorf("api_key is required"),
		},
		{
			name:          "missing-url",
			config:        &Config{APIKey: "AAA"},
			expectedError: fmt.Errorf("dataset_url is required"),
		},
		{
			name:          "missing-key",
			config:        &Config{DatasetURL: "bbb"},
			expectedError: fmt.Errorf("api_key is required"),
		},
		{
			name: "valid",
			config: &Config{
				DatasetURL: "https://app.eu.scalyr.com",
				APIKey:     "key-lib",
				BufferSettings: BufferSettings{
					MaxLifetime:          12345,
					PurgeOlderThan:       78901,
					GroupBy:              []string{"attributes.container_id"},
					RetryInitialInterval: time.Second,
					RetryMaxInterval:     time.Minute,
					RetryMaxElapsedTime:  time.Hour,
					RetryShutdownTimeout: time.Minute,
				},
				TracesSettings: newDefaultTracesSettings(),
				LogsSettings:   newDefaultLogsSettings(),
				ServerHostSettings: ServerHostSettings{
					UseHostName: true,
				},
				BackOffConfig:   configretry.NewDefaultBackOffConfig(),
				QueueSettings:   exporterhelper.NewDefaultQueueConfig(),
				TimeoutSettings: exporterhelper.NewDefaultTimeoutConfig(),
			},
			expectedError: nil,
		},
		{
			name:          "default",
			config:        defaultCfg,
			expectedError: nil,
		},
	}
}
