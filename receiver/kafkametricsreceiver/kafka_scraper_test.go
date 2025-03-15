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
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

func testScraperStartShutdown(t *testing.T, f func(newSaramaClientFunc, newSaramaClusterAdminClientFunc) (scraper.Metrics, error)) {
	t.Run("StartShutdown", func(t *testing.T) {
		scraper, err := f(mockNewSaramaClient, mockNewClusterAdmin)
		require.NoError(t, err)
		err = scraper.Start(context.Background(), componenttest.NewNopHost())
		require.NoError(t, err)
		err = scraper.Shutdown(context.Background())
		require.NoError(t, err)
	})
	t.Run("Shutdown_unstarted", func(t *testing.T) {
		scraper, err := newBrokerScraper(
			Config{}, receivertest.NewNopSettings(metadata.Type),
			nil, // newSaramaClient should not be called since we don't call Start
			nil, // newSaramaClusterAdmin should not be called since we don't call Start
		)
		assert.NoError(t, err)
		err = scraper.Shutdown(context.Background())
		assert.NoError(t, err)
	})
	t.Run("Start_newSaramaClientError", func(t *testing.T) {
		scraper, err := newBrokerScraper(
			Config{}, receivertest.NewNopSettings(metadata.Type),
			func(context.Context, configkafka.ClientConfig) (sarama.Client, error) {
				return nil, errors.New("new client failed")
			},
			nil, // newSaramaClusterAdminClient should never be called
		)
		assert.NoError(t, err)
		assert.NotNil(t, scraper)
		err = scraper.Start(context.Background(), componenttest.NewNopHost())
		assert.EqualError(t, err, "failed to create Kafka client: new client failed")
	})
	t.Run("Start_newSaramaClusterAdminClientError", func(t *testing.T) {
		scraper, err := newBrokerScraper(
			Config{}, receivertest.NewNopSettings(metadata.Type),
			mockNewSaramaClient,
			func(sarama.Client) (sarama.ClusterAdmin, error) {
				return nil, errors.New("new client failed")
			},
		)
		assert.NoError(t, err)
		assert.NotNil(t, scraper)
		err = scraper.Start(context.Background(), componenttest.NewNopHost())
		assert.EqualError(t, err, "failed to create Kafka cluster admin client: new client failed")
	})
}
