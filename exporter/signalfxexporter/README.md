# SignalFx Metrics Exporter

This exporter can be used to send metrics to SignalFx.

Apart from metrics, the exporter is also capable of sending metric metadata (properties and tags)
to SignalFx. Currently, only metric metadata updates from the [k8s_cluster receiver](../../receiver/k8sclusterreceiver/README.md)
are supported.

The following configuration options are required:

- `access_token` (no default): AccessToken is the authentication token provided
by SignalFx.
- `realm` (no default): SignalFx realm where the data will be received.

The following configuration options can also be configured:

- `headers` (no default): Headers to pass in the payload.
- `timeout` (default = 5s): Amount of time to wait for a send operation to complete.
- `ingest_url` (no default): Destination
where SignalFx metrics are sent. If `realm` is set, this option is derived and will be
`https://ingest.{realm}.signalfx.com/v2/datapoint`.  If a value is explicitly set, the
value of `realm` will not be used in determining `ingest_url`. The explicit value will
be used instead. If path is not specified, `/v2/datapoint` is used.
- `api_url` (no default): Destination to which SignalFx
[properties and tags](https://docs.signalfx.com/en/latest/metrics-metadata/metrics-metadata.html#metrics-metadata) are sent.
If `realm` is set, this option is derived and will be `https://api.{realm}.signalfx.com/`. If a value is explicitly
set, the value of `realm` will not be used in determining `api_url`. The explicit value will be used instead.
- `log_dimension_updates` (default = `false`): Whether or not to log dimension updates.
- `access_token_passthrough`: (default = `true`) Whether to use `"com.splunk.signalfx.access_token"` metric resource label, if any, as SFx access token.  In either case this label will be dropped during final translation.  Intended to be used in tandem with identical configuration option for [SignalFx receiver](../../receiver/signalfxreceiver/README.md) to preserve datapoint origin.

Note: Either `realm` or both `ingest_url` and `api_url` should be explicitly set.

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

Beyond standard YAML configuration as outlined in the sections that follow,
exporters that leverage the net/http package (all do today) also respect the
following proxy environment variables:

* HTTP_PROXY
* HTTPS_PROXY
* NO_PROXY

If set at Collector start time then exporters, regardless of protocol,
will or will not proxy traffic as defined by these environment variables.
