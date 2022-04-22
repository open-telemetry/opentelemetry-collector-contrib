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

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"
	"time"

	"github.com/Shopify/sarama"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
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

func (s *brokerScraper) shutdown(context.Context) error {
	if s.client != nil && !s.client.Closed() {
		return s.client.Close()
	}
	return nil
}

func (s *brokerScraper) scrape(context.Context) (pmetric.Metrics, error) {
	if s.client == nil {
		client, err := newSaramaClient(s.config.Brokers, s.saramaConfig)
		if err != nil {
			return pmetric.Metrics{}, fmt.Errorf("failed to create client in brokers scraper: %w", err)
		}
		s.client = client
	}

	brokers := s.client.Brokers()

	md := pmetric.NewMetrics()
	ilm := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName(instrumentationLibName)
	addIntGauge(ilm.Metrics(), metadata.M.KafkaBrokers.Name(), pcommon.NewTimestampFromTime(time.Now()), pcommon.NewMap(), int64(len(brokers)))

	return md, nil
}

func createBrokerScraper(_ context.Context, cfg Config, saramaConfig *sarama.Config, logger *zap.Logger) (scraperhelper.Scraper, error) {
	s := brokerScraper{
		logger:       logger,
		config:       cfg,
		saramaConfig: saramaConfig,
	}
	return scraperhelper.NewScraper(
		s.Name(),
		s.scrape,
		scraperhelper.WithShutdown(s.shutdown),
	)
}
