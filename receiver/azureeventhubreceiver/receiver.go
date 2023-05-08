// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"errors"
	"fmt"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type dataConsumer interface {
	consume(ctx context.Context, event *eventhub.Event) error
	setNextLogsConsumer(nextLogsConsumer consumer.Logs)
	setNextMetricsConsumer(nextLogsConsumer consumer.Metrics)
}

type eventLogsUnmarshaler interface {
	UnmarshalLogs(event *eventhub.Event) (plog.Logs, error)
}

type eventMetricsUnmarshaler interface {
	UnmarshalMetrics(event *eventhub.Event) (pmetric.Metrics, error)
}

type eventhubReceiver struct {
	eventHandler        eventHandler
	dataType            component.Type
	logger              *zap.Logger
	logsUnmarshaler     eventLogsUnmarshaler
	metricsUnmarshaler  eventMetricsUnmarshaler
	nextLogsConsumer    consumer.Logs
	nextMetricsConsumer consumer.Metrics
	obsrecv             *obsreport.Receiver
}

func (receiver *eventhubReceiver) Start(ctx context.Context, host component.Host) error {

	err := receiver.eventHandler.run(ctx, host)
	return err
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

func (receiver *eventhubReceiver) consume(ctx context.Context, event *eventhub.Event) error {

	switch receiver.dataType {
	case component.DataTypeLogs:
		return receiver.consumeLogs(ctx, event)
	case component.DataTypeMetrics:
		return receiver.consumeMetrics(ctx, event)
	default:
		return fmt.Errorf("invalid data type: %v", receiver.dataType)
	}
}

func (receiver *eventhubReceiver) consumeLogs(ctx context.Context, event *eventhub.Event) error {

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
	receiver.obsrecv.EndLogsOp(logsContext, typeStr, 1, err)

	return err
}

func (receiver *eventhubReceiver) consumeMetrics(ctx context.Context, event *eventhub.Event) error {

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

	receiver.obsrecv.EndMetricsOp(metricsContext, typeStr, 1, err)

	return err
}

func newReceiver(
	receiverType component.Type,
	logsUnmarshaler eventLogsUnmarshaler,
	metricsUnmarshaler eventMetricsUnmarshaler,
	eventHandler eventHandler,
	settings receiver.CreateSettings,
) (component.Component, error) {

	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             settings.ID,
		Transport:              "event",
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}

	eventhubReceiver := &eventhubReceiver{
		dataType:           receiverType,
		eventHandler:       eventHandler,
		logger:             settings.Logger,
		logsUnmarshaler:    logsUnmarshaler,
		metricsUnmarshaler: metricsUnmarshaler,
		obsrecv:            obsrecv,
	}

	eventHandler.setDataConsumer(eventhubReceiver)

	return eventhubReceiver, nil
}
