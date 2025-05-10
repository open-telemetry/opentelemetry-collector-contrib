// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
	mock "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/mockserver"
)

func TestScrape(t *testing.T) {
	ctx := context.Background()
	mockServer := mock.MockServer(t, false)
	defer mockServer.Close()

	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Endpoint:             mockServer.URL,
		Username:             mock.MockUsername,
		Password:             mock.MockPassword,
	}

	testScrape(ctx, t, cfg, "expected.yaml")
}

func TestScrapeConfigsEnabled(t *testing.T) {
	ctx := context.Background()
	mockServer := mock.MockServer(t, false)
	defer mockServer.Close()

	optConfigs := metadata.DefaultMetricsBuilderConfig()
	setResourcePoolMemoryUsageAttrFeatureGate(t, true)

	cfg := &Config{
		MetricsBuilderConfig: optConfigs,
		Endpoint:             mockServer.URL,
		Username:             mock.MockUsername,
		Password:             mock.MockPassword,
	}

	testScrape(ctx, t, cfg, "expected-all-enabled.yaml")
}

func TestScrape_TLS(t *testing.T) {
	ctx := context.Background()
	mockServer := mock.MockServer(t, true)
	defer mockServer.Close()

	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Endpoint:             mockServer.URL,
		Username:             mock.MockUsername,
		Password:             mock.MockPassword,
	}

	cfg.Insecure = true
	cfg.InsecureSkipVerify = true

	testScrape(ctx, t, cfg, "expected.yaml")
}

func testScrape(ctx context.Context, t *testing.T, cfg *Config, fileName string) {
	scraper := newVmwareVcenterScraper(zap.NewNop(), cfg, receivertest.NewNopSettings())

	metrics, err := scraper.scrape(ctx)
	require.NoError(t, err)
	require.NotEqual(t, 0, metrics.MetricCount())

	goldenPath := filepath.Join("testdata", "metrics", fileName)
	expectedMetrics, err := golden.ReadMetrics(goldenPath)
	require.NoError(t, err)

	err = pmetrictest.CompareMetrics(expectedMetrics, metrics,
		pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	)
	require.NoError(t, err)
	require.NoError(t, scraper.Shutdown(ctx))
}

func setResourcePoolMemoryUsageAttrFeatureGate(t *testing.T, val bool) {
	wasEnabled := enableResourcePoolMemoryUsageAttr.IsEnabled()
	err := featuregate.GlobalRegistry().Set(
		enableResourcePoolMemoryUsageAttr.ID(),
		val,
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := featuregate.GlobalRegistry().Set(
			enableResourcePoolMemoryUsageAttr.ID(),
			wasEnabled,
		)
		require.NoError(t, err)
	})
}

func TestScrape_NoClient(t *testing.T) {
	ctx := context.Background()
	scraper := &vcenterMetricScraper{
		client: nil,
		config: &Config{
			Endpoint: "http://vcsa.localnet",
		},
		mb:     metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopSettings()),
		logger: zap.NewNop(),
	}
	metrics, err := scraper.scrape(ctx)
	require.ErrorContains(t, err, "unable to connect to vSphere SDK")
	require.Equal(t, 0, metrics.MetricCount())
	require.NoError(t, scraper.Shutdown(ctx))
}

func TestStartFailures_Metrics(t *testing.T) {
	cases := []struct {
		desc string
		cfg  *Config
		err  error
	}{
		{
			desc: "bad client connect",
			cfg: &Config{
				Endpoint: "http://no-host",
			},
		},
		{
			desc: "unparsable endpoint",
			cfg: &Config{
				Endpoint: "<protocol>://some-host",
			},
		},
	}

	ctx := context.Background()
	for _, tc := range cases {
		scraper := newVmwareVcenterScraper(zap.NewNop(), tc.cfg, receivertest.NewNopSettings())
		err := scraper.Start(ctx, nil)
		if tc.err != nil {
			require.ErrorContains(t, err, tc.err.Error())
		} else {
			require.NoError(t, err)
		}
	}
}
