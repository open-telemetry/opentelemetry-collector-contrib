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
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter/internal/metadata"
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
			expected: &Config{
				ServerConfig: confighttp.ServerConfig{
					Endpoint: "1.2.3.4:1234",
					TLS: &configtls.ServerConfig{
						Config: configtls.Config{
							CertFile: "certs/server.crt",
							KeyFile:  "certs/server.key",
							CAFile:   "certs/ca.crt",
						},
					},
				},
				Namespace: "test-space",
				ConstLabels: map[string]string{
					"label1":        "value1",
					"another label": "spaced value",
				},
				SendTimestamps:    true,
				MetricExpiration:  60 * time.Minute,
				AddMetricSuffixes: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
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
