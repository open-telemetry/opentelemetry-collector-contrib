// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package awsprometheusremotewriteexporter provides a Prometheus Remote Write Exporter with AWS Sigv4 authentication
package awsprometheusremotewriteexporter

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
	prw "go.opentelemetry.io/collector/exporter/prometheusremotewriteexporter"
)

// TestLoadConfig checks whether yaml configuration can be loaded correctly
func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	// From the default configurations -- checks if a correct exporter is instantiated
	e0 := cfg.Exporters["awsprometheusremotewrite"]
	cfgDefault := factory.CreateDefaultConfig()
	// testing function equality is not supported in Go hence these will be ignored for this test
	cfgDefault.(*Config).HTTPClientSettings.CustomRoundTripper = nil
	e0.(*Config).HTTPClientSettings.CustomRoundTripper = nil
	assert.Equal(t, e0, cfgDefault)

	// checks if the correct Config struct can be instantiated from testdata/config.yaml
	e1 := cfg.Exporters["awsprometheusremotewrite/2"]
	cfgComplete := &Config{
		Config: prw.Config{
			ExporterSettings: configmodels.ExporterSettings{
				NameVal: "awsprometheusremotewrite/2",
				TypeVal: "awsprometheusremotewrite",
			},
			TimeoutSettings: exporterhelper.CreateDefaultTimeoutSettings(),
			QueueSettings: exporterhelper.QueueSettings{
				Enabled:      true,
				NumConsumers: 2,
				QueueSize:    10,
			},
			RetrySettings: exporterhelper.RetrySettings{
				Enabled:         true,
				InitialInterval: 10 * time.Second,
				MaxInterval:     1 * time.Minute,
				MaxElapsedTime:  10 * time.Minute,
			},
			Namespace:      "test-space",
			ExternalLabels: map[string]string{"key1": "value1", "key2": "value2"},
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint: "http://localhost:9009",
				TLSSetting: configtls.TLSClientSetting{
					TLSSetting: configtls.TLSSetting{
						CAFile: "/var/lib/mycert.pem",
					},
					Insecure: false,
				},
				ReadBufferSize: 0,

				WriteBufferSize: 512 * 1024,

				Timeout: 5 * time.Second,

				Headers: map[string]string{
					"prometheus-remote-write-version": "0.1.0",
					"x-scope-orgid":                   "234"},
			},
		},
		AuthConfig: AuthConfig{
			Region:  "us-west-2",
			Service: "service-name",
		},
	}
	// testing function equality is not supported in Go hence these will be ignored for this test
	cfgComplete.HTTPClientSettings.CustomRoundTripper = nil
	e1.(*Config).HTTPClientSettings.CustomRoundTripper = nil
	assert.Equal(t, e1, cfgComplete)
}
