// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver/internal/metadata"
)

func TestFactoryCreate(t *testing.T) {
	factory := NewFactory()
	require.Equal(t, metadata.Type, factory.Type())
}

func TestDefaultConfig(t *testing.T) {
	cfg := confighttp.NewDefaultClientConfig()
	cfg.Headers = map[string]configopaque.String{
		"Content-Type": "application/x-www-form-urlencoded",
	}
	cfg.Timeout = 60 * time.Second

	expectedConf := &Config{
		IdxEndpoint: cfg,
		SHEndpoint:  cfg,
		CMEndpoint:  cfg,
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 10 * time.Minute,
			InitialDelay:       1 * time.Second,
			Timeout:            60 * time.Second,
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}

	testConf := createDefaultConfig().(*Config)

	require.Equal(t, expectedConf, testConf)
}

func TestCreateMetrics(t *testing.T) {
	tests := []struct {
		desc string
		run  func(t *testing.T)
	}{
		{
			desc: "Defaults with valid config",
			run: func(t *testing.T) {
				t.Parallel()

				cfg := createDefaultConfig().(*Config)
				cfg.CMEndpoint.Endpoint = "https://123.12.12.12:80"
				cfg.IdxEndpoint.Endpoint = "https://123.12.12.12:80"
				cfg.SHEndpoint.Endpoint = "https://123.12.12.12:80"

				_, err := createMetricsReceiver(
					context.Background(),
					receivertest.NewNopSettings(metadata.Type),
					cfg,
					consumertest.NewNop(),
				)

				require.NoError(t, err, "failed to create metrics receiver with valid inputs")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, test.run)
	}
}
