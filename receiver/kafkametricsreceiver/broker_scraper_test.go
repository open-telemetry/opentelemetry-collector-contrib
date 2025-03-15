// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"context"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

func TestBrokerScraper_StartShutdown(t *testing.T) {
	testScraperStartShutdown(t, func(f1 newSaramaClientFunc, f2 newSaramaClusterAdminClientFunc) (scraper.Metrics, error) {
		return newBrokerScraper(Config{}, receivertest.NewNopSettings(metadata.Type), f1, f2)
	})
}

func TestBrokerScraper_ScrapeMetrics(t *testing.T) {
	client := newMockClient()
	client.Mock.On("Brokers").Return(testBrokers)
	config := Config{MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()}
	config.Metrics.KafkaBrokerLogRetentionPeriod.Enabled = true
	scraper := newTestBrokerScraper(t, config, client)

	md, err := scraper.ScrapeMetrics(context.Background())
	require.NoError(t, err)

	expected, err := golden.ReadMetrics("testdata/golden/" + t.Name() + ".yaml")
	require.NoError(t, err)
	assert.NoError(t, pmetrictest.CompareMetrics(
		expected, md,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
	))
}

func TestBrokerScraper_ScrapeMetrics_ClusterAlias(t *testing.T) {
	// FIXME
	t.Skip("broken, how did alias work before?")

	for _, clusterAlias := range []string{"", "guy_incognito"} {
		t.Run(clusterAlias, func(t *testing.T) {
			client := newMockClient()
			client.Mock.On("Brokers").Return(testBrokers)
			config := Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				ClusterAlias:         clusterAlias,
			}
			scraper := newTestBrokerScraper(t, config, client)

			md, err := scraper.ScrapeMetrics(context.Background())
			require.NoError(t, err)

			require.Equal(t, 1, md.ResourceMetrics().Len())
			require.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().Len())
			v, ok := md.ResourceMetrics().At(0).Resource().Attributes().Get("kafka.cluster.alias")
			if clusterAlias == "" {
				require.False(t, ok)
			} else {
				require.True(t, ok)
				assert.Equal(t, clusterAlias, v.Str())
			}
		})
	}
}

func newTestBrokerScraper(tb testing.TB, config Config, mockClient sarama.Client) scraper.Metrics {
	scraper, err := newBrokerScraper(
		config, receivertest.NewNopSettings(metadata.Type),
		func(context.Context, configkafka.ClientConfig) (sarama.Client, error) {
			return mockClient, nil
		},
		mockNewClusterAdmin,
	)
	require.NoError(tb, err)

	err = scraper.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(tb, err)
	tb.Cleanup(func() {
		assert.NoError(tb, scraper.Shutdown(context.Background()))
	})
	return scraper
}
