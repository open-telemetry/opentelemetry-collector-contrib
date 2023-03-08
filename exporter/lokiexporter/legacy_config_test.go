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
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_legacy.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(typeStr, "allsettings"),
			expected: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Headers: map[string]configopaque.String{
						"X-Custom-Header": "loki_rocks",
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
					Enabled:             true,
					InitialInterval:     10 * time.Second,
					MaxInterval:         1 * time.Minute,
					MaxElapsedTime:      10 * time.Minute,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				QueueSettings: exporterhelper.QueueSettings{
					Enabled:      true,
					NumConsumers: 2,
					QueueSize:    10,
				},
				TenantID: stringp("example"),
				Labels: &LabelsConfig{
					Attributes: map[string]string{
						conventions.AttributeContainerName:  "container_name",
						conventions.AttributeK8SClusterName: "k8s_cluster_name",
						"severity":                          "severity",
					},
					ResourceAttributes: map[string]string{
						"resource.name": "resource_name",
						"severity":      "severity",
					},
					RecordAttributes: map[string]string{
						"traceID": "traceid",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(typeStr, "json"),
			expected: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Headers:  map[string]configopaque.String{},
					Endpoint: "https://loki:3100/loki/api/v1/push",
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting: configtls.TLSSetting{
							CAFile:   "",
							CertFile: "",
							KeyFile:  "",
						},
						Insecure: false,
					},
					ReadBufferSize:  0,
					WriteBufferSize: 524288,
					Timeout:         time.Second * 30,
				},
				RetrySettings: exporterhelper.RetrySettings{
					Enabled:             true,
					InitialInterval:     5 * time.Second,
					MaxInterval:         30 * time.Second,
					MaxElapsedTime:      5 * time.Minute,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				QueueSettings: exporterhelper.QueueSettings{
					Enabled:      true,
					NumConsumers: 10,
					QueueSize:    5000,
				},
				TenantID: stringp("example"),
				Labels: &LabelsConfig{
					RecordAttributes: map[string]string{
						"traceID": "traceid",
					},
				},
				Format: stringp("json"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfig_validate(t *testing.T) {
	const validEndpoint = "https://validendpoint.local"
	tenantIDGlobex := "globex"

	validAttribLabelsConfig := &LabelsConfig{
		Attributes: testValidAttributesWithMapping,
	}
	validResourceLabelsConfig := &LabelsConfig{
		ResourceAttributes: testValidResourceWithMapping,
	}

	type fields struct {
		Endpoint       string
		Source         string
		CredentialFile string
		Audience       string
		Labels         *LabelsConfig
		TenantID       *string
		Tenant         *Tenant
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
				Labels:   validAttribLabelsConfig,
			},
			shouldError: false,
		},
		{
			name: "with missing endpoint",
			fields: fields{
				Endpoint: "",
				Labels:   validAttribLabelsConfig,
			},
			errorMessage: "\"endpoint\" must be a valid URL",
			shouldError:  true,
		},
		{
			name: "with invalid endpoint",
			fields: fields{
				Endpoint: "this://is:an:invalid:endpoint.com",
				Labels:   validAttribLabelsConfig,
			},
			errorMessage: "\"endpoint\" must be a valid URL",
			shouldError:  true,
		},
		{
			name: "with missing `labels.attributes` and missing `labels.resource`",
			fields: fields{
				Endpoint: validEndpoint,
				Labels: &LabelsConfig{
					Attributes:         nil,
					ResourceAttributes: nil,
				},
			},
			errorMessage: "\"labels.attributes\", \"labels.resource\", or \"labels.record\" must be configured with at least one attribute",
			shouldError:  true,
		},
		{
			name: "with missing `labels.attributes`",
			fields: fields{
				Endpoint: validEndpoint,
				Labels:   validResourceLabelsConfig,
			},
			shouldError: false,
		},
		{
			name: "with missing `labels.resource`",
			fields: fields{
				Endpoint: validEndpoint,
				Labels:   validAttribLabelsConfig,
			},
			shouldError: false,
		},
		{
			name: "with valid `labels.record`",
			fields: fields{
				Endpoint: validEndpoint,
				Labels: &LabelsConfig{
					RecordAttributes: map[string]string{
						"traceID":   "traceID",
						"spanID":    "spanID",
						"severity":  "severity",
						"severityN": "severityN",
					},
				},
			},
			shouldError: false,
		},
		{
			name: "with invalid `labels.record`",
			fields: fields{
				Endpoint: validEndpoint,
				Labels: &LabelsConfig{
					RecordAttributes: map[string]string{
						"invalid": "Invalid",
					},
				},
			},
			shouldError: true,
		},
		{
			name: "with missing `tenant.source`",
			fields: fields{
				Endpoint: validEndpoint,
				Labels:   validAttribLabelsConfig,
				Tenant:   &Tenant{},
			},
			shouldError: true,
		},
		{
			name: "with invalid `tenant.source`",
			fields: fields{
				Endpoint: validEndpoint,
				Labels:   validAttribLabelsConfig,
				Tenant: &Tenant{
					Source: "invalid",
				},
			},
			shouldError: true,
		},
		{
			name: "with valid `tenant.source`",
			fields: fields{
				Endpoint: validEndpoint,
				Labels:   validAttribLabelsConfig,
				Tenant: &Tenant{
					Source: "static",
					Value:  "acme",
				},
			},
			shouldError: false,
		},
		{
			name: "with both tenantID and tenant",
			fields: fields{
				Endpoint: validEndpoint,
				Labels:   validAttribLabelsConfig,
				Tenant: &Tenant{
					Source: "static",
					Value:  "acme",
				},
				TenantID: &tenantIDGlobex,
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.Endpoint = tt.fields.Endpoint
			cfg.Labels = tt.fields.Labels

			if tt.fields.TenantID != nil {
				cfg.TenantID = tt.fields.TenantID
			}

			if tt.fields.Tenant != nil {
				cfg.Tenant = tt.fields.Tenant
			}

			err := component.ValidateConfig(cfg)
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
				Attributes:         map[string]string{},
				ResourceAttributes: map[string]string{},
			},
			errorMessage: "\"labels.attributes\", \"labels.resource\", or \"labels.record\" must be configured with at least one attribute",
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
			name: "with valid resource label map",
			labels: LabelsConfig{
				ResourceAttributes: map[string]string{
					"other.attribute": "other",
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
			name: "with invalid resource label map",
			labels: LabelsConfig{
				ResourceAttributes: map[string]string{
					"other.attribute": "invalid.label.name",
				},
			},
			errorMessage: "the label `invalid.label.name` in \"labels.resource\" is not a valid label name. Label names must match " + model.LabelNameRE.String(),
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
			mapping := tt.labels.getAttributes(tt.labels.Attributes)

			assert.Equal(t, tt.expectedMapping, mapping)
		})
	}
}

func TestResourcesConfig_getAttributes(t *testing.T) {
	tests := []struct {
		name            string
		labels          LabelsConfig
		expectedMapping map[string]model.LabelName
	}{
		{
			name: "with attributes without label mapping",
			labels: LabelsConfig{
				ResourceAttributes: map[string]string{
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
				ResourceAttributes: map[string]string{
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
				ResourceAttributes: map[string]string{
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
			mapping := tt.labels.getAttributes(tt.labels.ResourceAttributes)

			assert.Equal(t, tt.expectedMapping, mapping)
		})
	}
}
