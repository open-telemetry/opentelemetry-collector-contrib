# OTelCol Context

The `otelcol` context exposes data passed through the OpenTelemetry Collector by its components, including client details, request information, and incoming gRPC metadata.
Access these paths under the top-level `otelcol` segment, optionally followed by one or more sub-segments.
This data is only available inside the OpenTelemetry Collector and is not included by default in telemetry exported from the Collector.

## Paths

The following paths are supported.

| path                               | field accessed                                                                                                            | type                                                                    |
|------------------------------------|---------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------|
| otelcol.client.addr                | the remote address string from the client info                                                                            | string                                                                  |
| otelcol.client.metadata            | client metadata attached to the request via `go.opentelemetry.io/collector/client`                                        | pcommon.Map                                                             |
| otelcol.client.metadata[""]        | the value for a specific metadata key                                                                                     | string or nil                                                           |
| otelcol.client.auth.attributes     | map of all auth attributes values extracted from `client.Info.Auth`. Unsupported value types are mapped as empty string   | pcommon.Map                                                             |
| otelcol.client.auth.attributes[""] | the value for a specific auth attribute key                                                                               | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| otelcol.grpc.metadata              | incoming gRPC metadata from the context                                                                                   | pcommon.Map                                                             |
| otelcol.grpc.metadata[""]          | values slice for a specific incoming gRPC metadata key                                                                    | string or nil                                                           |


> [!NOTE]
This context is read-only; any attempt to set these paths returns an error.

## Security and best practices

The `otelcol` context exposes client and request data that often contains **sensitive information**: authentication tokens (e.g. in `otelcol.client.auth.attributes`), HTTP headers and gRPC metadata (e.g. in `otelcol.client.metadata` and `otelcol.grpc.metadata`), and client address. 
OTTL can **read** these values and **write** them into span attributes, log body, resource attributes, or metric labels. 
Copying such data into telemetry can expose secrets to every system that receives it.

### Do not leak secrets into telemetry

Avoid writing `otelcol` path values into telemetry unless you have explicitly confirmed that the key and value are non-sensitive.

**Unsafe examples (do not do this):**

```yaml
# BAD: Copies client metadata (e.g. Authorization header) into span attributes
- set(span.attributes["auth_header"], otelcol.client.metadata["authorization"])

# BAD: Copies auth attributes into resource (it may contain tokens)
- set(resource.attributes["auth"], otelcol.client.auth.attributes["token"])

# BAD: Copies gRPC metadata into log body (may expose API keys or tokens)
- set(log.body, otelcol.grpc.metadata["x-api-key"])
```

**Safe examples:**

```yaml
# OK: Use otelcol paths only in conditions (not written to telemetry)
- set(span.attributes["routed"], true) where otelcol.client.metadata["x-tenant-id"] != nil

# OK: Copy only a non-sensitive, allowlisted key you control
- set(span.attributes["tenant_id"], otelcol.client.metadata["x-tenant-id"]) where IsMatch(otelcol.client.metadata["x-tenant-id"], "^[a-zA-Z0-9-]+$")
```

### Commonly sensitive keys

Treat the following (and similar) keys as sensitive. Do not copy their values into attributes, body, or resource:

| Key (typical)                    | Reason                                |
|----------------------------------|---------------------------------------|
| `authorization`                  | Bearer tokens, Basic auth credentials |
| `cookie`                         | Session and auth cookies              |
| `x-api-key`, `api-key`           | API keys                              |
| `x-auth-token`, `x-access-token` | Access tokens                         |
| `proxy-authorization`            | Proxy credentials                     |

This applies to **`otelcol.client.metadata["key"]`** and **`otelcol.grpc.metadata["key"]`**. 
For **`otelcol.client.auth.attributes`**, treat the **entire** map as sensitive unless you know the auth extension only exposes non-secret attributes. 
HTTP header names are often normalized to lowercase; consider that when allowlisting or blocklisting.

### Recommended practices

1. **Prefer using `otelcol` paths only in conditions**: Use them to decide what to do (filter, route, sample) without storing values in telemetry.
2. **Allowlist keys when copying to telemetry**: If you must copy, restrict to a small set of non-sensitive keys (e.g. `x-tenant-id`, `x-request-id`, `x-correlation-id`) and validate or sanitize values.
3. **Prefer single-key access**: Access only the key you need (e.g. `otelcol.client.metadata["x-tenant-id"]`) rather than the full map (e.g. `otelcol.client.metadata`). Passing the whole map into attributes or functions can expose all metadata, including sensitive keys you did not intend to use.
4. **Document and review**: Treat OTTL that references `otelcol` paths as security-sensitive; review and document which keys are read and whether any are written to telemetry.

### Safe use cases

- **Routing**: Send telemetry to different backends or pipelines based on non-sensitive metadata (e.g. `x-tenant-id`, `x-environment`).
- **Filtering**: Drop or sample based on metadata (e.g. internal vs external) without storing the value.
- **Enrichment**: Add only allowlisted, non-secret values (e.g. request ID, tenant ID) as attributes for correlation or multi-tenancy.

| Do                                                      | Don't                                                                            |
|---------------------------------------------------------|----------------------------------------------------------------------------------|
| Use `otelcol` paths in conditions for routing/filtering | Copy `authorization`, `cookie`, or API key headers into attributes/body/resource |
| Allowlist and validate keys when copying to telemetry   | Copy `otelcol.client.auth.attributes` or full metadata maps into exported data   |
| Document and review OTTL that uses `otelcol` context    | Assume all client/metadata values are safe to store in telemetry                 |


## Feature gate

The `otelcol` context is temporally controlled by the **`ottl.contexts.enableOTelColContext`** [feature gate](https://github.com/open-telemetry/opentelemetry-collector/blob/main/featuregate/README.md#collector-feature-gates).
This gate is **enabled by default**, so all components using OTTL have access to `otelcol.*` paths unless you explicitly disable it.
The feature gate is expected to be removed in a future release once the feature is stable; after that, the `otelcol` context will always be available with no opt-in.