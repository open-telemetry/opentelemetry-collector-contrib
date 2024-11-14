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
	"go.opentelemetry.io/collector/scraper/scraperhelper"

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
				MinDockerRetryWait: 1 * time.Second,
				MaxDockerRetryWait: 30 * time.Second,
				Logs: EventsConfig{
					Filters: map[string][]string{
						"type":  {"container", "image"},
						"event": {"start", "stop", "die"},
					},
					Since: "2024-01-01T00:00:00Z",
					Until: "2024-01-02T00:00:00Z",
				},
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
	tests := []struct {
		name        string
		cfg         *Config
		expectedErr string
	}{
		{
			name: "missing endpoint",
			cfg: &Config{
				Config: docker.Config{
					DockerAPIVersion: "1.25",
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: "endpoint must be specified",
		},
		{
			name: "outdated api version",
			cfg: &Config{
				Config: docker.Config{
					DockerAPIVersion: "1.21",
					Endpoint:         "someEndpoint",
				},
				ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: 1 * time.Second},
			},
			expectedErr: `"api_version" 1.21 must be at least 1.25`,
		},
		{
			name: "missing collection interval",
			cfg: &Config{
				Config: docker.Config{
					Endpoint:         "someEndpoint",
					DockerAPIVersion: "1.25",
				},
				ControllerConfig: scraperhelper.ControllerConfig{},
			},
			expectedErr: `"collection_interval": requires positive value`,
		},
		{
			name: "negative min retry wait",
			cfg: &Config{
				Config: docker.Config{
					Endpoint:         "unix:///var/run/docker.sock",
					DockerAPIVersion: "1.25",
				},
				MinDockerRetryWait: -1 * time.Second,
				MaxDockerRetryWait: 30 * time.Second,
			},
			expectedErr: "min_docker_retry_wait must be positive, got -1s",
		},
		{
			name: "negative max retry wait",
			cfg: &Config{
				Config: docker.Config{
					Endpoint:         "unix:///var/run/docker.sock",
					DockerAPIVersion: "1.25",
				},
				MinDockerRetryWait: 1 * time.Second,
				MaxDockerRetryWait: -1 * time.Second,
			},
			expectedErr: "max_docker_retry_wait must be positive, got -1s",
		},
		{
			name: "max less than min",
			cfg: &Config{
				Config: docker.Config{
					Endpoint:         "unix:///var/run/docker.sock",
					DockerAPIVersion: "1.25",
				},
				MinDockerRetryWait: 30 * time.Second,
				MaxDockerRetryWait: 1 * time.Second,
			},
			expectedErr: "max_docker_retry_wait must not be less than min_docker_retry_wait",
		},
		{
			name: "invalid since timestamp",
			cfg: &Config{
				Config: docker.Config{
					Endpoint:         "unix:///var/run/docker.sock",
					DockerAPIVersion: "1.25",
				},
				MinDockerRetryWait: 1 * time.Second,
				MaxDockerRetryWait: 30 * time.Second,
				Logs: EventsConfig{
					Since: "not-a-timestamp",
				},
			},
			expectedErr: "logs.since must be a Unix timestamp or RFC3339 time",
		},
		{
			name: "future since timestamp",
			cfg: &Config{
				Config: docker.Config{
					Endpoint:         "unix:///var/run/docker.sock",
					DockerAPIVersion: "1.25",
				},
				ControllerConfig:   scraperhelper.ControllerConfig{CollectionInterval: 1 * time.Second},
				MinDockerRetryWait: 1 * time.Second,
				MaxDockerRetryWait: 30 * time.Second,
				Logs: EventsConfig{
					Since: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
				},
			},
			expectedErr: "logs.since cannot be in the future",
		},
		{
			name: "since after until",
			cfg: &Config{
				Config: docker.Config{
					Endpoint:         "unix:///var/run/docker.sock",
					DockerAPIVersion: "1.25",
				},
				MinDockerRetryWait: 1 * time.Second,
				MaxDockerRetryWait: 30 * time.Second,
				Logs: EventsConfig{
					Since: "2024-01-02T00:00:00Z",
					Until: "2024-01-01T00:00:00Z",
				},
			},
			expectedErr: "logs.since must not be after logs.until",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := component.ValidateConfig(tt.cfg)
			assert.ErrorContains(t, err, tt.expectedErr)
		})
	}
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
