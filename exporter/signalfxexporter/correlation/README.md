# SignalFx Correlation Exporter

This exporter observes traces being sent for their service and environment. When a new service or environment is
seen it associates the source (e.g. host or pod) to that source or environment in SignalFx metrics so that those
metrics can be filtered by the service and environment.

Supported pipeline types: traces

## Configuration

The following configuration options are required:

- `endpoint` (required, no default): This is the base URL for API requests (e.g. https://api.signalfx.com).
- `access_token` (no default): The authentication token provided by SignalFx.
- `timeout` (default = 5s): Is the timeout for every attempt to send data to the backend.
- `stale_service_timeout` (default = 5 minutes): How long to wait after a span's service name is last seen before uncorrelating it.
- `max_requests` (default = 20): Max HTTP requests to be made in parallel.
- `max_buffered` (default = 10,000): Max number of correlation updates that can be buffered before updates are dropped.
- `max_retries` (default = 2): Max number of retries that will be made for failed correlation updates.
- `log_updates` (default = false): Whether or not to log correlation updates to dimensions (at `DEBUG` level).
- `retry_delay` (default = 30 seconds): How long to wait between retries.
- `cleanup_interval` (default = 1 minute): How frequently to purge duplicate requests.
- `sync_attributes` (default = `{"k8s.pod.uid": "k8s.pod.uid", "container.id": "container.id"}`) Map containing key of the attribute to read from spans to sync to dimensions specified as the value.

Example:

```yaml
exporters:
  signalfx_correlation:
    access_token: YOUR_ACCESS_TOKEN
    endpoint: https://api.YOUR_SIGNALFX_REALM.signalfx.com
```

The full list of settings exposed for this exporter are documented [here](config.go)
with detailed sample configurations [here](testdata/config.yaml).

This exporter also offers proxy support as documented
[here](https://github.com/open-telemetry/opentelemetry-collector/tree/master/exporter#proxy-support).
