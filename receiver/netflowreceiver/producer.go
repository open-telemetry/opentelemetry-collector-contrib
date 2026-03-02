// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package netflowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver"

import (
	"context"
	"fmt"

	"github.com/netsampler/goflow2/v2/producer"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver/internal/metadata"
)

// otelLogsProducerWrapper is a wrapper around a producer.ProducerInterface that sends the messages to a log consumer
type otelLogsProducerWrapper struct {
	wrapped     producer.ProducerInterface
	logConsumer consumer.Logs
	logger      *zap.Logger
	sendRaw     bool
}

// Produce converts the message into a list log records and sends them to log consumer
func (o *otelLogsProducerWrapper) Produce(msg any, args *producer.ProduceArgs) ([]producer.ProducerMessage, error) {
	defer func() {
		if pErr := recover(); pErr != nil {
			errMessage, _ := pErr.(string)
			o.logger.Error("unexpected error processing the message", zap.String("error", errMessage))
		}
	}()

	// First we let the proto producer parse the message
	// All the netflow protocol and structure is handled by the proto producer
	flowMessageSet, err := o.wrapped.Produce(msg, args)
	if err != nil {
		return flowMessageSet, err
	}

	// Create the otel log structure to hold our messages
	log := plog.NewLogs()
	scopeLog := log.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
	scopeLog.Scope().SetName(metadata.ScopeName)
	scopeLog.Scope().Attributes().PutStr("receiver", metadata.Type.String())
	logRecords := scopeLog.LogRecords()

	// A single netflow packet can contain multiple flow messages
	for _, msg := range flowMessageSet {
		logRecord := logRecords.AppendEmpty()
		if o.sendRaw {
			logRecord.Body().SetStr(fmt.Sprintf("%+v", msg))
		} else {
			// Parse the message and add the attributes to the log record
			err = addMessageAttributes(msg, &logRecord)
			if err != nil {
				o.logger.Error("error adding message attributes", zap.Error(err))
			}
		}
	}

	if len(flowMessageSet) == 0 {
		o.logger.Info("received a packet with no flow messages from", zap.String("agent", args.SamplerAddress.String()))
	}

	err = o.logConsumer.ConsumeLogs(context.Background(), log)
	if err != nil {
		return flowMessageSet, err
	}

	return flowMessageSet, nil
}

func (o *otelLogsProducerWrapper) Close() {
	o.wrapped.Close()
}

func (o *otelLogsProducerWrapper) Commit(flowMessageSet []producer.ProducerMessage) {
	o.wrapped.Commit(flowMessageSet)
}

func newOtelLogsProducer(wrapped producer.ProducerInterface, logConsumer consumer.Logs, logger *zap.Logger, sendRaw bool) producer.ProducerInterface {
	return &otelLogsProducerWrapper{
		wrapped:     wrapped,
		logConsumer: logConsumer,
		logger:      logger,
		sendRaw:     sendRaw,
	}
}
