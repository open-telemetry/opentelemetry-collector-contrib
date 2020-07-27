# SAPM Receiver 

The SAPM receiver builds on the Jaeger proto. This allows the collector to
receiver traces from other collectors or the SignalFx Smart Agent. SAPM proto
and some useful related utilities can be found
[here](https://github.com/signalfx/sapm-proto/).

## Configuration

The following settings are required:

* `endpoint` (default = `0.0.0.0:7276`): Address and port that the SAPM
  receiver should bind to.

The following setting are optional:

* `tls` (no default): This is an optional object used to specify if TLS should
  be used for incoming connections.
    * `cert_file`: Specifies the certificate file to use for TLS connection.
      Note: Both `key_file` and `cert_file` are required for TLS connection. 
    * `key_file`: Specifies the key file to use for TLS connection. Note: Both
      `key_file` and `cert_file` are required for TLS connection. 

Example:

```yaml
receivers:
  sapm:
    endpoint: localhost:7276
    access_token_passthrough: true
    tls:
      cert_file: /test.crt
      key_file: /test.key
```