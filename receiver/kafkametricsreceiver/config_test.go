// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	expectedClientConfig := configkafka.NewDefaultClientConfig()
	expectedClientConfig.Brokers = []string{"10.10.10.10:9092"}
	expectedClientConfig.Metadata.Full = false
	expectedClientConfig.Metadata.RefreshInterval = 10 * time.Second // set by refresh_frequency

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr string
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
				ClientConfig:     expectedClientConfig,

				ClusterAlias:         "kafka-test",
				TopicMatch:           "test_\\w+",
				GroupMatch:           "test_\\w+",
				RefreshFrequency:     10 * time.Second,
				Scrapers:             []string{"brokers", "topics", "consumers"},
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_franz"),
			expectedErr: "metadata max age 2h0m0s is larger than allowed 1h0m0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expectedErr != "" {
				assert.ErrorContains(t, xconfmap.Validate(cfg), tt.expectedErr)
			} else {
				assert.NoError(t, xconfmap.Validate(cfg))
				assert.Equal(t, tt.expected, cfg)
			}
		})
	}
}
