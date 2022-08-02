# Zabbix Receiver

| Status                   |                  |
| ------------------------ |------------------|
| Stability                | [in-development] |
| Supported pipeline types | metrics          |
| Distributions            | [contrib]        |

This receiver fetches stats from a Zabbox server using the [Zabbix api](https://www.zabbix.com/documentation/current/en/manual/api).

> :construction: This receiver is in **ALPHA**. Configuration fields and metric data model are subject to change.
## Prerequisites

This receiver supports Zabbix versions `6x.`.

TODO - document access and whatnot

## Configuration

The following settings are required:
- `username`
- `password`
- TODO is this enough

The following settings are optional:

TODO

### Example Configuration

```yaml
receivers:
  zabbix:
    endpoint: http://localhost:1053
    username: otelu
    password: $ZABBIX_PASSWORD
    collection_interval: 10s
```

The full list of settings exposed for this receiver are documented [here](./config.go) with detailed sample configurations [here](./testdata/config.yaml).

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](./metadata.yaml)

[in-development]: https://github.com/open-telemetry/opentelemetry-collector#in-development
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib