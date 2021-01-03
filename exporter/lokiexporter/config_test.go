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
		AttributesForLabels: []string{conventions.AttributeContainerName, conventions.AttributeK8sCluster, "severity"},
	}
	assert.Equal(t, &expectedCfg, actualCfg)
}
