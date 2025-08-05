// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

func TestBrokerShutdown(t *testing.T) {
	client := newMockClient()
	client.closed = false
	client.close = nil
	client.Mock.
		On("Close").Return(nil).
		On("Closed").Return(false)
	scraper := brokerScraper{
		client:   client,
		settings: receivertest.NewNopSettings(metadata.Type),
		config:   Config{},
	}
	_ = scraper.shutdown(context.Background())
	client.AssertExpectations(t)
}

func TestBrokerShutdown_closed(t *testing.T) {
	client := newMockClient()
	client.closed = true
	client.Mock.
		On("Closed").Return(true)
	scraper := brokerScraper{
		client:   client,
		settings: receivertest.NewNopSettings(metadata.Type),
		config:   Config{},
	}
	_ = scraper.shutdown(context.Background())
	client.AssertExpectations(t)
}

func TestBrokerScraper_createBrokerScraper(t *testing.T) {
	newSaramaClient = mockNewSaramaClient
	bs, err := createBrokerScraper(context.Background(), Config{}, receivertest.NewNopSettings(metadata.Type))
	assert.NoError(t, err)
	assert.NotNil(t, bs)
}

func TestBrokerScraperStart(t *testing.T) {
	newSaramaClient = mockNewSaramaClient
	bs, err := createBrokerScraper(context.Background(), Config{}, receivertest.NewNopSettings(metadata.Type))
	assert.NoError(t, err)
	assert.NotNil(t, bs)
	assert.NoError(t, bs.Start(context.Background(), nil))
}

func TestBrokerScraper_scrape_handles_client_error(t *testing.T) {
	newSaramaClient = func(context.Context, configkafka.ClientConfig) (sarama.Client, error) {
		return nil, errors.New("new client failed")
	}
	bs, err := createBrokerScraper(context.Background(), Config{}, receivertest.NewNopSettings(metadata.Type))
	assert.NoError(t, err)
	assert.NotNil(t, bs)
	_, err = bs.ScrapeMetrics(context.Background())
	assert.Error(t, err)
}

func TestBrokerScraper_shutdown_handles_nil_client(t *testing.T) {
	newSaramaClient = func(context.Context, configkafka.ClientConfig) (sarama.Client, error) {
		return nil, errors.New("new client failed")
	}
	bs, err := createBrokerScraper(context.Background(), Config{}, receivertest.NewNopSettings(metadata.Type))
	assert.NoError(t, err)
	assert.NotNil(t, bs)
	err = bs.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestBrokerScraper_empty_resource_attribute(t *testing.T) {
	client := newMockClient()
	client.Mock.On("Brokers").Return(testBrokers)
	bs := brokerScraper{
		client:   client,
		settings: receivertest.NewNopSettings(metadata.Type),
		config: Config{
			MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		},
		clusterAdmin: newMockClusterAdmin(),
	}
	require.NoError(t, bs.start(context.Background(), componenttest.NewNopHost()))
	md, err := bs.scrape(context.Background())
	assert.NoError(t, err)
	require.Equal(t, 1, md.ResourceMetrics().Len())
	require.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().Len())
	_, ok := md.ResourceMetrics().At(0).Resource().Attributes().Get("kafka.cluster.alias")
	require.False(t, ok)
}

func TestBrokerScraper_scrape(t *testing.T) {
	client := newMockClient()
	client.Mock.On("Brokers").Return(testBrokers)
	bs := brokerScraper{
		client:   client,
		settings: receivertest.NewNopSettings(metadata.Type),
		config: Config{
			MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			ClusterAlias:         testClusterAlias,
		},
		clusterAdmin: newMockClusterAdmin(),
	}
	require.NoError(t, bs.start(context.Background(), componenttest.NewNopHost()))
	md, err := bs.scrape(context.Background())
	assert.NoError(t, err)
	require.Equal(t, 1, md.ResourceMetrics().Len())
	require.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().Len())
	if val, ok := md.ResourceMetrics().At(0).Resource().Attributes().Get("kafka.cluster.alias"); ok {
		require.Equal(t, testClusterAlias, val.Str())
	}
	ms := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i)
		switch m.Name() {
		case "kafka.brokers":
			assert.Equal(t, m.Sum().DataPoints().At(0).IntValue(), int64(len(testBrokers)))
		case "kafka.broker.log_retention_period":
			assert.Equal(t, m.Gauge().DataPoints().At(0).IntValue(), int64(testLogRetentionHours*3600))
		}
	}
}

func TestBrokersScraper_createBrokerScraper(t *testing.T) {
	newSaramaClient = mockNewSaramaClient
	bs, err := createBrokerScraper(context.Background(), Config{}, receivertest.NewNopSettings(metadata.Type))
	assert.NoError(t, err)
	assert.NotNil(t, bs)
}
