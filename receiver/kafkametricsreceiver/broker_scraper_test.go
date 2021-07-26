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
	"go.uber.org/zap"
)

func TestBrokerShutdown(t *testing.T) {
	client := newMockClient()
	client.closed = false
	client.close = nil
	client.Mock.
		On("Close").Return(nil).
		On("Closed").Return(false)
	scraper := brokerScraper{
		client: client,
		logger: zap.NewNop(),
		config: Config{},
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
		client: client,
		logger: zap.NewNop(),
		config: Config{},
	}
	_ = scraper.shutdown(context.Background())
	client.AssertExpectations(t)
}

func TestBrokerScraper_Name(t *testing.T) {
	s := brokerScraper{}
	assert.Equal(t, s.Name(), brokersScraperName)
}

func TestBrokerScraper_createBrokerScraper(t *testing.T) {
	sc := sarama.NewConfig()
	newSaramaClient = mockNewSaramaClient
	ms, err := createBrokerScraper(context.Background(), Config{}, sc, zap.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, ms)
}

func TestBrokerScraperStart(t *testing.T) {
	newSaramaClient = mockNewSaramaClient
	sc := sarama.NewConfig()
	ms, err := createBrokerScraper(context.Background(), Config{}, sc, zap.NewNop())
	assert.NotNil(t, ms)
	assert.Nil(t, err)
	err = ms.Start(context.Background(), nil)
	assert.NoError(t, err)
}

func TestBrokerScraper_startBrokerScraper_handles_client_error(t *testing.T) {
	newSaramaClient = func(addrs []string, conf *sarama.Config) (sarama.Client, error) {
		return nil, fmt.Errorf("new client failed")
	}
	sc := sarama.NewConfig()
	ms, err := createBrokerScraper(context.Background(), Config{}, sc, zap.NewNop())
	assert.NotNil(t, ms)
	assert.Nil(t, err)
	err = ms.Start(context.Background(), nil)
	assert.Error(t, err)
}

func TestBrokerScraper_scrape(t *testing.T) {
	client := newMockClient()
	client.Mock.On("Brokers").Return(testBrokers)
	bs := brokerScraper{
		client: client,
		logger: zap.NewNop(),
		config: Config{},
	}
	ms, err := bs.scrape(context.Background())
	assert.Nil(t, err)
	expectedDp := int64(len(testBrokers))
	receivedMetrics := ms.At(0).InstrumentationLibraryMetrics().At(0).Metrics().At(0)
	receivedDp := receivedMetrics.Gauge().DataPoints().At(0).IntVal()
	assert.Equal(t, expectedDp, receivedDp)
	assert.NotNil(t, ms)
}

func TestBrokersScraper_createBrokerScraper(t *testing.T) {
	sc := sarama.NewConfig()
	newSaramaClient = mockNewSaramaClient
	ms, err := createBrokerScraper(context.Background(), Config{}, sc, zap.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, ms)
}
