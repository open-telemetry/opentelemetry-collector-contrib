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

package pulsarexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory

	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	require.NoError(t, err)
	require.Equal(t, 1, len(cfg.Exporters))

	c := cfg.Exporters[config.NewComponentID(typeStr)].(*Config)

	assert.Equal(t, &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		TimeoutSettings: exporterhelper.TimeoutSettings{
			Timeout: 20 * time.Second,
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
		Endpoint:              "pulsar://localhost:6650",
		Topic:                 "spans",
		Encoding:              "otlp-spans",
		TLSTrustCertsFilePath: "ca.pem",
		Authentication:        Authentication{TLS: &TLS{CertFile: "cert.pem", KeyFile: "key.pem"}},
	}, c)
}

func TestClientOptions(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory

	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	require.NoError(t, err)
	require.Equal(t, 1, len(cfg.Exporters))

	c := cfg.Exporters[config.NewComponentID(typeStr)].(*Config)

	options := c.clientOptions()

	assert.Equal(t, &pulsar.ClientOptions{
		URL:                   "pulsar://localhost:6650",
		TLSTrustCertsFilePath: "ca.pem",
		Authentication:        pulsar.NewAuthenticationTLS("cert.pem", "key.pem"),
	}, &options)

}
