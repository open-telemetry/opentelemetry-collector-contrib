Sentry Exporter Spec

## Summary

The Sentry exporter forwards OTLP traces and logs to Sentry's native OTLP ingestion endpoints. It routes telemetry to Sentry projects based on a resource attribute (default: `service.name`), resolves project endpoints via the Sentry Management API (no DSN mode), caches endpoints for reuse, and can optionally auto-create missing projects asynchronously. Rate limits returned by Sentry are honored per DSN and category to avoid overload.

## Background and Motivation

Sentry now ingests OTLP natively, so we no longer need the legacy span transformation pipeline. The goal is to deliver a vendor-neutral exporter that:

- sends traces and logs as OTLP protobuf over HTTP,
- routes data to the correct Sentry project without application changes, and
- leverages the Sentry Management API to discover endpoints and provision projects when allowed.

## Scope & Constraints

### Goals

- Forward OTLP traces and logs without transformation.
- Resolve and cache per-project OTLP endpoints via Sentry Management API.
- Route by resource attribute (default `service.name`) with optional mapping overrides.
- Optionally auto-create projects when missing, using the first available team in the org.
- Respect Sentry rate limits per DSN/category and surface throttle hints to the collector queue.

### Constraints

- Requires Sentry Management API access (`url`, `org_slug`, `auth_token`).
- Single-organization per exporter instance; routing does not span multiple orgs.
- No custom mutation of OTLP payloads; data is proxied as-is.
- Per-batch retries are avoided to prevent duplicating data when a batch targets multiple projects.

### Not in Scope

- Legacy DSN-only mode (removed).
- Self-hosted Sentry validation (designed and tested against sentry.io APIs).
- Fan-out to multiple orgs from a single exporter instance (use multiple exporters).
- Transforming payloads or enriching events beyond routing attributes.

## Architecture

### Components

- **endpointState** (shared across traces/logs exporters):
  - HTTP client created from `confighttp.ClientConfig`.
  - Sentry Management API client for project discovery, key lookup, and optional creation.
  - In-memory project -> OTLP endpoint cache with RW locking.
  - `singleflight` guard to avoid duplicate endpoint fetches.
  - Background project-creation worker with bounded queue (1000 requests) and guardrail of max 1000 cached projects.
  - Per-DSN/category rate limiter populated from response headers.
- **sentryExporter** (per-signal wrapper):
  - Splits incoming telemetry by project slug and forwards to `endpointState`.

### Startup Flow

1. Validate config; default timeout to 30s.
2. Build HTTP client and Sentry API client.
3. Start async project-creation worker (queue size 1000).
4. Fetch org projects; record the first team slug found to use for auto-create.
5. Preload endpoints from `/api/0/organizations/{org_slug}/project-keys/` and existing projects to warm the cache.

### Runtime Flow (per batch)

1. Group ResourceSpans/ResourceLogs by project slug (from routing attribute) and platform (`other` placeholder for now).
2. For each project:
   - Resolve endpoint from cache or fetch via Management API.
   - If missing and `auto_create_projects` is enabled, enqueue async creation and drop the current data with a warning; queue-full or missing default team returns an error.
   - If the cache entry was invalidated by a 403 containing `ProjectId`, evict and refetch once before failing.
3. Serialize OTLP protobuf and POST to the project OTLP endpoint:
   - Traces: `https://{host}/api/{projectID}/integration/otlp/v1/traces/`
   - Logs:   `https://{host}/api/{projectID}/integration/otlp/v1/logs/`
   - Header: `x-sentry-auth: sentry sentry_key=<public key>`
4. Parse `X-Sentry-Rate-Limits` (or HTTP 429) to update per-category deadlines; limited data is dropped with a throttle error so the collector queue can back off.

### Routing & Project Mapping

- Default routing attribute: `service.name` (configurable via `routing.project_from_attribute`).
- Attribute value must be a non-empty string. Missing/empty drops the resource with a warning.
- Optional `attribute_to_project_mapping` remaps attribute value -> project slug.

### Project Cache & Auto-Creation

- Cache lifetime is the exporter lifetime; guarded by RW mutex.
- Endpoint resolution uses `GetOTLPEndpoints` (project keys) under a `singleflight` to prevent duplicate fetches; first team slug seen during startup is kept as default for creation.
- Auto-create:
  - Requires `auto_create_projects: true` and a discovered default team slug; otherwise an error is returned.
  - Requests are queued (size 1000); data for that project is dropped until creation + endpoint fetch succeed.
  - Rate limits from Management API (429 + `X-Sentry-Rate-Limit-Reset`) are honored when re-queuing.
  - Guardrails: max 1000 cached projects; queue full results in an error.

### Rate Limiting

- Parses `X-Sentry-Rate-Limits` into per-category deadlines (`transaction`, `log`, `all`).
- On HTTP 429 without header, falls back to 60s backoff.
- Throttle errors surface to exporterhelper so the collector queue can retry later; limited data for that DSN/category is dropped until the deadline expires.

## Configuration

| Parameter | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `url` | string | yes | - | Base Sentry URL (e.g., `https://sentry.io`). |
| `org_slug` | string | yes | - | Organization slug. |
| `auth_token` | string | yes | - | Management API token (`project:read`; `project:write` for auto-create). |
| `auto_create_projects` | bool | no | `false` | Queue async creation when a project is missing. |
| `routing.project_from_attribute` | string | no | `service.name` | Resource attribute used for routing. |
| `routing.attribute_to_project_mapping` | map | no | - | Map attribute value -> project slug. |
| `http` | object | no | collector default | Standard `confighttp` client settings (timeout/TLS/headers). |
| `timeout` | duration | no | 30s | Exporter timeout. |
| `sending_queue` | object | no | enabled | Queue settings from exporterhelper. |

### Example

```yaml
exporters:
  sentry:
    url: https://sentry.io
    org_slug: my-org
    auth_token: ${SENTRY_AUTH_TOKEN}
    auto_create_projects: true
    routing:
      project_from_attribute: service.name
      attribute_to_project_mapping:
        api-service: backend-api
        worker: background-jobs
    http:
      timeout: 20s
```

## Trace Connectedness

This exporter forwards OTLP data only. To associate errors and maintain Sentry trace connectedness when also using the `sentry-go` SDK, enable the Sentry OTLP integration with `setup_otlp_traces_exporter=false`.

## Risks & Mitigations

- **Missing routing attribute:** resource data is dropped with a warning. Mitigation: ensure `service.name` (or configured attribute) is present.
- **Cache drift (project deleted/rotated keys):** 403 with `ProjectId` triggers cache eviction and refetch. Mitigation: cache is refreshed on failure; pre-create projects to reduce churn.
- **Auto-create race / first batch loss:** data routed to a new project is dropped while async creation completes. Mitigation: pre-create or accept initial loss; queue is bounded to avoid unbounded load.
- **Rate limiting:** per-DSN/category deadlines prevent hammering Sentry; throttle errors bubble to the collector queue for backoff.
- **Multi-project batches:** retries across multiple projects can duplicate accepted data, so per-batch retry is avoided by design.
