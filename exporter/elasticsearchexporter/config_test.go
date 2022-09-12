// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package elasticsearchexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoad_DeprecatedIndexConfigOption(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config-use-deprecated-index_option.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	r1 := cfg.Exporters[config.NewComponentIDWithName(typeStr, "log")].(*Config)
	assert.Equal(t, r1, &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "log")),
		Endpoints:        []string{"http://localhost:9200"},
		CloudID:          "TRNMxjXlNJEt",
		Index:            "my_log_index",
		LogsIndex:        "logs-generic-default",
		TracesIndex:      "traces-generic-default",
		Pipeline:         "mypipeline",
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
	})
}

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Exporters), 3)

	defaultCfg := factory.CreateDefaultConfig()
	defaultCfg.(*Config).Endpoints = []string{"https://elastic.example.com:9200"}
	r0 := cfg.Exporters[config.NewComponentID(typeStr)]
	assert.Equal(t, r0, defaultCfg)

	r1 := cfg.Exporters[config.NewComponentIDWithName(typeStr, "trace")].(*Config)
	assert.Equal(t, r1, &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "trace")),
		Endpoints:        []string{"https://elastic.example.com:9200"},
		CloudID:          "TRNMxjXlNJEt",
		Index:            "",
		LogsIndex:        "logs-generic-default",
		TracesIndex:      "trace_index",
		Pipeline:         "mypipeline",
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
	})

	r2 := cfg.Exporters[config.NewComponentIDWithName(typeStr, "log")].(*Config)
	assert.Equal(t, r2, &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "log")),
		Endpoints:        []string{"http://localhost:9200"},
		CloudID:          "TRNMxjXlNJEt",
		Index:            "",
		LogsIndex:        "my_log_index",
		TracesIndex:      "traces-generic-default",
		Pipeline:         "mypipeline",
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
	})

}

func withDefaultConfig(fns ...func(*Config)) *Config {
	cfg := createDefaultConfig().(*Config)
	for _, fn := range fns {
		fn(cfg)
	}
	return cfg
}
