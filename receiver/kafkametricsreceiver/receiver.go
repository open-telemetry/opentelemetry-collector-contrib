// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

// franzGoFeatureGateName is the name of the feature gate for franz-go
const franzGoFeatureGateName = "receiver.kafkametricsreceiver.UseFranzGo"

// franzGoFeatureGate is a feature gate that controls whether the Kafka receiver
// uses the franz-go client or the Sarama client for consuming messages. When enabled,
// the Kafka receiver will use the franz-go client, which is more performant and has
// better support for modern Kafka features.
var franzGoFeatureGate = featuregate.GlobalRegistry().MustRegister(
	franzGoFeatureGateName, featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, the Kafka Metrics receiver will use the franz-go client to connect to Kafka."),
	featuregate.WithRegisterFromVersion("v0.137.0"),
)

type createKafkaScraper func(context.Context, Config, receiver.Settings) (scraper.Metrics, error)

var (
	brokersScraperType   = component.MustNewType("brokers")
	topicsScraperType    = component.MustNewType("topics")
	consumersScraperType = component.MustNewType("consumers")

	// Default (Sarama) factories; franz-go selection happens in scrapersForCurrentGate().
	allScrapers = map[string]createKafkaScraper{
		brokersScraperType.String():   createBrokerScraper,
		topicsScraperType.String():    createTopicsScraper,
		consumersScraperType.String(): createConsumerScraper,
	}

	newSaramaClient = kafka.NewSaramaClient
	newClusterAdmin = sarama.NewClusterAdminFromClient
)

// scrapersForCurrentGate returns the appropriate scraper factory map
// depending on whether the franz-go feature gate is enabled.
func scrapersForCurrentGate() map[string]createKafkaScraper {
	if franzGoFeatureGate.IsEnabled() {
		// Use franz-go implementations
		return map[string]createKafkaScraper{
			brokersScraperType.String():   createBrokerScraperFranz,
			topicsScraperType.String():    createTopicsScraperFranz,
			consumersScraperType.String(): createConsumerScraperFranz,
		}
	}
	// Fall back to Sarama implementations
	return allScrapers
}

var newMetricsReceiver = func(
	ctx context.Context,
	config Config,
	params receiver.Settings,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	// Choose scrapers according to the feature gate at receiver construction time.
	activeScrapers := scrapersForCurrentGate()

	scraperControllerOptions := make([]scraperhelper.ControllerOption, 0, len(config.Scrapers))
	for _, key := range config.Scrapers {
		if factory, ok := activeScrapers[key]; ok {
			s, err := factory(ctx, config, params)
			if err != nil {
				return nil, err
			}
			scraperControllerOptions = append(scraperControllerOptions, scraperhelper.AddScraper(metadata.Type, s))
			continue
		}
		return nil, fmt.Errorf("no scraper found for key: %s", key)
	}

	return scraperhelper.NewMetricsController(
		&config.ControllerConfig,
		params,
		consumer,
		scraperControllerOptions...,
	)
}

// isRecoverableError checks if the error can be resolved by re-establishing connection
func isRecoverableError(err error) bool {
	if errors.Is(err, sarama.ErrOutOfBrokers) {
		return true
	}

	if errors.Is(err, sarama.ErrClosedClient) {
		return true
	}

	if errors.Is(err, os.ErrDeadlineExceeded) {
		// Error example: read tcp 10.2.3.4:62523->4.3.2.1:9093: i/o timeout
		return true
	}

	if errors.Is(err, syscall.EPIPE) {
		return true
	}

	if errors.Is(err, net.ErrClosed) {
		return true
	}

	if errors.Is(err, syscall.ECONNRESET) {
		// Error example: write tcp 1.2.3.4:56532->4.3.2.1:9093: write: connection reset by peer
		return true
	}

	if errors.Is(err, io.EOF) {
		return true
	}

	return false
}
