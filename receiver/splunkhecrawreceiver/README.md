# Splunk HEC Raw Receiver

The Splunk HEC raw receiver accepts data delimited by newline separators and ingests them as logs.
See [this example](https://docs.splunk.com/Documentation/Splunk/8.2.2/Data/HECExamples#Example_3:_Send_raw_text_to_HEC) for a use of this receiver.

Supported pipeline types: logs

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
  splunk_hec_raw:
  splunk_hec_raw/advanced:
    access_token_passthrough: true
    tls:
      cert_file: /test.crt
      key_file: /test.key
```

The full list of settings exposed for this receiver are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).
