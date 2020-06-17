# SignalFx Receiver 

The SignalFx receiver accepts metrics in the [SignalFx proto format](https://github.com/signalfx/com_signalfx_metrics_protobuf).
This allows the collector to receiver metrics from other collectors or the 
SignalFx Smart Agent.

## Configuration

Example:

```yaml
receivers:
  signalfx:
    endpoint: localhost:7276
    access_token_passthrough: true
    tls:
      cert_file: /test.crt
      key_file: /test.key
```

* `endpoint`: Address and port that the SignalFx receiver should bind to. Note that this must be 0.0.0.0:<port> instead of localhost if you want to receive spans from sources exporting to IPs other than localhost on the same host. For example, when the collector is deployed as a k8s deployment and exposed using a service.
* `access_token_passthrough`: (default = `false`) Whether to preserve incoming access token (`X-Sf-Token` header value) as `"com.splunk.signalfx.access_token"` metric resource label.  Can be used in tandem with identical configuration option for [SignalFx exporter](../../exporter/signalfxexporter/README.md) to preserve datapoint origin.
* `tls`: This is an optional object used to specify if TLS should be used for incoming connections.
    * `cert_file`: Specifies the certificate file to use for TLS connection.  Note: Both `key_file` and `cert_file` are required for TLS connection.
    * `key_file`: Specifies the key file to use for TLS connection. Note: Both `key_file` and `cert_file` are required for TLS connection.
