# vCenter Receiver

This receiver fetches metrics from a vCenter or ESXi host running VMware vSphere APIs as well as accepts logs forwarded from a vCenter syslog forwarder.

Supported pipeline types: `metrics` and `logs`

> :construction: This receiver is in **BETA**. Configuration fields and metric data model are subject to change.

## Prerequisites

This receiver has been built to support ESXi and vCenter versions:

- 7.0
- 6.7
- 7.5

A “Read Only” user assigned to a vSphere with permissions to the vCenter server, cluster and all subsequent resources being monitored. Must be specified in order for the receiver to retrieve information about them.

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
| tls | | TLSClientSetting | Not Required. Will use defaults for [configtls.TLSClientSetting](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md). By default insecure settings are rejected and certificate verification is on. |
| collection_interval | 5m | Duration | This receiver collects metrics on an interval. Valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h` |

### Example Metrics Configuration

```yaml
receivers:
  vcenter:
    metrics:
      endpoint: http://localhost:15672
      username: otelu
      password: $VCENTER_PASSWORD
      collection_interval: 5m
      metrics: []
```

The full list of settings exposed for this receiver are documented [here](./config.go) with detailed sample configurations [here](./testdata/config.yaml). TLS config is documented further under the [opentelemetry collector's configtls package](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md).

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](./metadata.yaml) with further documentation in [documentation.md](./documentation.md)

## Logging

### Logging Prerequisites

Logging is ingested via syslog forwarding from the vCenter. This [document](https://docs.vmware.com/en/VMware-vSphere/6.7/com.vmware.vsphere.vcsa.doc/GUID-9633A961-A5C3-4658-B099-B81E0512DC21.html) from VMware outlines how to configure syslog forwarding from your vCenter server.

This component wraps the [syslogreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/syslogreceiver) as the engine for ingesting these forwarded syslogs. Currently this receiver only supports these protocols:

- udp
- tcp
- tls

So please be sure to pick the same format on both configuration of the collector and the forwarder.

### Logging Configuration

Almost all configuration options are identical as the [syslogreceiver configuration options](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/syslogreceiver#configuration).

### Example Logging Configuration

```yaml
receivers:
  vcenter:
    logging:
      udp:
        listen_address: 0.0.0.0:5142
      protocol: rfc5424
```
