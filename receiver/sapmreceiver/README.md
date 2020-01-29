# SAPM Receiver 

SAPM exports builds on the Jaeger proto and adds additional batching on top. This allows
the collector to export traces from multiples nodes/services in a single batch. SAPM proto
and some useful related utilities can be found [here](https://github.com/signalfx/sapm-proto/).

## Configuration

Example:

```yaml
exporters:
  sapm:
    endpoint: localhost:7276
```

* `endpoint`: Address and port that the SAPM receiver should bind to. Note that this must be 0.0.0.0:<port> instead of localhost if you want to receive spans from sources exporting to IPs other than localhost on the same host. For example, when the collector is deployed as a k8s deployment and exposed using a service.
