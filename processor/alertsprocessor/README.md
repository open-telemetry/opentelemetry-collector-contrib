# alertsprocessor (OpenTelemetry Collector processor)

Evaluates alert rules over a short sliding window for **metrics, logs, and traces**, emits synthetic metrics (`otel_alert_*`), and sends notifications to a webhook (e.g., Alertmanager-compatible).

## Example `rules.yaml` (logs & traces)

```yaml
- id: high_error_logs
  name: HighErrorLogs
  signal: logs
  for: 0s
  labels: { severity: error }
  logs:
    severity_at_least: ERROR
    body_contains: "timeout"
    group_by: ["service.name"]
    count_threshold: 5

- id: slow_spans
  name: SlowSpans
  signal: traces
  for: 5s
  labels: { severity: warning }
  traces:
    latency_ms_gt: 500
    status_not_ok: false
    group_by: ["service.name","span.name"]
    count_threshold: 3
```

---

## How to use `alertsprocessor` in an OpenTelemetry Collector

> **Signals supported:** Logs and Traces (rule evaluation) + synthetic metrics output.

> **⚠️ Sliding window cost**
> **Heads up:** Increasing `sliding_window.duration` raises CPU and memory usage.
> Increasing `sliding_window.duration` increases CPU and memory usage. Keep it as small as possible (start at **5s**).

### 1) Write alert rules (YAML)

Create one or more rule files and point the processor at them via `rule_files.include` globs (see example above).

### 2) Recommended topology: **routingconnector** + per-group `alertsprocessor`

Use the **Routing Connector** to route telemetry by resource attributes into **separate pipelines**, each with its own `alertsprocessor` instance and its own rule files. This keeps groups isolated and makes scaling clean.

```yaml
receivers:
  otlp:
    protocols: { http: {}, grpc: {} }

connectors:
  routing:
    default_pipelines:
      logs:   [logs/default]
      traces: [traces/default]
      metrics: [metrics/default]
    table:
      - statement: route() where resource.attributes["k8s.namespace.name"] == "payments"
        pipelines:
          logs:   [logs/payments]
          traces: [traces/payments]
          metrics: [metrics/payments]
      - statement: route() where resource.attributes["service.name"] == "checkout"
        pipelines:
          logs:   [logs/checkout]
          traces: [traces/checkout]

processors:
  alertsprocessor/payments:
    sliding_window:
      duration: 5s    #Larger window ⇒ higher CPU & RAM
      max_samples: 100000
      overflow_behavior: ring_buffer
    evaluation:
      interval: 15s
      timeout: 10s
    statestore:
      instance_id: payments-engine
      external_labels: { group: payments, source: collector }
    rule_files:
      include: ["./rules/payments.yaml"]
    notifier:
      url: http://alertmanager:9093/api/v2/alerts

  alertsprocessor/checkout:
    sliding_window:
      duration: 5s
      max_samples: 100000
      overflow_behavior: ring_buffer
    evaluation:
      interval: 15s
      timeout: 10s
    statestore:
      instance_id: checkout-engine
      external_labels: { group: checkout, source: collector }
    rule_files:
      include: ["./rules/checkout.yaml"]
    notifier:
      url: http://alertmanager:9093/api/v2/alerts

exporters:
  prometheusremotewrite:
    endpoint: http://prometheus:9090/api/v1/write
  debug: {}

service:
  pipelines:
    logs/in:    { receivers: [otlp], exporters: [routing] }
    traces/in:  { receivers: [otlp], exporters: [routing] }
    metrics/in: { receivers: [otlp], exporters: [routing] }

    logs/payments:   { receivers: [routing], processors: [alertsprocessor/payments], exporters: [debug, prometheusremotewrite] }
    traces/payments: { receivers: [routing], processors: [alertsprocessor/payments], exporters: [debug, prometheusremotewrite] }
    logs/checkout:   { receivers: [routing], processors: [alertsprocessor/checkout], exporters: [debug, prometheusremotewrite] }
    traces/checkout: { receivers: [routing], processors: [alertsprocessor/checkout], exporters: [debug, prometheusremotewrite] }

    logs/default:    { receivers: [routing], exporters: [debug] }
    traces/default:  { receivers: [routing], exporters: [debug] }
    metrics/default: { receivers: [routing], exporters: [debug] }
```

### 3) Output series (Prometheus Remote Write)

* `otel_alert_state{rule_id, signal, ...} = 0|1` (gauge)
* `otel_alert_transitions_total{rule_id, from, to, signal, ...}` (counter)
* `alertsprocessor_evaluation_duration_seconds` (self-telemetry)

### Operational notes

* **Sliding window cost:** Larger `sliding_window.duration` ⇒ higher CPU & RAM.
* **Log body type:** If a log’s `Body` isn’t a string, we log a **WARN** and stringify for `body_contains`.
* **Rules scope:** Current rules cover **logs** & **traces** (metrics rules can be added similarly).
* **Statestore:** Use `statestore.instance_id` & `statestore.external_labels` to distinguish engines.

### Recommended: Routing Connector

Use the Routing Connector to keep groups segregated and scalable:
[https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/routingconnector](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/routingconnector)

