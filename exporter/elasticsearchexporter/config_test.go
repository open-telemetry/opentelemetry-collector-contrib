// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metadata"
)

func TestLoad_DeprecatedIndexConfigOption(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config-use-deprecated-index_option.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "log").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	assert.Equal(t, cfg, &Config{
		QueueSettings: exporterhelper.QueueSettings{
			Enabled:      false,
			NumConsumers: exporterhelper.NewDefaultQueueSettings().NumConsumers,
			QueueSize:    exporterhelper.NewDefaultQueueSettings().QueueSize,
		},
		Endpoints:   []string{"http://localhost:9200"},
		CloudID:     "TRNMxjXlNJEt",
		Index:       "my_log_index",
		LogsIndex:   "logs-generic-default",
		TracesIndex: "traces-generic-default",
		Pipeline:    "mypipeline",
		HTTPClientSettings: HTTPClientSettings{
			Authentication: AuthenticationSettings{
				User:     "elastic",
				Password: "search",
				APIKey:   "AvFsEiPs==",
			},
			Timeout: 2 * time.Minute,
			Headers: map[string]string{
				"myheader": "test",
			},
		},
		Discovery: DiscoverySettings{
			OnStart: true,
		},
		Flush: FlushSettings{
			Bytes: 10485760,
		},
		Retry: RetrySettings{
			Enabled:         true,
			MaxRequests:     5,
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     1 * time.Minute,
		},
		Mapping: MappingsSettings{
			Mode:  "ecs",
			Dedup: true,
			Dedot: true,
		},
		LogstashFormat: LogstashFormatSettings{
			Enabled:         false,
			PrefixSeparator: "-",
			DateFormat:      "%Y.%m.%d",
		},
	})
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	defaultCfg := createDefaultConfig()
	defaultCfg.(*Config).Endpoints = []string{"https://elastic.example.com:9200"}

	defaultLogstashFormatCfg := createDefaultConfig()
	defaultLogstashFormatCfg.(*Config).Endpoints = []string{"http://localhost:9200"}
	defaultLogstashFormatCfg.(*Config).LogstashFormat.Enabled = true

	tests := []struct {
		configFile string
		id         component.ID
		expected   component.Config
	}{
		{
			id:         component.NewIDWithName(metadata.Type, ""),
			configFile: "config.yaml",
			expected:   defaultCfg,
		},
		{
			id:         component.NewIDWithName(metadata.Type, "trace"),
			configFile: "config.yaml",
			expected: &Config{
				QueueSettings: exporterhelper.QueueSettings{
					Enabled:      false,
					NumConsumers: exporterhelper.NewDefaultQueueSettings().NumConsumers,
					QueueSize:    exporterhelper.NewDefaultQueueSettings().QueueSize,
				},
				Endpoints:   []string{"https://elastic.example.com:9200"},
				CloudID:     "TRNMxjXlNJEt",
				Index:       "",
				LogsIndex:   "logs-generic-default",
				TracesIndex: "trace_index",
				Pipeline:    "mypipeline",
				HTTPClientSettings: HTTPClientSettings{
					Authentication: AuthenticationSettings{
						User:     "elastic",
						Password: "search",
						APIKey:   "AvFsEiPs==",
					},
					Timeout: 2 * time.Minute,
					Headers: map[string]string{
						"myheader": "test",
					},
				},
				Discovery: DiscoverySettings{
					OnStart: true,
				},
				Flush: FlushSettings{
					Bytes: 10485760,
				},
				Retry: RetrySettings{
					Enabled:         true,
					MaxRequests:     5,
					InitialInterval: 100 * time.Millisecond,
					MaxInterval:     1 * time.Minute,
				},
				Mapping: MappingsSettings{
					Mode:  "ecs",
					Dedup: true,
					Dedot: true,
				},
				LogstashFormat: LogstashFormatSettings{
					Enabled:         false,
					PrefixSeparator: "-",
					DateFormat:      "%Y.%m.%d",
				},
			},
		},
		{
			id:         component.NewIDWithName(metadata.Type, "log"),
			configFile: "config.yaml",
			expected: &Config{
				QueueSettings: exporterhelper.QueueSettings{
					Enabled:      true,
					NumConsumers: exporterhelper.NewDefaultQueueSettings().NumConsumers,
					QueueSize:    exporterhelper.NewDefaultQueueSettings().QueueSize,
				},
				Endpoints:   []string{"http://localhost:9200"},
				CloudID:     "TRNMxjXlNJEt",
				Index:       "",
				LogsIndex:   "my_log_index",
				TracesIndex: "traces-generic-default",
				Pipeline:    "mypipeline",
				HTTPClientSettings: HTTPClientSettings{
					Authentication: AuthenticationSettings{
						User:     "elastic",
						Password: "search",
						APIKey:   "AvFsEiPs==",
					},
					Timeout: 2 * time.Minute,
					Headers: map[string]string{
						"myheader": "test",
					},
				},
				Discovery: DiscoverySettings{
					OnStart: true,
				},
				Flush: FlushSettings{
					Bytes: 10485760,
				},
				Retry: RetrySettings{
					Enabled:         true,
					MaxRequests:     5,
					InitialInterval: 100 * time.Millisecond,
					MaxInterval:     1 * time.Minute,
				},
				Mapping: MappingsSettings{
					Mode:  "ecs",
					Dedup: true,
					Dedot: true,
				},
				LogstashFormat: LogstashFormatSettings{
					Enabled:         false,
					PrefixSeparator: "-",
					DateFormat:      "%Y.%m.%d",
				},
			},
		},
		{
			id:         component.NewIDWithName(metadata.Type, "logstash_format"),
			configFile: "config.yaml",
			expected:   defaultLogstashFormatCfg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			cm, err := confmaptest.LoadConf(filepath.Join("testdata", tt.configFile))
			require.NoError(t, err)

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func withDefaultConfig(fns ...func(*Config)) *Config {
	cfg := createDefaultConfig().(*Config)
	for _, fn := range fns {
		fn(cfg)
	}
	return cfg
}
