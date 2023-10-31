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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver/internal/metadata"
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
			id: component.NewIDWithName(metadata.Type, "all_settings"),
			expected: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "localhost:1234",
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting: configtls.TLSSetting{
							CAFile:   "path",
							CertFile: "path",
							KeyFile:  "path",
						},
						InsecureSkipVerify: true,
					},
				},
				CollectionInterval: 30 * time.Second,
				MetricsPath:        "/v2/metrics",
				Params:             url.Values{"columns": []string{"name", "messages"}, "key": []string{"foo", "bar"}},
				UseServiceAccount:  true,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "partial_settings"),
			expected: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "localhost:1234",
					TLSSetting: configtls.TLSClientSetting{
						Insecure: true,
					},
				},
				CollectionInterval: 30 * time.Second,
				MetricsPath:        "/metrics",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "partial_tls_settings"),
			expected: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "localhost:1234",
				},
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
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
