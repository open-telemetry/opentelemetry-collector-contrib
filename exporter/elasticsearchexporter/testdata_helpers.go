// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

// createSampleLogRecord creates a comprehensive sample log record for golden file generation
func createSampleLogRecord() plog.LogRecord {
	record := plog.NewLogRecord()

	// Set timestamp
	timestamp := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	record.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	record.SetObservedTimestamp(pcommon.NewTimestampFromTime(timestamp.Add(time.Millisecond)))

	// Set severity
	record.SetSeverityText("INFO")
	record.SetSeverityNumber(plog.SeverityNumberInfo)

	// Set body
	record.Body().SetStr("Sample log message with important information")

	// Set trace context
	traceID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	record.SetTraceID(traceID)
	record.SetSpanID(spanID)
	record.SetFlags(1)

	// Add comprehensive attributes covering common use cases
	attrs := record.Attributes()
	attrs.PutStr("http.method", "GET")
	attrs.PutInt("http.status_code", 200)
	attrs.PutStr("http.url", "https://example.com/api/v1/users")
	attrs.PutStr("event.name", "user.login")
	attrs.PutStr("user.id", "12345")
	attrs.PutBool("success", true)
	attrs.PutDouble("duration_ms", 45.67)

	return record
}

// createSampleSpan creates a comprehensive sample span for golden file generation
func createSampleSpan() ptrace.Span {
	span := ptrace.NewSpan()

	// Set basic span properties
	span.SetName("GET /api/v1/users")
	span.SetKind(ptrace.SpanKindServer)

	// Set timestamps
	startTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	endTime := startTime.Add(150 * time.Millisecond)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	// Set trace context
	traceID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	parentSpanID := [8]byte{0, 1, 2, 3, 4, 5, 6, 7}
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)

	// Set status
	span.Status().SetCode(ptrace.StatusCodeOk)
	span.Status().SetMessage("Success")

	// Add comprehensive attributes
	attrs := span.Attributes()
	attrs.PutStr("http.method", "GET")
	attrs.PutStr("http.url", "https://example.com/api/v1/users")
	attrs.PutInt("http.status_code", 200)
	attrs.PutStr("http.target", "/api/v1/users")
	attrs.PutStr("http.scheme", "https")
	attrs.PutStr("http.host", "example.com")
	attrs.PutStr("db.system", "postgresql")
	attrs.PutStr("db.namespace", "users_db")
	attrs.PutStr("db.query.text", "SELECT * FROM users WHERE id = $1")

	// Add span event
	event := span.Events().AppendEmpty()
	event.SetName("query.executed")
	event.SetTimestamp(pcommon.NewTimestampFromTime(startTime.Add(50 * time.Millisecond)))
	event.Attributes().PutInt("rows.returned", 1)

	// Add span link
	link := span.Links().AppendEmpty()
	linkedTraceID := [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	linkedSpanID := [8]byte{8, 7, 6, 5, 4, 3, 2, 1}
	link.SetTraceID(linkedTraceID)
	link.SetSpanID(linkedSpanID)
	link.Attributes().PutStr("link.type", "follows_from")

	return span
}

// createSampleIndex creates a sample Elasticsearch index for golden file generation
func createSampleIndex(signalType string) elasticsearch.Index {
	return elasticsearch.NewDataStreamIndex(signalType, "generic", "default")
}


// createSampleContext creates a sample encoding context for golden file generation
func createSampleContext() encodingContext {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("service.name", "test-service")
	resource.Attributes().PutStr("service.version", "1.0.0")
	resource.Attributes().PutStr("service.instance.id", "instance-123")
	resource.Attributes().PutStr("deployment.environment", "production")
	resource.Attributes().PutStr("host.name", "prod-server-01")
	resource.Attributes().PutStr("host.arch", "amd64")
	resource.Attributes().PutStr("os.type", "linux")
	resource.Attributes().PutStr("os.version", "5.15.0")
	resource.Attributes().PutStr("telemetry.sdk.name", "opentelemetry")
	resource.Attributes().PutStr("telemetry.sdk.language", "go")
	resource.Attributes().PutStr("telemetry.sdk.version", "1.21.0")

	scope := pcommon.NewInstrumentationScope()
	scope.SetName("github.com/example/myapp")
	scope.SetVersion("1.0.0")
	scope.Attributes().PutStr("scope.attribute", "example-value")

	return encodingContext{
		resource:          resource,
		resourceSchemaURL: "https://opentelemetry.io/schemas/1.21.0",
		scope:             scope,
		scopeSchemaURL:    "https://opentelemetry.io/schemas/1.21.0",
	}
}


// supportsMetrics checks if a mapping mode supports metrics
func supportsMetrics(mode MappingMode) bool {
	return mode == MappingOTel || mode == MappingECS
}

// supportsTraces checks if a mapping mode supports traces
func supportsTraces(mode MappingMode) bool {
	return mode != MappingBodyMap
}

// supportsLogs checks if a mapping mode supports logs
func supportsLogs(mode MappingMode) bool {
	return true // All modes support logs
}
