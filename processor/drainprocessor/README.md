# Drain Processor

| Status        |           |
| ------------- |-----------|
| Stability     | [development]: logs |
| Distributions | [contrib] |
| Issues        | [![Open issues](https://img.shields.io/github/issues-search/open-telemetry/opentelemetry-collector-contrib?query=is%3Aissue%20is%3Aopen%20label%3Aprocessor%2Fdrain%20&label=open&color=orange&logo=opentelemetry)](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aopen+is%3Aissue+label%3Aprocessor%2Fdrain) [![Closed issues](https://img.shields.io/github/issues-search/open-telemetry/opentelemetry-collector-contrib?query=is%3Aissue%20is%3Aclosed%20label%3Aprocessor%2Fdrain%20&label=closed&color=blue&logo=opentelemetry)](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aclosed+is%3Aissue+label%3Aprocessor%2Fdrain) |

[development]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#development
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib

The drain processor applies the [Drain log clustering algorithm](https://jiemingzhu.github.io/pub/pjhe_icws2017.pdf) to log records as they pass through the pipeline. For each record it derives a template string (e.g. `"user <*> logged in from <*>"`) and attaches it as an attribute on the record.

This processor **annotates**; it does not filter. Use the [filter processor](../filterprocessor/README.md) downstream to act on the `log.record.template` attribute — for example, to drop entire classes of noisy logs by pattern.

## How it works

Drain builds a parse tree from the token structure of log lines. Lines with similar structure are grouped into a **cluster**, and a **template** is derived by replacing variable tokens with `<*>` wildcards. As more logs arrive the templates become more accurate and stable.

Templates are derived from the token structure of each log line and become more stable as more logs are observed. Use the template **string** for filtering rules; it converges to the same value across instances given the same configuration and log patterns (see [Deployment considerations](#deployment-considerations)).

## Configuration

```yaml
processors:
  drain:
    # Drain parse tree parameters
    log_cluster_depth: 4       # default: 4 (minimum: 3)
    sim_threshold: 0.4         # default: 0.4, range [0.0, 1.0]
    max_children: 100          # default: 100
    max_clusters: 0            # default: 0 (unlimited, LRU eviction when > 0)
    extra_delimiters: []       # default: [] (extra token delimiters beyond whitespace)

    # Body extraction
    body_field: ""             # default: "" (use full body string)

    # Output attribute name
    template_attribute: "log.record.template"    # default

    # Seeding (optional)
    seed_templates: []
    seed_logs: []

    # Warmup mode
    warmup_mode: passthrough   # default: "passthrough" | "buffer"
    warmup_min_clusters: 10    # default: 10 (only used when warmup_mode: buffer)
    warmup_buffer_max_logs: 10000  # default: 10000 (only used when warmup_mode: buffer)
```

### Parameters

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `log_cluster_depth` | int | `4` | Max depth of the Drain parse tree. Higher values produce more specific templates. Minimum: 3. |
| `sim_threshold` | float | `0.4` | Similarity threshold in [0.0, 1.0]. Lines below this threshold create a new cluster rather than merging with an existing one. |
| `max_children` | int | `100` | Maximum children per parse tree node. |
| `max_clusters` | int | `0` | Maximum clusters tracked. When exceeded, the least-recently-used cluster is evicted. `0` means unlimited. |
| `extra_delimiters` | []string | `[]` | Additional token delimiters beyond whitespace (e.g. `[",", ":"]`). |
| `body_field` | string | `""` | If set, and the log body is a structured map, the value of this top-level key is used as the text to template instead of the full body. |
| `template_attribute` | string | `"log.record.template"` | Attribute key written with the derived template string. |
| `seed_templates` | []string | `[]` | Template strings to pre-load at startup (see [Seeding](#seeding)). |
| `seed_logs` | []string | `[]` | Raw example log lines to train on at startup (see [Seeding](#seeding)). |
| `warmup_mode` | string | `"passthrough"` | Controls behavior during the warmup period. `"passthrough"` (default) or `"buffer"` (see [Warmup mode](#warmup-mode)). |
| `warmup_min_clusters` | int | `10` | Minimum distinct clusters before warmup ends. Only used when `warmup_mode: buffer`. |
| `warmup_buffer_max_logs` | int | `10000` | Maximum records to buffer before flushing regardless of cluster count. Only used when `warmup_mode: buffer`. Must be > 0. |

## Seeding

Seeding pre-populates the Drain tree before any live logs arrive. This is the primary mechanism for stable templates across restarts.

### `seed_templates`

Provide known template strings directly. The processor trains on each entry at startup, establishing clusters for those patterns immediately.

```yaml
processors:
  drain:
    seed_templates:
      - "user <*> logged in from <*>"
      - "connected to <*>"
      - "heartbeat ping <*>"
```

### `seed_logs`

Provide raw example log lines. The processor trains on them at startup, letting Drain derive the templates itself. Useful when exact template strings are not known in advance.

```yaml
processors:
  drain:
    seed_logs:
      - "user alice logged in from 10.0.0.1"
      - "user bob logged in from 192.168.1.1"
      - "connected to 10.0.0.1"
```

Empty and whitespace-only entries in both lists are silently skipped.

## Deployment considerations

### Multiple collector instances

Each collector instance builds its Drain parse tree independently in memory. Two instances processing the same log patterns will converge on identical templates because the Drain algorithm is deterministic: given the same configuration and a representative sample of log forms, the same token structure produces the same template string.

The main caveat is the **early training phase**. Before an instance has seen enough lines to abstract a wildcard (e.g. before `"user alice logged in"` and `"user bob logged in"` have both been observed), different instances may temporarily produce different templates for the same logical pattern. This is most noticeable at startup with low-volume or highly variable log streams.

**Mitigations:**
- Use `seed_templates` or `seed_logs` to pre-load known patterns at startup. With a comprehensive seed set, instances start in an already-converged state and live training only fills in the gaps.
- Use `buffer` warmup mode if downstream consumers require stable templates from the first record they receive.

### Warmup modes

The `warmup_mode` setting controls what happens before the parse tree has stabilized — i.e. before it has observed enough distinct log forms to produce reliable, abstracted templates.

| Mode | Behavior | Trade-off |
|------|----------|-----------|
| `passthrough` (default) | Annotates every record immediately. Early records may receive less-abstracted templates (e.g. a raw line rather than a wildcarded form) that change as more data arrives. | No latency or memory overhead. Downstream consumers must tolerate template churn at startup. |
| `buffer` | Holds records in memory until `warmup_min_clusters` distinct templates have been observed, or `warmup_buffer_max_logs` is reached. Flushes all buffered records at once, fully annotated. | Templates are stable from the first record downstream sees. Adds startup latency and memory pressure proportional to buffer size. |

Choose `passthrough` when:
- Downstream consumers are tolerant of occasional template changes (e.g. they use templates for volume aggregation where a brief inconsistency is acceptable).
- You are using `seed_templates` or `seed_logs` to pre-stabilize the tree.

Choose `buffer` when:
- A downstream `filter` processor must reliably match templates from the very first record — emitting an unstabilised template could cause records to pass through a filter they should have been dropped by.
- You have strict ordering or completeness requirements and cannot tolerate records being annotated with different templates for the same log pattern.

```yaml
processors:
  drain:
    warmup_mode: buffer
    warmup_min_clusters: 20
    warmup_buffer_max_logs: 5000
```

> **Memory note**: in buffer mode, all records are held in memory until flush. Size the buffer with `warmup_buffer_max_logs` according to your available memory and expected log volume during startup.

## Metrics

The processor emits the following internal telemetry metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `otelcol_processor_drain_clusters_active` | gauge | Current number of active clusters in the Drain parse tree. Useful for tracking tree growth and stability over time. |
| `otelcol_processor_drain_log_records_annotated` | counter | Number of log records successfully annotated with a template. |
| `otelcol_processor_drain_log_records_unannotated` | counter | Number of log records not annotated — empty body, Train error, or no cluster returned by Drain. |

## Output attributes

The processor sets the following attribute on each log record:

| Attribute | Type | Example | Description |
|-----------|------|---------|-------------|
| `log.record.template` | string | `"user <*> logged in from <*>"` | The Drain-derived template string. Stable within an instance once the tree has warmed up. Use this for filtering rules. |

The attribute name is configurable via `template_attribute`.

> **Semantic conventions**: `log.record.template` aligns with the proposed OTel attribute in [open-telemetry/semantic-conventions#1283](https://github.com/open-telemetry/semantic-conventions/issues/1283) and [#2064](https://github.com/open-telemetry/semantic-conventions/issues/2064). These names may be updated if a convention is formally adopted.

## Example pipeline

The following pipeline annotates logs with Drain templates and then drops known noisy patterns using the filter processor:

```yaml
processors:
  drain:
    log_cluster_depth: 4
    sim_threshold: 0.4
    max_clusters: 500
    seed_templates:
      - "user <*> logged in from <*>"
      - "connected to <*>"
      - "heartbeat ping <*>"
    warmup_mode: buffer
    warmup_min_clusters: 20
    warmup_buffer_max_logs: 5000

  filter/drop_noisy:
    error_mode: ignore
    logs:
      log_record:
        - attributes["log.record.template"] == "heartbeat ping <*>"
        - attributes["log.record.template"] == "connected to <*>"

service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [drain, filter/drop_noisy]
      exporters: [otlp]
```

## `body_field`

`body_field` is a convenience for pipelines where the log body is a structured map and you do not have full control over how upstream processors shape it.

If you **do** control the pipeline, the preferred approach is a `move` operator in the filelog receiver (or equivalent) to promote the message field back to a plain string body before the drain processor sees the record:

```yaml
operators:
  - type: json_parser
  - type: move
    from: body.message
    to: body
```

If you **cannot** do that — for example, logs arrive via OTLP already structured — set `body_field` to the map key whose value should be fed to Drain:

```yaml
processors:
  drain:
    body_field: "message"
```

Given a log body `{"level": "info", "message": "user alice logged in from 10.0.0.1"}`, only the `message` value is fed to Drain. The full body is used unchanged if the field is absent or the body is not a map.

> **Note**: `body_field` only supports a single top-level key. Full OTTL path expressions (e.g. `body["event"]["message"]`) are not supported and are noted as a future extension.

## Future extensions

- **Snapshot persistence**: save and restore the Drain tree state across restarts, eliminating the need for seeding. This requires serialization support and is tracked as a future improvement.
- **OTTL body extraction**: support full OTTL path expressions for `body_field` instead of a single top-level key name.
- **Multi-instance synchronization**: optional shared snapshot file or gossip-based tree merging for consistent templates across horizontally scaled deployments.
