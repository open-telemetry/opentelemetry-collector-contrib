// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
)

func TestNewReceiver_invalid_version_err(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.ProtocolVersion = "invalid"
	r, err := newMetricsReceiver(context.Background(), *c, receivertest.NewNopSettings(), nil)
	assert.Error(t, err)
	assert.Nil(t, r)
}

func TestNewReceiver_invalid_scraper_error(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.Scrapers = []string{"brokers", "cpu"}
	mockScraper := func(context.Context, Config, *sarama.Config, receiver.Settings) (scraperhelper.Scraper, error) {
		return nil, nil
	}
	allScrapers["brokers"] = mockScraper
	r, err := newMetricsReceiver(context.Background(), *c, receivertest.NewNopSettings(), nil)
	assert.Nil(t, r)
	expectedError := errors.New("no scraper found for key: cpu")
	if assert.Error(t, err) {
		assert.Equal(t, expectedError, err)
	}
}

func TestNewReceiver_invalid_auth_error(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.Authentication = kafka.Authentication{
		TLS: &configtls.ClientConfig{
			Config: configtls.Config{
				CAFile: "/invalid",
			},
		},
	}
	r, err := newMetricsReceiver(context.Background(), *c, receivertest.NewNopSettings(), nil)
	assert.ErrorContains(t, err, "failed to load TLS config")
	assert.Nil(t, r)
}

func TestNewReceiver(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.Scrapers = []string{"brokers"}
	mockScraper := func(context.Context, Config, *sarama.Config, receiver.Settings) (scraperhelper.Scraper, error) {
		return scraperhelper.NewScraper(brokersScraperType, func(context.Context) (pmetric.Metrics, error) {
			return pmetric.Metrics{}, nil
		})
	}
	allScrapers["brokers"] = mockScraper
	r, err := newMetricsReceiver(context.Background(), *c, receivertest.NewNopSettings(), consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewReceiver_handles_scraper_error(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.Scrapers = []string{"brokers"}
	mockScraper := func(context.Context, Config, *sarama.Config, receiver.Settings) (scraperhelper.Scraper, error) {
		return nil, errors.New("fail")
	}
	allScrapers["brokers"] = mockScraper
	r, err := newMetricsReceiver(context.Background(), *c, receivertest.NewNopSettings(), consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, r)
}
