// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver/internal/metadata"
)

const (
	// The receiver scope name
	receiverScopeName = "otelcol/" + metadata.Type + "receiver"
)

var (
	errUnexpectedConfigurationType = errors.New("failed to cast configuration to azure event hub config")
)

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
		receiver.WithMetrics(f.createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func (f *eventhubReceiverFactory) createLogsReceiver(
	_ context.Context,
	settings receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (receiver.Logs, error) {

	receiver, err := f.getReceiver(component.DataTypeLogs, cfg, settings)
	if err != nil {
		return nil, err
	}

	receiver.(dataConsumer).setNextLogsConsumer(nextConsumer)

	return receiver, nil
}

func (f *eventhubReceiverFactory) createMetricsReceiver(
	_ context.Context,
	settings receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {

	receiver, err := f.getReceiver(component.DataTypeMetrics, cfg, settings)
	if err != nil {
		return nil, err
	}

	receiver.(dataConsumer).setNextMetricsConsumer(nextConsumer)

	return receiver, nil
}

func (f *eventhubReceiverFactory) getReceiver(
	receiverType component.Type,
	cfg component.Config,
	settings receiver.CreateSettings,
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
		switch receiverType {
		case component.DataTypeLogs:
			if logFormat(receiverConfig.Format) == rawLogFormat {
				logsUnmarshaler = newRawLogsUnmarshaler(settings.Logger)
			} else {
				logsUnmarshaler = newAzureResourceLogsUnmarshaler(settings.BuildInfo, settings.Logger)
			}
		case component.DataTypeMetrics:
			if logFormat(receiverConfig.Format) == rawLogFormat {
				metricsUnmarshaler = nil
				err = errors.New("raw format not supported for Metrics")
			} else {
				metricsUnmarshaler = newAzureResourceMetricsUnmarshaler(settings.BuildInfo, settings.Logger)
			}
		case component.DataTypeTraces:
			err = errors.New("unsupported traces data")
		}

		if err != nil {
			return nil
		}

		eventHandler := newEventhubHandler(receiverConfig, settings)

		var receiver component.Component
		receiver, err = newReceiver(receiverType, logsUnmarshaler, metricsUnmarshaler, eventHandler, settings)
		return receiver
	})

	if err != nil {
		return nil, err
	}

	return r.Unwrap(), err
}
