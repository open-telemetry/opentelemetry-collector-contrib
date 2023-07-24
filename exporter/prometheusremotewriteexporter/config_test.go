// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "2"),
			expected: &Config{
				TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
				RetrySettings: exporterhelper.RetrySettings{
					Enabled:             true,
					InitialInterval:     10 * time.Second,
					MaxInterval:         1 * time.Minute,
					MaxElapsedTime:      10 * time.Minute,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				RemoteWriteQueue: RemoteWriteQueue{
					Enabled:      true,
					QueueSize:    2000,
					NumConsumers: 10,
				},
				AddMetricSuffixes: false,
				Namespace:         "test-space",
				ExternalLabels:    map[string]string{"key1": "value1", "key2": "value2"},
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "localhost:8888",
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting: configtls.TLSSetting{
							CAFile: "/var/lib/mycert.pem", // This is subject to change, but currently I have no idea what else to put here lol
						},
						Insecure: false,
					},
					ReadBufferSize:  0,
					WriteBufferSize: 512 * 1024,
					Timeout:         5 * time.Second,
					Headers: map[string]configopaque.String{
						"Prometheus-Remote-Write-Version": "0.1.0",
						"X-Scope-OrgID":                   "234"},
				},
				ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
				TargetInfo: &TargetInfo{
					Enabled: true,
				},
				ScopeInfo: &ScopeInfo{
					Enabled: false,
				},
				CreatedMetric: &CreatedMetric{Enabled: true},
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "negative_queue_size"),
			errorMessage: "remote write queue size can't be negative",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "negative_num_consumers"),
			errorMessage: "remote write consumer number can't be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			if tt.expected == nil {
				assert.EqualError(t, component.ValidateConfig(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestDisabledQueue(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "disabled_queue").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	assert.False(t, cfg.(*Config).RemoteWriteQueue.Enabled)
}

func TestDisabledTargetInfo(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "disabled_target_info").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	assert.False(t, cfg.(*Config).TargetInfo.Enabled)
}
