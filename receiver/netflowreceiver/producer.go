package netflowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver"

import (
	"context"
	"encoding/json"

	"github.com/netsampler/goflow2/v2/producer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver/internal/metadata"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// OtelLogsProducerWrapper is a wrapper around a producer.ProducerInterface that sends the messages to a log consumer
type OtelLogsProducerWrapper struct {
	wrapped     producer.ProducerInterface
	logConsumer consumer.Logs
}

// Produce converts the message into a list log records and sends them to log consumer
func (o *OtelLogsProducerWrapper) Produce(msg interface{}, args *producer.ProduceArgs) ([]producer.ProducerMessage, error) {

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

		// Convert each one to the Otel semantic dictionary format
		otelMessage, err := ConvertToOtel(msg)
		if err != nil {
			continue
		}

		logRecord := logRecords.AppendEmpty()
		logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(otelMessage.Flow.Start))
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(otelMessage.Flow.TimeReceived))

		// The bytes of the message in JSON format
		m, err := json.Marshal(otelMessage)
		if err != nil {
			continue
		}

		// Convert to a map[string]
		// https://opentelemetry.io/docs/specs/otel/logs/data-model/#type-mapstring-any
		sec := map[string]interface{}{}
		if err = json.Unmarshal(m, &sec); err != nil {
			continue
		}

		// Set the map to the log record body
		err = logRecord.Body().SetEmptyMap().FromRaw(sec)
		if err != nil {
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

func NewOtelLogsProducer(wrapped producer.ProducerInterface, logConsumer consumer.Logs) producer.ProducerInterface {
	return &OtelLogsProducerWrapper{
		wrapped:     wrapped,
		logConsumer: logConsumer,
	}
}
