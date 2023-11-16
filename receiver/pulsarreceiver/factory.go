// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver/internal/metadata"
)

const (
	defaultEncoding     = "otlp_proto"
	defaultTraceTopic   = "otlp_spans"
	defaultMeticsTopic  = "otlp_metrics"
	defaultLogsTopic    = "otlp_logs"
	defaultConsumerName = ""
	defaultSubscription = "otlp_subscription"
	defaultServiceURL   = "pulsar://localhost:6650"
)

// NewFactory creates Pulsar receiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithTraces(createTracesReceiver, metadata.TracesStability),
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
	)
}

func createTracesReceiver(
	_ context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (receiver.Traces, error) {
	c := *(cfg.(*Config))
	if len(c.Topic) == 0 {
		c.Topic = defaultTraceTopic
	}
	r, err := newTracesReceiver(c, set, nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func createMetricsReceiver(
	_ context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	c := *(cfg.(*Config))
	if len(c.Topic) == 0 {
		c.Topic = defaultMeticsTopic
	}
	r, err := newMetricsReceiver(c, set, nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func createLogsReceiver(
	_ context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (receiver.Logs, error) {
	c := *(cfg.(*Config))
	if len(c.Topic) == 0 {
		c.Topic = defaultLogsTopic
	}
	r, err := newLogsReceiver(c, set, nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func createDefaultConfig() component.Config {
	return &Config{
		ConsumerName: defaultConsumerName,
		Subscription: defaultSubscription,
		Endpoint:     defaultServiceURL,
	}
}
