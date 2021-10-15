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
	"time"

	"github.com/Shopify/sarama"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scraperhelper"
)

type brokerScraper struct {
	client       sarama.Client
	logger       *zap.Logger
	config       Config
	saramaConfig *sarama.Config
}

func (s *brokerScraper) Name() string {
	return brokersScraperName
}

func (s *brokerScraper) start(context.Context, component.Host) error {
	client, err := newSaramaClient(s.config.Brokers, s.saramaConfig)
	if err != nil {
		return fmt.Errorf("failed to create client while starting brokers scraper: %w", err)
	}
	s.client = client
	return nil
}

func (s *brokerScraper) shutdown(context.Context) error {
	if !s.client.Closed() {
		return s.client.Close()
	}
	return nil
}

func (s *brokerScraper) scrape(context.Context) (pdata.ResourceMetricsSlice, error) {
	brokers := s.client.Brokers()

	rms := pdata.NewResourceMetricsSlice()
	ilm := rms.AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName(instrumentationLibName)
	addIntGauge(ilm.Metrics(), metadata.M.KafkaBrokers.Name(), pdata.NewTimestampFromTime(time.Now()), pdata.NewAttributeMap(), int64(len(brokers)))

	return rms, nil
}

func createBrokerScraper(_ context.Context, cfg Config, saramaConfig *sarama.Config, logger *zap.Logger) (scraperhelper.Scraper, error) {
	s := brokerScraper{
		logger:       logger,
		config:       cfg,
		saramaConfig: saramaConfig,
	}
	ms := scraperhelper.NewResourceMetricsScraper(
		config.NewComponentID(config.Type(s.Name())),
		s.scrape,
		scraperhelper.WithShutdown(s.shutdown),
		scraperhelper.WithStart(s.start),
	)
	return ms, nil
}
