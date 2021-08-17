# Trace ID aware load-balancing exporter

Supported pipeline types: traces, logs

This is an exporter that will consistently export spans and logs belonging to the same trace to the same backend.

It requires a source of backend information to be provided: static, with a fixed list of backends, or DNS, with a hostname that will resolve to all IP addresses to use. The DNS resolver will periodically check for updates.

Note that only the Trace ID is used for the decision on which backend to use: the actual backend load isn't taken into consideration. Even though this load-balancer won't do round-robin balancing of the batches, the load distribution should be very similar among backends with a standard deviation under 5% at the current configuration.

This load balancer is especially useful for backends configured with tail-based samplers, which make a decision based on the view of the full trace.

When a list of backends is updated, around 1/n of the space will be changed, so that the same trace ID might be directed to a different backend, where n is the number of backends. This should be stable enough for most cases, and the higher the number of backends, the less disruption it should cause. Still, if routing stability is important for your use case and your list of backends are constantly changing, consider using the `groupbytrace` processor. This way, traces are dispatched atomically to this exporter, and the same decision about the backend is made for the trace as a whole.

## Configuration

Refer to [config.yaml](./testdata/config.yaml) for detailed examples on using the processor.

* The `otlp` property configures the template used for building the OTLP exporter. Refer to the OTLP Exporter documentation for information on which options are available. Note that the `endpoint` property should not be set and will be overridden by this exporter with the backend endpoint.
* The `resolver` accepts either a `static` node, or a `dns`. If both are specified, `dns` takes precedence.
* The `hostname` property inside a `dns` node specifies the hostname to query in order to obtain the list of IP addresses.
* The `dns` node also accepts an optional property `port` to specify the port to be used for exporting the traces to the IP addresses resolved from `hostname`. If `port` is not specified, the default port 4317 is used.


Simple example
```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: localhost:4317

processors:

exporters:
  logging:
  loadbalancing:
    protocol:
      otlp:
        # all options from the OTLP exporter are supported
        # except the endpoint
        timeout: 1s
    resolver:
      static:
        hostnames:
        - backend-1:4317
        - backend-2:4317
        - backend-3:4317
        - backend-4:4317

service:
  pipelines:
    traces:
      receivers:
        - otlp
      processors: []
      exporters:
        - loadbalancing
    logs:
      receivers:
        - otlp
      processors: []
      exporters:
        - loadbalancing
```

For testing purposes, the following configuration can be used, where both the load balancer and all backends are running locally:
```yaml
receivers:
  otlp/loadbalancer:
    protocols:
      grpc:
        endpoint: localhost:4317
  otlp/backend-1:
    protocols:
      grpc:
        endpoint: localhost:55690
  otlp/backend-2:
    protocols:
      grpc:
        endpoint: localhost:55700
  otlp/backend-3:
    protocols:
      grpc:
        endpoint: localhost:55710
  otlp/backend-4:
    protocols:
      grpc:
        endpoint: localhost:55720

processors:

exporters:
  logging:
  loadbalancing:
    protocol:
      otlp:
        timeout: 1s
        insecure: true
    resolver:
      static:
        hostnames:
        - localhost:55690
        - localhost:55700
        - localhost:55710
        - localhost:55720

service:
  pipelines:
    traces/loadbalancer:
      receivers:
        - otlp/loadbalancer
      processors: []
      exporters:
        - loadbalancing

    traces/backend-1:
      receivers:
        - otlp/backend-1
      processors: []
      exporters:
        - logging

    traces/backend-2:
      receivers:
        - otlp/backend-2
      processors: []
      exporters:
        - logging

    traces/backend-3:
      receivers:
        - otlp/backend-3
      processors: []
      exporters:
        - logging

    traces/backend-4:
      receivers:
        - otlp/backend-4
      processors: []
      exporters:
        - logging

    logs/loadbalancer:
      receivers:
        - otlp/loadbalancer
      processors: []
      exporters:
        - loadbalancing
    logs/backend-1:
      receivers:
        - otlp/backend-1
      processors: []
      exporters:
        - logging
    logs/backend-2:
      receivers:
        - otlp/backend-2
      processors: []
      exporters:
        - logging
    logs/backend-3:
      receivers:
        - otlp/backend-3
      processors: []
      exporters:
        - logging
    logs/backend-4:
      receivers:
        - otlp/backend-4
      processors: []
      exporters:
        - logging
```

## Metrics

The following metrics are recorded by this processor:

* `otelcol_loadbalancer_num_resolutions` represents the total number of resolutions performed by the resolver specified in the tag `resolver`, split by their outcome (`success=true|false`). For the static resolver, this should always be `1` with the tag `success=true`.
* `otelcol_loadbalancer_num_backends` informs how many backends are currently in use. It should always match the number of items specified in the configuration file in case the `static` resolver is used, and should eventually (seconds) catch up with the DNS changes. Note that DNS caches that might exist between the load balancer and the record authority will influence how long it takes for the load balancer to see the change.
* `otelcol_loadbalancer_num_backend_updates` records how many of the resolutions resulted in a new list of backends. Use this information to understand how frequent your backend updates are and how often the ring is rebalanced. If the DNS hostname is always returning the same list of IP addresses but this metric keeps increasing, it might indicate a bug in the load balancer.
* `otelcol_loadbalancer_backend_latency` measures the latency for each backend.
* `otelcol_loadbalancer_backend_outcome` counts what the outcomes were for each endpoint, `success=true|false`.
