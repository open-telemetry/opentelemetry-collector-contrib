package rabbitmqexporter

import (
	"go.opentelemetry.io/collector/pdata/plog"
)

type publishingData struct {
	ContentType     string
	ContentEncoding string
	Body            []byte
}

type LogsMarshaler interface {
	// Marshal serializes logs into sarama's ProducerMessages
	Marshal(logs plog.Logs) (*publishingData, error)
}

type defaultLogsMarshaler struct {
	impl *plog.JSONMarshaler
}

func newLogMarshaler() LogsMarshaler {
	return &defaultLogsMarshaler{
		impl: &plog.JSONMarshaler{},
	}
}

func (m *defaultLogsMarshaler) Marshal(logs plog.Logs) (*publishingData, error) {
	body, err := m.impl.MarshalLogs(logs)
	if err != nil {
		return nil, err
	}

	return &publishingData{
		ContentType:     "text/plain",
		ContentEncoding: "",
		Body:            body,
	}, nil
}
