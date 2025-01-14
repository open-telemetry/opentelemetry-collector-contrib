// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver/internal/metadata"
)

var errUnexpectedConfigurationType = errors.New("failed to cast configuration to azure event hub config")

type eventhubReceiverFactory struct {
	receivers *sharedcomponent.SharedComponents
}

// NewFactory creates a factory for the Azure Event Hub receiver.
func NewFactory() receiver.Factory {
	f := &eventhubReceiverFactory{
		receivers: sharedcomponent.NewSharedComponents(),
	}

	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(f.createLogsReceiver, metadata.LogsStability),
		receiver.WithMetrics(f.createMetricsReceiver, metadata.MetricsStability),
		receiver.WithTraces(f.createTracesReceiver, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func (f *eventhubReceiverFactory) createLogsReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (receiver.Logs, error) {
	receiver, err := f.getReceiver(pipeline.SignalLogs, cfg, settings)
	if err != nil {
		return nil, err
	}

	receiver.(dataConsumer).setNextLogsConsumer(nextConsumer)

	return receiver, nil
}

func (f *eventhubReceiverFactory) createMetricsReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	receiver, err := f.getReceiver(pipeline.SignalMetrics, cfg, settings)
	if err != nil {
		return nil, err
	}

	receiver.(dataConsumer).setNextMetricsConsumer(nextConsumer)

	return receiver, nil
}

func (f *eventhubReceiverFactory) createTracesReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (receiver.Traces, error) {
	receiver, err := f.getReceiver(pipeline.SignalTraces, cfg, settings)
	if err != nil {
		return nil, err
	}

	receiver.(dataConsumer).setNextTracesConsumer(nextConsumer)

	return receiver, nil
}

func (f *eventhubReceiverFactory) getReceiver(
	signal pipeline.Signal,
	cfg component.Config,
	settings receiver.Settings,
) (component.Component, error) {
	var err error
	r := f.receivers.GetOrAdd(cfg, func() component.Component {
		receiverConfig, ok := cfg.(*Config)
		if !ok {
			err = errUnexpectedConfigurationType
			return nil
		}

		var logsUnmarshaler eventLogsUnmarshaler
		var metricsUnmarshaler eventMetricsUnmarshaler
		var tracesUnmarshaler eventTracesUnmarshaler
		switch signal {
		case pipeline.SignalLogs:
			if logFormat(receiverConfig.Format) == rawLogFormat {
				logsUnmarshaler = newRawLogsUnmarshaler(settings.Logger)
			} else {
				logsUnmarshaler = newAzureResourceLogsUnmarshaler(settings.BuildInfo, settings.Logger, receiverConfig.ApplySemanticConventions, receiverConfig.TimeFormats.Logs)
			}
		case pipeline.SignalMetrics:
			if logFormat(receiverConfig.Format) == rawLogFormat {
				metricsUnmarshaler = nil
				err = errors.New("raw format not supported for Metrics")
			} else {
				metricsUnmarshaler = newAzureResourceMetricsUnmarshaler(settings.BuildInfo, settings.Logger, receiverConfig.TimeFormats.Metrics)
			}
		case pipeline.SignalTraces:
			if logFormat(receiverConfig.Format) == rawLogFormat {
				tracesUnmarshaler = nil
				err = errors.New("raw format not supported for Traces")
			} else {
				tracesUnmarshaler = newAzureTracesUnmarshaler(settings.BuildInfo, settings.Logger, receiverConfig.TimeFormats.Traces)
			}
		}

		if err != nil {
			return nil
		}

		eventHandler := newEventhubHandler(receiverConfig, settings)

		var rcvr component.Component
		rcvr, err = newReceiver(signal, logsUnmarshaler, metricsUnmarshaler, tracesUnmarshaler, eventHandler, settings)
		return rcvr
	})

	if err != nil {
		return nil, err
	}

	return r.Unwrap(), err
}
