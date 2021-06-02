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
func loadConfig(t *testing.T, file string) (*config.Config, error) {
	// Initialize exporter factory
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory

	// Load configurations
	return configtest.LoadConfigAndValidate(path.Join(".", "testdata", file), factories)
}

// Helper method to handle boilerplate of loading exporter configuration from file
func loadExporterConfig(t *testing.T, file string, id config.ComponentID) (config.Exporter, *Config) {
	// Initialize exporter factory
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory

	// Load configurations
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", file), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	actual := cfg.Exporters[id]

	def := factory.CreateDefaultConfig().(*Config)
	require.NotNil(t, def)

	return actual, def
}

func TestLoadWithDefaults(t *testing.T) {
	// Arrange / Act
	actual, expected := loadExporterConfig(t, "config.yaml", config.NewID(typeStr))
	expected.Traces.IngestToken = "00000000-0000-0000-0000-0000000000000"
	expected.Endpoint = "https://cloud.humio.com/"

	// Assert
	assert.Equal(t, expected, actual)
}

func TestLoadInvalidCompression(t *testing.T) {
	// Act
	_, err := loadConfig(t, "invalid-compression.yaml")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "the Content-Encoding header must")
}

func TestLoadInvalidTagStrategy(t *testing.T) {
	// Act
	_, err := loadConfig(t, "invalid-tag.yaml")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tagging strategy must be one of")
}

func TestLoadAllSettings(t *testing.T) {
	// Arrange
	expected := &Config{
		ExporterSettings: config.NewExporterSettings(config.NewIDWithName(typeStr, "allsettings")),

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
			Endpoint:        "http://localhost:8080/",
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

		DisableCompression: true,
		Tag:                TagTraceID,
		Logs: LogsConfig{
			IngestToken: "00000000-0000-0000-0000-0000000000000",
			LogParser:   "custom-parser",
		},
		Traces: TracesConfig{
			IngestToken:    "00000000-0000-0000-0000-0000000000001",
			UnixTimestamps: true,
		},
	}

	// Act
	actual, _ := loadExporterConfig(t, "config.yaml", config.NewIDWithName(typeStr, "allsettings"))

	// Assert
	assert.Equal(t, expected, actual)
}

func TestValidate(t *testing.T) {
	// Arrange
	testCases := []struct {
		desc    string
		cfg     *Config
		wantErr bool
	}{
		{
			desc: "Valid minimal configuration",
			cfg: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
				Tag:              TagNone,
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost:8080",
				},
			},
			wantErr: false,
		},
		{
			desc: "Valid custom headers",
			cfg: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
				Tag:              TagNone,
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost:8080",
					Headers: map[string]string{
						"user-agent":       "Humio",
						"content-type":     "application/json",
						"content-encoding": "gzip",
					},
				},
			},
			wantErr: false,
		},
		{
			desc: "Valid compression disabled",
			cfg: &Config{
				ExporterSettings:   config.NewExporterSettings(config.NewID(typeStr)),
				DisableCompression: true,
				Tag:                TagNone,
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost:8080",
				},
			},
			wantErr: false,
		},
		{
			desc: "Missing endpoint",
			cfg: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
				Tag:              TagNone,
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "",
				},
			},
			wantErr: true,
		},
		{
			desc: "Override tag strategy",
			cfg: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
				Tag:              TagServiceName,
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "e",
				},
			},
			wantErr: false,
		},
		{
			desc: "Unix time",
			cfg: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
				Tag:              TagNone,
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "e",
				},
				Traces: TracesConfig{
					UnixTimestamps: true,
				},
			},
			wantErr: false,
		},
		{
			desc: "Error creating URLs",
			cfg: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
				Tag:              TagNone,
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "\n\t",
				},
			},
			wantErr: true,
		},
		{
			desc: "Invalid Content-Type header",
			cfg: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
				Tag:              TagNone,
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "e",
					Headers: map[string]string{
						"content-type": "text/plain",
					},
				},
			},
			wantErr: true,
		},
		{
			desc: "User-provided Authorization header",
			cfg: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
				Tag:              TagNone,
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "e",
					Headers: map[string]string{
						"authorization": "Bearer mytoken",
					},
				},
			},
			wantErr: true,
		},
		{
			desc: "Invalid content encoding",
			cfg: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
				Tag:              TagNone,
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "e",
					Headers: map[string]string{
						"content-encoding": "compress",
					},
				},
			},
			wantErr: true,
		},
		{
			desc: "Content encoding without compression",
			cfg: &Config{
				ExporterSettings:   config.NewExporterSettings(config.NewID(typeStr)),
				DisableCompression: true,
				Tag:                TagNone,
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "e",
					Headers: map[string]string{
						"content-encoding": "gzip",
					},
				},
			},
			wantErr: true,
		},
	}

	// Act / Assert
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			if err := tC.cfg.Validate(); (err != nil) != tC.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tC.wantErr)
			}
		})
	}
}

func TestSanitizeValid(t *testing.T) {
	//Arrange
	cfg := &Config{
		ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "http://localhost:8080",
		},
	}

	// Act
	err := cfg.sanitize()

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, cfg.unstructuredEndpoint)
	assert.Equal(t, "localhost:8080", cfg.unstructuredEndpoint.Host)
	assert.Equal(t, unstructuredPath, cfg.unstructuredEndpoint.Path)

	assert.NotNil(t, cfg.structuredEndpoint)
	assert.Equal(t, "localhost:8080", cfg.structuredEndpoint.Host)
	assert.Equal(t, structuredPath, cfg.structuredEndpoint.Path)

	assert.Equal(t, map[string]string{
		"content-type":     "application/json",
		"content-encoding": "gzip",
		"user-agent":       "opentelemetry-collector-contrib Humio",
	}, cfg.Headers)
}

func TestSanitizeCustomHeaders(t *testing.T) {
	//Arrange
	cfg := &Config{
		ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "http://localhost:8080",
			Headers: map[string]string{
				"user-agent":       "Humio",
				"content-type":     "application/json",
				"content-encoding": "gzip",
			},
		},
	}

	// Act
	err := cfg.sanitize()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"content-type":     "application/json",
		"content-encoding": "gzip",
		"user-agent":       "Humio",
	}, cfg.Headers)
}

func TestSanitizeNoCompression(t *testing.T) {
	//Arrange
	cfg := &Config{
		ExporterSettings:   config.NewExporterSettings(config.NewID(typeStr)),
		DisableCompression: true,
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "http://localhost:8080",
		},
	}

	// Act
	err := cfg.sanitize()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"content-type": "application/json",
		"user-agent":   "opentelemetry-collector-contrib Humio",
	}, cfg.Headers)
}

func TestGetEndpoint(t *testing.T) {
	// Arrange
	expected := &url.URL{
		Scheme: "http",
		Host:   "localhost:8080",
		Path:   structuredPath,
	}

	cfg := Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "http://localhost:8080",
		},
	}

	// Act
	actual, err := cfg.getEndpoint(structuredPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestGetEndpointError(t *testing.T) {
	// Arrange
	cfg := Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "\n\t",
		},
	}

	// Act
	result, err := cfg.getEndpoint(structuredPath)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
}
