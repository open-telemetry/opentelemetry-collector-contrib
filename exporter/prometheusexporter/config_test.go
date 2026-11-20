// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "2"),
			expected: func() component.Config {
				serverConfig := confighttp.NewDefaultServerConfig()
				serverConfig.NetAddr.Endpoint = "1.2.3.4:1234"
				serverConfig.TLS = configoptional.Some(configtls.ServerConfig{
					Config: configtls.Config{
						CertFile: "certs/server.crt",
						KeyFile:  "certs/server.key",
					},
					ClientCAFile: "certs/ca.crt",
				})

				return &Config{
					ServerConfig: serverConfig,
					Namespace:    "test-space",
					ConstLabels: map[string]string{
						"label1":        "value1",
						"another label": "spaced value",
					},
					SendTimestamps:    true,
					MetricExpiration:  60 * time.Minute,
					AddMetricSuffixes: false,
					QueueBatchConfig:  createDefaultConfig().(*Config).QueueBatchConfig,
				}
			}(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "3"),
			expected: func() component.Config {
				cfg := createDefaultConfig().(*Config)
				cfg.ResourceConstantLabels = resourcetotelemetry.Settings{
					Included: []string{"service.name", "k8s.pod.name"},
				}
				return cfg
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, confmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ResourceConstantLabels = resourcetotelemetry.Settings{Included: []string{"foo"}}
	require.NoError(t, cfg.Validate())

	invalidCfg := createDefaultConfig().(*Config)
	invalidCfg.ResourceConstantLabels.Enabled = true //nolint:staticcheck // testing deprecated field rejection
	require.ErrorContains(t, invalidCfg.Validate(), "enabled and exclude_service_attributes are not supported under resource_constant_labels")

	cfg = createDefaultConfig().(*Config)
	cfg.ResourceToTelemetrySettings = resourcetotelemetry.Settings{Included: []string{"*"}}
	require.ErrorContains(t, cfg.Validate(), "resource_to_telemetry_conversion is disabled by the exporter.prometheus.DisableResourceToTelemetryConversion feature gate")

	defer testutil.SetFeatureGateForTest(t, metadata.ExporterPrometheusDisableResourceToTelemetryConversionFeatureGate, false)()
	require.NoError(t, cfg.Validate())

	cfg.ResourceConstantLabels = resourcetotelemetry.Settings{Included: []string{"foo"}}
	require.ErrorContains(t, cfg.Validate(), "cannot configure both resource_to_telemetry_conversion and resource_constant_labels")

	cfg.ResourceToTelemetrySettings = resourcetotelemetry.Settings{}
	require.NoError(t, cfg.Validate())
}

func TestConfigValidate_SendingQueue(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.NoError(t, cfg.Validate())

	// Setting sending_queue explicitly should be valid when the feature gate is disabled and enabled is not true
	queueCfg := exporterhelper.NewDefaultQueueConfig()
	cfg.QueueBatchConfig = configoptional.Some(queueCfg)
	assert.NoError(t, cfg.Validate())

	// Setting sending_queue.enabled explicitly must fail validation
	cfg.sendingQueueEnabled = true
	assert.ErrorContains(t, cfg.Validate(), "sending_queue.enabled is not supported; the Prometheus exporter is pull-based and does not support queuing")
	cfg.sendingQueueEnabled = false

	// Setting sending_queue explicitly must fail when the feature gate is enabled
	defer testutil.SetFeatureGateForTest(t, metadata.ExporterPrometheusDisableSendingQueueFeatureGate, true)()
	assert.ErrorContains(t, cfg.Validate(), "sending_queue is disabled by the exporter.prometheus.DisableSendingQueue feature gate")

	// Default config (HasValue() == false) must remain valid even when the feature gate is enabled
	defaultCfg := createDefaultConfig().(*Config)
	assert.NoError(t, defaultCfg.Validate())
}

func TestConfigUnmarshal_SendingQueueEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		enabled     bool
		validateErr string
	}{
		{
			name:        "sending_queue enabled true rejected in validate",
			enabled:     true,
			validateErr: "sending_queue.enabled is not supported; the Prometheus exporter is pull-based and does not support queuing",
		},
		{
			name:    "sending_queue enabled false accepted when gate disabled",
			enabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			confMap := confmap.NewFromStringMap(map[string]any{
				"endpoint": "1.2.3.4:1234",
				"sending_queue": map[string]any{
					"enabled": tt.enabled,
				},
			})
			require.NoError(t, confMap.Unmarshal(cfg))

			if tt.validateErr != "" {
				assert.ErrorContains(t, cfg.Validate(), tt.validateErr)
			} else {
				assert.NoError(t, cfg.Validate())
			}
		})
	}
}
