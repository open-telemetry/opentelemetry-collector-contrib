# Coralogix Exporter

| Status                   |                        |
| ------------------------ |----------------------- |
| Stability                | traces, metrics [beta] |
|                          | logs [alpha]           |
| Supported pipeline types | traces, metrics, logs  |
| Distributions            | [contrib]              |

The Coralogix exporter sends traces, metrics and logs to [Coralogix](https://coralogix.com/).

> Please review the Collector's [security
> documentation](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/security.md),
> which contains recommendations on securing sensitive information such as the
> API key required by this exporter.

## Configuration

Example configuration:
```yaml
exporters:
  coralogix:
    # The Coralogix traces ingress endpoint
    traces:
      endpoint: "otel-traces.coralogix.com:443"
    metrics:
      endpoint: "otel-metrics.coralogix.com:443"
    logs:
      endpoint: "otel-logs.coralogix.com:443"

    # Your Coralogix private key is sensitive
    private_key: "xxx"

    # Traces, Metrics and Logs emitted by this OpenTelemetry exporter 
    # should be tagged in Coralogix with the following application and subsystem names
    application_name: "MyBusinessEnvironment"
    subsystem_name: "MyBusinessSystem"

    # (Optional) Timeout is the timeout for every attempt to send data to the backend.
    timeout: 30s
```
### Tracing deprecation 

The v0.60 version deprecates old Jaeger based tracing endpoint in favour of Opentelemetry based one.

To migrate, please remove the old endpoint field, and change the configuration to `traces.endpoint` using the new Tracing endpoint.

Old configuration:
```
exporters:
  coralogix:
    # The Coralogix traces ingress endpoint
    endpoint: "tracing-ingress.coralogix.com:9443"
```

New configuration:
```
exporters
  coralogix:
    # The Coralogix traces ingress endpoint
    traces:
      endpoint: "otel-traces.coralogix.com:443"
```

### Coralogix's Endpoints 

Depending on your region, you might need to use a different endpoint. Here are the available Endpoints:

| Region  | Traces Endpoint                          | Metrics Endpoint                     | Logs Endpoint                     |
|---------|------------------------------------------|------------------------------------- | --------------------------------- |
| USA1    | `otel-traces.coralogix.us:443`      | `otel-metrics.coralogix.us:443`      | `otel-logs.coralogix.us:443`      |
| APAC1   | `otel-traces.app.coralogix.in:443`  | `otel-metrics.coralogix.in:443`      | `otel-logs.coralogix.in:443`      | 
| APAC2   | `otel-traces.coralogixsg.com:443`   | `otel-metrics.coralogixsg.com:443`   | `otel-logs.coralogixsg.com:443`   |
| EUROPE1 | `otel-traces.coralogix.com:443`     | `otel-metrics.coralogix.com:443`     | `otel-logs.coralogix.com:443`     |
| EUROPE2 | `otel-traces.eu2.coralogix.com:443` | `otel-metrics.eu2.coralogix.com:443` | `otel-logs.eu2.coralogix.com:443` |

### Need help?
We love to assist our customers, simply [book your implementation session](https://calendly.com/info-coralogix/implementation),
and we will walk you through setting up this exporter, step by step.

[alpha]:https://github.com/open-telemetry/opentelemetry-collector#alpha
[beta]:https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
