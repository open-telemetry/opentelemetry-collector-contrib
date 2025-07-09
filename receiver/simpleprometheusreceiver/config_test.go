// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package simpleprometheusreceiver

import (
	"net/url"
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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	clientConfigPath := confighttp.NewDefaultClientConfig()
	clientConfigPath.Endpoint = "localhost:1234"
	clientConfigPath.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile:   "path",
			CertFile: "path",
			KeyFile:  "path",
		},
		InsecureSkipVerify: true,
	}

	clientConfigTLS := confighttp.NewDefaultClientConfig()
	clientConfigTLS.Endpoint = "localhost:1234"
	clientConfigTLS.TLS = configtls.ClientConfig{
		Insecure: true,
	}

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "localhost:1234"

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "all_settings"),
			expected: &Config{
				ClientConfig:       clientConfigPath,
				CollectionInterval: 30 * time.Second,
				MetricsPath:        "/v2/metrics",
				JobName:            "job123",
				Params:             url.Values{"columns": []string{"name", "messages"}, "key": []string{"foo", "bar"}},
				UseServiceAccount:  true,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "partial_settings"),
			expected: &Config{
				ClientConfig:       clientConfigTLS,
				CollectionInterval: 30 * time.Second,
				MetricsPath:        "/metrics",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "partial_tls_settings"),
			expected: &Config{
				ClientConfig:       clientConfig,
				CollectionInterval: 30 * time.Second,
				MetricsPath:        "/metrics",
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
