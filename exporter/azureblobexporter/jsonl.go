package azureblobexporter

import (
	"bytes"
	"encoding/json"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type tracesJSONLMarshaler struct{}

func (m *tracesJSONLMarshaler) MarshalTraces(td ptrace.Traces) ([]byte, error) {
	// This is a simplified marshaler. A more robust implementation would avoid creating a temporary marshaler.
	marshaler := &ptrace.JSONMarshaler{}
	var buf bytes.Buffer
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		tempTd := ptrace.NewTraces()
		rss.At(i).CopyTo(tempTd.ResourceSpans().AppendEmpty())
		jsonBytes, err := marshaler.MarshalTraces(tempTd)
		if err != nil {
			return nil, err
		}

		// The marshaler returns a full JSON document, we need to extract the resourceSpans array's content.
		var data map[string][]json.RawMessage
		if err := json.Unmarshal(jsonBytes, &data); err != nil {
			return nil, err
		}
		if len(data["resourceSpans"]) == 1 {
			buf.Write(data["resourceSpans"][0])
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes(), nil
}

type metricsJSONLMarshaler struct{}

func (m *metricsJSONLMarshaler) MarshalMetrics(md pmetric.Metrics) ([]byte, error) {
	marshaler := &pmetric.JSONMarshaler{}
	var buf bytes.Buffer
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		tempMd := pmetric.NewMetrics()
		rms.At(i).CopyTo(tempMd.ResourceMetrics().AppendEmpty())
		jsonBytes, err := marshaler.MarshalMetrics(tempMd)
		if err != nil {
			return nil, err
		}

		var data map[string][]json.RawMessage
		if err := json.Unmarshal(jsonBytes, &data); err != nil {
			return nil, err
		}
		if len(data["resourceMetrics"]) == 1 {
			buf.Write(data["resourceMetrics"][0])
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes(), nil
}

type logsJSONLMarshaler struct{}

func (m *logsJSONLMarshaler) MarshalLogs(ld plog.Logs) ([]byte, error) {
	marshaler := &plog.JSONMarshaler{}
	var buf bytes.Buffer
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		tempLd := plog.NewLogs()
		rls.At(i).CopyTo(tempLd.ResourceLogs().AppendEmpty())
		jsonBytes, err := marshaler.MarshalLogs(tempLd)
		if err != nil {
			return nil, err
		}

		var data map[string][]json.RawMessage
		if err := json.Unmarshal(jsonBytes, &data); err != nil {
			return nil, err
		}
		if len(data["resourceLogs"]) == 1 {
			buf.Write(data["resourceLogs"][0])
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes(), nil
}
