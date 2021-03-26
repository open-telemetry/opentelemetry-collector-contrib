# Humio Exporter
Exports data to Humio using JSON over the HTTP [Ingest API](https://docs.humio.com/reference/api/ingest/).

Supported pipeline types: logs (with metrics and traces to follow soon)

> :construction: This exporter is currently intended for evaluation purposes only

## Configuration
This exporter requires the following configuration options:

- `ingest_token` (no default): The token that has been issued in relation to the Humio repository to export data into. This token grants write-only access to a single, specific Humio repository. See [Ingest Tokens](https://docs.humio.com/docs/ingesting-data/ingest-tokens/) for more details.

In addition, the following global configuration options can be overridden:

- `endpoint` (default: `localhost:8080`): The base URL on which the Humio backend can be reached.
- `tags` (no default): A series of key-value pairs used to target specific Data Sources for storage inside a Humio repository. Refer to [Humio Tagging](https://docs.humio.com/docs/parsers/tagging/) for more details.
- `enable_service_tag` (default: `true`): By default, the service name will be used to tag all exported events in addition to user-provided tags. If disabled, only the user-provided tags will be used. However, at least one tag <u>must</u> be specified.

### Logs
For exporting unstructured data (logs), the following configuration options are available:

- `log_parser` (no default): The [parser](https://docs.humio.com/docs/parsers/) used by Humio, in case no parser has been attached to the ingest token.
<!-- - `fields` (no default): A series of key-value pairs used to annotate every exported event. Useful for assigning metadata that should not be provided as tags. -->

### Traces
For exporting structured data (traces), the following configuration options are available:

- `iso_timestamps` (default: `true`): Whether to use ISO 8601 formatted timestamps when exporting data to Humio. If this is set to `false`, timestamps will instead be represented in milliseconds (Unix time) in UTC.
- `timezone` (no default): When specifying timestamps in milliseconds, this option <u>must</u> be provided to specify the local timezone of the events.
- `enable_rawstrings` (default: false): Whether to serialize structured traces as raw strings when exporting to Humio. These raw strings will be exported in addition to a structured representation of the trace.

> TODO: Can we control the number of HTTP connections we make to the ingest API? Would it be feasible to open up, say, a hundred connections?

> TODO: Should we also make timeouts configurable?

> TODO: How can we represent the UTC timezone in a format understood by Humio?

### External configurations
This exporter also supports queued retry, as documented [here](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md).

## Example Configuration
```yaml
exporters:
    humio:
        ingest_token: "00000000-0000-0000-0000-0000000000000"
    humio/allsettings:
        ingest_token: "00000000-0000-0000-0000-0000000000000"
        endpoint: "localhost:8080"
        enable_service_tag: false
        tags:
            host: "web_server"
            environment: "production"
        logs:
            log_parser: "custom-parser"
        traces:
            iso_timestamps: false
            timezone: "Europe/Copenhagen"
            enable_rawstrings: true
```