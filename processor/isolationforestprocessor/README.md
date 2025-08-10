# Isolation Forest Processor

> **Status:** In Development – interfaces and behaviour may still evolve.

The **Isolation Forest processor** adds inline, unsupervised anomaly detection to any OpenTelemetry Collector pipeline (traces, metrics, or logs). It embeds a lightweight implementation of the Isolation Forest algorithm that automatically learns normal behaviour from recent telemetry and tags, scores, or optionally drops anomalies *in‑flight* – no external ML service required.

---

## ✨ Key Features

| Capability                    | Description                                                                                                                                         |
| ----------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Realtime Isolation Forest** | Builds an ensemble of random trees over a sliding window of recent data and assigns a 0–1 anomaly score on ingestion (≈ *O(log n)* per point).      |
| **Multi‑signal support**      | Can be inserted into **traces**, **metrics**, **logs** pipelines – one config powers all three.                                                     |
| **Per‑entity modelling**      | `features` config lets you maintain a separate model per unique combination of resource / attribute keys (e.g. per‑pod, per‑service).               |
| **Flexible output**           | • Add an attribute `iforest.is_anomaly=true` <br>• Emit a gauge metric `iforest.anomaly_score` <br>• Drop anomalous telemetry entirely.             |
| **Config‑driven**             | Tune tree count, subsample size, contamination rate, sliding‑window length, retraining interval, target metrics, and more – all in `collector.yml`. |
| **Zero external deps**        | Pure Go implementation; runs wherever the Collector does (edge, gateway, or backend).                                                               |

---

## ⚙️ How it Works

1. **Training window** – The processor keeps up to `window_size` of the most recent data points for every feature‑group.
2. **Periodic (re‑)training** – Every `training_interval`, it draws `subsample_size` points from that window and grows `forest_size` random isolation trees.
3. **Scoring** – Each new point is pushed through the forest. Shorter average path length ⇒ higher anomaly score.
4. **Post‑processing** –

   * If `add_anomaly_score: true`, a gauge metric `iforest.anomaly_score` is emitted with identical attributes/timestamp.
   * If the score ≥ `anomaly_threshold`, the original span/metric/log is flagged with `iforest.is_anomaly=true`.
   * If `drop_anomalous_data: true`, flagged items are removed from the batch instead of being forwarded.

> **Contamination rate** – instead of hard‑coding `anomaly_threshold`, you can supply `contamination_rate` (expected % of outliers). The processor then auto‑derives a dynamic threshold equal to the `(1 – contamination_rate)` quantile of recent scores.

Performance is linear in `forest_size` and logarithmic in `window_size`; a default of 100 trees and a 1 k‑point window easily sustains 10–50 k points/s on a vCPU.

---

## 🔧 Configuration

| Field                 | Type        | Default   | Notes                                                                          |
| --------------------- | ----------- | --------- | ------------------------------------------------------------------------------ |
| `forest_size`         | int         | `100`     | Number of trees in the ensemble. Higher → smoother scores, more CPU.           |
| `subsample_size`      | int         | `256`     | Rows sampled to build **each** tree. Must be ≤ `window_size`.                  |
| `window_size`         | int         | `1000`    | Sliding window of recent data maintained per feature‑group.                    |
| `contamination_rate`  | float (0–1) | `0.10`    | Fraction of points expected to be outliers; used to auto‑tune threshold.       |
| `anomaly_threshold`   | float (0–1) | *derived* | Manual override – score ≥ this ⇒ anomaly. Ignored if `contamination_rate` set. |
| `training_interval`   | duration    | `5m`      | Model is retrained no sooner than this interval.                               |
| `features`            | \[]string   | `[]`      | Resource/attribute keys that define **grouping**. Blank ⇒ single global model. |
| `metrics_to_analyze`  | \[]string   | `[]`      | Only these metric names are scored (metrics pipeline only). Blank ⇒ all.       |
| `add_anomaly_score`   | bool        | `false`   | Emit `iforest.anomaly_score` metric.                                           |
| `drop_anomalous_data` | bool        | `false`   | Remove anomalous items from the batch instead of forwarding.                   |

See the sample below for context.

---

## 📄 Sample `config.yml`

```yaml
receivers:
  otlp:
    protocols:
      grpc:            # → listen on 0.0.0.0:4317

processors:
  isolationforest:
    # ─── core algorithm parameters ────────────────────────────────
    forest_size:        150          # trees per forest
    subsample_size:     512          # rows per tree
    contamination_rate: 0.05         # 5 % expected outliers
    threshold:          0.0          # 0 ⇒ let contamination_rate drive the cut-off
    mode:               both         # enrich + filter (see docstring)
    training_window:    24h          # window of data kept for training
    update_frequency:   5m           # retrain every 5 minutes
    min_samples:        1000         # wait until this many points seen

    # ─── where to write results on each data point ───────────────
    score_attribute:          anomaly.isolation_score   # float 0–1
    classification_attribute: anomaly.is_anomaly        # bool

    # ─── which numeric features the model should look at ─────────
    features:
      traces:  [duration]           # span duration (µs / ns)
      metrics: [value]              # the sample’s numeric value
      logs:    [severity_number]    # log severity enum

    # ─── performance guard-rails (optional) ──────────────────────
    performance:
      max_memory_mb:     512
      batch_size:        1000
      parallel_workers:  4

exporters:
  prometheus:
    endpoint: "0.0.0.0:9464"   # Prom-server will scrape /metrics here
    send_timestamps: true      # (field is valid in the standard exporter)

service:
  pipelines:
    metrics:
      receivers:  [otlp]
      processors: [isolationforest]
      exporters:  [prometheus]

```

### What the example does

| Signal      | What’s scored                                              | Feature grouping               | Output                                    | Notes                                                                                            |
| ----------- | ---------------------------------------------------------- | ------------------------------ | ----------------------------------------- | ------------------------------------------------------------------------------------------------ |
| **Traces**  | Span **duration** (ns)                                     | `service.name`, `k8s.pod.name` | `iforest.is_anomaly` attr + optional drop | Use a span/trace exporter to route anomalies.                                                    |
| **Metrics** | Only `system.cpu.utilization`, `system.memory.utilization` | Same                           | Attribute + score metric                  | The score appears as `iforest.anomaly_score` gauge.                                              |
| **Logs**    | Size of the log payload (bytes) by default                 | Same                           | Attribute flag                            | You can expose a numeric log attribute and configure the processor to use that via code changes. |

---

## 🚀 Best Practices

* **Tune `forest_size` vs. latency** – start with 100 trees; raise to 200–300 if scores look noisy.
* **Use per‑entity models** – add `features` (service, pod, host) to avoid global comparisons across very different series.
* **Let contamination drive threshold** – set `contamination_rate` to the % of traffic you’re comfortable labelling outlier; avoid hand‑tuning `anomaly_threshold`.
* **Route anomalies** – keep `drop_anomalous_data=false` and add a simple \[routing‑processor] downstream to ship anomalies to a dedicated exporter or topic.
* **Monitor model health** – the emitted `iforest.anomaly_score` metric is perfect for a Grafana panel; watch its distribution and adapt window / contamination accordingly.

---

## 🏗️ Internals (High‑Level)

```text
               ┌───────────────────────────────────────────────────┐
               │ IsolationForestProcessor (per Collector instance)│
               │ ───────────────────────────────────────────────── │
               │  • Sliding window (per feature‑group)           │
               │  • Forest of N trees (per feature‑group)        │
Telemetry ───▶  │  • Score calculator & anomaly decision         │ ───▶  Next processor/exporter
               └───────────────────────────────────────────────────┘
```

*Training cost*: **O(window\_size × forest\_size × log subsample\_size)** every `training_interval`
*Scoring cost*: **O(forest\_size × log subsample\_size)** per item

---

## 📦 Building & Enabling

1. Add `isolationforestprocessor` to your Collector’s go‑builder manifest or vendor the contrib repo at the `feature/isolation-forest-processor` branch.
2. Rebuild the Collector (`make otelcontribcol`).
3. Enable the processor in your YAML as shown above and restart the service.

> **Note** – until merged upstream, you need a custom build of `otelcontribcol` that includes this processor.

---

## 🤝 Contributing

* **Bugs / Questions** – please open an issue in the fork first.
* **Planned enhancements**

  * Multivariate scoring (multiple numeric attributes per point).
  * Adaptive window size.
  * Expose Prometheus counters for training time / CPU cost.

PRs welcome – please include unit tests and doc updates.

---

