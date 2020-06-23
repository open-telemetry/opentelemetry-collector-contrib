# SignalFx Receiver 

The SignalFx receiver accepts metrics in the [SignalFx proto
format](https://github.com/signalfx/com_signalfx_metrics_protobuf). This allows
the collector to receiver metrics from other collectors or the SignalFx Smart
Agent.

## Configuration

The following settings are required:

* `endpoint` (default = `0.0.0.0:9943`): Address and port that the SignalFx
  receiver should bind to.

The following settings are optional:

* `access_token_passthrough`: (default = `false`) Whether to preserve incoming
  access token (`X-Sf-Token` header value) as
  `"com.splunk.signalfx.access_token"` metric resource label.  Can be used in
  tandem with identical configuration option for [SignalFx
  exporter](../../exporter/signalfxexporter/README.md) to preserve datapoint
  origin.
* `tls` (no default): This is an optional object used to specify if TLS should be used for
  incoming connections.
    * `cert_file`: Specifies the certificate file to use for TLS connection.
      Note: Both `key_file` and `cert_file` are required for TLS connection.
    * `key_file`: Specifies the key file to use for TLS connection. Note: Both
      `key_file` and `cert_file` are required for TLS connection.

Example:

```yaml
receivers:
  signalfx:
  signalfx/advanced:
    access_token_passthrough: true
    tls:
      cert_file: /test.crt
      key_file: /test.key
```
