package rabbitmqexporter

import "go.opentelemetry.io/collector/pdata/plog"

type publishingData struct {
	ContentType     string
	ContentEncoding string
	Body            []byte
}

func marshalLogs(logs plog.Logs) (*publishingData, error) {
	return &publishingData{
		ContentType:     "text/plain",
		ContentEncoding: "",
		Body:            []byte("foo"),
	}, nil
}
