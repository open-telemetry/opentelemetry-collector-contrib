package rabbitmqexporter

import (
	"context"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
)

type rabbitMqLogsProducer struct {
	set           component.TelemetrySettings
	channelCacher *amqpChannelCacher
}

func newLogsExporter(conf config, set exporter.CreateSettings) (*rabbitMqLogsProducer, error) {
	amqpChannelCacher, err := newExporterChannelCacher(conf, set, "otel-logs")
	if err != nil {
		return nil, err
	}

	logsProducer := &rabbitMqLogsProducer{set: set.TelemetrySettings, channelCacher: amqpChannelCacher}
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

func (e *rabbitMqLogsProducer) logsDataPusher(ctx context.Context, ld plog.Logs) error {
	var errs error
	return errs
}

func (e *rabbitMqLogsProducer) Close(context.Context) error {
	return e.channelCacher.close()
}
