# Syslog Receiver

Parses Syslogs from tcp/udp using the [opentelemetry-log-collection](https://github.com/open-telemetry/opentelemetry-log-collection) library.

Supported pipeline types: logs

> :construction: This receiver is in alpha and configuration fields are subject to change.

## Configuration

| Field      | Default          | Description                                                  |
| ---------- | ---------------- | ------------------------------------------------------------ |
| `id`       | `syslog_input`   | A unique identifier for the operator                         |
| `udp`      |`nil`                | Defined udp_input operator. (see the UDP configuration section)  |
| `tcp`      | `nil`               | Defined tcp_input operator. (see the TCP configuration section)  |
| `protocol`    | required         | The protocol to parse the syslog messages as. Options are `rfc3164` and `rfc5424`                                                                                                                                                        |
| `location`    | `UTC`            | The geographic location (timezone) to use when parsing the timestamp (Syslog RFC 3164 only). The available locations depend on the local IANA Time Zone database. [This page](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones) contains many examples, such as `America/New_York`. |
| `timestamp`   | `nil`            | An optional [timestamp](https://github.com/open-telemetry/opentelemetry-log-collection/blob/main/docs/types/timestamp.md) block which will parse a timestamp field before passing the entry to the output operator                                                                                               |
| `severity`    | `nil`            | An optional [severity](https://github.com/open-telemetry/opentelemetry-log-collection/blob/main/docs/docs/types/severity.md) block which will parse a severity field before passing the entry to the output operator
| `labels`   | {}               | A map of `key: value` labels to add to the entry's labels    |
| `resource` | {}               | A map of `key: value` labels to add to the entry's resource  |

### UDP Configuration

| Field             | Default          | Description                                                                       |
| ---               | ---              | ---                                                                               |
| `listen_address`  | required         | A listen address of the form `<ip>:<port>`                                        |

### TCP Configuration

| Field             | Default          | Description                                                                       |
| ---               | ---              | ---                                                                               |
| `max_buffer_size` | `1024kib`        | Maximum size of buffer that may be allocated while reading TCP input              |
| `listen_address`  | required         | A listen address of the form `<ip>:<port>`                                        |
| `tls`             |                  | An optional `TLS` configuration (see the TLS configuration section)               |

#### TLS Configuration

The `tcp_input` operator supports TLS, disabled by default.

| Field             | Default          | Description                               |
| ---               | ---              | ---                                       |
| `enable`          | `false`          | Boolean value to enable or disable TLS    |
| `certificate`     |                  | File path for the X509 certificate chain  |
| `private_key`     |                  | File path for the X509 private key        |



## Example Configurations

TCP Configuration:

```yaml
- type: syslog_input
  protocol: rfc5424
  tcp:
    listen_address: "0.0.0.0:54526"
```

UDP Configuration:
```yaml
- type: syslog_input
  udp:
    listen_address: "0.0.0.0:54526"
  protocol: rfc3164
  location: UTC
```
