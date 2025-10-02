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
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

type createKafkaScraper func(context.Context, Config, receiver.Settings) (scraper.Metrics, error)

var (
	brokersScraperType   = component.MustNewType("brokers")
	topicsScraperType    = component.MustNewType("topics")
	consumersScraperType = component.MustNewType("consumers")
	allScrapers          = map[string]createKafkaScraper{
		brokersScraperType.String():   createBrokerScraper,
		topicsScraperType.String():    createTopicsScraper,
		consumersScraperType.String(): createConsumerScraper,
	}

	newSaramaClient = kafka.NewSaramaClient
	newClusterAdmin = sarama.NewClusterAdminFromClient
)

var newMetricsReceiver = func(
	ctx context.Context,
	config Config,
	params receiver.Settings,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	scraperControllerOptions := make([]scraperhelper.ControllerOption, 0, len(config.Scrapers))
	for _, scraper := range config.Scrapers {
		if s, ok := allScrapers[scraper]; ok {
			s, err := s(ctx, config, params)
			if err != nil {
				return nil, err
			}
			scraperControllerOptions = append(scraperControllerOptions, scraperhelper.AddScraper(metadata.Type, s))
			continue
		}
		return nil, fmt.Errorf("no scraper found for key: %s", scraper)
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
