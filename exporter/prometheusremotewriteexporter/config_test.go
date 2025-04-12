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
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "localhost:8888"
	clientConfig.TLSSetting = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile: "/var/lib/mycert.pem", // This is subject to change, but currently I have no idea what else to put here lol
		},
		Insecure: false,
	}
	clientConfig.ReadBufferSize = 0
	clientConfig.WriteBufferSize = 512 * 1024
	clientConfig.Timeout = 5 * time.Second
	clientConfig.Headers = map[string]configopaque.String{
		"Prometheus-Remote-Write-Version": "0.1.0",
		"X-Scope-OrgID":                   "234",
	}
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
				MaxBatchSizeBytes:          3000000,
				MaxBatchRequestParallelism: toPtr(10),
				TimeoutSettings:            exporterhelper.NewDefaultTimeoutConfig(),
				BackOffConfig: configretry.BackOffConfig{
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
				AddMetricSuffixes:           false,
				Namespace:                   "test-space",
				ExternalLabels:              map[string]string{"key1": "value1", "key2": "value2"},
				ClientConfig:                clientConfig,
				ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
				TargetInfo: &TargetInfo{
					Enabled: true,
				},
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
		{
			id:           component.NewIDWithName(metadata.Type, "less_than_1_max_batch_request_parallelism"),
			errorMessage: "max_batch_request_parallelism can't be set to below 1",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "non_snappy_compression_type"),
			errorMessage: "compression type must be snappy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expected == nil {
				assert.EqualError(t, xconfmap.Validate(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
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
	require.NoError(t, sub.Unmarshal(cfg))

	assert.False(t, cfg.(*Config).RemoteWriteQueue.Enabled)
}

func TestDisabledTargetInfo(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "disabled_target_info").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.False(t, cfg.(*Config).TargetInfo.Enabled)
}

func toPtr[T any](val T) *T {
	return &val
}
