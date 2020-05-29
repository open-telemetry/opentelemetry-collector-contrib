# SAPM Receiver 

The SAPM receiver builds on the Jaeger proto. This allows the collector to receiver traces 
from other collectors or the SignalFx Smart Agent. SAPM proto and some useful related 
utilities can be found [here](https://github.com/signalfx/sapm-proto/).

## Configuration

Example:

```yaml
receivers:
  sapm:
    endpoint: localhost:7276
    tls:
      cert_file: /test.crt
      key_file: /test.key
```

* `endpoint`: Address and port that the SAPM receiver should bind to. Note that this must be 0.0.0.0:<port> instead of localhost if you want to receive spans from sources exporting to IPs other than localhost on the same host. For example, when the collector is deployed as a k8s deployment and exposed using a service.
* `tls`: This is an optional object used to specify if TLS should be used for incoming connections.
    * `cert_file`: Specifies the certificate file to use for TLS connection.  Note: Both `key_file` and `cert_file` are required for TLS connection. 
    * `key_file`: Specifies the key file to use for TLS connection. Note: Both `key_file` and `cert_file` are required for TLS connection. 
 