// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"context"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver/internal/metadata"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

func TestFactoryCreate(t *testing.T) {
    factory := NewFactory()
    require.EqualValues(t, "splunkenterprise", factory.Type())
}

func TestDefaultConfig(t *testing.T) {
    expectedConf := &Config{
        MaxSearchWaitTime: 60 * time.Second,
        ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
            CollectionInterval: 10 * time.Minute,
            InitialDelay: 1 * time.Second,
        },
        MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
    }

    testConf := createDefaultConfig().(*Config)

    require.Equal(t, expectedConf, testConf)
}

func TestCreateMetricsReceiver(t *testing.T) {
    tests := []struct {
		desc string
		run  func(t *testing.T)
	}{
		{
			desc: "Defaults with valid config",
			run: func(t *testing.T) {
				t.Parallel()

				cfg := createDefaultConfig().(*Config)

				_, err := createMetricsReceiver(
					context.Background(),
					receivertest.NewNopCreateSettings(),
					cfg,
					consumertest.NewNop(),
				)

				require.NoError(t, err, "failed to create metrics receiver with valid inputs")
			},
		},
		{
			desc: "Missing consumer",
			run: func(t *testing.T) {
				t.Parallel()

				cfg := createDefaultConfig().(*Config)

				_, err := createMetricsReceiver(
					context.Background(),
					receivertest.NewNopCreateSettings(),
					cfg,
					nil,
				)

				require.Error(t, err, "created metrics receiver without consumer")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, test.run)
	}
}
