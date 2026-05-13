# Schema Processor — Operators Guide

## Who is this for?

This guide is for operators running OpenTelemetry Collectors who need to manage semantic convention versions across their telemetry pipeline. If your services use different versions of the OpenTelemetry semantic conventions — or you need to update to a newer version — the schema processor helps you do that without breaking your existing queries, dashboards, and alerts.

## The problem

OpenTelemetry semantic conventions evolve over time. Attribute names change between versions — for example, `http.method` was renamed to `http.request.method`. In a real environment, different services across your fleet will be using different versions of instrumentation libraries, SDKs, and auto-instrumentation agents. Each version of an instrumentation library may adopt a different version of the semantic conventions, meaning an update to the library can change the attribute names it emits.

This creates two problems:

1. **Inconsistent attribute names** — your backend sees both `http.method` and `http.request.method` for the same thing, depending on which service sent it. Queries that filter on one name miss data from services using the other.

2. **Painful version transitions** — when you want to standardize on a newer convention version, anything referencing old attribute names silently stops matching. This affects your entire observability stack:
   - **Collector pipeline** — processors downstream of the schema processor that filter, route, or transform based on attribute names (e.g., `filterprocessor`, `transformprocessor`, `routingconnector`)
   - **Backend queries and dashboards** — any saved queries or dashboard panels referencing old names
   - **Alerts** — monitoring rules that depend on specific attribute names
   
   There's no error — just missing data. This discourages operators from updating their target version, even as their services naturally adopt newer instrumentation.

The schema processor solves both problems: it **normalizes all telemetry to a single version** regardless of what each service emits, and the migration feature lets you **transition between versions safely** with both old and new names preserved during the changeover.

## How it works

OpenTelemetry publishes [schema translation files](https://opentelemetry.io/docs/specs/otel/schemas/) that describe what changed between each version of the semantic conventions — for example, which attributes were renamed and what their old and new names are. The processor uses these files to translate incoming telemetry from whatever schema version it was emitted with to a target version you specify.

When telemetry arrives, the processor checks its schema URL, fetches the relevant translation file (caching it for future use), and applies the renames needed to bring it to the target version.

```
Service A (v1.21)  ─┐
Service B (v1.24)  ─┤──▶ [Schema Processor (target: v1.26)] ──▶ Backend
Service C (v1.26)  ─┘        all output is v1.26
```

## Getting started

### Basic setup

Add the schema processor to your collector config with a target schema version:

```yaml
processors:
  schema:
    targets:
      - https://opentelemetry.io/schemas/1.26.0

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [schema]
      exporters: [otlp]
    metrics:
      receivers: [otlp]
      processors: [schema]
      exporters: [otlp]
    logs:
      receivers: [otlp]
      processors: [schema]
      exporters: [otlp]
```

The processor should be the **first processor** in each pipeline. This ensures all downstream processors and exporters see consistent attribute names.

### Pre-fetching schemas

To avoid latency on the first request, pre-fetch schema files at startup:

```yaml
processors:
  schema:
    prefetch:
      - https://opentelemetry.io/schemas/1.26.0
    targets:
      - https://opentelemetry.io/schemas/1.26.0
```

### What gets translated

The processor applies renames defined in the [schema file format](https://opentelemetry.io/docs/specs/otel/schemas/file_format_v1.1.0/):

- **Resource attributes** — renamed on all signal types
- **Span attributes** — optionally scoped to specific span names
- **Span event attributes** — optionally scoped to specific span and event names
- **Log record attributes** — renamed on all log records
- **Metric data point attributes** — optionally scoped to specific metric names
- **Metric names** — renamed
- **Span event names** — renamed

## Migrating between schema versions

When you're ready to update your target schema version, the migration feature helps you do it safely.

### Why you need migration mode

Without migration mode, changing the target version is a hard cutover:

1. You change `targets` from `1.24.0` to `1.26.0`
2. The processor immediately starts renaming attributes to the 1.26 names
3. Downstream processors in your pipeline that reference old attribute names stop matching
4. Queries, dashboards, and alerts in your backend stop working
5. You scramble to update everything before anyone notices

With migration mode:

1. You change `targets` to `1.26.0` and add a `migration` entry
2. The processor writes **both** old and new attribute names
3. Downstream pipeline processors continue to work (old names still present)
4. You update your pipeline config, queries, dashboards, and alerts at your own pace
5. You remove the `migration` entry when you're done

### Step-by-step migration

**Step 1: Enable migration mode**

Update your config to set the new target and add a migration entry:

```yaml
processors:
  schema:
    targets:
      - https://opentelemetry.io/schemas/1.26.0
    migration:
      - target: https://opentelemetry.io/schemas/1.26.0
        from: https://opentelemetry.io/schemas/1.24.0
```

This tells the processor: "translate everything to 1.26, but for renames between 1.24 and 1.26, keep both the old and new attribute names."

**Step 2: Deploy and verify**

After deploying, your telemetry will have both old and new attribute names for any renames that happened between 1.24 and 1.26. Your existing queries continue to work because the old names are still present.

You can verify migration is active by checking the `processor_schema.translated` metric — it will include a `migration_from_schema_url` attribute.

**Step 3: Update your queries**

Update your queries, dashboards, and alerts to use the new attribute names. Take your time — both names are present, so nothing breaks during this step.

**Step 4: Disable migration mode**

Once everything is updated, remove the `migration` section:

```yaml
processors:
  schema:
    targets:
      - https://opentelemetry.io/schemas/1.26.0
```

The processor now performs hard renames only. The old attribute names are no longer emitted.

### Downgrading

The processor also handles downgrades — translating newer schema versions back to an older target. This is useful when a service updates its instrumentation library and starts emitting a newer semconv version before you're ready to adopt it, or when you need to roll back.

Without migration mode, the processor silently renames the newer attributes back to the older names. This is usually fine if your pipeline and backend already use the older names.

Migration mode works the same way for downgrades — in your migration entry, set `from` to the version you're moving away from. Both old and new attribute names are preserved during the transition.

## Monitoring

The schema processor emits internal metrics to help you understand what it's doing:

| Metric | Description |
|--------|-------------|
| `processor_schema.translated` | Number of schema translations. Includes `from_schema_url`, `to_schema_url`, and (when migration is active) `migration_from_schema_url` attributes. |
| `processor_schema_cache.hits` | Schema file cache hits |
| `processor_schema_cache.misses` | Schema file cache misses (triggers a fetch) |
| `processor_schema_resource.failed` | Resource translations that failed |
| `processor_schema_{signal}.failed` | Scope translations that failed (per signal type) |
| `processor_schema_{signal}.skipped` | Scopes skipped because no schema URL was present |

### Monitoring migration progress

When migration is active, the `processor_schema.translated` metric includes a `migration_from_schema_url` attribute. You can use this to:

- **Detect migration is active** — filter for records where `migration_from_schema_url` is present
- **Track what versions are arriving** — group by `from_schema_url` to see the distribution of incoming schema versions
- **Alert if migration is left on too long** — set an alert on the presence of `migration_from_schema_url`

## Recommendations

### Place the processor first in your pipeline

The schema processor should be the first processor in each pipeline. This ensures all downstream processors see consistent attribute names, regardless of what version the source instrumentation used.

### Use `prefetch` to avoid cold-start latency

The first time the processor encounters a schema URL, it needs to fetch the schema file over HTTP. Use the `prefetch` option to download schemas at startup.

### Keep migration windows short

While migration mode is active, every renamed attribute is emitted twice. This increases attribute cardinality, which may affect storage costs and query performance depending on your backend. Use migration mode as a temporary bridge, not a permanent configuration.

### One version hop at a time

For large version jumps (e.g., 1.10 to 1.26), consider migrating one or two versions at a time rather than jumping all at once. This keeps the number of duplicated attributes manageable and makes it easier to track which queries need updating.
