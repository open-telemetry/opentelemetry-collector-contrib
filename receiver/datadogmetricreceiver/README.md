# Datadog Metrics Receiver

## Configuration

Example:

```yaml
receivers:
  datadogmetrics:
    endpoint: localhost:8122
    read_timeout: 60s
```
### read_timeout (Optional)
The read timeout of the HTTP Server

Default: 60s
