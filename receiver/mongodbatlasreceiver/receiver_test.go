// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbatlasreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.Equal(t, cfg.(*Config).ScraperControllerSettings.CollectionInterval, 3*time.Minute)
	recv, err := createMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, recv, "receiver creation failed")

	err = recv.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestTimeConstraints(t *testing.T) {
	tt := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "initial lookback is now() - collection_interval",
			run: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig().(*Config)
				// lastRun is nil
				recv := receiver{
					cfg: cfg,
				}
				now := time.Now()
				tc := recv.timeConstraints(now)
				require.NotNil(t, tc)
				require.Equal(t, tc.start, now.Add(cfg.CollectionInterval*-1).UTC().Format(time.RFC3339))
			},
		},
		{
			name: "lookback for subsequent runs is now() - lastRun",
			run: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig().(*Config)
				now := time.Now()
				recv := receiver{
					cfg: cfg,
					// set last run to 1 collection ago
					lastRun: now.Add(cfg.CollectionInterval * -1),
				}
				tc := recv.timeConstraints(now)
				require.NotNil(t, tc)
				require.Equal(t, tc.start, recv.lastRun.UTC().Format(time.RFC3339))
			},
		},
	}

	for _, testCase := range tt {
		t.Run(testCase.name, testCase.run)
	}
}
