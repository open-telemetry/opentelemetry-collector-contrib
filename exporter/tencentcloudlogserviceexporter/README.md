# TencentCloud LogService Exporter

This exporter supports sending OpenTelemetry log data to [LogService](https://cloud.tencent.com/product/cls).

# Configuration options:

- `endpoint` (required): LogService's [Endpoint](https://cloud.tencent.com/document/product/614/56473).
- `logset` (required): LogService's LogSet ID.
- `topic` (required): LogService's topic ID.
- `secret_id` (optional): TencentCloud secret id.
- `secret_key` (optional): TencentCloud secret key.

# Example:
## Simple Log Data

```yaml
receivers:
  examplereceiver:

exporters:
  tencentcloud_logservice:
    endpoint: "ap-beijing.cls.tencentcs.com"
    logset: "test-logset"
    topic: "test-topic"
    secret_id: "secret-id"
    secret_key: "secret-key"

service:
  pipelines:
    traces:
      receivers: [examplereceiver]
      exporters: [tencentcloud_logservice]
```