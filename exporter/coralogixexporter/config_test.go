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
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories, _ := componenttest.NopFactories()
	factory := NewFactory()
	factories.Exporters[typeStr] = factory

	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	require.NoError(t, err)

	apiConfig := cfg.Exporters[config.NewComponentID(typeStr)].(*Config)
	err = apiConfig.Validate()
	require.NoError(t, err)

	assert.Equal(t, apiConfig, &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID("coralogix")),
		QueueSettings:    exporterhelper.NewDefaultQueueSettings(),
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
		PrivateKey:       "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		AppName:          "APP_NAME",
		// Deprecated: [v0.47.0] SubSystem will remove in the next version
		SubSystem:       "SUBSYSTEM_NAME",
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
		Metrics: configgrpc.GRPCClientSettings{
			Endpoint:        "https://",
			Compression:     "gzip",
			WriteBufferSize: 512 * 1024,
			Headers:         map[string]string{},
		},
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
	factories.Exporters[typeStr] = factory
	cfg, _ := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	apiConfig := cfg.Exporters[config.NewComponentID(typeStr)].(*Config)
	params := componenttest.NewNopExporterCreateSettings()
	te, err := newCoralogixExporter(apiConfig, params)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")
	assert.NoError(t, te.client.startConnection(context.Background(), componenttest.NewNopHost()))
	td := ptrace.NewTraces()
	assert.NoError(t, te.tracesPusher(context.Background(), td))
}
