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
type saramaMetrics map[string]map[string]interface{} // saramaMetrics is a map of metric name to tags


type brokerScraper struct {
	client       sarama.Client
	settings     receiver.CreateSettings
	config       Config
	saramaConfig *sarama.Config
	mb           *metadata.MetricsBuilder
}

var nrMetricsPrefix = [...]string{
	"consumer-fetch-rate-for-broker-",
	"incoming-byte-rate-for-broker-",
	"outgoing-byte-rate-for-broker-",
	"request-rate-for-broker-",
	"response-rate-for-broker-",
	"response-size-for-broker-",
	"request-size-for-broker-",
	"requests-in-flight-for-broker-",
	"request-latency-in-ms-for-broker-",
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

func (s *brokerScraper) scrapeMetric(now pcommon.Timestamp, allMetrics saramaMetrics, brokerID int64, prefix string) {
	key := fmt.Sprint(prefix, brokerID)

	if metric, ok := allMetrics[key]; ok {
		switch prefix {
		case "consumer-fetch-rate-for-broker-":
			if v, ok := metric["mean.rate"].(float64); ok {
				s.mb.RecordKafkaBrokersConsumerFetchRateDataPoint(now, v, brokerID)
			}
		case "incoming-byte-rate-for-broker-":
			if v, ok := metric["mean.rate"].(float64); ok {
				s.mb.RecordKafkaBrokersIncomingByteRateDataPoint(now, v, brokerID)
			}
		case "outgoing-byte-rate-for-broker-":
			if v, ok := metric["mean.rate"].(float64); ok {
				s.mb.RecordKafkaBrokersOutgoingByteRateDataPoint(now, v, brokerID)
			}
		case "request-rate-for-broker-":
			if v, ok := metric["mean.rate"].(float64); ok {
				s.mb.RecordKafkaBrokersRequestRateDataPoint(now, v, brokerID)
			}
		case "response-rate-for-broker-":
			if v, ok := metric["mean.rate"].(float64); ok {
				s.mb.RecordKafkaBrokersResponseRateDataPoint(now, v, brokerID)
			}
		case "response-size-for-broker-":
			if v, ok := metric["mean"].(float64); ok {
				s.mb.RecordKafkaBrokersResponseSizeDataPoint(now, v, brokerID)
			}
		case "request-size-for-broker-":
			if v, ok := metric["mean"].(float64); ok {
				s.mb.RecordKafkaBrokersRequestSizeDataPoint(now, v, brokerID)
			}
		case "requests-in-flight-for-broker-":
			if v, ok := metric["count"].(int64); ok {
				s.mb.RecordKafkaBrokersRequestsInFlightDataPoint(now, v, brokerID)
			}
		case "request-latency-in-ms-for-broker-":
			if v, ok := metric["mean"].(float64); ok {
				s.mb.RecordKafkaBrokersRequestLatencyDataPoint(now, v, brokerID)
			}
		default:
			fmt.Printf("undefined for prefix %s\n", prefix)
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
		for _, prefix := range nrMetricsPrefix {
			s.scrapeMetric(now, allMetrics, brokerID, prefix)
		}
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
