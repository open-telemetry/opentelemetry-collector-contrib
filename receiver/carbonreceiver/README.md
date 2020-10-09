# Carbon Receiver

The [Carbon](https://github.com/graphite-project/carbon) receiver supports
Carbon's [plaintext
protocol](https://graphite.readthedocs.io/en/stable/feeding-carbon.html#the-plaintext-protocol).

Supported pipeline types: metrics

> :information_source: The `wavefront` receiver is based on Carbon and binds to the
same port by default. This means the `carbon` and `wavefront` receivers
cannot both be enabled with their respective default configurations. To
support running both receivers in parallel, change the `endpoint` port on one
of the receivers.

## Configuration

The following settings are required:

- `endpoint` (default = `0.0.0.0:2003`): Address and port that the
  receiver should bind to.
- `transport` (default = `tcp`): Must be either `tcp` or `udp`.

The following setting are optional:

- `tcp_idle_timeout` (default = `30s`): The maximum duration that a tcp
  connection will idle wait for new data. This value is ignored if the
  transport is not `tcp`.

In addition, a `parser` section can be defined with the following settings:

- `type` (default `plaintext`): Specifies the type of parser to be used
  and must be either `plaintext` or `regex`.
- `config`: Specifies any special configuration of the selected parser.

Example:

```yaml
receivers:
  carbon/receiver_settings:
    endpoint: localhost:8080
    transport: udp
  carbon/regex:
    parser:
      type: regex
      config:
        rules:
          - regexp: "(?P<key_base>test)\\.env(?P<key_env>[^.]*)\\.(?P<key_host>[^.]*)"
            name_prefix: "name-prefix"
            labels:
              dot.key: dot.value
              key: value
            type: cumulative
          - regexp: "(?P<key_just>test)\\.(?P<key_match>.*)"
        name_separator: "_"
```

The full list of settings exposed for this receiver are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).
