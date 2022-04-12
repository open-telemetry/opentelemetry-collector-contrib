# vCenter Receiver

This receiver fetches metrics from a vCenter or ESXi host running VMware vSphere APIs.

Supported pipeline types: `metrics` and `logs`

> :construction: This receiver is in **BETA**. Configuration fields and metric data model are subject to change.

## Prerequisites

This receiver has been built to support ESXi and vCenter versions:

- 6.5
- 6.7
- 7.0

But may also work on older versions such as:

- 5.1
- 5.5
- 6.0

A user with at least “Read Only” assigned to a vSphere User with permissions to the vCenter server, cluster and all subsequent resources being monitored must be specified in order for the receiver to retrieve information about them.

## Configuration

Configuration is split up into `metrics` and `logging`

```yaml
receivers:
    vcenter:
      metrics:
      logging:
```

### Metrics Configuration

| Parameter | Default | Type | Notes |
| --- | --- | --- | --- |
| endpoint |  | String | Endpoint to the vCenter Server or ESXi host that has the sdk path enabled. Required. The expected format is `<protocol>://<hostname>` <br><br> i.e: `https://vcsa.hostname.localnet` |
| username |  | String | Required |
| password |  | String | Required |
| tls | | TLSClientSetting | Not Required. Will use defaults for configtls.TLSClientSetting. By default insecure settings are rejected and certificate verification is on. |
| collection_interval | 5m | Duration | This receiver collects metrics on an interval. Valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h` |

### Example Configuration

```yaml
receivers:
  vcenter:
    metrics:
        endpoint: http://localhost:15672
        username: otelu
        password: $RABBITMQ_PASSWORD
        collection_interval: 10s
        metrics: []
```

The full list of settings exposed for this receiver are documented [here](./config.go) with detailed sample configurations [here](./testdata/config.yaml). TLS config is documented further under the [opentelemetry collector's configtls package](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md).

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](./metadata.yaml) with further documentation in [documentation.md](./documentation.md)
