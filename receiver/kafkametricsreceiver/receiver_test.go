// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kafkametricsreceiver

import (
	"context"
	"fmt"
	"testing"

	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scraperhelper"
)

func TestNewReceiver_invalid_version_err(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.ProtocolVersion = "invalid"
	r, err := newMetricsReceiver(context.Background(), *c, componenttest.NewNopReceiverCreateSettings(), nil)
	assert.Error(t, err)
	assert.Nil(t, r)
}

func TestNewReceiver_invalid_scraper_error(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.Scrapers = []string{"brokers", "cpu"}
	mockScraper := func(context.Context, Config, *sarama.Config, *zap.Logger) (scraperhelper.Scraper, error) {
		return nil, nil
	}
	allScrapers["brokers"] = mockScraper
	r, err := newMetricsReceiver(context.Background(), *c, componenttest.NewNopReceiverCreateSettings(), nil)
	assert.Nil(t, r)
	expectedError := fmt.Errorf("no scraper found for key: cpu")
	if assert.Error(t, err) {
		assert.Equal(t, expectedError, err)
	}
}

func TestNewReceiver_invalid_auth_error(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.Authentication = kafkaexporter.Authentication{
		TLS: &configtls.TLSClientSetting{
			TLSSetting: configtls.TLSSetting{
				CAFile: "/invalid",
			},
		},
	}
	r, err := newMetricsReceiver(context.Background(), *c, componenttest.NewNopReceiverCreateSettings(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load TLS config")
	assert.Nil(t, r)
}

func TestNewReceiver(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.Scrapers = []string{"brokers"}
	mockScraper := func(context.Context, Config, *sarama.Config, *zap.Logger) (scraperhelper.Scraper, error) {
		return nil, nil
	}
	allScrapers["brokers"] = mockScraper
	r, err := newMetricsReceiver(context.Background(), *c, componenttest.NewNopReceiverCreateSettings(), consumertest.NewNop())
	assert.Nil(t, err)
	assert.NotNil(t, r)
}

func TestNewReceiver_handles_scraper_error(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.Scrapers = []string{"brokers"}
	mockScraper := func(context.Context, Config, *sarama.Config, *zap.Logger) (scraperhelper.Scraper, error) {
		return nil, fmt.Errorf("fail")
	}
	allScrapers["brokers"] = mockScraper
	r, err := newMetricsReceiver(context.Background(), *c, componenttest.NewNopReceiverCreateSettings(), consumertest.NewNop())
	assert.NotNil(t, err)
	assert.Nil(t, r)
}
