// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerstatsreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "allsettings"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: 2 * time.Second,
				},

				Endpoint:         "http://example.com/",
				Timeout:          20 * time.Second,
				DockerAPIVersion: 1.24,

				ExcludedImages: []string{
					"undesired-container",
					"another-*-container",
				},

				ContainerLabelsToMetricLabels: map[string]string{
					"my.container.label":       "my-metric-label",
					"my.other.container.label": "my-other-metric-label",
				},

				EnvVarsToMetricLabels: map[string]string{
					"MY_ENVIRONMENT_VARIABLE":       "my-metric-label",
					"MY_OTHER_ENVIRONMENT_VARIABLE": "my-other-metric-label",
				},
				MetricsBuilderConfig: func() metadata.MetricsBuilderConfig {
					m := metadata.DefaultMetricsBuilderConfig()
					m.Metrics.ContainerCPUUsageSystem = metadata.MetricConfig{
						Enabled: false,
					}
					m.Metrics.ContainerMemoryTotalRss = metadata.MetricConfig{
						Enabled: true,
					}
					return m
				}(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			if diff := cmp.Diff(tt.expected, cfg, cmpopts.IgnoreUnexported(metadata.MetricConfig{})); diff != "" {
				t.Errorf("Config mismatch (-expected +actual):\n%s", diff)
			}
		})
	}
}

func TestValidateErrors(t *testing.T) {
	cfg := &Config{}
	assert.Equal(t, "endpoint must be specified", component.ValidateConfig(cfg).Error())

	cfg = &Config{Endpoint: "someEndpoint"}
	assert.Equal(t, "collection_interval must be a positive duration", component.ValidateConfig(cfg).Error())

	cfg = &Config{ScraperControllerSettings: scraperhelper.ScraperControllerSettings{CollectionInterval: 1 * time.Second}, Endpoint: "someEndpoint", DockerAPIVersion: 1.21}
	assert.Equal(t, "api_version must be at least 1.22", component.ValidateConfig(cfg).Error())
}
