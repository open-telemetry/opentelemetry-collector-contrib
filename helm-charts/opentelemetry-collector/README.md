# OpenTelemetry Collector Helm Chart

The helm chart installs [OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector) 
in kubernetes cluster.

## Prerequisites

- Helm 3.0+

## Installing the Chart

TBD

## Configuration

### Default configuration

By default this chart will deploy an OpenTelemetry Collector as daemonset with two pipelines (traces and metrics) 
and logging exporter enabled by default. Besides daemonset (agent), it can be also installed as standalone deployment. 
Both modes can be enabled together, in that case metrics and traces will be flowing from agents to standalone collectors.

*Example*: Install collector as a standalone deployment, and do not run it as an agent.
```yaml
agentCollector:
  enabled: false
standaloneCollector:
  enabled: true
```

By default collector has the following receivers enabled:
- **metrics**: OTLP and prometheus. Prometheus is configured only for scraping collector's own metrics.
- **traces**: zipkin and jaeger (thrift and grpc).
 
There are two ways to configure collector pipelines, which can be used together as well.

### Basic top level configuration with `telemetry` property

*Example*: Disable metrics pipeline and send traces to zipkin exporter:
```yaml
telemetry:
  metrics:
    enabled: false
  traces:
    exporter:
      type: zipkin
      config:
        endpoint: zipkin-all-in-one:14250
```

### Configuration with `agentCollector` and `standaloneCollector` properties
   
`agentCollector` and `standaloneCollector` properties allow to override collector configurations 
and default parameters applied on the k8s pods.

`agentCollector(standaloneCollector).configOverride` property allows to provide an extra 
configuration that will be merged into the default configuration.

*Example*: Enable host metrics receiver on the agents:
```yaml
agentCollector:
  configOverride:
    receivers:
      hostmetrics:
        scrapers:
          cpu:
          load:
          memory:
          disk:
    service:
      pipelines:
        metrics: 
          receivers: [prometheus, hostmetrics]
  extraEnvs:
  - name: HOST_PROC
    value: /hostfs/proc
  - name: HOST_SYS
    value: /hostfs/sys
  - name: HOST_ETC
    value: /hostfs/etc
  - name: HOST_VAR
    value: /hostfs/var
  - name: HOST_RUN
    value: /hostfs/run
  - name: HOST_DEV
    value: /hostfs/dev
  extraHostPathMounts:
  - name: hostfs
    hostPath: /
    mountPath: /hostfs
    readOnly: true
    mountPropagation: HostToContainer
```

### Other configuration options 

The [values.yaml](./values.yaml) file contains information about all other configuration 
options for this chart.