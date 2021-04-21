# Honeycomb Exporter

**NOTE:** Honeycomb now supports OTLP ingest directly. This means you can use an [OTLP](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlpexporter) exporter and no longer need this exporter to send data to Honeycomb.

This exporter supports sending trace data to [Honeycomb](https://www.honeycomb.io).

The following configuration options are supported:

* `api_key` (Required): This is the API key (also called Write Key) for your Honeycomb account.
* `dataset` (Required): The Honeycomb dataset that you want to send events to.
* `api_url` (Optional): You can set the hostname to send events to. Useful for debugging, defaults to `https://api.honeycomb.io`
* `sample_rate` (Optional): Constant sample rate. Can be used to send 1 / x events to Honeycomb. Defaults to 1 (always sample).
* `sample_rate_attribute` (Optional): The name of an attribute that contains the sample_rate for each span. If the attribute is on the span, it takes precedence over the static sample_rate configuration
* `debug` (Optional): Set this to true to get debug logs from the honeycomb SDK. Defaults to false.
* `retry_on_failure` (Optional):
  - `enabled` (default = true)
  - `initial_interval` (default = 5s): Time to wait after the first failure before retrying; ignored if `enabled` is `false`
  - `max_interval` (default = 30s): Is the upper bound on backoff; ignored if `enabled` is `false`
  - `max_elapsed_time` (default = 120s): Is the maximum amount of time spent trying to send a batch; ignored if `enabled` is `false`
* `sending_queue` (Optional):
  - `enabled` (default = true)
  - `num_consumers` (default = 10): Number of consumers that dequeue batches; ignored if `enabled` is `false`
  - `queue_size` (default = 5000): Maximum number of batches kept in memory before data; ignored if `enabled` is `false`;

Example:

```yaml
exporters:
  honeycomb:
    api_key: "my-api-key"
    dataset: "my-dataset"
    api_url: "https://api.testhost.io"
    sample_rate: 25
    sample_rate_attribute: "hny.sample_rate"
    debug: true
    retry_on_failure:
      enabled: true
      initial_interval: 5
      max_interval: 30
      max_elapsed_time: 120
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 10000
```
