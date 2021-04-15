# Humio Exporter
Exports data to Humio using JSON over the HTTP [Ingest API](https://docs.humio.com/reference/api/ingest/).

Supported pipeline types: traces (with metrics and logs to follow soon)

> :construction: This exporter is currently intended for evaluation purposes only! It has yet to be enabled in the build.

## Getting Started
This exporter provides a set of global configuration options, as well as options specific to each telemetry data type. An example structure is illustrated below:

```yaml
exporters:
    humio:
        endpoint: "my-global-endpoint"
        
        traces:
            ingest_token: "my-traces-token"
```

Required global options must always be specified, while options specific to each type of telemetry data are only required if that telemetry type has been enabled in a pipeline. For instance, the pipeline below will not require configuration options for logs or metrics:

```yaml
service:
  pipelines:
    traces:
      receivers: [nop]
      processors: [nop]
      exporters: [humio]
```

The following sections outline the required and optional settings for both global options as well as options specific to each type of telemetry data.

### Global options
This exporter requires as a minimum the following global configuration options:

- `endpoint` (no default): The global base URL on which the Humio backend can be reached, in the form `host:port`. For testing this locally with the Humio Docker image, the endpoint could be `http://localhost:8080/`. For use with the Humio cloud, the URLs are as follows, both of which use port `80`:
    - EU: `https://cloud.humio.com/`
    - US: `https://cloud.us.humio.com/`

In addition, the following optional settings specific to this exporter can be overridden:

- `disable_compression` (default: `false`): Whether to stop compressing payloads with gzip before sending them to Humio. This should only be disabled if compression can be shown to have a negative impact on performance in your specific deployment.
- `tag` (default: `none`): The strategy to use for tagging telemetry data sent to Humio. By default, tagging is disabled, since it is a complex topic. See [Tagging](#Tagging) for more information, including possible values.

This exporter also supports inherited configuration options as described in [Inherited Options](#Inherited-Options). As defined in the [TLS Configuration Settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md#tls-configuration-settings), TLS is enabled by default. This can be disabled by overriding the following configuration options:

- `insecure` (default: `false`): Whether to enable client transport security for the exporter's HTTP connection. Not recommended for production deployments.
- `insecure_skip_verify` (default: `false`): Whether to skip verifying the server's certificate chain or not. Not recommended for production deployments.

### Traces
For exporting traces, the following configuration options are required:

- `ingest_token` (no default): The token that has been issued in relation to the Humio repository to export traces into. This token grants write-only access to a single, specific Humio repository. See [Ingest Tokens](https://docs.humio.com/docs/ingesting-data/ingest-tokens/) for more details.

In addition, the following optional settings can be overridden:

- `unix_timestamps` (default: `false`): Whether to use Unix or ISO 8601 formatted timestamps when exporting data to Humio. If this is set to `true`, timestamps will be represented in milliseconds (Unix time) in UTC, and the time zone of the event is stored separately in the payload sent to Humio.

## Example Configuration
Below are two examples of configurations specific to this exporter, the first of which is the minimal required configuration for traces. For a more advanced example with all available configuration options, see [This Example](testdata/config.yaml).

```yaml
exporters:
    humio:
        endpoint: "https://cloud.humio.com/"
        traces:
            ingest_token: "00000000-0000-0000-0000-0000000000000"
    humio/advanced:
        endpoint: "http://localhost:8080/"
        timeout: 10s
        disable_compression: true
        tag: trace_id
        traces:
            ingest_token: "00000000-0000-0000-0000-0000000000000"
            unix_timestamps: true
```

## Advaced Configuration
### Inherited Options
This exporter, like many others, includes shared configuration helpers for the following advanced settings:

- [HTTP Client Configuration](https://github.com/open-telemetry/opentelemetry-collector/tree/main/config/confighttp#client-configuration)
- [TLS Configuration](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md#tls-configuration-settings)
- [Queueing, Retry, and Timeout Configuration](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md#configuration)

### Tagging
Tagging is a strategy in Humio to optimize search speeds by sharding ingested data into specific data sources. This allows for making queries that quickly rule out the majority of data to search through. See [Humio Tagging](https://docs.humio.com/docs/parsers/tagging/) for more details.

If tagging is disabled on this exporter, Humio will still use its own tags internally to organize the data. One such tag is the `#type` tag, which takes its value from the name of the parser assigned to the [ingest token](https://docs.humio.com/docs/ingesting-data/ingest-tokens/)). This is the only officially recommended tag.

However, due to the nature of OpenTelemetry, the service name or trace ID can make decent tags as well, depending on the intended queries in the backend. For such tags to work well, however, you need to enable tag grouping. See [Tag Grouping](https://docs.humio.com/reference/api/cluster-management-api/#setup-grouping-of-tags) for a thorough discussion.

This exporter supports the following strategies for tags, which thus makes up the legal values for the `tag` option:

| Strategy      | Tag field in Humio |
| ------------- | ------------------ |
| none          | -                  |
| trace_id      | #trace_id          |
| service_name  | #service_name      |

For instance, to enable the `trace_id` strategy, you must create a tag group as such:

```bash
curl $YOUR_HUMIO_URL/api/v1/repositories/$REPOSITORY_NAME/taggrouping \
  -X POST \
  -H "Authorization: Bearer $YOUR_API_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '[ {"field": "trace_id","modulus": 16} ]'
```
