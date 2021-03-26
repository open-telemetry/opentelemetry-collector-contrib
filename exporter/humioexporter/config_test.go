// Copyright 2021, OpenTelemetry Authors
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

package humioexporter

import (
	"net/url"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Helper method to handle boilerplate of loading configuration from file
func loadConfig(t *testing.T, name string) (configmodels.Exporter, *Config) {
	// Initialize exporter factory
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[configmodels.Type(typeStr)] = factory

	// Load configurations
	config, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, config)
	actual := config.Exporters[name]

	def := factory.CreateDefaultConfig().(*Config)
	require.NotNil(t, def)

	return actual, def
}

func TestLoadWithDefaults(t *testing.T) {
	// Arrange / Act
	actual, expected := loadConfig(t, typeStr)
	expected.IngestToken = "00000000-0000-0000-0000-0000000000000"

	// Assert
	assert.Equal(t, expected, actual)
}

func TestLoadAllSettings(t *testing.T) {
	// Arrange
	expected := &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr + "/allsettings",
		},

		QueueSettings: exporterhelper.QueueSettings{
			Enabled:      false,
			NumConsumers: 20,
			QueueSize:    2500,
		},
		RetrySettings: exporterhelper.RetrySettings{
			Enabled:         false,
			InitialInterval: 8 * time.Second,
			MaxInterval:     2 * time.Minute,
			MaxElapsedTime:  5 * time.Minute,
		},

		IngestToken:      "00000000-0000-0000-0000-0000000000000",
		Endpoint:         "localhost:8080",
		EnableServiceTag: false,
		Tags: map[string]string{
			"host":        "web_server",
			"environment": "production",
		},
		Logs: LogsConfig{
			LogParser: "custom-parser",
		},
		Traces: TracesConfig{
			IsoTimestamps:    false,
			TimeZone:         "Europe/Copenhagen",
			EnableRawstrings: true,
		},
	}

	// Act
	actual, _ := loadConfig(t, typeStr+"/allsettings")

	// Assert
	assert.Equal(t, expected, actual)
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "Valid configuration",
			config: &Config{
				IngestToken:      "t",
				Endpoint:         "e",
				EnableServiceTag: true,
				Traces: TracesConfig{
					IsoTimestamps: true,
				},
			},
			wantErr: false,
		},
		{
			name: "Missing ingest token",
			config: &Config{
				IngestToken:      "",
				Endpoint:         "e",
				EnableServiceTag: true,
				Traces: TracesConfig{
					IsoTimestamps: true,
				},
			},
			wantErr: true,
		},
		{
			name: "Missing endpoint",
			config: &Config{
				IngestToken:      "t",
				Endpoint:         "",
				EnableServiceTag: true,
				Traces: TracesConfig{
					IsoTimestamps: true,
				},
			},
			wantErr: true,
		},
		{
			name: "Override tags",
			config: &Config{
				IngestToken:      "t",
				Endpoint:         "e",
				EnableServiceTag: false,
				Tags:             map[string]string{"k": "v"},
				Traces: TracesConfig{
					IsoTimestamps: true,
				},
			},
			wantErr: false,
		},
		{
			name: "Missing custom tags",
			config: &Config{
				IngestToken:      "t",
				Endpoint:         "e",
				EnableServiceTag: false,
				Traces: TracesConfig{
					IsoTimestamps: true,
				},
			},
			wantErr: true,
		},
		{
			name: "Unix with time zone",
			config: &Config{
				IngestToken:      "t",
				Endpoint:         "e",
				EnableServiceTag: true,
				Traces: TracesConfig{
					IsoTimestamps: false,
					TimeZone:      "z",
				},
			},
			wantErr: false,
		},
		{
			name: "Missing time zone",
			config: &Config{
				IngestToken:      "t",
				Endpoint:         "e",
				EnableServiceTag: true,
				Traces: TracesConfig{
					IsoTimestamps: false,
				},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.config.validate(); (err != nil) != test.wantErr {
				t.Errorf("Config.validate() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestGetEndpoint(t *testing.T) {
	// Arrange
	expected := &url.URL{
		Scheme: "http",
		Host:   "localhost:8080",
		Path:   structuredPath,
	}

	c := Config{
		IngestToken:      "t",
		Endpoint:         "http://localhost:8080",
		EnableServiceTag: true,
		Traces: TracesConfig{
			IsoTimestamps: true,
		},
	}

	// Act
	actual, err := c.getEndpoint(structuredPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}
