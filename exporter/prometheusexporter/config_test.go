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
						CAFile:   "certs/ca.crt",
					},
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
	assert.NoError(t, cfg.Validate())

	invalidCfg := createDefaultConfig().(*Config)
	invalidCfg.ResourceConstantLabels.Enabled = true //nolint:staticcheck // testing deprecated field rejection
	assert.Error(t, invalidCfg.Validate())

	cfg.ResourceToTelemetrySettings.Enabled = true //nolint:staticcheck // ignore deprecated field
	assert.Error(t, cfg.Validate())

	defer testutil.SetFeatureGateForTest(t, metadata.ExporterPrometheusDisableResourceToTelemetryConversionFeatureGate, true)()
	assert.Error(t, cfg.Validate())

	cfg.ResourceToTelemetrySettings = resourcetotelemetry.Settings{}
	assert.NoError(t, cfg.Validate())
}
