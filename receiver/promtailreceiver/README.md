# Promtail Receiver

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [alpha]   |
| Supported pipeline types | logs      |
| Distributions            | [contrib] |

This receiver runs [promtail client](https://grafana.com/docs/loki/latest/clients/promtail/) to scrape logs

## Unsupported features
The Promtail receiver is meant to minimally be a drop-in replacement for Promtail Client. However,
there are advanced features of Promtail that we don't support and thus explicitly will return
an error for if the receiver's configuration YAML/code contains any of the following

- [x] [server](https://grafana.com/docs/loki/latest/clients/promtail/configuration/#server)
- [x] [clients](https://grafana.com/docs/loki/latest/clients/promtail/configuration/#clients)
- [x] [limits_config](https://grafana.com/docs/loki/latest/clients/promtail/configuration/#limits_config)
- [x] [options_config](https://grafana.com/docs/loki/latest/clients/promtail/configuration/#options_config)


## Getting Started

This receiver is a drop-in replacement for getting Promtail to scrape your
services. It supports [the full set of Promtail configuration in `scrape_configs`][sc].
Just like you would write in a YAML configuration
file before starting Promtail, such as with:

**Note**: Since the collector configuration supports env variable substitution
`$` characters in your prometheus configuration are interpreted as environment
variables.  If you want to use $ characters in your promtail configuration,
you must escape them using `$$`.

```shell
promtail --config.file=prom.yaml
```

You can copy and paste that same configuration under:

```yaml
receivers:
  promtail:
    config:
```

For example:

```yaml
receivers:
    promtail:
      config:
        positions:
          filename: /tmp/positions.yaml
        scrape_configs:
          - job_name: varlogs
            static_configs:
              - labels:
                job: varlogs
                __path__: /var/log/*.log
          - job_name: loki_push
            server:
              http_listen_port: 3101
              grpc_listen_port: 3600
            labels:
              pushserver: push1
              use_incoming_timestamp: true
        target_config:
          sync_period: 5s
```


[sc]: https://grafana.com/docs/loki/latest/clients/promtail/configuration/#scrape_configs

[beta]: https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
[core]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol