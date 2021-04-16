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

package humioexporter

import (
	"net/url"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Helper method to handle boilerplate of loading configuration from file
func loadConfig(t *testing.T, name string) (config.Exporter, *Config) {
	// Initialize exporter factory
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[config.Type(typeStr)] = factory

	// Load configurations
	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	actual := cfg.Exporters[name]
	// TODO: Remove this, when Validate is fixed. A validate function should not mutate the config.
	actual.(*Config).structuredEndpoint = nil
	actual.(*Config).unstructuredEndpoint = nil
	actual.(*Config).Headers = map[string]string{}

	def := factory.CreateDefaultConfig().(*Config)
	require.NotNil(t, def)

	return actual, def
}

func TestLoadWithDefaults(t *testing.T) {
	// Arrange / Act
	actual, expected := loadConfig(t, typeStr)
	expected.IngestToken = "00000000-0000-0000-0000-0000000000000"
	expected.Endpoint = "https://my-humio-host:8080"

	// Assert
	assert.Equal(t, expected, actual)
}

func TestLoadAllSettings(t *testing.T) {
	// Arrange
	expected := &Config{
		ExporterSettings: &config.ExporterSettings{
			TypeVal: config.Type(typeStr),
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

		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint:        "https://my-humio-host:8080",
			Headers:         map[string]string{},
			Timeout:         10 * time.Second,
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			TLSSetting: configtls.TLSClientSetting{
				Insecure:           false,
				InsecureSkipVerify: false,
				ServerName:         "",
				TLSSetting: configtls.TLSSetting{
					CAFile:   "server.crt",
					CertFile: "client.crt",
					KeyFile:  "client.key",
				},
			},
		},

		IngestToken:       "00000000-0000-0000-0000-0000000000000",
		DisableServiceTag: true,
		Tags: map[string]string{
			"host":        "web_server",
			"environment": "production",
		},
		Logs: LogsConfig{
			LogParser: "custom-parser",
		},
		Traces: TracesConfig{
			UnixTimestamps: true,
			TimeZone:       "Europe/Copenhagen",
		},
	}

	// Act
	actual, _ := loadConfig(t, typeStr+"/allsettings")

	// Assert
	assert.Equal(t, expected, actual)
}

func TestValidateValid(t *testing.T) {
	//Arrange
	config := &Config{
		ExporterSettings: config.NewExporterSettings(typeStr),
		IngestToken:      "token",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "http://localhost:8080",
		},
	}

	// Act
	err := config.Validate()

	// Assert
	require.NoError(t, err)

	assert.NotNil(t, config.unstructuredEndpoint)
	assert.Equal(t, "localhost:8080", config.unstructuredEndpoint.Host)
	assert.Equal(t, unstructuredPath, config.unstructuredEndpoint.Path)

	assert.NotNil(t, config.structuredEndpoint)
	assert.Equal(t, "localhost:8080", config.structuredEndpoint.Host)
	assert.Equal(t, structuredPath, config.structuredEndpoint.Path)

	assert.Equal(t, map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer token",
		"User-Agent":    "opentelemetry-collector-contrib Humio",
	}, config.Headers)
}

func TestValidateCustomHeaders(t *testing.T) {
	//Arrange
	config := &Config{
		ExporterSettings: config.NewExporterSettings(typeStr),
		IngestToken:      "token",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "http://localhost:8080",
			Headers: map[string]string{
				"User-Agent":   "Humio",
				"Content-Type": "application/json",
			},
		},
	}

	// Act
	err := config.Validate()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer token",
		"User-Agent":    "Humio",
	}, config.Headers)
}

func TestValidateErrors(t *testing.T) {
	// Arrange
	testCases := []struct {
		desc    string
		config  *Config
		wantErr bool
	}{
		{
			desc: "Missing ingest token",
			config: &Config{
				ExporterSettings: config.NewExporterSettings(typeStr),
				IngestToken:      "",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "e",
				},
			},
			wantErr: true,
		},
		{
			desc: "Missing endpoint",
			config: &Config{
				ExporterSettings: config.NewExporterSettings(typeStr),
				IngestToken:      "t",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "",
				},
			},
			wantErr: true,
		},
		{
			desc: "Override tags",
			config: &Config{
				ExporterSettings: config.NewExporterSettings(typeStr),
				IngestToken:      "t",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "e",
				},
				DisableServiceTag: true,
				Tags:              map[string]string{"k": "v"},
			},
			wantErr: false,
		},
		{
			desc: "Missing custom tags",
			config: &Config{
				ExporterSettings: config.NewExporterSettings(typeStr),
				IngestToken:      "t",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "e",
				},
				DisableServiceTag: true,
			},
			wantErr: true,
		},
		{
			desc: "Unix with time zone",
			config: &Config{
				ExporterSettings: config.NewExporterSettings(typeStr),
				IngestToken:      "t",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "e",
				},
				Traces: TracesConfig{
					UnixTimestamps: true,
					TimeZone:       "z",
				},
			},
			wantErr: false,
		},
		{
			desc: "Missing time zone",
			config: &Config{
				ExporterSettings: config.NewExporterSettings(typeStr),
				IngestToken:      "t",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "e",
				},
				Traces: TracesConfig{
					UnixTimestamps: true,
				},
			},
			wantErr: true,
		},
		{
			desc: "Error creating URLs",
			config: &Config{
				ExporterSettings: config.NewExporterSettings(typeStr),
				IngestToken:      "t",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "\n\t",
				},
			},
			wantErr: true,
		},
		{
			desc: "Invalid Content-Type header",
			config: &Config{
				ExporterSettings: config.NewExporterSettings(typeStr),
				IngestToken:      "t",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "e",
					Headers: map[string]string{
						"Content-Type": "text/plain",
					},
				},
			},
			wantErr: true,
		},
		{
			desc: "User-provided Authorization header",
			config: &Config{
				ExporterSettings: config.NewExporterSettings(typeStr),
				IngestToken:      "t",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "e",
					Headers: map[string]string{
						"Authorization": "Bearer mytoken",
					},
				},
			},
			wantErr: true,
		},
	}

	// Act / Assert
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			if err := tC.config.Validate(); (err != nil) != tC.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tC.wantErr)
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
		IngestToken: "t",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "http://localhost:8080",
		},
	}

	// Act
	actual, err := c.getEndpoint(structuredPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestGetEndpointError(t *testing.T) {
	// Arrange
	c := Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "\n\t",
		},
	}

	// Act
	result, err := c.getEndpoint(structuredPath)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
}
