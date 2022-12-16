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
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/featuregate"
)

// setPdataFeatureGateForTest changes the pdata feature gate during a test.
// usage: defer SetPdataFeatureGateForTest(true)()
func setPdataFeatureGateForTest(t testing.TB, enabled bool) func() {
	originalValue := featuregate.GetRegistry().IsEnabled(pdataExporterFeatureGate)
	require.NoError(t, featuregate.GetRegistry().Apply(map[string]bool{pdataExporterFeatureGate: enabled}))
	return func() {
		require.NoError(t, featuregate.GetRegistry().Apply(map[string]bool{pdataExporterFeatureGate: originalValue}))
	}
}

func TestLoadConfig(t *testing.T) {
	defer setPdataFeatureGateForTest(t, true)()
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(typeStr, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	assert.Equal(t, sanitize(cfg.(*Config)), sanitize(factory.CreateDefaultConfig().(*Config)))

	sub, err = cm.Sub(component.NewIDWithName(typeStr, "customname").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	assert.Equal(t, sanitize(cfg.(*Config)),
		&Config{
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
