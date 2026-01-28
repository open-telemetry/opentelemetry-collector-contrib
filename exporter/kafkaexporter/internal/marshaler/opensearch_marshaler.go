package marshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"

import (
	"encoding/hex"
	"encoding/json"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// OpenSearchLogsMarshaler marshals logs to OpenSearch SS4O format.
// It produces JSON documents compatible with OpenSearch's Simple Schema for Observability (SS4O),
// enabling direct consumption of Kafka messages by OpenSearch without additional transformation.
//
// The marshaler preserves all OpenTelemetry semantic information including:
// - Resource attributes (converted to string map)
// - Instrumentation scope with attributes and schema URL
// - Log body, severity, and attributes
// - Trace context (trace ID and span ID) when available
// - Schema URL from the ScopeLogs level
//
// Configuration:
// - unixTimestamps: If true, timestamps are formatted as Unix epoch milliseconds;
//   otherwise, timestamps use ISO 8601 format (RFC3339Nano)
type OpenSearchLogsMarshaler struct {
	unixTimestamps bool
}

// MarshalLogs implements LogsMarshaler
func (m *OpenSearchLogsMarshaler) MarshalLogs(logs plog.Logs) ([]Message, error) {
	var messages []Message

	resourceLogs := logs.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		rl := resourceLogs.At(i)
		resource := rl.Resource()

		scopeLogs := rl.ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			sl := scopeLogs.At(j)
			scope := sl.Scope()
			schemaURL := sl.SchemaUrl()

			logRecords := sl.LogRecords()
			for k := 0; k < logRecords.Len(); k++ {
				record := logRecords.At(k)

				// Encode to SS4O format
				doc, err := m.encodeLogRecord(record, resource, scope, schemaURL)
				if err != nil {
					return nil, err
				}

				// Use trace ID as key if available, otherwise nil
				var key []byte
				if !record.TraceID().IsEmpty() {
					tid := record.TraceID()
					key = tid[:]
				}

				messages = append(messages, Message{
					Key:   key,
					Value: doc,
				})
			}
		}
	}

	return messages, nil
}

// encodeLogRecord converts a plog.LogRecord to SS4O JSON
func (m *OpenSearchLogsMarshaler) encodeLogRecord(
	record plog.LogRecord,
	resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
	schemaURL string,
) ([]byte, error) {
	doc := map[string]any{
		"@timestamp": m.formatTimestamp(record.Timestamp()),
		"body":       record.Body().AsString(),
	}

	// Add observed timestamp if present
	if record.ObservedTimestamp() != 0 {
		doc["observedTimestamp"] = m.formatTimestamp(record.ObservedTimestamp())
	}

	// Add trace/span IDs if present
	if !record.TraceID().IsEmpty() {
		tid := record.TraceID()
		doc["traceId"] = hex.EncodeToString(tid[:])
	}
	if !record.SpanID().IsEmpty() {
		sid := record.SpanID()
		doc["spanId"] = hex.EncodeToString(sid[:])
	}

	// Add severity
	doc["severity"] = map[string]any{
		"text":   record.SeverityText(),
		"number": int64(record.SeverityNumber()),
	}

	// Add attributes
	attributes := make(map[string]any)
	record.Attributes().Range(func(k string, v pcommon.Value) bool {
		attributes[k] = v.AsRaw()
		return true
	})
	doc["attributes"] = attributes

	// Add resource attributes
	resourceMap := make(map[string]string)
	resource.Attributes().Range(func(k string, v pcommon.Value) bool {
		resourceMap[k] = v.AsString()
		return true
	})
	doc["resource"] = resourceMap

	// Add schema URL if present
	if schemaURL != "" {
		doc["schemaUrl"] = schemaURL
	}

	// Add instrumentation scope
	scopeAttrs := make(map[string]any)
	scope.Attributes().Range(func(k string, v pcommon.Value) bool {
		scopeAttrs[k] = v.AsRaw()
		return true
	})

	instrumentationScope := map[string]any{
		"name":    scope.Name(),
		"version": scope.Version(),
	}
	if schemaURL != "" {
		instrumentationScope["schemaUrl"] = schemaURL
	}
	if len(scopeAttrs) > 0 {
		instrumentationScope["attributes"] = scopeAttrs
	}

	doc["instrumentationScope"] = instrumentationScope

	// Marshal to JSON
	return json.Marshal(doc)
}

// formatTimestamp converts pcommon.Timestamp to appropriate format
func (m *OpenSearchLogsMarshaler) formatTimestamp(ts pcommon.Timestamp) any {
	if ts == 0 {
		return nil
	}
	t := ts.AsTime()
	if m.unixTimestamps {
		return t.UnixMilli()
	}
	return t.Format(time.RFC3339Nano)
}

// Encoding returns the encoding name
func (m *OpenSearchLogsMarshaler) Encoding() string {
	return "opensearch_json"
}
