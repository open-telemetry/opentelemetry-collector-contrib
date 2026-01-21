// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

func setFranzGo(tb testing.TB, value bool) {
	currentFranzState := metadata.ReceiverKafkametricsreceiverUseFranzGoFeatureGate.IsEnabled()
	require.NoError(tb, featuregate.GlobalRegistry().Set(metadata.ReceiverKafkametricsreceiverUseFranzGoFeatureGate.ID(), value))
	tb.Cleanup(func() {
		require.NoError(tb, featuregate.GlobalRegistry().Set(metadata.ReceiverKafkametricsreceiverUseFranzGoFeatureGate.ID(), currentFranzState))
	})
}

func TestNewReceiver_invalid_scraper_error(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.Scrapers = []string{"brokers", "cpu"}
	mockScraper := func(_ context.Context, _ Config, _ receiver.Settings) (scraper.Metrics, error) {
		return scraper.NewMetrics(func(context.Context) (pmetric.Metrics, error) {
			return pmetric.Metrics{}, nil
		})
	}
	allScrapers["brokers"] = mockScraper
	r, err := newMetricsReceiver(t.Context(), *c, receivertest.NewNopSettings(metadata.Type), nil)
	assert.Nil(t, r)
	expectedError := errors.New("no scraper found for key: cpu")
	if assert.Error(t, err) {
		assert.Equal(t, expectedError, err)
	}
}

func TestNewReceiver_refresh_frequency(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.RefreshFrequency = 1
	r, err := newMetricsReceiver(t.Context(), *c, receivertest.NewNopSettings(receivertest.NopType), nil)
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewReceiver(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.Scrapers = []string{"brokers"}
	mockScraper := func(_ context.Context, _ Config, _ receiver.Settings) (scraper.Metrics, error) {
		return scraper.NewMetrics(
			func(context.Context) (pmetric.Metrics, error) {
				return pmetric.Metrics{}, nil
			})
	}
	allScrapers["brokers"] = mockScraper
	r, err := newMetricsReceiver(t.Context(), *c, receivertest.NewNopSettings(metadata.Type), consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewReceiver_handles_scraper_error(t *testing.T) {
	setFranzGo(t, false)
	c := createDefaultConfig().(*Config)
	c.Scrapers = []string{"brokers"}
	mockScraper := func(context.Context, Config, receiver.Settings) (scraper.Metrics, error) {
		return nil, errors.New("fail")
	}
	allScrapers["brokers"] = mockScraper
	r, err := newMetricsReceiver(t.Context(), *c, receivertest.NewNopSettings(metadata.Type), consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, r)
}

func TestReceiver_UsesSaramaWhenFranzDisabled(t *testing.T) {
	setFranzGo(t, false)

	m := scrapersForCurrentGate()
	require.NotNil(t, m)

	saramaBrokers := allScrapers[brokersScraperType.String()]
	saramaTopics := allScrapers[topicsScraperType.String()]
	saramaConsumers := allScrapers[consumersScraperType.String()]

	require.Equal(t,
		reflect.ValueOf(saramaBrokers).Pointer(),
		reflect.ValueOf(m[brokersScraperType.String()]).Pointer(),
		"brokers scraper should match allScrapers when gate is disabled",
	)
	require.Equal(t,
		reflect.ValueOf(saramaTopics).Pointer(),
		reflect.ValueOf(m[topicsScraperType.String()]).Pointer(),
		"topics scraper should match allScrapers when gate is disabled",
	)
	require.Equal(t,
		reflect.ValueOf(saramaConsumers).Pointer(),
		reflect.ValueOf(m[consumersScraperType.String()]).Pointer(),
		"consumers scraper should match allScrapers when gate is disabled",
	)
}

func TestReceiver_UsesFranzWhenFranzEnabled(t *testing.T) {
	setFranzGo(t, true)

	m := scrapersForCurrentGate()
	require.NotNil(t, m)

	franzBrokers := allScrapers[brokersScraperType.String()]
	franzTopics := allScrapers[topicsScraperType.String()]
	franzConsumers := allScrapers[consumersScraperType.String()]

	require.NotEqual(t,
		reflect.ValueOf(franzBrokers).Pointer(),
		reflect.ValueOf(m[brokersScraperType.String()]).Pointer(),
		"brokers scraper should NOT be the Sarama default when gate is enabled",
	)
	require.NotEqual(t,
		reflect.ValueOf(franzTopics).Pointer(),
		reflect.ValueOf(m[topicsScraperType.String()]).Pointer(),
		"topics scraper should NOT be the Sarama default when gate is enabled",
	)
	require.NotEqual(t,
		reflect.ValueOf(franzConsumers).Pointer(),
		reflect.ValueOf(m[consumersScraperType.String()]).Pointer(),
		"consumers scraper should NOT be the Sarama default when gate is enabled",
	)
}
