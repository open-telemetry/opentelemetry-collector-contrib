// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filereceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filereceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		receiver.WithTraces(createTracesReceiver, metadata.TracesStability),
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
	)
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.CreateSettings,
	cc component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := cc.(*Config)
	return &fileReceiver{
		consumer: consumerType{
			metricsConsumer: consumer,
		},
		path:        cfg.Path,
		logger:      settings.Logger,
		throttle:    cfg.Throttle,
		format:      cfg.FormatType,
		compression: cfg.Compression,
	}, nil
}

func createTracesReceiver(
	_ context.Context,
	settings receiver.CreateSettings,
	cc component.Config,
	consumer consumer.Traces,
) (receiver.Traces, error) {
	cfg := cc.(*Config)
	return &fileReceiver{
		consumer: consumerType{
			tracesConsumer: consumer,
		},
		path:        cfg.Path,
		logger:      settings.Logger,
		throttle:    cfg.Throttle,
		format:      cfg.FormatType,
		compression: cfg.Compression,
	}, nil
}

func createLogsReceiver(
	_ context.Context,
	settings receiver.CreateSettings,
	cc component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	cfg := cc.(*Config)
	return &fileReceiver{
		consumer: consumerType{
			logsConsumer: consumer,
		},
		path:        cfg.Path,
		logger:      settings.Logger,
		throttle:    cfg.Throttle,
		format:      cfg.FormatType,
		compression: cfg.Compression,
	}, nil
}
