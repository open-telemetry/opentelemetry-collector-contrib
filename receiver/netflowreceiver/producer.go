// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package netflowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver"

import (
	"context"

	"github.com/netsampler/goflow2/v2/producer"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver/internal/metadata"
)

// OtelLogsProducerWrapper is a wrapper around a producer.ProducerInterface that sends the messages to a log consumer
type OtelLogsProducerWrapper struct {
	wrapped     producer.ProducerInterface
	logConsumer consumer.Logs
}

// Produce converts the message into a list log records and sends them to log consumer
func (o *OtelLogsProducerWrapper) Produce(msg any, args *producer.ProduceArgs) ([]producer.ProducerMessage, error) {
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
		parseErr := addMessageAttributes(msg, &logRecord)
		if parseErr != nil {
			continue
		}

	}

	// Send the logs to the collector, it is difficult to pass the context here
	err = o.logConsumer.ConsumeLogs(context.TODO(), log)
	if err != nil {
		return flowMessageSet, err
	}

	return flowMessageSet, nil
}

func (o *OtelLogsProducerWrapper) Close() {
	o.wrapped.Close()
}

func (o *OtelLogsProducerWrapper) Commit(flowMessageSet []producer.ProducerMessage) {
	o.wrapped.Commit(flowMessageSet)
}

func newOtelLogsProducer(wrapped producer.ProducerInterface, logConsumer consumer.Logs) producer.ProducerInterface {
	return &OtelLogsProducerWrapper{
		wrapped:     wrapped,
		logConsumer: logConsumer,
	}
}
