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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfigNewExporter(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 2, len(cfg.Exporters))

	actualCfg := cfg.Exporters[config.NewComponentIDWithName(typeStr, "allsettings")].(*Config)
	expectedCfg := Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "allsettings")),
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Headers: map[string]string{
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
	}
	require.Equal(t, &expectedCfg, actualCfg)
}

func TestIsLegacy(t *testing.T) {
	testCases := []struct {
		desc    string
		cfg     *Config
		outcome bool
	}{
		{
			// the default mode for an empty config is the new logic
			desc: "not legacy",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "https://loki.example.com",
				},
			},
			outcome: false,
		},
		{
			desc: "format is set to body",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "https://loki.example.com",
				},
				Format: stringp("body"),
			},
			outcome: true,
		},
		{
			desc: "a label is specified",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "https://loki.example.com",
				},
				Labels: &LabelsConfig{
					Attributes: map[string]string{"some_attribute": "some_value"},
				},
			},
			outcome: true,
		},
		{
			desc: "a tenant is specified",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "https://loki.example.com",
				},
				Tenant: &Tenant{
					Source: "static",
					Value:  "acme",
				},
			},
			outcome: true,
		},
		{
			desc: "a tenant ID is specified",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "https://loki.example.com",
				},
				TenantID: stringp("acme"),
			},
			outcome: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			assert.Equal(t, tC.outcome, tC.cfg.isLegacy())

			// all configs from this table test are valid:
			assert.NoError(t, tC.cfg.Validate())
		})
	}
}

func stringp(str string) *string {
	return &str
}
