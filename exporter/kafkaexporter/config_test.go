// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
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
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 10 * time.Second,
				},
				BackOffConfig: func() configretry.BackOffConfig {
					config := configretry.NewDefaultBackOffConfig()
					config.InitialInterval = 10 * time.Second
					config.MaxInterval = 60 * time.Second
					config.MaxElapsedTime = 10 * time.Minute
					return config
				}(),
				QueueSettings: exporterhelper.QueueBatchConfig{
					Enabled:      true,
					NumConsumers: 2,
					QueueSize:    10,
					Sizer:        exporterhelper.RequestSizerTypeRequests,
				},
				ClientConfig: func() configkafka.ClientConfig {
					config := configkafka.NewDefaultClientConfig()
					config.Brokers = []string{"foo:123", "bar:456"}
					return config
				}(),
				Producer: func() configkafka.ProducerConfig {
					config := configkafka.NewDefaultProducerConfig()
					config.MaxMessageBytes = 10000000
					config.RequiredAcks = configkafka.WaitForAll
					return config
				}(),
				Topic:                                "spans",
				Encoding:                             "otlp_proto",
				PartitionTracesByID:                  true,
				PartitionMetricsByResourceAttributes: true,
				PartitionLogsByResourceAttributes:    true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
