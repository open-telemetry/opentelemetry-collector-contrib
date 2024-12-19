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
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver/internal/metadata"
)

func loadConf(tb testing.TB, path string, id component.ID) *confmap.Conf {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", path))
	require.NoError(tb, err)
	sub, err := cm.Sub(id.String())
	require.NoError(tb, err)
	return sub
}

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
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 2 * time.Second,
					InitialDelay:       time.Second,
					Timeout:            20 * time.Second,
				},
				Config: docker.Config{
					Endpoint:         "http://example.com/",
					DockerAPIVersion: "1.40",

					Timeout: 20 * time.Second,
					ExcludedImages: []string{
						"undesired-container",
						"another-*-container",
					},
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
			sub := loadConf(t, "config.yaml", tt.id)
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			if diff := cmp.Diff(tt.expected, cfg, cmpopts.IgnoreUnexported(metadata.MetricConfig{}), cmpopts.IgnoreUnexported(metadata.ResourceAttributeConfig{})); diff != "" {
				t.Errorf("Config mismatch (-expected +actual):\n%s", diff)
			}
		})
	}
}

func TestValidateErrors(t *testing.T) {
	cfg := &Config{ControllerConfig: scraperhelper.NewDefaultControllerConfig(), Config: docker.Config{
		DockerAPIVersion: "1.25",
	}}
	assert.Equal(t, "endpoint must be specified", component.ValidateConfig(cfg).Error())

	cfg = &Config{
		Config: docker.Config{
			DockerAPIVersion: "1.21",
			Endpoint:         "someEndpoint",
		},
		ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: 1 * time.Second},
	}
	assert.Equal(t, `"api_version" 1.21 must be at least 1.25`, component.ValidateConfig(cfg).Error())

	cfg = &Config{
		Config: docker.Config{
			Endpoint:         "someEndpoint",
			DockerAPIVersion: "1.25",
		},
		ControllerConfig: scraperhelper.ControllerConfig{},
	}
	assert.Equal(t, `"collection_interval": requires positive value`, component.ValidateConfig(cfg).Error())
}

func TestApiVersionCustomError(t *testing.T) {
	sub := loadConf(t, "api_version_float.yaml", component.NewID(metadata.Type))
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	err := sub.Unmarshal(cfg)
	assert.ErrorContains(t, err,
		`Hint: You may want to wrap the 'api_version' value in quotes (api_version: "1.40")`,
	)

	sub = loadConf(t, "api_version_string.yaml", component.NewID(metadata.Type))
	err = sub.Unmarshal(cfg)
	require.NoError(t, err)
}
