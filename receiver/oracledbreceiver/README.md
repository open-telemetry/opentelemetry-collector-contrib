# Oracle DB receiver

| Status                   |                            |
| ------------------------ |----------------------------|
| Stability                | [in-development]           |
| Supported pipeline types | metrics                    |
| Distributions            | [contrib]                  |

This receiver collects metrics from an Oracle Database.

The receiver connects to a database host and performs periodically queries.

Supported pipeline types: metrics

## Getting Started

The following settings are required:

- `username`: Oracle database account username
- `password`: Oracle database account password
- `endpoint`: Oracle database connection endpoint, of the form `host:port`
- `service_name`: Oracle database service name

Example:

```yaml
receivers:
  oracledb:
    username: otel
    password: password
    endpoint: localhost:51521
    service_name: XE
```