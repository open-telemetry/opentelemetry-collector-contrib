# New Relic Oracle Receiver

The New Relic Oracle Receiver is an OpenTelemetry receiver that collects Oracle database metrics and formats them in a way that's compatible with New Relic's monitoring infrastructure.

## Features

This receiver currently collects the following Oracle metrics:

- **Session Count**: Total number of active Oracle database sessions (`newrelicoracledb.sessions.count`)

## Configuration

The receiver supports the following configuration options:

### Basic Configuration

```yaml
receivers:
  newrelicoracledb:
    datasource: "oracle://username:password@hostname:port/service"
    collection_interval: 10s
```

### Alternative Configuration (Individual Parameters)

```yaml
receivers:
  newrelicoracledb:
    endpoint: "hostname:1521"
    username: "oracle_user"
    password: "oracle_password"
    service: "XE"
    collection_interval: 10s
```

### Configuration Parameters

| Parameter | Description | Required | Default |
|-----------|-------------|----------|---------|
| `datasource` | Complete Oracle connection string | No* | |
| `endpoint` | Oracle database host and port (host:port) | No* | |
| `username` | Oracle database username | No* | |
| `password` | Oracle database password | No* | |
| `service` | Oracle service name | No* | |
| `collection_interval` | How often to collect metrics | No | 10s |

*Either `datasource` OR all of `endpoint`, `username`, `password`, and `service` must be provided.

## Prerequisites

- Oracle database (tested with Oracle 11g, 12c, 19c, and later)
- Oracle user with appropriate permissions to query system views:
  - `SELECT` permission on `v$session`

## Metrics

### `newrelicoracledb.sessions.count`

- **Description**: Total number of active Oracle database sessions
- **Type**: Gauge
- **Unit**: sessions
- **Attributes**: 
  - `newrelic.entity_name`: New Relic entity name for the metric

## Resource Attributes

- `newrelicoracledb.instance.name`: The name of the Oracle instance
- `host.name`: The host name of the Oracle server

## Example Configuration

```yaml
receivers:
  newrelicoracledb:
    datasource: "oracle://myuser:mypassword@localhost:1521/XE"
    collection_interval: 30s
    metrics:
      newrelicoracledb.sessions.count:
        enabled: true

processors:
  batch:

exporters:
  logging:
    loglevel: debug

service:
  pipelines:
    metrics:
      receivers: [newrelicoracledb]
      processors: [batch]
      exporters: [logging]
```

## Troubleshooting

### Common Issues

1. **Connection Refused**: Verify Oracle database is running and accessible from the collector host
2. **Authentication Failed**: Check username and password are correct
3. **Permission Denied**: Ensure the Oracle user has SELECT permissions on system views

### Logs

Enable debug logging to see detailed information about the receiver's operation:

```yaml
service:
  telemetry:
    logs:
      level: debug
```
