# SAPM Exporter

The SAPM exporter builds on the Jaeger proto and adds additional batching on top. This allows
the collector to export traces from multiples nodes/services in a single batch. The SAPM proto
and some useful related utilities can be found [here](https://github.com/signalfx/sapm-proto/).

Supported pipeline types: traces

## Configuration

The following configuration options are required:

- `access_token` (no default): AccessToken is the authentication token provided by SignalFx or
another backend that supports the SAPM proto.
- `endpoint` (no default): This is the destination to where traces will be sent to in SAPM
format. It must be a full URL and include the scheme, port and path e.g,
https://ingest.us0.signalfx.com/v2/trace. This can be pointed to the SignalFx backend or to
another Otel collector that has the SAPM receiver enabled.

The following configuration options can also be configured:

- `max_connections` (default = 100): MaxConnections is used to set a limit to the maximum
idle HTTP connection the exporter can keep open.
- `num_workers` (default = 8): NumWorkers is the number of workers that should be used to
export traces. Exporter can make as many requests in parallel as the number of workers. Note
that this will likely be removed in future in favour of processors handling parallel exporting.
- `access_token_passthrough`: (default = `true`) Whether to use `"com.splunk.signalfx.access_token"`
trace resource attribute, if any, as SFx access token.  In either case this attribute will be deleted
during final translation.  Intended to be used in tandem with identical configuration option for
[SAPM receiver](../../receiver/sapmreceiver/README.md) to preserve trace origin.
- `timeout` (default = 5s): Is the timeout for every attempt to send data to the backend.
- `correlation`
  - `enabled` (default = false): Whether to enable spans to metric correlation. If this block is not set at all then correlation is not enabled due to the default.
  - `endpoint` (default = ""): Required if `enabled` is `true`. This is the base URL for API requests (e.g. https://api.signalfx.com).
  - `stale_service_timeout` (default = 5 minutes): How long to wait after a span's service name is last seen before uncorrelating it.
  - `max_requests` (default = 20): Max HTTP requests to be made in parallel.
  - `max_buffered` (default = 10,000): Max number of correlation updates that can be buffered before updates are dropped.
  - `max_retries` (default = 2): Max number of retries that will be made for failed correlation updates.
  - `log_updates` (default = false): Whether or not to log correlation updates to dimensions (at `DEBUG` level).
  - `retry_delay` (default = 30 seconds): How long to wait between retries.
  - `cleanup_interval` (default = 1 minute): How frequently to purge duplicate requests.
  - `sync_attributes` (default = `{"k8s.pod.uid": "k8s.pod.uid", "container.id": "container.id"}`) Map containing key of the attribute to read from spans to sync to dimensions specified as the value.

In addition, this exporter offers queued retry which is enabled by default.
Information about queued retry configuration parameters can be found
[here](https://github.com/open-telemetry/opentelemetry-collector/blob/master/exporter/exporterhelper/README.md).

Example:

```yaml
exporters:
  sapm:
    access_token: YOUR_ACCESS_TOKEN
    access_token_passthrough: true
    endpoint: https://ingest.YOUR_SIGNALFX_REALM.signalfx.com/v2/trace
    max_connections: 100
    num_workers: 8
```

The full list of settings exposed for this exporter are documented [here](config.go)
with detailed sample configurations [here](testdata/config.yaml).

This exporter also offers proxy support as documented
[here](https://github.com/open-telemetry/opentelemetry-collector/tree/master/exporter#proxy-support).
