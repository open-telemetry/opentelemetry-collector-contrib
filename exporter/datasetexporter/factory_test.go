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
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datasetexporter/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, &Config{
		BufferSettings:  newDefaultBufferSettings(),
		TracesSettings:  newDefaultTracesSettings(),
		RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
	}, cfg, "failed to create default config")

	assert.Nil(t, componenttest.CheckConfigStruct(cfg))
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	assert.Nil(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, "minimal"),
			expected: &Config{
				DatasetURL:      "https://app.scalyr.com",
				APIKey:          "key-minimal",
				BufferSettings:  newDefaultBufferSettings(),
				TracesSettings:  newDefaultTracesSettings(),
				RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
				QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
				TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "lib"),
			expected: &Config{
				DatasetURL: "https://app.eu.scalyr.com",
				APIKey:     "key-lib",
				BufferSettings: BufferSettings{
					MaxLifetime:          345 * time.Millisecond,
					GroupBy:              []string{"attributes.container_id", "attributes.log.file.path"},
					RetryInitialInterval: bufferRetryInitialInterval,
					RetryMaxInterval:     bufferRetryMaxInterval,
					RetryMaxElapsedTime:  bufferRetryMaxElapsedTime,
				},
				TracesSettings:  newDefaultTracesSettings(),
				RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
				QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
				TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "full"),
			expected: &Config{
				DatasetURL: "https://app.scalyr.com",
				APIKey:     "key-full",
				BufferSettings: BufferSettings{
					MaxLifetime:          3456 * time.Millisecond,
					GroupBy:              []string{"body.map.kubernetes.pod_id", "body.map.kubernetes.docker_id", "body.map.stream"},
					RetryInitialInterval: 21 * time.Second,
					RetryMaxInterval:     22 * time.Second,
					RetryMaxElapsedTime:  23 * time.Second,
				},
				TracesSettings: TracesSettings{
					MaxWait: 3 * time.Second,
				},
				RetrySettings: exporterhelper.RetrySettings{
					Enabled:             true,
					InitialInterval:     11 * time.Nanosecond,
					RandomizationFactor: 11.3,
					Multiplier:          11.6,
					MaxInterval:         12 * time.Nanosecond,
					MaxElapsedTime:      13 * time.Nanosecond,
				},
				QueueSettings: exporterhelper.QueueSettings{
					Enabled:      true,
					NumConsumers: 14,
					QueueSize:    15,
				},
				TimeoutSettings: exporterhelper.TimeoutSettings{
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
			require.Nil(t, err)
			require.Nil(t, component.UnmarshalConfig(sub, cfg))
			if assert.Nil(t, component.ValidateConfig(cfg)) {
				assert.Equal(t, tt.expected, cfg)
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
	return []CreateTest{
		{
			name:          "broken",
			config:        &Config{},
			expectedError: fmt.Errorf("cannot get DataSetExpoter: cannot convert config: DatasetURL: ; BufferSettings: {MaxLifetime:0s GroupBy:[] RetryInitialInterval:0s RetryMaxInterval:0s RetryMaxElapsedTime:0s}; TracesSettings: {Aggregate:false MaxWait:0s}; RetrySettings: {Enabled:false InitialInterval:0s RandomizationFactor:0 Multiplier:0 MaxInterval:0s MaxElapsedTime:0s}; QueueSettings: {Enabled:false NumConsumers:0 QueueSize:0 StorageID:<nil>}; TimeoutSettings: {Timeout:0s}; config is not valid: api_key is required"),
		},
		{
			name: "valid",
			config: &Config{
				DatasetURL: "https://app.eu.scalyr.com",
				APIKey:     "key-lib",
				BufferSettings: BufferSettings{
					MaxLifetime:          12345,
					GroupBy:              []string{"attributes.container_id"},
					RetryInitialInterval: time.Second,
					RetryMaxInterval:     time.Minute,
					RetryMaxElapsedTime:  time.Hour,
				},
				TracesSettings: TracesSettings{
					Aggregate: true,
					MaxWait:   5 * time.Second,
				},
				RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
				QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
				TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
			},
			expectedError: nil,
		},
	}
}
