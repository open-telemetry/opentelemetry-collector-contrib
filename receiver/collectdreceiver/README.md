# CollectD `write_http` plugin JSON receiver

This receiver can receive data exported by the CollectD's `write_http`
plugin. Only JSON format is supported. Authentication is not supported at
this time.

This receiver was donated by SignalFx and ported from SignalFx's Gateway
(https://github.com/signalfx/gateway/tree/master/protocol/collectd). As a
result, this receiver supports some additional features that are technically
not compatible with stock CollectD's write_http plugin. That said, in
practice such incompatibilities should never surface. For example, this
receiver supports extracting labels from different fields. Given a field
value `field[a=b, k=v]`, this receiver will extract `a` and `b` as label keys
and, `k` and `v` as the respective label values.

Supported pipeline types: metrics

## Configuration

The following settings are required:

- `endpoint` (default = `localhost:8081`): Address to reach the desired Docker daemon.

The following settings are optional:

- `attributes_prefix` (no default): Used to add query parameters in key=value format to all metrics.
- `timeout` (default = `30s`): The request timeout for any docker daemon query.

Example:

```yaml
receivers:
  collectd:
  collectd/one:
    attributes_prefix: "dap_"
    endpoint: "localhost:12345"
    timeout: "50s"
```

The full list of settings exposed for this receiver are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).
