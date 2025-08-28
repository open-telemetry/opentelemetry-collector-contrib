// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = defaultEndpoint
	clientConfig.Timeout = 10 * time.Second

	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "creates a new factory with correct type",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				require.Equal(t, metadata.Type, factory.Type())
			},
		},
		{
			desc: "creates a new factory with valid default config",
			testFunc: func(t *testing.T) {
				factory := NewFactory()

				var expectedCfg component.Config = &Config{
					ControllerConfig: scraperhelper.ControllerConfig{
						CollectionInterval: 10 * time.Second,
						InitialDelay:       time.Second,
					},
					ClientConfig:         clientConfig,
					MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				}

				require.Equal(t, expectedCfg, factory.CreateDefaultConfig())
			},
		},
		{
			desc: "creates a new factory and CreateMetrics returns no error",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				_, err := factory.CreateMetrics(
					t.Context(),
					receivertest.NewNopSettings(metadata.Type),
					cfg,
					consumertest.NewNop(),
				)
				require.NoError(t, err)
			},
		},
		{
			desc: "creates a new factory and CreateMetrics returns error with incorrect config",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				_, err := factory.CreateMetrics(
					t.Context(),
					receivertest.NewNopSettings(metadata.Type),
					nil,
					consumertest.NewNop(),
				)
				require.ErrorIs(t, err, errConfigNotRabbit)
			},
		},
		{
            desc: "default config enables all node metrics",
            testFunc: func(t *testing.T) {
                factory := NewFactory()
                cfg := factory.CreateDefaultConfig().(*Config)
                
                // Verify key node metrics are enabled by default
                require.True(t, cfg.Metrics.RabbitmqNodeDiskFree.Enabled)
                require.True(t, cfg.Metrics.RabbitmqNodeMemUsed.Enabled)
                require.True(t, cfg.Metrics.RabbitmqNodeFdUsed.Enabled)
                require.True(t, cfg.Metrics.RabbitmqNodeSocketsUsed.Enabled)
                require.True(t, cfg.Metrics.RabbitmqNodeProcUsed.Enabled)
                require.True(t, cfg.Metrics.RabbitmqNodeUptime.Enabled)
                require.True(t, cfg.Metrics.RabbitmqNodeContextSwitches.Enabled)
                require.True(t, cfg.Metrics.RabbitmqNodeGcNum.Enabled)
                
                // Verify queue metrics still enabled
                require.True(t, cfg.Metrics.RabbitmqConsumerCount.Enabled)
                require.True(t, cfg.Metrics.RabbitmqMessageAcknowledged.Enabled)
            },
        },
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}
