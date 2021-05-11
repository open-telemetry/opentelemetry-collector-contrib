# Carbon Exporter

The [Carbon](https://github.com/graphite-project/carbon) exporter supports
Carbon's [plaintext
protocol](https://graphite.readthedocs.io/en/stable/feeding-carbon.html#the-plaintext-protocol).

Supported pipeline types: metrics

## Configuration

The following settings are required:

- `endpoint` (default = `localhost:2003`): Address and port that the
  exporter should send data to.
- `timeout` (default = `5s`): Maximum duration allowed to connect
  and send data to the configured `endpoint`.

Example:

```yaml
exporters:
  carbon:
    # by default it will export to localhost:2003 using tcp
  carbon/allsettings:
    # use endpoint to specify alternative destinations for the exporter,
    # the default is localhost:2003
    endpoint: localhost:8080
    # timeout is the maximum duration allowed to connecting and sending the
    # data to the configured endpoint.
    # The default is 5 seconds.
    timeout: 10s
```

The full list of settings exposed for this receiver are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).
