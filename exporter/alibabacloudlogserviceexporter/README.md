# AlibabaCloud LogService Exporter

This exporter supports sending OpenTelemetry data to [LogService](https://www.alibabacloud.com/product/log-service)

Configuration options:

- `endpoint` (required): Elastic APM Server URL.
- `project` (required): LogService's Project Name.
- `logstore` (required): LogService's Logstore Name.
- `access_key_id` (optional): AlibabaCloud access key id.
- `access_key_secret` (optional): AlibabaCloud access key secret.
- `ecs_ram_role` (optional): set AlibabaCLoud ECS ram role if you are using ACK.
- `max_retry` (optional): max retry count when send fail.
- `max_buffer_size` (optional): max buffer used in memory.

Example:

```yaml
exporters:
  alibabacloud_logservice:
    endpoint: "cn-hangzhou.log.aliyuncs.com"
    project: "demo-project"
    logstore: "demo-logstore"
    access_key_id: "access-key-id"
    access_key_secret: "access-key-secret"
```