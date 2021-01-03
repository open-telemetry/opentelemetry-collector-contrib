// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lokiexporter

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/translator/conventions"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 2, len(cfg.Exporters))

	actualCfg := cfg.Exporters["loki/allsettings"].(*Config)
	expectedCfg := Config{
		ExporterSettings: configmodels.ExporterSettings{TypeVal: typeStr, NameVal: "loki/allsettings"},
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Headers: map[string]string{
				"x-scope-orgid": "example",
			},
			Endpoint: "https://loki:3100/loki/api/v1/push",
			TLSSetting: configtls.TLSClientSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile:   "/var/lib/mycert.pem",
					CertFile: "certfile",
					KeyFile:  "keyfile",
				},
				Insecure: true,
			},
			ReadBufferSize:  123,
			WriteBufferSize: 345,
			Timeout:         time.Second * 10,
		},
		RetrySettings: exporterhelper.RetrySettings{
			Enabled:         true,
			InitialInterval: 10 * time.Second,
			MaxInterval:     1 * time.Minute,
			MaxElapsedTime:  10 * time.Minute,
		},
		QueueSettings: exporterhelper.QueueSettings{
			Enabled:      true,
			NumConsumers: 2,
			QueueSize:    10,
		},
		Labels: LabelsConfig{
			Default:             map[string]string{"example": "value"},
			AttributesForLabels: []string{conventions.AttributeContainerName, conventions.AttributeK8sCluster, "severity"},
		},
	}
	assert.Equal(t, &expectedCfg, actualCfg)
}

func TestConfig_validate(t *testing.T) {
	const validEndpoint = "https://validendpoint.local"
	validLabelsConfig := LabelsConfig{
		Default:             map[string]string{},
		AttributesForLabels: []string{"container.name", "k8s.cluster.name", "severity"},
	}

	type fields struct {
		ExporterSettings configmodels.ExporterSettings
		Endpoint         string
		Source           string
		CredentialFile   string
		Audience         string
		Labels           LabelsConfig
	}
	tests := []struct {
		name         string
		fields       fields
		errorMessage string
		shouldError  bool
	}{
		{
			name: "Test valid endpoint",
			fields: fields{
				Endpoint: validEndpoint,
				Labels:   validLabelsConfig,
			},
			shouldError: false,
		},
		{
			name: "Test missing endpoint",
			fields: fields{
				Endpoint: "",
				Labels:   validLabelsConfig,
			},
			errorMessage: "\"endpoint\" must be a valid URL",
			shouldError:  true,
		},
		{
			name: "Test invalid endpoint",
			fields: fields{
				Endpoint: "this://is:an:invalid:endpoint.com",
				Labels:   validLabelsConfig,
			},
			errorMessage: "\"endpoint\" must be a valid URL",
			shouldError:  true,
		},
		{
			name: "Test missing `labels.attributes_to_labels`",
			fields: fields{
				Endpoint: validEndpoint,
				Labels: LabelsConfig{
					Default:             map[string]string{},
					AttributesForLabels: nil,
				},
			},
			errorMessage: "\"labels.attributes_for_labels\" must have a least one label",
			shouldError:  true,
		},
		{
			name: "Test valid `labels` config",
			fields: fields{
				Endpoint: validEndpoint,
				Labels:   validLabelsConfig,
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.ExporterSettings = tt.fields.ExporterSettings
			cfg.Endpoint = tt.fields.Endpoint
			cfg.Labels = tt.fields.Labels

			err := cfg.validate()
			if (err != nil) != tt.shouldError {
				t.Errorf("validate() error = %v, shouldError %v", err, tt.shouldError)
				return
			}

			if tt.shouldError {
				assert.Error(t, err)
				if len(tt.errorMessage) != 0 {
					assert.Equal(t, tt.errorMessage, err.Error())
				}
			}
		})
	}
}
