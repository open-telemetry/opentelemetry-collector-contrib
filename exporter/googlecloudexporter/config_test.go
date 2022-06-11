// Copyright 2019, OpenTelemetry Authors
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

package googlecloudexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/service/featuregate"
	"go.opentelemetry.io/collector/service/servicetest"
)

// setPdataFeatureGateForTest changes the pdata feature gate during a test.
// usage: defer SetPdataFeatureGateForTest(true)()
func setPdataFeatureGateForTest(enabled bool) func() {
	originalValue := featuregate.GetRegistry().IsEnabled(pdataExporterFeatureGate)
	featuregate.GetRegistry().Apply(map[string]bool{pdataExporterFeatureGate: enabled})
	return func() {
		featuregate.GetRegistry().Apply(map[string]bool{pdataExporterFeatureGate: originalValue})
	}
}

func TestLoadConfig(t *testing.T) {
	defer setPdataFeatureGateForTest(true)()
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Exporters), 2)

	r0 := cfg.Exporters[config.NewComponentID(typeStr)].(*Config)
	assert.Equal(t, sanitize(r0), sanitize(factory.CreateDefaultConfig().(*Config)))

	r1 := cfg.Exporters[config.NewComponentIDWithName(typeStr, "customname")].(*Config)
	assert.Equal(t, sanitize(r1),
		&Config{
			ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "customname")),
			TimeoutSettings: exporterhelper.TimeoutSettings{
				Timeout: 20 * time.Second,
			},
			Config: collector.Config{
				ProjectID: "my-project",
				UserAgent: "opentelemetry-collector-contrib {{version}}",
				MetricConfig: collector.MetricConfig{
					Prefix:                           "prefix",
					SkipCreateMetricDescriptor:       true,
					KnownDomains:                     []string{"googleapis.com", "kubernetes.io", "istio.io", "knative.dev"},
					CreateMetricDescriptorBufferSize: 10,
					InstrumentationLibraryLabels:     true,
					ServiceResourceLabels:            true,
					ClientConfig: collector.ClientConfig{
						Endpoint:    "test-metric-endpoint",
						UseInsecure: true,
					},
				},
				TraceConfig: collector.TraceConfig{
					ClientConfig: collector.ClientConfig{
						Endpoint:    "test-trace-endpoint",
						UseInsecure: true,
					},
				},
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
		})
}

func sanitize(cfg *Config) *Config {
	cfg.Config.MetricConfig.MapMonitoredResource = nil
	cfg.Config.MetricConfig.GetMetricName = nil
	return cfg
}
