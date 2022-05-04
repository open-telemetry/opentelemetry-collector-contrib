# MongoDB Receiver

This receiver fetches metrics important to run virtual networking using NSX-T. The receiver ingests metrics via the [NSX Rest API](https://docs.vmware.com/en/VMware-NSX-Data-Center-for-vSphere/6.4/nsx_64_api.pdf). As for logs, it wraps the [syslogreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/syslogreceiver) which can be used to ingest NSX forwarded logs.

Supported pipeline types: `metrics` and `logs`

> :construction: This receiver is in **BETA**. Configuration fields and metric data model are subject to change.

## Purpose

The purpose of this receiver is to allow users to monitor metrics from NSX-T environments.

## Prerequisites

Needs a “Network Admin” user for NSX Manager Resources in the vSphere Client (can be created via Web UI or CLI)

The collector must be able to reach the NSX Manager with port 443 open.

This receiver supports NSX-T Datacenter versions:

- 3.2.0
- 3.1.2

## Metrics Configuration

Under the `metrics` key of the NSX configuration are these parameters:

- `settings` (default: see DefaultMetricsSettings [here])(./internal/metadata/generated_metrics_v2.go): Allows enabling and disabling specific metrics from being collected in this receiver.

- `endpoint`: Endpoint of the NSX Manager. Must be formatted as `{scheme}://{host}:{port}`. Schems supported are `http` and `https`

- `username`: Username of the `Network Admin` user

- `password`: Password of the `Network Admin` user

- `collection_interval`: (default = `1m`): This receiver collects metrics on an interval. This value must be a string readable by Golang's [time.ParseDuration](https://pkg.go.dev/time#ParseDuration). Valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`.

- `timeout`: (default = `1m`) The timeout of running commands against the NSX REST API.

### Example Configuration

```yaml
receivers:
  nsx:
    endpoint: https://nsx-manager
    username: admin
    password: password
    timeout: 60s
    settings:
      nsx.node.cpu.utilization:
        enabled: false

exporters:
  file:
    path: "./content.json"

service:
  pipelines:
    metrics:
      receivers: [nsx]
      exporters: [file]
    logs:
      receivers: [nsx]
      exporters: [file]
```

The full list of settings exposed for this receiver are documented [here](./config.go) with detailed sample configurations [here](./testdata/config.yaml).

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](./metadata.yaml)

## Logging Configuration

Logging is ingested via syslog forwarding from the NSX. This [document](https://docs.vmware.com/en/VMware-NSX-Data-Center-for-vSphere/6.4/com.vmware.nsx.logging.doc/GUID-EA70974C-07F8-469D-8A9D-0ED54F0C8F34.html) from VMware outlines how to configure syslog forwarding from your NSX Data Center.

### Logging Prerequisites

This technology should use the [syslogreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/syslogreceiver) as the engine for ingesting these forwarded syslogs. Currently this receiver only supports these protocols:

- udp
- tcp
- tls

So please be sure to pick the same format on both configuration of the collector and the forwarder.

All configuration options can be found here the [syslogreceiver configuration options](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/syslogreceiver#configuration).

### Example Logging Configuration

```yaml
receivers:
  syslog:
    udp:
      listen_address: 0.0.0.0:5142
    protocol: rfc5424
```
