# Splunk HTTP Event Collector (HEC) Exporter

How to send metrics to a Splunk HEC endpoint.

Supported pipeline types: logs, metrics, traces

> :construction: This receiver is in beta and configuration fields are subject to change.

## Configuration

The following configuration options are required:

- `token` (no default): HEC requires a token to authenticate incoming traffic.
- `endpoint` (no default): Splunk HEC URL.

The following configuration options can also be configured:

- `source` (no default): Optional Splunk source: https://docs.splunk.com/Splexicon:Source
- `sourcetype` (no default): Optional Splunk source type: https://docs.splunk.com/Splexicon:Sourcetype
- `index` (no default): Splunk index, optional name of the Splunk index targeted
- `max_connections` (default: 100): Maximum HTTP connections to use simultaneously when sending data.
- `disable_compression` (default: false): Whether to disable gzip compression over HTTP.
- `timeout` (default: 10s): HTTP timeout when sending data.
- `insecure_skip_verify` (default: false): Whether to skip checking the certificate of the HEC endpoint when sending data over HTTPS.

In addition, this exporter offers queued retry which is enabled by default.
Information about queued retry configuration parameters can be found
[here](https://github.com/open-telemetry/opentelemetry-collector/blob/master/exporter/exporterhelper/README.md).

Example:

```yaml
exporters:
  splunk_hec:
    # Splunk HTTP Event Collector token.
    token: "00000000-0000-0000-0000-0000000000000"
    # URL to a Splunk instance to send data to.
    endpoint: "https://splunk:8088/services/collector"
    # Optional Splunk source: https://docs.splunk.com/Splexicon:Source
    source: "otel"
    # Optional Splunk source type: https://docs.splunk.com/Splexicon:Sourcetype
    sourcetype: "otel"
    # Splunk index, optional name of the Splunk index targeted.
    index: "metrics"
    # Maximum HTTP connections to use simultaneously when sending data. Defaults to 100.
    max_connections: 200
    # Whether to disable gzip compression over HTTP. Defaults to false.
    disable_compression: false
    # HTTP timeout when sending data. Defaults to 10s.
    timeout: 10s
    # Whether to skip checking the certificate of the HEC endpoint when sending data over HTTPS. Defaults to false.
    insecure_skip_verify: false
```

The full list of settings exposed for this exporter are documented [here](config.go)
with detailed sample configurations [here](testdata/config.yaml).

This exporter also offers proxy support as documented
[here](https://github.com/open-telemetry/opentelemetry-collector/tree/master/exporter#proxy-support).
