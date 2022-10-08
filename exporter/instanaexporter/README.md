# Instana Exporter

| Status                   |                  |
| ------------------------ |------------------|
| Stability                | [alpha]          |
| Supported pipeline types | traces           |
| Distributions            | [contrib]        |

The Instana Exporter converts OpenTelemetry trace data and then sends it to the [Instana Backend](https://www.ibm.com/docs/en/instana-observability/current?topic=setting-up-managing-instana).

## Exporter Configuration

The following exporter configuration parameters are supported.


| Parameter      | Description |
|----------------|-------------|
| endpoint | The Instana backend endpoint that the Exporter connects to. It depends on your region and it starts with ``https://serverless-``. It corresponds to the Instana environment variable ``INSTANA_ENDPOINT_URL`` |
| agent_key      | Your Instana Agent key. The same agent key can be used for host agents and serverless monitoring. It corresponds to the Instana environment variable ``INSTANA_AGENT_KEY`` |

> These parameters match the Instana Serverless Monitoring environment variables and can be found [here](https://www.ibm.com/docs/en/instana-observability/current?topic=references-environment-variables#serverless-monitoring).

### Sample Configuration

The code snippet below shows how your configuration file should look like:

```yaml
[...]

exporters:
  instana:
    endpoint: ${INSTANA_ENDPOINT_URL}
    agent_key: ${INSTANA_AGENT_KEY}

[...]

service:
  pipelines:
    traces:
      exporters: [instana]

[...]
```

### Full Example

```yaml
receivers:
  otlp:
    protocols:
      grpc:
      http:

processors:
  batch:
exporters:
  logging:
    loglevel: debug
  instana:
    loglevel: debug
    endpoint: ${INSTANA_ENDPOINT_URL}
    agent_key: ${INSTANA_AGENT_KEY}

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [instana]
```

[alpha]:https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
