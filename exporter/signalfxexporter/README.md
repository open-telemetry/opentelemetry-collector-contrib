# SignalFx Metrics Exporter

This exporter can be used to send metrics and events to SignalFx.

Apart from metrics, the exporter is also capable of sending metric metadata
(properties and tags) to SignalFx. Currently, only metric metadata updates from
the [k8s_cluster receiver](../../receiver/k8sclusterreceiver/README.md) are
supported.

Supported pipeline types: logs (events), metrics

## Configuration

The following configuration options are required:

- `access_token` (no default): The access token is the authentication token
  provided by SignalFx.
- Either `realm` or both `api_url` and `ingest_url`. Both `api_url` and
  `ingest_url` take precedence over `realm`.
  - `realm` (no default): SignalFx realm where the data will be received.
  - `api_url` (no default): Destination to which SignalFx [properties and
    tags](https://docs.signalfx.com/en/latest/metrics-metadata/metrics-metadata.html#metrics-metadata)
    are sent. If `realm` is set, this option is derived and will be
    `https://api.{realm}.signalfx.com/`. If a value is explicitly set, the
    value of `realm` will not be used in determining `api_url`. The explicit
    value will be used instead.
  - `ingest_url` (no default): Destination where SignalFx metrics are sent. If
    `realm` is set, this option is derived and will be
    `https://ingest.{realm}.signalfx.com/v2/datapoint`.  If a value is
    explicitly set, the value of `realm` will not be used in determining
    `ingest_url`. The explicit value will be used instead. If path is not
    specified, `/v2/datapoint` is used.

The following configuration options can also be configured:

- `access_token_passthrough`: (default = `true`) Whether to use
  `"com.splunk.signalfx.access_token"` metric resource label, if any, as the
  SignalFx access token.  In either case this label will be dropped during
  final translation.  Intended to be used in tandem with identical
  configuration option for [SignalFx
  receiver](../../receiver/signalfxreceiver/README.md) to preserve datapoint
  origin.
- `exclude_metrics`: metric names that will be excluded from sending
  to Signalfx backend. If `send_compatible_metrics` or `translation_rules`
  options are enabled, the exclusion will be applied on translated metrics.
- `headers` (no default): Headers to pass in the payload.
- `log_dimension_updates` (default = `false`): Whether or not to log dimension
  updates.
- `send_compatible_metrics` (default = `false`): Whether metrics must be
  translated to a format backward-compatible with SignalFx naming conventions.
- `timeout` (default = 5s): Amount of time to wait for a send operation to
  complete.
- `translation_rules`: Set of rules on how to translate metrics to a SignalFx
  compatible format. Rules defined in `translation/constants.go` are used by
  default. Applicable only when `send_compatible_metrics` set to `true`.
- `sync_host_metadata`: Defines whether the exporter should scrape host metadata
  and send it as property updates to SignalFx backend. Disabled by default.
  IMPORTANT: Host metadata synchronization relies on `resourcedetection`
  processor. If this option is enabled make sure that `resourcedetection`
  processor is enabled in the pipeline with one of the cloud provider detectors
  or environment variable detector setting a unique value to `host.name` attribute
  within your k8s cluster. And keep `override=true` in resourcedetection config.

In addition, this exporter offers queued retry which is enabled by default.
Information about queued retry configuration parameters can be found
[here](https://github.com/open-telemetry/opentelemetry-collector/blob/master/exporter/exporterhelper/README.md).

Example:

```yaml
exporters:
  signalfx:
    access_token: <replace_with_actual_access_token>
    access_token_passthrough: true
    headers:
      added-entry: "added value"
      dot.test: test
    realm: us1
    timeout: 5s
```

> :warning: When enabling the SignalFx receiver or exporter, configure both the `metrics` and `logs` pipelines.

```yaml
service:
  pipelines:
    metrics:
      receivers: [signalfx]
      processors: [memory_limiter, batch]
      exporters: [signalfx]
    logs:
      receivers: [signalfx]
      processors: [memory_limiter, batch]
      exporters: [signalfx]
```

The full list of settings exposed for this exporter are documented [here](config.go)
with detailed sample configurations [here](testdata/config.yaml).

This exporter also offers proxy support as documented
[here](https://github.com/open-telemetry/opentelemetry-collector/tree/master/exporter#proxy-support).
