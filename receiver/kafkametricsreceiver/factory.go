// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

const (
	defaultBroker     = "localhost:9092"
	defaultGroupMatch = ".*"
	defaultTopicMatch = "^[^_].*$"
	defaultClientID   = "otel-metrics-receiver"
)

// NewFactory creates kafkametrics receiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	config := &Config{
		ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
		Brokers:              []string{defaultBroker},
		GroupMatch:           defaultGroupMatch,
		TopicMatch:           defaultTopicMatch,
		ClientID:             defaultClientID,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
	if config.ClusterAlias != "" {
		config.MetricsBuilderConfig.ResourceAttributes.KafkaClusterAlias.Enabled = true
	}
	return config
}

func createMetricsReceiver(
	ctx context.Context,
	params receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics) (receiver.Metrics, error) {
	c := cfg.(*Config)
	r, err := newMetricsReceiver(ctx, *c, params, nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}
