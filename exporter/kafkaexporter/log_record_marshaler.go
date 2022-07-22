package kafkaexporter

import (
	"encoding/json"

	"github.com/Shopify/sarama"
	"go.opentelemetry.io/collector/pdata/plog"
)

const (
	logRecordJSON     = "log_record_json"
	logRecordProtobuf = "log_record_protobuf"
)

type logRecordMarshaler struct {
	encoding string
}

func (p logRecordMarshaler) Marshal(ld plog.Logs, topic string) ([]*sarama.ProducerMessage, error) {
	kafkaMessages := []*sarama.ProducerMessage{}

	logRecords := getLogRecords(ld)
	for _, logRecord := range logRecords {
		var bytes []byte
		var err error

		switch p.encoding {
		case logRecordProtobuf:
			// TODO: Add
		case logRecordJSON:
			bytes, err = json.Marshal(logRecord)
			if err != nil {
				return nil, err
			}
		}
		kafkaMessage := &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.ByteEncoder(bytes),
		}
		kafkaMessages = append(kafkaMessages, kafkaMessage)

	}

	return kafkaMessages, nil
}

func (p logRecordMarshaler) Encoding() string {
	return p.encoding
}

func newLogRecordMarshaler(encoding string) LogsMarshaler {
	return logRecordMarshaler{
		encoding: encoding,
	}
}

func getLogRecords(ld plog.Logs) []plog.LogRecord {
	logRecords := []plog.LogRecord{}

	resourceLogs := ld.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		resourceLog := resourceLogs.At(i)
		//resource := resourceLog.Resource()
		scopeLogs := resourceLog.ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			logs := scopeLogs.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {
				logRecord := logs.At(k)
				// TODO: Add resource attributes as record attributes
				logRecords = append(logRecords, logRecord)
			}
		}
	}

	return logRecords
}
