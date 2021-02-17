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
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Exporters), 2)

	r0 := cfg.Exporters["elasticsearch"]
	assert.Equal(t, r0, factory.CreateDefaultConfig())

	r1 := cfg.Exporters["elasticsearch/customname"].(*Config)
	assert.Equal(t, r1, &Config{
		ExporterSettings: configmodels.ExporterSettings{TypeVal: configmodels.Type(typeStr), NameVal: "elasticsearch/customname"},
		Endpoints:        []string{"https://elastic.example.com:9200"},
		CloudID:          "TRNMxjXlNJEt",
		Index:            "myindex",
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

func withDefaultConfig(fn func(*Config)) *Config {
	cfg := createDefaultConfig().(*Config)
	if fn != nil {
		fn(cfg)
	}
	return cfg
}
