// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
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
	ctx context.Context,
	params receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	c := cfg.(*Config)
	// Enable the kafka.cluster.alias resource attribute when the user has
	// configured a cluster_alias. This must be done here (after user config is
	// applied) rather than in createDefaultConfig, where ClusterAlias is always
	// the zero value.
	if c.ClusterAlias != "" {
		c.ResourceAttributes.KafkaClusterAlias.Enabled = true
	}
	r, err := newMetricsReceiver(ctx, *c, params, nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}
