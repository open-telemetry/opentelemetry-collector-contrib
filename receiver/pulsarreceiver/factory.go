// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver/internal/metadata"
)

const (
	defaultEncoding     = "otlp_proto"
	defaultConsumerName = ""
	defaultTopicName    = ""
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

func createDefaultConfig() component.Config {
	return &Config{
		Trace: ReceiverOption{
			Topic:        defaultTopicName,
			Encoding:     defaultEncoding,
			ConsumerName: defaultConsumerName,
			Subscription: defaultSubscription,
		},
		Log: ReceiverOption{
			Topic:        defaultTopicName,
			Encoding:     defaultEncoding,
			ConsumerName: defaultConsumerName,
			Subscription: defaultSubscription,
		},
		Metric: ReceiverOption{
			Topic:        defaultTopicName,
			Encoding:     defaultEncoding,
			ConsumerName: defaultConsumerName,
			Subscription: defaultSubscription,
		},
		Endpoint: defaultServiceURL,
	}
}
