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
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id                component.ID
		enableFeatureGate bool
		expected          component.Config
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
			id:                component.NewIDWithName(metadata.Type, "resource_constant_labels"),
			enableFeatureGate: true,
			expected: func() component.Config {
				cfg := createDefaultConfig().(*Config)
				cfg.ResourceConstantLabels = configoptional.Some(ResourceConstantLabels{
					Included: []string{"service.name", "k8s.namespace.name"},
					Excluded: []string{"service.instance.id"},
				})
				return cfg
			}(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "legacy_resource_to_telemetry"),
			expected: func() component.Config {
				cfg := createDefaultConfig().(*Config)
				cfg.ResourceToTelemetrySettings = resourcetotelemetry.Settings{
					Enabled:                  true,
					ExcludeServiceAttributes: true,
				}
				cfg.resourceToTelemetrySettingsConfigured = true
				return cfg
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			oldValue := metadata.ExporterPrometheusexporterResourceConstantLabelsFeatureGate.IsEnabled()
			testutil.SetFeatureGateForTest(t, metadata.ExporterPrometheusexporterResourceConstantLabelsFeatureGate, tt.enableFeatureGate)
			defer testutil.SetFeatureGateForTest(t, metadata.ExporterPrometheusexporterResourceConstantLabelsFeatureGate, oldValue)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidateResourceConstantLabelsFeatureGate(t *testing.T) {
	tests := []struct {
		name              string
		enableFeatureGate bool
		cfg               *Config
		wantErr           string
	}{
		{
			name:              "resource constant labels require feature gate",
			enableFeatureGate: false,
			cfg: &Config{
				ResourceConstantLabels: configoptional.Some(ResourceConstantLabels{
					Included: []string{"service.name"},
				}),
			},
			wantErr: `resource_constant_labels requires feature gate "exporter.prometheusexporter.ResourceConstantLabels"`,
		},
		{
			name:              "legacy disabled by feature gate",
			enableFeatureGate: true,
			cfg: &Config{
				ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
			},
			wantErr: `resource_to_telemetry_conversion is disabled by feature gate "exporter.prometheusexporter.ResourceConstantLabels"; use resource_constant_labels instead`,
		},
		{
			name:              "legacy block presence disabled by feature gate",
			enableFeatureGate: true,
			cfg: &Config{
				resourceToTelemetrySettingsConfigured: true,
			},
			wantErr: `resource_to_telemetry_conversion is disabled by feature gate "exporter.prometheusexporter.ResourceConstantLabels"; use resource_constant_labels instead`,
		},
		{
			name:              "legacy and new are mutually exclusive",
			enableFeatureGate: true,
			cfg: &Config{
				ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
				ResourceConstantLabels: configoptional.Some(ResourceConstantLabels{
					Included: []string{"service.name"},
				}),
			},
			wantErr: "resource_constant_labels and resource_to_telemetry_conversion cannot be configured at the same time",
		},
		{
			name:              "new config allowed with feature gate",
			enableFeatureGate: true,
			cfg: &Config{
				ResourceConstantLabels: configoptional.Some(ResourceConstantLabels{
					Included: []string{"service.name"},
				}),
			},
		},
		{
			name:              "legacy config allowed without feature gate",
			enableFeatureGate: false,
			cfg: &Config{
				ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldValue := metadata.ExporterPrometheusexporterResourceConstantLabelsFeatureGate.IsEnabled()
			testutil.SetFeatureGateForTest(t, metadata.ExporterPrometheusexporterResourceConstantLabelsFeatureGate, tt.enableFeatureGate)
			defer testutil.SetFeatureGateForTest(t, metadata.ExporterPrometheusexporterResourceConstantLabelsFeatureGate, oldValue)

			err := tt.cfg.Validate()
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
