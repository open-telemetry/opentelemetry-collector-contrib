# Splunk HEC Receiver

The Splunk HEC receiver accepts metrics in the [Splunk HEC
format](https://docs.splunk.com/Documentation/Splunk/8.0.5/Data/FormateventsforHTTPEventCollector).
This allows the collector to receive metrics, traces and logs.

Supported pipeline types: logs, metrics, traces

> :construction: This receiver is in beta and configuration fields are subject to change.

## Configuration

The following settings are required:

* `endpoint` (default = `0.0.0.0:8088`): Address and port that the Splunk HEC
  receiver should bind to.

The following settings are optional:

* `access_token_passthrough` (default = `false`): Whether to preserve incoming
  access token (`Splunk` header value) as
  `"com.splunk.hec.access_token"` metric resource label.  Can be used in
  tandem with identical configuration option for [Splunk HEC
  exporter](../../exporter/splunkhecexporter/README.md) to preserve datapoint
  origin.
* `tls_settings` (no default): This is an optional object used to specify if TLS should be used for
  incoming connections.
    * `cert_file`: Specifies the certificate file to use for TLS connection.
      Note: Both `key_file` and `cert_file` are required for TLS connection.
    * `key_file`: Specifies the key file to use for TLS connection. Note: Both
      `key_file` and `cert_file` are required for TLS connection.

Example:

```yaml
receivers:
  splunk_hec:
  splunk_hec/advanced:
    access_token_passthrough: true
    tls:
      cert_file: /test.crt
      key_file: /test.key
```

The full list of settings exposed for this receiver are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).
