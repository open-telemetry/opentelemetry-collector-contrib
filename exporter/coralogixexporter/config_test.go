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

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories, _ := componenttest.NopFactories()
	factory := NewFactory()
	factories.Exporters[typestr] = factory
	// t.Log("new exporter " + typestr)
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("example", "config.yaml"), factories)
	require.NoError(t, err)
	apiConfig := cfg.Exporters[config.NewComponentID(typestr)].(*Config)
	err = apiConfig.Validate()
	require.NoError(t, err)
	assert.Equal(t, apiConfig, &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID("coralogix")),
		QueueSettings:    exporterhelper.NewDefaultQueueSettings(),
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
		PrivateKey:       "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		AppName:          "APP_NAME",
		TimeoutSettings:  exporterhelper.DefaultTimeoutSettings(),
		GRPCClientSettings: configgrpc.GRPCClientSettings{
			Endpoint:    "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			Compression: "",
			TLSSetting: configtls.TLSClientSetting{
				TLSSetting:         configtls.TLSSetting{},
				Insecure:           false,
				InsecureSkipVerify: false,
				ServerName:         "",
			},
			ReadBufferSize:  0,
			WriteBufferSize: 0,
			WaitForReady:    false,
			Headers:         map[string]string{"ACCESS_TOKEN": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", "appName": "APP_NAME"},
			BalancerName:    "",
		},
	})
}

func TestExporter(t *testing.T) {
	factories, _ := componenttest.NopFactories()
	factory := NewFactory()
	factories.Exporters[typestr] = factory
	cfg, _ := servicetest.LoadConfigAndValidate(filepath.Join("example", "config.yaml"), factories)
	apiConfig := cfg.Exporters[config.NewComponentID(typestr)].(*Config)
	params := componenttest.NewNopExporterCreateSettings()
	te := newCoralogixExporter(apiConfig, params)
	te.client.startConnection(context.Background(), componenttest.NewNopHost())
	assert.NotNil(t, te, "failed to create trace exporter")
	td := pdata.NewTraces()
	err := te.tracesPusher(context.Background(), td)
	assert.Nil(t, err)
}
