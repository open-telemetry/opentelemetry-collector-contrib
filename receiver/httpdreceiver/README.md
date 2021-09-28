# HTTPd Receiver

This receiver fetches stats from a Httpd server using the `server-status?auto` endpoint.

Supported pipeline types: `metrics`

> :construction: This receiver is in **BETA**. Configuration fields and metric data model are subject to change.

## Prerequisites

This receiver supports Apache httpd version 2.4+

### mod_status module

In order to receive server statistics, you must configure the server's `httpd.conf` file to [enable status support](https://httpd.apache.org/docs/2.4/mod/mod_status.html).


### Configuration

The following settings are required:
- `endpoint` (default: `http://localhost:8080/server-status?auto`): The URL of the httpd status endpoint

The following settings are optional:
- `collection_interval` (default = `10s`): This receiver collects metrics on an interval. This value must be a string readable by Golang's [time.ParseDuration](https://pkg.go.dev/time#ParseDuration). Valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`.

### Example Configuration

```yaml
receivers:
  httpd:
    endpoint: "http://localhost:8080/server-status?auto"
```

The full list of settings exposed for this receiver are documented [here](./config.go) with detailed sample configurations [here](./testdata/config.yaml).

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](./metadata.yaml)
