# SAP HANA Receiver

This receiver can fetch stats from a SAP HANA instance.

> :construction: This receiver is currently in **BETA**.

## Details

## Configuration

> :information_source: This receiver is in beta and configuration fields are subject to change.

The following settings are required:

- `endpoint` (default: `localhost:33015`): The hostname/IP address and port of the SAP HANA instance
- `username`
- `password`

The following settings are optional:

- `collection_interval` (default = `10s`): This receiver runs on an interval.
Each time it runs, it queries memcached, creates metrics, and sends them to the
next consumer. The `collection_interval` configuration option tells this
receiver the duration between runs. This value must be a string readable by
Golang's `ParseDuration` function (example: `1h30m`). Valid time units are
`ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`.
- `tls`:
  - `insecure` (default = true): whether to disable client transport security for the exporter's connection.
  - `ca_file`: path to the CA cert. For a client this verifies the server certificate. Should only be used if `insecure` is set to false.
  - `cert_file`: path to the TLS cert to use for TLS required connections. Should only be used if `insecure` is set to false.
  - `key_file`: path to the TLS key to use for TLS required connections. Should only be used if `insecure` is set to false.

Example:

```yaml
receivers:
  saphana:
    endpoint: "localhost:33015"
    collection_interval: 10s
```

The full list of settings exposed for this receiver are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).
