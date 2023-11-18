package rabbitmqexporter

import (
	"context"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
)

type rabbitMqLogsProducer struct{}

func newLogsExporter(config config, set exporter.CreateSettings) (*rabbitMqLogsProducer, error) {
	return &rabbitMqLogsProducer{}, nil
}

func (e *rabbitMqLogsProducer) logsDataPusher(ctx context.Context, ld plog.Logs) error {
	var errs error
	return errs
}

//func (e *rabbitMqLogsProducer) Close(context.Context) error {
//	return nil
//}
