# SAPM Exporter

The SAPM exporter builds on the Jaeger proto and adds additional batching on top. This allows
the collector to export traces from multiples nodes/services in a single batch. The SAPM proto
and some useful related utilities can be found [here](https://github.com/signalfx/sapm-proto/).

- `access_token` (no default): AccessToken is the authentication token provided by SignalFx or
another backend that supports the SAPM proto.
- `endpoint` (no default): This is the destination to where traces will be sent to in SAPM
format. It must be a full URL and include the scheme, port and path e.g,
https://ingest.us0.signalfx.com/v2/trace. This can be pointed to the SignalFx backend or to
another Otel collector that has the SAPM receiver enabled.
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
- `retry_on_failure`
  - `enabled` (default = true)
  - `initial_interval` (default = 5s): Time to wait after the first failure before retrying; ignored if `enabled` is `false`
  - `max_interval` (default = 30s): Is the upper bound on backoff; ignored if `enabled` is `false`
  - `max_elapsed_time` (default = 120s): Is the maximum amount of time spent trying to send a batch; ignored if `enabled` is `false`
- `sending_queue`
  - `enabled` (default = false)
  - `num_consumers` (default = 10): Number of consumers that dequeue batches; ignored if `enabled` is `false`
  - `queue_size` (default = 5000): Maximum number of batches kept in memory before data; ignored if `enabled` is `false`;
  User should calculate this as `num_seconds * requests_per_second` where:
    - `num_seconds` is the number of seconds to buffer in case of a backend outage
    - `requests_per_second` is the average number of requests per seconds.

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

Beyond standard YAML configuration as outlined in the sections that follow,
exporters that leverage the net/http package (all do today) also respect the
following proxy environment variables:

* HTTP_PROXY
* HTTPS_PROXY
* NO_PROXY

If set at Collector start time then exporters, regardless of protocol,
will or will not proxy traffic as defined by these environment variables.
