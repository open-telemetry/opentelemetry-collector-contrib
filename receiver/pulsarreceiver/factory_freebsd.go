// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build freebsd

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver/internal/metadata"
)

const (
	defaultEncoding     = "otlp_proto"
	defaultConsumerName = ""
	defaultSubscription = "otlp_subscription"
	defaultServiceURL   = "pulsar://localhost:6650"
)

var errFreeBSDUnsupported = errors.New("pulsar receiver is not supported on freebsd")

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
		Encoding:     defaultEncoding,
		ConsumerName: defaultConsumerName,
		Subscription: defaultSubscription,
		Endpoint:     defaultServiceURL,
	}
}

func createTracesReceiver(context.Context, receiver.Settings, component.Config, consumer.Traces) (receiver.Traces, error) {
	return nil, errFreeBSDUnsupported
}

func createMetricsReceiver(context.Context, receiver.Settings, component.Config, consumer.Metrics) (receiver.Metrics, error) {
	return nil, errFreeBSDUnsupported
}

func createLogsReceiver(context.Context, receiver.Settings, component.Config, consumer.Logs) (receiver.Logs, error) {
	return nil, errFreeBSDUnsupported
}
