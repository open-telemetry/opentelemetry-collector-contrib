package rabbitmqexporter

import (
	"context"
	"errors"
	"fmt"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"time"
)

type rabbitMqLogsProducer struct {
	set           component.TelemetrySettings
	config        config
	channelCacher *amqpChannelCacher
	marshaller    LogsMarshaler
}

func newLogsExporter(conf config, set exporter.CreateSettings) (*rabbitMqLogsProducer, error) {
	amqpChannelCacher, err := newExporterChannelCacher(conf, set, "otel-logs")
	if err != nil {
		return nil, err
	}

	logsProducer := &rabbitMqLogsProducer{
		set:           set.TelemetrySettings,
		config:        conf,
		channelCacher: amqpChannelCacher,
		marshaller:    newLogMarshaler(),
	}
	return logsProducer, nil
}

func newExporterChannelCacher(conf config, set exporter.CreateSettings, connectionName string) (*amqpChannelCacher, error) {
	connectionConfig := &connectionConfig{
		logger:            set.Logger,
		connectionUrl:     conf.connectionUrl,
		connectionName:    connectionName,
		channelPoolSize:   conf.channelPoolSize,
		connectionTimeout: conf.connectionTimeout,
		heartbeatInterval: conf.connectionHeartbeatInterval,
		confirmationMode:  conf.confirmMode,
	}
	return newAmqpChannelCacher(connectionConfig)
}

func (e *rabbitMqLogsProducer) logsDataPusher(ctx context.Context, data plog.Logs) error {
	e.channelCacher.restoreConnectionIfUnhealthy()
	channelWrapper, err := e.channelCacher.requestHealthyChannelFromPool()

	if err != nil {
		return err
	}

	err, healthyChannel := e.pushData(ctx, data, channelWrapper)
	e.channelCacher.returnChannelToPool(channelWrapper, healthyChannel)
	return err
}

func (e *rabbitMqLogsProducer) pushData(ctx context.Context, data plog.Logs, wrapper *amqpChannelManager) (err error, healthyChannel bool) {
	publishingData, err := e.marshaller.Marshal(data)

	if err != nil {
		return err, true
	}

	deliveryMode := amqp.Transient
	if e.config.durable {
		deliveryMode = amqp.Persistent
	}

	confirmation, err := wrapper.channel.PublishWithDeferredConfirmWithContext(ctx, "", e.config.routingKey, false, false, amqp.Publishing{
		Headers:         amqp.Table{},
		ContentType:     publishingData.ContentType,
		ContentEncoding: publishingData.ContentEncoding,
		DeliveryMode:    deliveryMode,
		Body:            publishingData.Body,
	})

	if err != nil {
		return err, false
	}

	select {
	case <-confirmation.Done():
		if confirmation.Acked() {
			e.set.Logger.Debug("Received ack", zap.Int("channelId", wrapper.id), zap.Uint64("deliveryTag", confirmation.DeliveryTag))
			return nil, true
		}
		e.set.Logger.Warn("Received nack from rabbitmq publishing confirmation", zap.Uint64("deliveryTag", confirmation.DeliveryTag))
		err := errors.New("received nack from rabbitmq publishing confirmation")
		return err, true

	case <-time.After(e.config.publishConfirmationTimeout):
		e.set.Logger.Warn("Timeout waiting for publish confirmation", zap.Duration("timeout", e.config.publishConfirmationTimeout), zap.Uint64("deliveryTag", confirmation.DeliveryTag))
		err := fmt.Errorf("timeout waiting for publish confirmation after %s", e.config.publishConfirmationTimeout)
		return err, false
	}
}

func (e *rabbitMqLogsProducer) Close(context.Context) error {
	return e.channelCacher.close()
}
