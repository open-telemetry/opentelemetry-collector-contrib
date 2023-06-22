// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"
	"time"

	"github.com/Shopify/sarama"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

type saramaMetrics map[string]map[string]interface{}

type brokerScraper struct {
	client       sarama.Client
	settings     receiver.CreateSettings
	config       Config
	saramaConfig *sarama.Config
	mb           *metadata.MetricsBuilder
}

func (s *brokerScraper) Name() string {
	return brokersScraperName
}

func (s *brokerScraper) start(_ context.Context, _ component.Host) error {
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings)
	return nil
}

func (s *brokerScraper) shutdown(context.Context) error {
	if s.client != nil && !s.client.Closed() {
		return s.client.Close()
	}
	return nil
}

func (s *brokerScraper) scrapeConsumerFetch(now pcommon.Timestamp, allMetrics saramaMetrics, brokerID int64) {
	key := fmt.Sprint("consumer-fetch-rate-for-broker-", brokerID)

	if metric, ok := allMetrics[key]; ok {
		if v, ok := metric["mean.rate"].(float64); ok {
			s.mb.RecordKafkaBrokersConsumerFetchRateDataPoint(now, v, brokerID)
		}
	}
}

func (s *brokerScraper) scrapeIncomingByteRate(now pcommon.Timestamp, allMetrics saramaMetrics, brokerID int64) {
	key := fmt.Sprint("incoming-byte-rate-for-broker-", brokerID)

	if metric, ok := allMetrics[key]; ok {
		if v, ok := metric["mean.rate"].(float64); ok {
			s.mb.RecordKafkaBrokersIncomingByteRateDataPoint(now, v, brokerID)
		}
	}
}

func (s *brokerScraper) scrapeOutgoingByteRate(now pcommon.Timestamp, allMetrics saramaMetrics, brokerID int64) {
	key := fmt.Sprint("outgoing-byte-rate-for-broker-", brokerID)

	if metric, ok := allMetrics[key]; ok {
		if v, ok := metric["mean.rate"].(float64); ok {
			s.mb.RecordKafkaBrokersOutgoingByteRateDataPoint(now, v, brokerID)
		}
	}
}

func (s *brokerScraper) scrapeRequestRate(now pcommon.Timestamp, allMetrics saramaMetrics, brokerID int64) {
	key := fmt.Sprint("request-rate-for-broker-", brokerID)

	if metric, ok := allMetrics[key]; ok {
		if v, ok := metric["mean.rate"].(float64); ok {
			s.mb.RecordKafkaBrokersRequestRateDataPoint(now, v, brokerID)
		}
	}
}

func (s *brokerScraper) scrapeResponseRate(now pcommon.Timestamp, allMetrics saramaMetrics, brokerID int64) {
	key := fmt.Sprint("response-rate-for-broker-", brokerID)

	if metric, ok := allMetrics[key]; ok {
		if v, ok := metric["mean.rate"].(float64); ok {
			s.mb.RecordKafkaBrokersResponseRateDataPoint(now, v, brokerID)
		}
	}
}

func (s *brokerScraper) scrapeResponseSize(now pcommon.Timestamp, allMetrics saramaMetrics, brokerID int64) {
	key := fmt.Sprint("response-size-for-broker-", brokerID)

	if metric, ok := allMetrics[key]; ok {
		if v, ok := metric["mean"].(float64); ok {
			s.mb.RecordKafkaBrokersResponseSizeDataPoint(now, v, brokerID)
		}
	}
}

func (s *brokerScraper) scrapeRequestSize(now pcommon.Timestamp, allMetrics saramaMetrics, brokerID int64) {
	key := fmt.Sprint("request-size-for-broker-", brokerID)

	if metric, ok := allMetrics[key]; ok {
		if v, ok := metric["mean"].(float64); ok {
			s.mb.RecordKafkaBrokersRequestSizeDataPoint(now, v, brokerID)
		}
	}
}

func (s *brokerScraper) scrapeRequestsInFlight(now pcommon.Timestamp, allMetrics saramaMetrics, brokerID int64) {
	key := fmt.Sprint("requests-in-flight-for-broker-", brokerID)

	if metric, ok := allMetrics[key]; ok {
		if v, ok := metric["count"].(int64); ok {
			s.mb.RecordKafkaBrokersRequestsInFlightDataPoint(now, v, brokerID)
		}
	}
}

func (s *brokerScraper) scrapeRequestLatency(now pcommon.Timestamp, allMetrics saramaMetrics, brokerID int64) {
	key := fmt.Sprint("request-latency-in-ms-for-broker-", brokerID)
	if metric, ok := allMetrics[key]; ok {
		if v, ok := metric["mean"].(float64); ok {
			s.mb.RecordKafkaBrokersRequestLatencyDataPoint(now, v, brokerID)
		}
	}
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

	allMetrics := make(map[string]map[string]interface{})

	if s.saramaConfig != nil {
		allMetrics = s.saramaConfig.MetricRegistry.GetAll()
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	for _, broker := range brokers {
		brokerID := int64(broker.ID())
		s.scrapeConsumerFetch(now, allMetrics, brokerID)
		s.scrapeIncomingByteRate(now, allMetrics, brokerID)
		s.scrapeOutgoingByteRate(now, allMetrics, brokerID)
		s.scrapeRequestLatency(now, allMetrics, brokerID)
		s.scrapeRequestRate(now, allMetrics, brokerID)
		s.scrapeRequestSize(now, allMetrics, brokerID)
		s.scrapeRequestsInFlight(now, allMetrics, brokerID)
		s.scrapeResponseRate(now, allMetrics, brokerID)
		s.scrapeResponseSize(now, allMetrics, brokerID)
	}

	brokerCount := int64(len(brokers))
	// kafka.brokers is deprecated. This should be removed in a future release.
	s.mb.RecordKafkaBrokersDataPoint(now, brokerCount)

	// kafka.brokers.count should replace kafka.brokers.
	s.mb.RecordKafkaBrokersCountDataPoint(now, brokerCount)

	return s.mb.Emit(), nil
}

func createBrokerScraper(_ context.Context, cfg Config, saramaConfig *sarama.Config,
	settings receiver.CreateSettings) (scraperhelper.Scraper, error) {
	s := brokerScraper{
		settings:     settings,
		config:       cfg,
		saramaConfig: saramaConfig,
	}
	return scraperhelper.NewScraper(
		s.Name(),
		s.scrape,
		scraperhelper.WithStart(s.start),
		scraperhelper.WithShutdown(s.shutdown),
	)
}
