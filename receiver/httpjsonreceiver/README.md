# HTTP JSON Receiver

[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org)
[![OpenTelemetry](https://img.shields.io/badge/OpenTelemetry-Collector-orange.svg)](https://opentelemetry.io/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

The HTTP JSON Receiver is a custom OpenTelemetry Collector receiver that fetches metrics from HTTP endpoints returning JSON data. It uses JSONPath expressions to extract metric values from the JSON responses.

## Features

- 🌐 **HTTP/HTTPS Support**: Fetch data from any HTTP or HTTPS endpoint
- 🔍 **JSONPath Extraction**: Use powerful JSONPath expressions to extract values from complex JSON structures
- 📊 **Multiple Metric Types**: Support for gauge, counter, and histogram metrics
- ⏰ **Configurable Collection**: Set custom collection intervals and timeouts
- 🔧 **Flexible Configuration**: Support for custom headers, HTTP methods, and request bodies
- 🏷️ **Rich Attributes**: Add custom attributes to metrics for better categorization
- 🔄 **Multiple Endpoints**: Scrape multiple endpoints with different configurations
- 📈 **Performance Optimized**: Efficient JSON parsing and metric generation

## Quick Start

### 1. Configuration

Create a configuration file (\`config.yaml\`):

\`\`\`yaml
receivers:
  httpjson:
    collection_interval: 30s
    initial_delay: 5s
    timeout: 10s
    resource_attributes:
      service.name: "my-service"
      environment: "production"
    endpoints:
      - name: "api_metrics"
        url: "https://api.example.com/metrics"
        method: "GET"
        headers:
          Authorization: "Bearer your-token"
          User-Agent: "otel-collector/1.0"
        metrics:
          - name: "api_response_time"
            json_path: "response_time_ms"
            type: "gauge"
            unit: "ms"
            description: "API response time in milliseconds"
            attributes:
              service: "example-api"
              endpoint: "/metrics"

processors:
  batch:

exporters:
  logging:
    loglevel: debug

service:
  pipelines:
    metrics:
      receivers: [httpjson]
      processors: [batch]
      exporters: [logging]
\`\`\`

### 2. Build and Run

\`\`\`bash
# Build collector with your receiver
ocb --config builder-config.yaml

# Run the collector
./dist/otelcol-custom --config config.yaml
\`\`\`

## Configuration Reference

### Receiver Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| \`collection_interval\` | duration | \`60s\` | How often to collect metrics |
| \`initial_delay\` | duration | \`1s\` | Delay before first collection |
| \`timeout\` | duration | \`10s\` | HTTP request timeout |
| \`resource_attributes\` | map | \`{}\` | Attributes added to all metrics |
| \`endpoints\` | array | \`[]\` | List of endpoints to scrape |

### Endpoint Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| \`name\` | string | \`""\` | Friendly name for the endpoint |
| \`url\` | string | **required** | HTTP/HTTPS URL to scrape |
| \`method\` | string | \`GET\` | HTTP method (GET, POST, PUT, etc.) |
| \`headers\` | map | \`{}\` | HTTP headers to send |
| \`body\` | string | \`""\` | Request body for POST/PUT requests |
| \`timeout\` | duration | inherit | Per-endpoint timeout override |
| \`metrics\` | array | **required** | Metrics to extract from response |

### Metric Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| \`name\` | string | **required** | Metric name |
| \`json_path\` | string | **required** | JSONPath expression to extract value |
| \`type\` | string | \`gauge\` | Metric type: \`gauge\`, \`counter\`, \`histogram\` |
| \`description\` | string | \`""\` | Metric description |
| \`unit\` | string | \`""\` | Metric unit |
| \`value_type\` | string | \`double\` | Value type: \`int\`, \`double\` |
| \`attributes\` | map | \`{}\` | Static attributes for this metric |

## JSONPath Examples

The receiver uses the [gjson](https://github.com/tidwall/gjson) library for JSONPath expressions:

### Basic Examples

\`\`\`json
{
  "temperature": 23.5,
  "humidity": 65,
  "status": "healthy",
  "uptime": "2h15m"
}
\`\`\`

| JSONPath | Value | Description |
|----------|-------|-------------|
| \`temperature\` | \`23.5\` | Simple field access |
| \`humidity\` | \`65\` | Number field |
| \`status\` | \`"healthy"\` | String field (converted to 1 if parsing as number) |

### Nested Objects

\`\`\`json
{
  "system": {
    "cpu": {
      "usage": 45.2,
      "cores": 8
    },
    "memory": {
      "used": 1024,
      "total": 4096
    }
  }
}
\`\`\`

| JSONPath | Value | Description |
|----------|-------|-------------|
| \`system.cpu.usage\` | \`45.2\` | Nested field access |
| \`system.memory.used\` | \`1024\` | Deep nesting |

### Arrays

\`\`\`json
{
  "servers": [
    {"name": "web1", "cpu": 20.5},
    {"name": "web2", "cpu": 35.1},
    {"name": "db1", "cpu": 55.8}
  ],
  "values": [10, 20, 30, 40, 50]
}
\`\`\`

| JSONPath | Value | Description |
|----------|-------|-------------|
| \`servers.#\` | \`3\` | Array length |
| \`servers.0.cpu\` | \`20.5\` | First element's field |
| \`servers.#.cpu\` | \`[20.5,35.1,55.8]\` | All CPU values |
| \`values.#\` | \`5\` | Number array length |

### Advanced Queries

\`\`\`json
{
  "services": [
    {"name": "api", "status": "running", "cpu": 15.2},
    {"name": "db", "status": "running", "cpu": 45.1},
    {"name": "cache", "status": "stopped", "cpu": 0}
  ]
}
\`\`\`

| JSONPath | Value | Description |
|----------|-------|-------------|
| \`services.#(status=running)\` | \`2\` | Count of running services |
| \`services.#(status=running).cpu\` | \`[15.2,45.1]\` | CPU of running services |
| \`services.#(cpu>20).name\` | \`["db"]\` | Names where CPU > 20 |

## Configuration Examples

### Basic API Monitoring

\`\`\`yaml
receivers:
  httpjson:
    collection_interval: 30s
    endpoints:
      - name: "service_health"
        url: "http://api.example.com/health"
        metrics:
          - name: "service_uptime_seconds"
            json_path: "uptime"
            type: "counter"
            unit: "s"
          - name: "service_response_time"
            json_path: "response_time_ms"
            type: "gauge"
            unit: "ms"
\`\`\`

### Multiple Endpoints with Authentication

\`\`\`yaml
receivers:
  httpjson:
    endpoints:
      - name: "api_server"
        url: "https://api.example.com/metrics"
        headers:
          Authorization: "Bearer \${API_TOKEN}"
        metrics:
          - name: "api_requests_total"
            json_path: "requests.total"
            type: "counter"
          - name: "api_errors_total"
            json_path: "requests.errors"
            type: "counter"
            
      - name: "database"
        url: "http://db.internal:8080/stats"
        headers:
          X-API-Key: "\${DB_API_KEY}"
        metrics:
          - name: "db_connections_active"
            json_path: "connections.active"
            type: "gauge"
          - name: "db_query_duration_avg"
            json_path: "queries.avg_duration_ms"
            type: "gauge"
            unit: "ms"
\`\`\`

### POST Requests with Body

\`\`\`yaml
receivers:
  httpjson:
    endpoints:
      - name: "custom_query"
        url: "https://api.example.com/query"
        method: "POST"
        headers:
          Content-Type: "application/json"
          Authorization: "Bearer token"
        body: |
          {
            "query": "SELECT COUNT(*) as total FROM users",
            "format": "json"
          }
        metrics:
          - name: "users_total"
            json_path: "results.0.total"
            type: "gauge"
\`\`\`

### Complex JSON Parsing

\`\`\`yaml
receivers:
  httpjson:
    endpoints:
      - name: "server_farm"
        url: "http://monitoring.example.com/servers"
        metrics:
          - name: "servers_total"
            json_path: "servers.#"
            type: "gauge"
            description: "Total number of servers"
            
          - name: "servers_running"
            json_path: "servers.#(status=running)"
            type: "gauge"
            description: "Number of running servers"
            
          - name: "average_cpu_usage"
            json_path: "servers.#.cpu_percent"
            type: "gauge"
            unit: "%"
            description: "Average CPU usage across all servers"
            
          - name: "high_cpu_servers"
            json_path: "servers.#(cpu_percent>80)"
            type: "gauge"
            description: "Servers with high CPU usage"
\`\`\`

## Metric Types

### Gauge
Best for values that can go up and down:
- Temperature, CPU usage, memory usage
- Current number of connections
- Queue size

\`\`\`yaml
- name: "cpu_usage_percent"
  json_path: "system.cpu.usage"
  type: "gauge"
  unit: "%"
\`\`\`

### Counter
Best for monotonically increasing values:
- Total requests served
- Total bytes transferred
- Error counts

\`\`\`yaml
- name: "requests_total"
  json_path: "stats.requests.total"
  type: "counter"
  unit: "1"
\`\`\`

### Histogram
Best for distributions and percentiles:
- Response times
- Request sizes
- Processing durations

\`\`\`yaml
- name: "response_time"
  json_path: "metrics.response_time_ms"
  type: "histogram"
  unit: "ms"
\`\`\`

## Deployment

### Using OpenTelemetry Collector Builder (OCB)

1. **Create builder configuration** (\`builder-config.yaml\`):

\`\`\`yaml
dist:
  name: otelcol-custom
  description: Custom collector with HTTP JSON receiver
  output_path: ./dist
  version: 1.0.0
  otelcol_version: 0.134.0

receivers:
  - gomod: httpjsonreceiver v1.0.0

processors:
  - gomod: go.opentelemetry.io/collector/processor/batchprocessor v0.134.0

exporters:
  - gomod: go.opentelemetry.io/collector/exporter/prometheusexporter v0.134.0

replaces:
  - httpjsonreceiver => ./receiver/httpjsonreceiver
\`\`\`

2. **Build and run**:

\`\`\`bash
# Build custom collector
ocb --config builder-config.yaml

# Run collector
./dist/otelcol-custom --config config.yaml
\`\`\`

### Docker Deployment

1. **Create Dockerfile**:

\`\`\`dockerfile
FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY dist/otelcol-custom .
COPY config.yaml .

EXPOSE 8888 8889
CMD ["./otelcol-custom", "--config=config.yaml"]
\`\`\`

2. **Build and run**:

\`\`\`bash
docker build -t httpjson-collector .
docker run -p 8888:8888 -p 8889:8889 httpjson-collector
\`\`\`

### Kubernetes Deployment

\`\`\`yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: httpjson-collector
spec:
  replicas: 1
  selector:
    matchLabels:
      app: httpjson-collector
  template:
    metadata:
      labels:
        app: httpjson-collector
    spec:
      containers:
      - name: collector
        image: httpjson-collector:latest
        ports:
        - containerPort: 8888
        - containerPort: 8889
        env:
        - name: API_TOKEN
          valueFrom:
            secretKeyRef:
              name: api-secrets
              key: token
        volumeMounts:
        - name: config
          mountPath: /etc/otel
      volumes:
      - name: config
        configMap:
          name: collector-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: collector-config
data:
  config.yaml: |
    # Your collector configuration here
\`\`\`

## Troubleshooting

### Common Issues

1. **JSON parsing errors**:
   \`\`\`
   WARN Failed to extract metric: invalid JSON response
   \`\`\`
   - Verify the endpoint returns valid JSON
   - Check Content-Type headers
   - Test the endpoint manually with curl

2. **JSONPath not found**:
   \`\`\`
   WARN Failed to extract metric: JSONPath "field.path" not found
   \`\`\`
   - Verify the JSON structure matches your JSONPath
   - Test JSONPath expressions using online tools
   - Check for case sensitivity

3. **Authentication failures**:
   \`\`\`
   ERROR HTTP request failed with status 401
   \`\`\`
   - Verify API tokens and headers
   - Check token expiration
   - Ensure proper header format

4. **Timeout issues**:
   \`\`\`
   ERROR HTTP request failed: context deadline exceeded
   \`\`\`
   - Increase timeout values
   - Check network connectivity
   - Verify endpoint performance

### Debug Mode

Enable debug logging to troubleshoot issues:

\`\`\`yaml
service:
  telemetry:
    logs:
      level: debug
\`\`\`

### Testing JSONPath

Use online JSONPath testers or the \`gjson\` CLI tool:

\`\`\`bash
# Install gjson CLI
go install github.com/tidwall/gjson/cmd/gjson@latest

# Test JSONPath expressions
echo '{"users":{"active":42}}' | gjson users.active
\`\`\`

## Performance Considerations

- **Collection Interval**: Balance between data freshness and load
- **Timeout Settings**: Set appropriate timeouts for your endpoints
- **Concurrent Requests**: The receiver processes endpoints sequentially
- **JSON Size**: Large JSON responses may impact memory usage
- **Network Latency**: Consider network conditions when setting timeouts

## Best Practices

1. **Use meaningful metric names** following OpenTelemetry conventions
2. **Add appropriate units** to metrics (ms, %, bytes, etc.)
3. **Include descriptive attributes** for better metric categorization
4. **Set reasonable collection intervals** to avoid overwhelming endpoints
5. **Implement proper authentication** and secure API access
6. **Monitor collector health** and set up alerts for failures
7. **Use resource attributes** to identify metric sources
8. **Test JSONPath expressions** before deploying to production

## Contributing

1. Fork the repository
2. Create a feature branch (\`git checkout -b feature/amazing-feature\`)
3. Make your changes
4. Add tests for new functionality
5. Ensure tests pass (\`go test ./...\`)
6. Commit your changes (\`git commit -m 'Add amazing feature'\`)
7. Push to the branch (\`git push origin feature/amazing-feature\`)
8. Open a Pull Request

## Testing

\`\`\`bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run benchmarks
go test -bench=. ./...

# Run specific tests
go test -run TestScraper ./...
\`\`\`

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Related Projects

- [OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector)
- [OpenTelemetry Collector Contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib)
- [gjson JSONPath Library](https://github.com/tidwall/gjson)

## Support

- 📚 [Documentation](https://opentelemetry.io/docs/collector/)
- 💬 [Community Slack](https://cloud-native.slack.com/channels/otel-collector)
- �� [Issue Tracker](https://github.com/yourusername/httpjson-receiver/issues)
- 📧 [Mailing List](https://lists.cncf.io/g/cncf-opentelemetry-community)
