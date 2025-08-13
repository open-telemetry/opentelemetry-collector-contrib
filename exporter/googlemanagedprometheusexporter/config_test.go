// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlemanagedprometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlemanagedprometheusexporter"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/googlemanagedprometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlemanagedprometheusexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[metadata.Type] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Len(t, cfg.Exporters, 2)

	r0 := cfg.Exporters[component.NewID(metadata.Type)].(*Config)
	assert.Equal(t, r0, factory.CreateDefaultConfig().(*Config))

	r1 := cfg.Exporters[component.NewIDWithName(metadata.Type, "customname")].(*Config)
	assert.Equal(t, &Config{
		TimeoutSettings: exporterhelper.TimeoutConfig{
			Timeout: 20 * time.Second,
		},
		GMPConfig: GMPConfig{
			ProjectID: "my-project",
			UserAgent: "opentelemetry-collector-contrib {{version}}",
			MetricConfig: MetricConfig{
				Config: googlemanagedprometheus.Config{
					AddMetricSuffixes: false,
					ExtraMetricsConfig: googlemanagedprometheus.ExtraMetricsConfig{
						EnableTargetInfo: false,
						EnableScopeInfo:  false,
					},
				},
				Prefix: "my-metric-domain.com",
				ResourceFilters: []collector.ResourceFilter{
					{
						Prefix: "cloud",
					},
					{
						Prefix: "k8s",
					},
					{
						Prefix: "faas",
					},
					{
						Regex: "container.id",
					},
					{
						Regex: "process.pid",
					},
					{
						Regex: "host.name",
					},
					{
						Regex: "host.id",
					},
				},
				CumulativeNormalization: false,
			},
		},
		QueueSettings: exporterhelper.QueueBatchConfig{
			Enabled:      true,
			NumConsumers: 2,
			QueueSize:    10,
			Sizer:        exporterhelper.RequestSizerTypeRequests,
		},
	}, r1)
}
