# Hydrolix Exporter - Example Payloads

This directory contains example JSON payloads that the OpenTelemetry Hydrolix exporter sends to Hydrolix for metrics, logs, and traces.

## Files

### 1. `metrics_payload.json`
Example metrics payloads showing different metric types:
- **Histogram** - HTTP request duration with bucket counts, bounds, and exemplars
- **Sum** - Monotonic counter for API requests with exemplars
- **Gauge** - Current memory usage snapshot
- **Exponential Histogram** - Request latency with exponential bucketing
- **Summary** - Response time distribution with quantiles

**Key Fields:**
- `tags` - Array of metric attributes (custom labels)
- `serviceTags` - Array of resource attributes (service metadata)
- `exemplars` - Sample data points with trace context linking metrics to traces
- `traceId` / `spanId` - Top-level trace context fields extracted from attributes
- `httpStatusCode`, `httpRoute`, `httpMethod` - Common HTTP fields extracted for convenience

### 2. `logs_payload.json`
Example log records at different severity levels:
- **INFO** - Successful user authentication
- **ERROR** - Database connection timeout
- **WARN** - Cache miss warning
- **DEBUG** - Request validation details
- **FATAL** - Critical system failure

**Key Fields:**
- `body` - The main log message content
- `severity_text` / `severity_number` - Log level (text and numeric)
- `traceId` / `spanId` - Trace context for correlating logs with traces
- `tags` - Array of log attributes (contextual data)
- `serviceTags` - Array of resource attributes (service metadata)
- `observed_timestamp` - When the log was observed by the collector
- `timestamp` - When the log event occurred

### 3. `traces_payload.json`
Example trace spans showing different span types:
- **Server Span** - HTTP server request handling
- **Client Span** - Database query execution
- **Error Span** - Failed order creation with error details
- **Cache Span** - Redis GET operation
- **Producer Span** - Kafka message publishing

**Key Fields:**
- `traceId` - Unique trace identifier linking all spans in a trace
- `spanId` - Unique span identifier
- `parentSpanId` - Parent span ID for span hierarchy
- `spanKind` / `spanKindString` - Span type (Server, Client, Producer, Consumer, Internal)
- `duration` - Span duration in nanoseconds
- `logs` - Array of span events (important moments during span execution)
- `statusCode` / `statusCodeString` - Span status (Ok, Error, Unset)
- `tags` - Array of span attributes
- `serviceTags` - Array of resource attributes

## Common Patterns

### Trace Context Propagation
All three signal types (metrics, logs, traces) include trace context fields:
```json
{
  "traceId": "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
  "spanId": "1a2b3c4d5e6f7g8h"
}
```

This enables correlation across observability signals in Hydrolix.

### Attributes Structure
Attributes are formatted as an array of single-key objects:
```json
"tags": [
  {"key1": "value1"},
  {"key2": "value2"}
]
```

This structure is optimized for Hydrolix's columnar storage and querying.

### Resource vs Metric/Log/Span Attributes
- **`serviceTags`** - Resource attributes (same for all data from a service instance)
- **`tags`** - Metric/Log/Span specific attributes (unique to each data point)

### HTTP Fields
Common HTTP fields are extracted to top-level for easier querying:
- `httpStatusCode` - HTTP response status code
- `httpRoute` - HTTP route/endpoint pattern
- `httpMethod` - HTTP request method

## Using These Examples

### Testing Your Transform
You can use these examples to test your Hydrolix transforms:

```bash
curl -X POST "https://your-cluster.hydrolix.io/ingest" \
  -H "Content-Type: application/json" \
  -H "x-hdx-table: your_table" \
  -H "x-hdx-transform: your_transform" \
  -u "username:password" \
  -d @metrics_payload.json
```

### Understanding Exemplars
Exemplars in metrics provide sample data points that link metrics to specific traces:
```json
"exemplars": [
  {
    "filtered_attributes": {"user_id": "12345"},
    "timestamp": 1762471635031414000,
    "value": 234.5,
    "span_id": "1a2b3c4d5e6f7g8h",
    "trace_id": "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"
  }
]
```

This allows you to:
1. See a histogram of request durations (metric)
2. Jump to specific slow requests (exemplar trace_id)
3. Investigate the full trace to understand why it was slow

### Span Events (Logs within Traces)
Spans can include events (logs) that occurred during the span:
```json
"logs": [
  {
    "name": "authentication.success",
    "timestamp": 1762471640050414000,
    "field": [
      {"message": "User authenticated successfully"},
      {"user.id": "user-12345"}
    ]
  }
]
```

## Schema Considerations

When creating your Hydrolix table schema, consider:

1. **Timestamps** - Use `datetime64` type for nanosecond precision
2. **Arrays** - Use `Array(Map(String, String))` or flatten to multiple columns
3. **Trace IDs** - Use `String` or `FixedString(32)` for hex-encoded trace IDs
4. **Indexing** - Index on `traceId`, `serviceName`, `timestamp` for fast queries
5. **Partitioning** - Partition by `timestamp` for time-series queries

## Additional Resources

- [OpenTelemetry Specification](https://opentelemetry.io/docs/specs/otel/)
- [Hydrolix Documentation](https://docs.hydrolix.io/)
- [OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)
