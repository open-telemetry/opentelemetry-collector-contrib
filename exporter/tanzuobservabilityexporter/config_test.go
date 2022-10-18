// Copyright The OpenTelemetry Authors
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

package tanzuobservabilityexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(config.NewComponentIDWithName(exporterType, "").String())
	require.NoError(t, err)
	require.NoError(t, config.UnmarshalExporter(sub, cfg))

	expected := &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID("tanzuobservability")),
		Traces: TracesConfig{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http://localhost:40001"},
		},
		Metrics: MetricsConfig{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http://localhost:2916"},
			ResourceAttributes: resourcetotelemetry.Settings{Enabled: true},
		},
		QueueSettings: exporterhelper.QueueSettings{
			Enabled:      true,
			NumConsumers: 2,
			QueueSize:    10,
		},
		RetrySettings: exporterhelper.RetrySettings{
			Enabled:         true,
			InitialInterval: 10 * time.Second,
			MaxInterval:     60 * time.Second,
			MaxElapsedTime:  10 * time.Minute,
		},
	}
	assert.Equal(t, expected, cfg)
}

func TestConfigRequiresValidEndpointUrl(t *testing.T) {
	c := &Config{
		Traces: TracesConfig{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http#$%^&#$%&#"},
		},
	}
	assert.Error(t, c.Validate())
}

func TestMetricsConfigRequiresValidEndpointUrl(t *testing.T) {
	c := &Config{
		Metrics: MetricsConfig{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http#$%^&#$%&#"},
		},
	}

	assert.Error(t, c.Validate())
}

func TestDifferentHostNames(t *testing.T) {
	c := &Config{
		Traces: TracesConfig{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http://localhost:30001"},
		},
		Metrics: MetricsConfig{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http://foo.com:2878"},
		},
	}
	assert.Error(t, c.Validate())
}

func TestConfigNormal(t *testing.T) {
	c := &Config{
		Traces: TracesConfig{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http://localhost:40001"},
		},
		Metrics: MetricsConfig{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http://localhost:2916"},
		},
	}
	assert.NoError(t, c.Validate())
}
