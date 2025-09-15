# Audit Log Receiver

The Audit Log Receiver is an OpenTelemetry Collector receiver that accepts audit log data via HTTP and processes it asynchronously using file storage for persistence.

## Architecture

The receiver implements a persistence memory pattern:

1. **HTTP Handler**: Accepts audit log requests via POST to `/v1/logs`
2. **Immediate Response**: Returns HTTP 202 Accepted immediately after storing the request
3. **File Storage**: Uses the file storage extension for persistence
4. **Background Processing**: A goroutine processes stored audit logs asynchronously

## Configuration

### Receiver Configuration

```yaml
receivers:
  auditlogreceiver:
    endpoint: 0.0.0.0:8080
```

### Complete Example

```yaml
receivers:
  auditlogreceiver:
    endpoint: 0.0.0.0:8080

exporters:
  logging:
    loglevel: debug

service:
  pipelines:
    logs:
      receivers: [auditlogreceiver]
      exporters: [logging]
```

## API Usage

### Send Audit Log

```bash
curl -X POST http://localhost:8080/v1/logs \
  -H "Content-Type: application/json" \
  -d '{"event": "user_login", "user": "john.doe", "timestamp": "2024-01-01T00:00:00Z"}'
```

**Response**: HTTP 202 Accepted

## Data Flow

1. **Request Received**: HTTP POST request arrives at `/v1/logs`
2. **Store**: Request body is stored in file storage with unique ID
3. **Response**: HTTP 202 Accepted is returned immediately
4. **Background Processing**: Goroutine processes stored entries every second
5. **Log Processing**: Each entry is processed and marked as completed

## Storage Format

Audit log entries are stored as JSON with the following structure:

```json
{
  "id": "audit_log_1",
  "timestamp": "2024-01-01T00:00:00Z",
  "body": "{\"event\": \"user_login\", \"user\": \"john.doe\"}",
  "processed": false
}
```

## Testing

### Building and Running

1. **Build the receiver:**
   ```bash
   go build .
   ```

2. **Run with test configuration:**
   ```bash
   otelcol-contrib --config testdata/config.yaml
   ```

3. **Test the receiver:**
   ```bash
   # On Windows
   test_audit_log.bat
   
   # On Linux/Mac
   chmod +x test_audit_log.sh
   ./test_audit_log.sh
   ```

### Manual Testing

Send audit logs using curl:

```bash
curl -X POST http://localhost:8080/v1/logs \
  -H "Content-Type: application/json" \
  -d '{"event": "user_login", "user": "john.doe", "timestamp": "2024-01-01T00:00:00Z"}'
```

Expected response: `HTTP 202 Accepted`

## Benefits

- **High Throughput**: Immediate HTTP responses allow for high request rates
- **In-Memory Storage**: Fast storage with background processing
- **Asynchronous Processing**: Background processing prevents blocking HTTP requests
- **Scalability**: Can handle burst traffic by queuing requests
- **OTLP Protocol Support**: Accepts both JSON and protobuf content types

## Architecture Changes

This receiver has been refactored from using Kafka to using in-memory storage:

- **Before**: Direct Kafka producer integration
- **After**: In-memory storage with background processing

The new architecture provides better reliability and simpler deployment without external dependencies.
