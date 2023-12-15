// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	assert.Equal(t, sanitize(cfg.(*Config)), sanitize(factory.CreateDefaultConfig().(*Config)))

	sub, err = cm.Sub(component.NewIDWithName(metadata.Type, "customname").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	assert.Equal(t,
		&Config{
			TimeoutSettings: exporterhelper.TimeoutSettings{
				Timeout: 20 * time.Second,
			},
			Config: collector.Config{
				ProjectID: "my-project",
				UserAgent: "opentelemetry-collector-contrib {{version}}",
				LogConfig: collector.LogConfig{
					ServiceResourceLabels: true,
				},
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
			QueueSettings: exporterhelper.QueueSettings{
				Enabled:      true,
				NumConsumers: 2,
				QueueSize:    10,
			},
		},
		sanitize(cfg.(*Config)))
}

func sanitize(cfg *Config) *Config {
	cfg.Config.MetricConfig.MapMonitoredResource = nil
	cfg.Config.MetricConfig.GetMetricName = nil
	return cfg
}
