# Uptrace Exporter

This exporter sends trace data to [Uptrace.dev](https://uptrace.dev).

## Configuration

| Option | Description                                          |
| ------ | ---------------------------------------------------- |
| `dsn`  | Data source name for your Uptrace project. Required. |

Example:

```yaml
exporters:
  uptrace:
    dsn: "https://<key>@api.uptrace.dev/<project_id>"
```

Also the following optional settings can be used to configure the `*http.Client`:

| Option              | Default      | Description                                                                                                                                              |
| ------------------- | ------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `timeout`           | `30s`        | HTTP request time limit. For details see https://golang.org/pkg/net/http/#Client                                                                         |
| `read_buffer_size`  | `0`          | ReadBufferSize for HTTP client.                                                                                                                          |
| `write_buffer_size` | `512 * 1024` | WriteBufferSize for HTTP client.                                                                                                                         |
| `insecure`          | `false`      | When set to true disables verifying the server's certificate chain and host name. The connection is still encrypted but server identity is not verified. |
| `ca_file`           |              | Path to the CA cert. For a client this verifies the server certificate. Should only be used if `insecure` is set to false.                               |
| `cert_file`         |              | Path to the TLS cert to use for TLS required connections. Should only be used if `insecure` is set to false.                                             |
| `key_file`          |              | Path to the TLS key to use for TLS required connections. Should only be used if `insecure` is set to false.                                              |

The full list of settings exposed for this exporter are documented [here](./config.go).
