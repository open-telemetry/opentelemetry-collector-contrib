// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

const (
	defaultGroupMatch = ".*"
	defaultTopicMatch = "^[^_].*$"
)

// NewFactory creates kafkametrics receiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
		ClientConfig:         configkafka.NewDefaultClientConfig(),
		GroupMatch:           defaultGroupMatch,
		TopicMatch:           defaultTopicMatch,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

func createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	c := cfg.(*Config)
	r, err := newMetricsReceiver(
		*c, set, nextConsumer,
		kafka.NewSaramaClient,
		sarama.NewClusterAdminFromClient,
	)
	if err != nil {
		return nil, err
	}
	return r, nil
}
