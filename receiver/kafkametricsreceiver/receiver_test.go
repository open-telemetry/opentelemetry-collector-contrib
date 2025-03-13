// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

func TestNewReceiver_invalid_version_err(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.ProtocolVersion = "invalid"
	r, err := newMetricsReceiver(context.Background(), *c, receivertest.NewNopSettings(metadata.Type), nil)
	assert.Error(t, err)
	assert.Nil(t, r)
}

func TestNewReceiver_invalid_scraper_error(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.Scrapers = []string{"brokers", "cpu"}
	mockScraper := func(_ context.Context, _ Config, _ *sarama.Config, _ receiver.Settings) (scraper.Metrics, error) {
		return scraper.NewMetrics(func(context.Context) (pmetric.Metrics, error) {
			return pmetric.Metrics{}, nil
		})
	}
	allScrapers["brokers"] = mockScraper
	r, err := newMetricsReceiver(context.Background(), *c, receivertest.NewNopSettings(metadata.Type), nil)
	assert.Nil(t, r)
	expectedError := errors.New("no scraper found for key: cpu")
	if assert.Error(t, err) {
		assert.Equal(t, expectedError, err)
	}
}

func TestNewReceiver_refresh_frequency(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.RefreshFrequency = 1
	r, err := newMetricsReceiver(context.Background(), *c, receivertest.NewNopSettings(receivertest.NopType), nil)
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewReceiver(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.Scrapers = []string{"brokers"}
	mockScraper := func(_ context.Context, _ Config, _ *sarama.Config, _ receiver.Settings) (scraper.Metrics, error) {
		return scraper.NewMetrics(
			func(context.Context) (pmetric.Metrics, error) {
				return pmetric.Metrics{}, nil
			})
	}
	allScrapers["brokers"] = mockScraper
	r, err := newMetricsReceiver(context.Background(), *c, receivertest.NewNopSettings(metadata.Type), consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewReceiver_handles_scraper_error(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.Scrapers = []string{"brokers"}
	mockScraper := func(context.Context, Config, *sarama.Config, receiver.Settings) (scraper.Metrics, error) {
		return nil, errors.New("fail")
	}
	allScrapers["brokers"] = mockScraper
	r, err := newMetricsReceiver(context.Background(), *c, receivertest.NewNopSettings(metadata.Type), consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, r)
}
