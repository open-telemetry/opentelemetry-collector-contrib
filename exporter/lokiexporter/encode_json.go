package lokiexporter

import (
	"encoding/json"

	"go.opentelemetry.io/collector/model/pdata"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
)

// JSON representation of the LogRecord as described by https://developers.google.com/protocol-buffers/docs/proto3#json

type lokiEntry struct {
	Name       string                 `json:"name,omitempty"`
	Body       string                 `json:"body,omitempty"`
	TraceID    string                 `json:"traceid,omitempty"`
	SpanID     string                 `json:"spanid,omitempty"`
	Severity   string                 `json:"severity,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

func encodeJSON(lr pdata.LogRecord) (string, error) {
	var logRecord lokiEntry
	var jsonRecord []byte

	logRecord = lokiEntry{
		Name:       lr.Name(),
		Body:       lr.Body().StringVal(),
		TraceID:    lr.TraceID().HexString(),
		SpanID:     lr.SpanID().HexString(),
		Severity:   lr.SeverityText(),
		Attributes: tracetranslator.AttributeMapToMap(lr.Attributes())}

	jsonRecord, err := json.Marshal(logRecord)
	if err != nil {
		return "", err
	}
	return string(jsonRecord), nil
}
