# SignalFx Metrics Exporter

How to send metrics to SignalFx.

The following configuration options are required:

- `access_token` (no default): AccessToken is the authentication token provided
by SignalFx or another backend that supports the SAPM proto.
- `realm` (default = us0): SignalFx realm where the data will be received.

The following configuration options can also be configured:

- `headers` (no default): Headers to pass in payload
- `timeout` (default = 5s): Amount of time to wait for send operation to complete.
- `url` (default = https://ingest.`realm`.signalfx.com/v2/datapoint): Destination
where SignalFx metrics are sent. If this option is specified, `realm` is ignored.
If path is not specified, `/v2/datapoint` is used.

Example:

```yaml
exporters:
  signalfx:
    access_token: <replace_with_actual_access_token>
    headers:
      added-entry: "added value"
      dot.test: test
    realm: us1
    timeout: 5s
```

Beyond standard YAML configuration as outlined in the sections that follow,
exporters that leverage the net/http package (all do today) also respect the
following proxy environment variables:

* HTTP_PROXY
* HTTPS_PROXY
* NO_PROXY

If set at Collector start time then exporters, regardless of protocol,
will or will not proxy traffic as defined by these environment variables.
