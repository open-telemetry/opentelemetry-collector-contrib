# Design: `resourceexhaustedretryextension` — gRPC/HTTP RetryInfo Middleware

**Date:** 2026-06-08
**Status:** Approved

---

## Background

When an OTel Collector receiver returns a gRPC `RESOURCE_EXHAUSTED` error (e.g. admission queue full, Arrow decode OOM, memory limiter extension firing), the `otelarrowexporter` treats it as a **permanent error and drops the data** unless the response includes a `RetryInfo` error detail:

```go
// exporter/otelarrowexporter/otelarrow.go
case codes.ResourceExhausted:
    return retryInfo != nil  // nil → permanent → data dropped
```

No receiver currently sets `RetryInfo`, so all `RESOURCE_EXHAUSTED` responses cause data loss on the exporter side. The fix must be composable — applicable to any gRPC receiver, not just Arrow — and configurable so the default behaviour (no retries) is preserved unless explicitly opted in.

For HTTP, `otlphttpexporter` already retries 429s but uses client-side backoff without server guidance. Without a `Retry-After` header, all throttled clients retry simultaneously, causing stampeding herds. Server-controlled delay with per-response jitter solves this.

---

## Goals

- Attach `RetryInfo` to gRPC `RESOURCE_EXHAUSTED` errors so exporters retry instead of dropping data.
- Attach `Retry-After` to HTTP 429 responses to give the server control over retry timing.
- Add configurable jitter so concurrent clients spread their retries and avoid stampeding herds.
- Default behaviour is unchanged — the extension is a no-op when `retry_delay` is zero.
- Work with any gRPC or HTTP receiver that supports the `extensionmiddleware` mechanism.

## Non-Goals

- Does not change retry behaviour for any other error code.
- Does not modify any receiver (Arrow, OTLP, or otherwise).
- Does not implement dynamic (load-proportional) delay calculation.

---

## Design

### Component

**Type:** `resource_exhausted_retry`
**Package:** `extension/resourceexhaustedretryextension/`
**Implements:** `extensionmiddleware.GRPCServer` and `extensionmiddleware.HTTPServer`

The extension is a pure gRPC/HTTP middleware. It intercepts outgoing error responses, and if the response is a retryable `RESOURCE_EXHAUSTED` / 429, injects the configured delay hint before the response reaches the client. It does not interact with the pipeline, consumers, or admission control directly.

### Config

```go
type Config struct {
    // RetryDelay is the base duration advertised in RetryInfo (gRPC) or Retry-After (HTTP).
    // Zero (default) disables injection — the extension is a complete no-op.
    RetryDelay time.Duration `mapstructure:"retry_delay"`

    // Jitter is the maximum additional random duration added to RetryDelay per response.
    // Each response independently samples: effective_delay = RetryDelay + rand[0, Jitter]
    // Zero means no jitter.
    Jitter time.Duration `mapstructure:"jitter"`
}

func (c *Config) Validate() error {
    if c.RetryDelay < 0 {
        return errors.New("retry_delay must be non-negative")
    }
    if c.Jitter < 0 {
        return errors.New("jitter must be non-negative")
    }
    return nil
}
```

**Default config:** both fields zero — extension registered but no-op.

### Effective Delay

Each intercepted response independently samples a delay:

```
effective_delay = RetryDelay + rand[0, Jitter]
```

This means concurrent clients hitting the same server each receive a slightly different retry hint, spreading their retries across the `[RetryDelay, RetryDelay+Jitter]` window.

### gRPC Interceptors

The extension returns both a unary and a stream interceptor via `GetGRPCServerOptions`. Both call `wrapGRPCError` on the returned error.

```go
func (e *extension) wrapGRPCError(err error) error {
    if err == nil || e.cfg.RetryDelay == 0 {
        return err
    }
    // Permanent errors must never be retried
    if consumererror.IsPermanent(err) {
        return err
    }
    st, ok := status.FromError(err)
    if !ok || st.Code() != codes.ResourceExhausted {
        return err
    }
    // Respect RetryInfo already set by an upstream middleware
    for _, d := range st.Details() {
        if _, ok := d.(*errdetails.RetryInfo); ok {
            return err
        }
    }
    st, _ = st.WithDetails(&errdetails.RetryInfo{
        RetryDelay: durationpb.New(e.effectiveDelay()),
    })
    return st.Err()
}
```

Decision logic in priority order:

1. `RetryDelay == 0` → no-op
2. `consumererror.IsPermanent(err)` → no-op (permanent errors are never retryable)
3. Not a gRPC status, or not `RESOURCE_EXHAUSTED` → no-op
4. `RetryInfo` already present → no-op (respect upstream signal)
5. Otherwise → inject `RetryInfo` with sampled effective delay

**Coverage:** This intercepts all stream-level gRPC errors. This covers:
- Arrow receiver admission queue full (`ErrTooManyWaiters`) — terminates the stream
- Arrow receiver Arrow decode OOM (`ErrConsumerMemoryLimit`) — terminates the stream
- `memorylimiterextension` — fires before the stream handler runs, error propagates to our interceptor

### HTTP Handler Wrapper

The extension implements `GetHTTPHandler`, wrapping the base handler with a `ResponseWriter` that intercepts 429 responses:

```go
type retryResponseWriter struct {
    http.ResponseWriter
    delay       time.Duration
    wroteHeader bool
}

func (rw *retryResponseWriter) WriteHeader(code int) {
    if code == http.StatusTooManyRequests && !rw.wroteHeader {
        seconds := int(math.Ceil(rw.delay.Seconds()))
        if seconds < 1 {
            seconds = 1
        }
        rw.Header().Set("Retry-After", strconv.Itoa(seconds))
    }
    rw.wroteHeader = true
    rw.ResponseWriter.WriteHeader(code)
}
```

`Retry-After` value is `ceil(effective_delay.Seconds())`, minimum 1 (HTTP spec requires integer seconds). The `otlphttpexporter` already reads and respects `Retry-After` on 429/503 responses via `exporterhelper.NewThrottleRetry`.

### Example Configuration

```yaml
extensions:
  resource_exhausted_retry:
    retry_delay: 2s
    jitter: 3s   # clients retry anywhere between 2s–5s

receivers:
  otelarrow:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
        middlewares:
          - id: resource_exhausted_retry
    admission:
      request_limit_mib: 64

  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4318
        middlewares:
          - id: resource_exhausted_retry
      http:
        endpoint: 0.0.0.0:4319
        middlewares:
          - id: resource_exhausted_retry

service:
  extensions: [resource_exhausted_retry]
  pipelines:
    traces:
      receivers: [otelarrow, otlp]
      processors: []
      exporters: [...]
```

---

## File Structure

```
extension/resourceexhaustedretryextension/
├── config.go            // Config + Validate()
├── extension.go         // implements GRPCServer + HTTPServer, wrapGRPCError, effectiveDelay
├── factory.go           // NewFactory(), createDefaultConfig()
├── metadata.yaml        // type: resource_exhausted_retry, stability: development
├── go.mod
├── README.md
└── extension_test.go
```

---

## Testing

| Test | What it verifies |
|---|---|
| `TestWrapGRPCError_Disabled` | `retry_delay: 0` → error unchanged, no RetryInfo |
| `TestWrapGRPCError_ResourceExhausted` | RESOURCE_EXHAUSTED → RetryInfo injected with correct delay |
| `TestWrapGRPCError_Permanent` | `consumererror.IsPermanent` error → not modified even if RESOURCE_EXHAUSTED |
| `TestWrapGRPCError_ExistingRetryInfo` | RESOURCE_EXHAUSTED with RetryInfo already set → not overridden |
| `TestWrapGRPCError_OtherCodes` | UNAVAILABLE, INTERNAL etc. → not modified |
| `TestWrapGRPCError_Jitter` | With jitter configured → effective delay in `[retry_delay, retry_delay+jitter]` |
| `TestHTTPMiddleware_429` | 429 response → `Retry-After` header injected |
| `TestHTTPMiddleware_429_Disabled` | `retry_delay: 0`, 429 → no `Retry-After` header |
| `TestHTTPMiddleware_OtherStatus` | 500, 503 responses → no `Retry-After` header |
| `TestHTTPMiddleware_Jitter` | `Retry-After` value in `[retry_delay, retry_delay+jitter]` seconds |
| `TestConfig_Validate` | negative `retry_delay` or `jitter` → validation error |

---

## Alternatives Considered

### Embed in Arrow receiver only

Adding `RetryInfo` injection directly inside `recvOne` in the Arrow receiver and an internal gRPC interceptor on its server. Rejected because it only benefits the Arrow receiver — OTLP and other gRPC receivers would need separate implementations. The middleware extension approach is composable and follows the existing `memorylimiterextension` pattern.

### Separate delay knobs for gRPC vs HTTP

Separate `grpc_retry_delay` and `http_retry_delay` fields. Rejected as unnecessary complexity — operators configuring this extension are responding to general resource exhaustion on this node, and a single delay applies uniformly. Can be revisited if differentiation is needed.

### Dynamic delay proportional to queue pressure

Computing the delay based on current admission queue utilisation. Rejected as overly complex for a first implementation. Static delay + jitter is sufficient to prevent stampeding herds without coupling the middleware to admission internals.
