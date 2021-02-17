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

	"github.com/prometheus/common/model"
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
				"x-custom-header": "loki_rocks",
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
		TenantID: "example",
		Labels: LabelsConfig{
			Attributes: map[string]string{
				conventions.AttributeContainerName: "container_name",
				conventions.AttributeK8sCluster:    "k8s_cluster_name",
				"severity":                         "severity",
			},
		},
	}
	require.Equal(t, &expectedCfg, actualCfg)
}

func TestConfig_validate(t *testing.T) {
	const validEndpoint = "https://validendpoint.local"

	validLabelsConfig := LabelsConfig{
		Attributes: testValidAttributesWithMapping,
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
			name: "with valid endpoint",
			fields: fields{
				Endpoint: validEndpoint,
				Labels:   validLabelsConfig,
			},
			shouldError: false,
		},
		{
			name: "with missing endpoint",
			fields: fields{
				Endpoint: "",
				Labels:   validLabelsConfig,
			},
			errorMessage: "\"endpoint\" must be a valid URL",
			shouldError:  true,
		},
		{
			name: "with invalid endpoint",
			fields: fields{
				Endpoint: "this://is:an:invalid:endpoint.com",
				Labels:   validLabelsConfig,
			},
			errorMessage: "\"endpoint\" must be a valid URL",
			shouldError:  true,
		},
		{
			name: "with missing `labels.attributes`",
			fields: fields{
				Endpoint: validEndpoint,
				Labels: LabelsConfig{
					Attributes: nil,
				},
			},
			errorMessage: "\"labels.attributes\" must be configured with at least one attribute",
			shouldError:  true,
		},
		{
			name: "with `labels.attributes` set",
			fields: fields{
				Endpoint: validEndpoint,
				Labels: LabelsConfig{
					Attributes: testValidAttributesWithMapping,
				},
			},
			shouldError: false,
		},
		{
			name: "with valid `labels` config",
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

func TestLabelsConfig_validate(t *testing.T) {
	tests := []struct {
		name         string
		labels       LabelsConfig
		errorMessage string
		shouldError  bool
	}{
		{
			name: "with no attributes",
			labels: LabelsConfig{
				Attributes: map[string]string{},
			},
			errorMessage: "\"labels.attributes\" must be configured with at least one attribute",
			shouldError:  true,
		},
		{
			name: "with valid attribute label map",
			labels: LabelsConfig{
				Attributes: map[string]string{
					"some.attribute": "some_attribute",
				},
			},
			shouldError: false,
		},
		{
			name: "with invalid attribute label map",
			labels: LabelsConfig{
				Attributes: map[string]string{
					"some.attribute": "invalid.label.name",
				},
			},
			errorMessage: "the label `invalid.label.name` in \"labels.attributes\" is not a valid label name. Label names must match " + model.LabelNameRE.String(),
			shouldError:  true,
		},
		{
			name: "with attribute having an invalid label name and no map configured",
			labels: LabelsConfig{
				Attributes: map[string]string{
					"invalid.attribute": "",
				},
			},
			errorMessage: "the label `invalid.attribute` in \"labels.attributes\" is not a valid label name. Label names must match " + model.LabelNameRE.String(),
			shouldError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.labels.validate()
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

func TestLabelsConfig_getAttributes(t *testing.T) {
	tests := []struct {
		name            string
		labels          LabelsConfig
		expectedMapping map[string]model.LabelName
	}{
		{
			name: "with attributes without label mapping",
			labels: LabelsConfig{
				Attributes: map[string]string{
					"attribute_1": "",
					"attribute_2": "",
				},
			},
			expectedMapping: map[string]model.LabelName{
				"attribute_1": model.LabelName("attribute_1"),
				"attribute_2": model.LabelName("attribute_2"),
			},
		},
		{
			name: "with attributes and label mapping",
			labels: LabelsConfig{
				Attributes: map[string]string{
					"attribute.1": "attribute_1",
					"attribute.2": "attribute_2",
				},
			},
			expectedMapping: map[string]model.LabelName{
				"attribute.1": model.LabelName("attribute_1"),
				"attribute.2": model.LabelName("attribute_2"),
			},
		},
		{
			name: "with attributes and without label mapping",
			labels: LabelsConfig{
				Attributes: map[string]string{
					"attribute.1": "attribute_1",
					"attribute2":  "",
				},
			},
			expectedMapping: map[string]model.LabelName{
				"attribute.1": model.LabelName("attribute_1"),
				"attribute2":  model.LabelName("attribute2"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapping := tt.labels.getAttributes()

			assert.Equal(t, tt.expectedMapping, mapping)
		})
	}
}
