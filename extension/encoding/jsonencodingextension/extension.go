package jsonencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jsonencodingextension"

import (
	"context"
	"time"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

var _ plog.Unmarshaler = &jsonExtension{}
var _ plog.Marshaler = &jsonExtension{}

type jsonExtension struct {
}

func (e *jsonExtension) MarshalLogs(ld plog.Logs) ([]byte, error) {
	marshaler := &plog.JSONMarshaler{}
	return marshaler.MarshalLogs(ld)
}

func (e *jsonExtension) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	p := plog.NewLogs()

	// get json logs from the buffer
	jsonVal := map[string]interface{}{}
	if err := jsoniter.Unmarshal(buf, &jsonVal); err != nil {
		return p, err
	}

	// create a new log record
	logRecords := p.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	logRecords.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// Set the unmarshaled jsonVal as the body of the log record
	if err := logRecords.Body().SetEmptyMap().FromRaw(jsonVal); err != nil {
		return p, err
	}
	return p, nil
}

func (e *jsonExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (e *jsonExtension) Shutdown(_ context.Context) error {
	return nil
}
