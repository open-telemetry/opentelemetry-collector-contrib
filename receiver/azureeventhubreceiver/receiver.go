// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver/internal/metadata"
)

type dataConsumer interface {
	consume(ctx context.Context, event *azureEvent) error
	setNextLogsConsumer(nextLogsConsumer consumer.Logs)
	setNextMetricsConsumer(nextLogsConsumer consumer.Metrics)
	setNextTracesConsumer(nextTracesConsumer consumer.Traces)
}

type eventLogsUnmarshaler interface {
	UnmarshalLogs(event *azureEvent) (plog.Logs, error)
}

type eventMetricsUnmarshaler interface {
	UnmarshalMetrics(event *azureEvent) (pmetric.Metrics, error)
}

type eventTracesUnmarshaler interface {
	UnmarshalTraces(event *azureEvent) (ptrace.Traces, error)
}

type encodingLogsUnmarshaler struct {
	unmarshaler plog.Unmarshaler
}

func (e encodingLogsUnmarshaler) UnmarshalLogs(event *azureEvent) (plog.Logs, error) {
	return e.unmarshaler.UnmarshalLogs(event.Data())
}

type encodingMetricsUnmarshaler struct {
	unmarshaler pmetric.Unmarshaler
}

func (e encodingMetricsUnmarshaler) UnmarshalMetrics(event *azureEvent) (pmetric.Metrics, error) {
	return e.unmarshaler.UnmarshalMetrics(event.Data())
}

type encodingTracesUnmarshaler struct {
	unmarshaler ptrace.Unmarshaler
}

func (e encodingTracesUnmarshaler) UnmarshalTraces(event *azureEvent) (ptrace.Traces, error) {
	return e.unmarshaler.UnmarshalTraces(event.Data())
}

type eventhubReceiver struct {
	eventHandler        *eventhubHandler
	signal              pipeline.Signal
	encodingID          *component.ID
	logger              *zap.Logger
	logsUnmarshaler     eventLogsUnmarshaler
	metricsUnmarshaler  eventMetricsUnmarshaler
	tracesUnmarshaler   eventTracesUnmarshaler
	nextLogsConsumer    consumer.Logs
	nextMetricsConsumer consumer.Metrics
	nextTracesConsumer  consumer.Traces
	obsrecv             *receiverhelper.ObsReport
}

func (receiver *eventhubReceiver) Start(ctx context.Context, host component.Host) error {
	if receiver.encodingID != nil {
		if err := receiver.setEncodingUnmarshaler(host); err != nil {
			return err
		}
	}
	return receiver.eventHandler.run(ctx, host)
}

func (receiver *eventhubReceiver) setEncodingUnmarshaler(host component.Host) error {
	ext, ok := host.GetExtensions()[*receiver.encodingID]
	if !ok {
		return fmt.Errorf("encoding extension %q not found", receiver.encodingID)
	}

	switch receiver.signal {
	case pipeline.SignalLogs:
		unmarshaler, ok := ext.(plog.Unmarshaler)
		if !ok {
			return fmt.Errorf("extension %q is not a logs unmarshaler", receiver.encodingID)
		}
		receiver.logsUnmarshaler = encodingLogsUnmarshaler{unmarshaler}
	case pipeline.SignalMetrics:
		unmarshaler, ok := ext.(pmetric.Unmarshaler)
		if !ok {
			return fmt.Errorf("extension %q is not a metrics unmarshaler", receiver.encodingID)
		}
		receiver.metricsUnmarshaler = encodingMetricsUnmarshaler{unmarshaler}
	case pipeline.SignalTraces:
		unmarshaler, ok := ext.(ptrace.Unmarshaler)
		if !ok {
			return fmt.Errorf("extension %q is not a traces unmarshaler", receiver.encodingID)
		}
		receiver.tracesUnmarshaler = encodingTracesUnmarshaler{unmarshaler}
	default:
		return fmt.Errorf("invalid data type: %v", receiver.signal)
	}

	return nil
}

func (receiver *eventhubReceiver) Shutdown(ctx context.Context) error {
	return receiver.eventHandler.close(ctx)
}

func (receiver *eventhubReceiver) setNextLogsConsumer(nextLogsConsumer consumer.Logs) {
	receiver.nextLogsConsumer = nextLogsConsumer
}

func (receiver *eventhubReceiver) setNextMetricsConsumer(nextMetricsConsumer consumer.Metrics) {
	receiver.nextMetricsConsumer = nextMetricsConsumer
}

func (receiver *eventhubReceiver) setNextTracesConsumer(nextTracesConsumer consumer.Traces) {
	receiver.nextTracesConsumer = nextTracesConsumer
}

func (receiver *eventhubReceiver) consume(ctx context.Context, event *azureEvent) error {
	switch receiver.signal {
	case pipeline.SignalLogs:
		return receiver.consumeLogs(ctx, event)
	case pipeline.SignalMetrics:
		return receiver.consumeMetrics(ctx, event)
	case pipeline.SignalTraces:
		return receiver.consumeTraces(ctx, event)
	default:
		return fmt.Errorf("invalid data type: %v", receiver.signal)
	}
}

func (receiver *eventhubReceiver) consumeLogs(ctx context.Context, event *azureEvent) error {
	if receiver.nextLogsConsumer == nil {
		return nil
	}

	if receiver.logsUnmarshaler == nil {
		return errors.New("unable to unmarshal logs with configured format")
	}

	logsContext := receiver.obsrecv.StartLogsOp(ctx)

	logs, err := receiver.logsUnmarshaler.UnmarshalLogs(event)
	if err != nil {
		return fmt.Errorf("failed to unmarshal logs: %w", err)
	}

	receiver.logger.Debug("Log Records", zap.Any("logs", logs))
	err = receiver.nextLogsConsumer.ConsumeLogs(logsContext, logs)
	receiver.obsrecv.EndLogsOp(logsContext, metadata.Type.String(), 1, err)

	return err
}

func (receiver *eventhubReceiver) consumeMetrics(ctx context.Context, event *azureEvent) error {
	if receiver.nextMetricsConsumer == nil {
		return nil
	}

	if receiver.metricsUnmarshaler == nil {
		return errors.New("unable to unmarshal metrics with configured format")
	}

	metricsContext := receiver.obsrecv.StartMetricsOp(ctx)

	metrics, err := receiver.metricsUnmarshaler.UnmarshalMetrics(event)
	if err != nil {
		return fmt.Errorf("failed to unmarshal metrics: %w", err)
	}

	receiver.logger.Debug("Metric Records", zap.Any("metrics", metrics))
	err = receiver.nextMetricsConsumer.ConsumeMetrics(metricsContext, metrics)

	receiver.obsrecv.EndMetricsOp(metricsContext, metadata.Type.String(), 1, err)

	return err
}

func (receiver *eventhubReceiver) consumeTraces(ctx context.Context, event *azureEvent) error {
	if receiver.nextTracesConsumer == nil {
		return nil
	}

	if receiver.tracesUnmarshaler == nil {
		return errors.New("unable to unmarshal traces with configured format")
	}

	tracesContext := receiver.obsrecv.StartTracesOp(ctx)

	traces, err := receiver.tracesUnmarshaler.UnmarshalTraces(event)
	if err != nil {
		return fmt.Errorf("failed to unmarshal traces: %w", err)
	}

	receiver.logger.Debug("traces Records", zap.Any("traces", traces))
	err = receiver.nextTracesConsumer.ConsumeTraces(tracesContext, traces)

	receiver.obsrecv.EndTracesOp(tracesContext, metadata.Type.String(), 1, err)

	return err
}

func newReceiver(
	signal pipeline.Signal,
	encodingID *component.ID,
	logsUnmarshaler eventLogsUnmarshaler,
	metricsUnmarshaler eventMetricsUnmarshaler,
	tracesUnmarshaler eventTracesUnmarshaler,
	eventHandler *eventhubHandler,
	settings receiver.Settings,
) (component.Component, error) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              "event",
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}

	eventhubReceiver := &eventhubReceiver{
		signal:             signal,
		encodingID:         encodingID,
		eventHandler:       eventHandler,
		logger:             settings.Logger,
		logsUnmarshaler:    logsUnmarshaler,
		metricsUnmarshaler: metricsUnmarshaler,
		tracesUnmarshaler:  tracesUnmarshaler,
		obsrecv:            obsrecv,
	}

	eventHandler.setDataConsumer(eventhubReceiver)

	return eventhubReceiver, nil
}
