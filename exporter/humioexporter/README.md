# Humio Exporter
Exports data to Humio using JSON over the HTTP [Ingest API](https://docs.humio.com/reference/api/ingest/).

Supported pipeline types: traces (with metrics and logs to follow soon)

> :construction: This exporter is currently intended for evaluation purposes only!

## Getting Started
This exporter requires the following configuration options:

- `ingest_token` (no default): The token that has been issued in relation to the Humio repository to export data into. This token grants write-only access to a single, specific Humio repository. See [Ingest Tokens](https://docs.humio.com/docs/ingesting-data/ingest-tokens/) for more details.
- `endpoint` (no default): The base URL on which the Humio backend can be reached, in the form `host:port`. For testing this locally with the Humio Docker image, the endpoint could be `http://localhost:8080/`. For use with the Humio cloud, the URLs are as follows, both of which use port `80`:
    - EU: `https://cloud.humio.com/`
    - US: `https://cloud.us.humio.com/`

As defined in the [TLS Configuration Settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md#tls-configuration-settings), TLS is enabled by default. This can be disabled by overriding the following configuration options:

- `insecure` (default: `false`): Whether to enable client transport security for the exporter's HTTP connection. Not recommended for production deployments.
- `insecure_skip_verify` (default: `false`): Whether to skip verifying the server's certificate chain or not. Not recommended for production deployments.

In addition, the following global configuration options can be overridden:

- `tags` (no default): A series of key-value pairs used to target specific Data Sources for storage inside a Humio repository. Refer to [Humio Tagging](https://docs.humio.com/docs/parsers/tagging/) for more details.
- `disable_service_tag` (default: `false`): By default, the service name will be used to tag all exported events in addition to user-provided tags. If disabled, only the user-provided tags will be used. However, at least one tag _must_ be specified.

### Traces
For exporting structured data (traces), the following configuration options are available:

- `unix_timestamps` (default: `false`): Whether to use Unix or ISO 8601 formatted timestamps when exporting data to Humio. If this is set to `true`, timestamps will be represented in milliseconds (Unix time) in UTC.
- `timezone` (default: host timezone): When using Unix timestamps, this option can be provided to specify the local timezone of the events. If not specified, the local time zone of the host is used instead. An example is `Europe/Copenhagen`.

## Advaced Configuration
This exporter, like many others, includes shared configuration helpers for the following advanced settings:

- [HTTP Client Configuration](https://github.com/open-telemetry/opentelemetry-collector/tree/main/config/confighttp#client-configuration)
- [TLS Configuration](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md#tls-configuration-settings)
- [Queueing, Retry, and Timeout Configuration](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md#configuration)

## Example Configuration
Below are two examples of configurations specific to this exporter. For a more advanced example with all available configuration options, see [This Example](testdata/config.yaml).

```yaml
exporters:
    humio:
        ingest_token: "00000000-0000-0000-0000-0000000000000"
        endpoint: "https://my-humio-host:8080"
    humio/advanced:
        ingest_token: "00000000-0000-0000-0000-0000000000000"
        endpoint: "http://localhost:8080"
        timeout: 10s
        disable_service_tag: true
        tags:
            host: "web_server"
            environment: "production"
        logs:
            log_parser: "custom-parser"
        traces:
            unix_timestamps: true
            timezone: "Europe/Copenhagen"
```
