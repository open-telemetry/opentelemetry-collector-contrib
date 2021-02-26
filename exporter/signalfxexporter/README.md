# SignalFx Metrics Exporter

This exporter can be used to send metrics, events, and trace correlation to SignalFx.

Apart from metrics, the exporter is also capable of sending metric metadata
(properties and tags) to SignalFx. Currently, only metric metadata updates from
the [k8s_cluster receiver](../../receiver/k8sclusterreceiver/README.md) are
supported.

Supported pipeline types: logs (events), metrics, traces (trace to metric correlation only)

## Metrics Configuration

The following configuration options are required:

- `access_token` (no default): The access token is the authentication token
  provided by SignalFx. The SignalFx access token can be obtained from the
  web app. For details on how to do so please refer the documentation [here](https://docs.signalfx.com/en/latest/admin-guide/tokens.html#access-tokens).
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
- `exclude_metrics`: List of metric filters that will determine metrics to be
  excluded from sending to Signalfx backend. If `translation_rules` options
  are enabled, the exclusion will be applied on translated metrics.
  See [here](./testdata/config.yaml) for examples.
- `include_metrics`: List of filters to override exclusion of any metrics.
  This option can be used to included metrics that are otherwise dropped by
  default. See [here](./translation/default_metrics.go) for a list of metrics
  that are dropped by default. For example, the following configuration can be
  used to send through some of that are dropped by default.
  ```yaml
  include_metrics:
    # When sending in translated metrics.
    - metric_names: [cpu.interrupt, cpu.user, cpu.system]
    # When sending in metrics in OTel convention.
    - metric_name: system.cpu.time
      dimensions:
        state: [interrupt, user, system]
  ```
- `headers` (no default): Headers to pass in the payload.
- `log_dimension_updates` (default = `false`): Whether or not to log dimension
  updates.
- `timeout` (default = 5s): Amount of time to wait for a send operation to
  complete.
- `translation_rules`: Set of rules on how to translate metrics to a SignalFx
  compatible format. Rules defined in `translation/constants.go` are used by
  default. Set this option to `[]` to override the default behavior.
- `sync_host_metadata`: Defines whether the exporter should scrape host metadata
  and send it as property updates to SignalFx backend. Disabled by default.
  IMPORTANT: Host metadata synchronization relies on `resourcedetection`
  processor. If this option is enabled make sure that `resourcedetection`
  processor is enabled in the pipeline with one of the cloud provider detectors
  or environment variable detector setting a unique value to `host.name` attribute
  within your k8s cluster. And keep `override=true` in resourcedetection config.
- `nonalphanumeric_dimension_chars`: (default = `"_-."`) A string of characters 
that are allowed to be used as a dimension key in addition to alphanumeric 
characters. Each nonalphanumeric dimension key character that isn't in this string 
will be replaced with a `_`.

In addition, this exporter offers queued retry which is enabled by default.
Information about queued retry configuration parameters can be found
[here](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md).

## Traces Configuration (correlation only)

:warning: _Note that traces must still be sent in using [sapmexporter](../sapmexporter) to see them in SignalFx._

When traces are sent to the signalfx exporter it correlates traces to metrics. When a new service or environment is
seen it associates the source (e.g. host or pod) to that service or environment in SignalFx. Metrics can then be
filtered based on that trace service and environment (`sf_service` and `sf_environment`).

One of `realm` and `api_url` are required.

- `access_token` (required, no default): The access token is the authentication token
  provided by SignalFx.
- `realm` (no default): SignalFx realm where the data will be received.
- `api_url` (default = `https://api.{realm}.signalfx.com/`): Destination to which correlation updates
   are sent. If a value is explicitly set, the value of `realm` will not be used in determining `api_url`.
   The explicit value will be used instead.
- `correlation` Contains options controlling the syncing of service and environment properties onto dimensions.
  - `endpoint` (required, default = `api_url` or `https://api.{realm}.signalfx.com/`): This is the base URL for API requests (e.g. `https://api.us0.signalfx.com`).
  - `timeout` (default = 5s): Is the timeout for every attempt to send data to the backend.
  - `stale_service_timeout` (default = 5 minutes): How long to wait after a span's service name is last seen before uncorrelating it.
  - `max_requests` (default = 20): Max HTTP requests to be made in parallel.
  - `max_buffered` (default = 10,000): Max number of correlation updates that can be buffered before updates are dropped.
  - `max_retries` (default = 2): Max number of retries that will be made for failed correlation updates.
  - `log_updates` (default = false): Whether or not to log correlation updates to dimensions (at `DEBUG` level).
  - `retry_delay` (default = 30 seconds): How long to wait between retries.
  - `cleanup_interval` (default = 1 minute): How frequently to purge duplicate requests.
  - `sync_attributes` (default = `{"k8s.pod.uid": "k8s.pod.uid", "container.id": "container.id"}`) Map containing key of the attribute to read from spans to sync to dimensions specified as the value.

## Example

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
    traces:
      receivers: [zipkin]
      processors: []
      exporters: [signalfx]
```

The full list of settings exposed for this exporter are documented [here](config.go)
with detailed sample configurations [here](testdata/config.yaml).

This exporter also offers proxy support as documented
[here](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter#proxy-support).
