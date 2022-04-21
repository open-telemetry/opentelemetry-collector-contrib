## `syslog_input` operator

The `syslog_input` operator listens for syslog format logs from UDP/TCP packages.

### Configuration Fields

| Field        | Default          | Description |
| ---          | ---              | ---         |
| `id`         | `syslog_input`   | A unique identifier for the operator. |
| `output`     | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `tcp`        | {}               | A [tcp_input config](./tcp_input.md#configuration-fields)  to defined syslog_parser operator. |
| `udp`        | {}               | A [udp_input config](./udp_input.md#configuration-fields)  to defined syslog_parser operator. |
| `syslog`     | required         | A [syslog parser config](./syslog_parser.md#configuration-fields)  to defined syslog_parser operator. |
| `attributes` | {}               | A map of `key: value` pairs to add to the entry's attributes. |
| `resource`   | {}               | A map of `key: value` pairs to add to the entry's resource. |





### Example Configurations

#### Simple

TCP Configuration:
```yaml
- type: syslog_input
  tcp:
     listen_address: "0.0.0.0:54526"
  syslog:
     protocol: rfc5424
```

UDP Configuration:

```yaml
- type: syslog_input
  udp:
     listen_address: "0.0.0.0:54526"
  syslog:
     protocol: rfc3164
     location: UTC
```

