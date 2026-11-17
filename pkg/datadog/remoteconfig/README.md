# Collector RC adapter

This public module adapts the existing Agent RC engine to the Collector lifecycle and SDK HTTP protocol. It does not import DDOT or start an Agent application. The Datadog extension and DDOT use this implementation.

Collector 0.164 / stable 1.70 is the minimum supported API. Module tests run at that minimum; the Datadog extension integration tests use the repository's current Collector version. The `collector-version: minimum` marker keeps the distribution version check from raising this library's minimum automatically.

The RC engine uses the published Agent `pkg/config/remote` module at `v0.84.0-devel.0.20261030015547-4fcf7d423463`. No unmerged Agent changes or companion Agent are required. Standalone builds must supply compatible Agent protocol version metadata; the Agent default `6.0.0` does not receive configurations from the tested backend. DDOT supplies that metadata in its build. This identity contract still needs RC team review before shipping.

`NewSDKServer` adds a shared SDK listener with independently enabled RC and instrumentation telemetry. `New` remains the RC-only constructor. Telemetry-only mode starts no Agent RC service and requires no cache or Agent configuration reader. Both upstream and DDOT factories use `NewSDKServer`.

The forwarding path preserves opaque compressed bodies, uses Collector credentials and bounded synchronous delivery, and reports downstream status. RC-specific metrics in `telemetry.go` remain separate from SDK instrumentation telemetry.

## Published-engine POC limits

The Agent engine owns its backend HTTP client. It retains shared polling, caching, signed configuration verification, targeting and the SDK RC protocol. This adapter does not copy the Agent coordinator.

RC backend requests use Agent HTTP behavior rather than the full Collector HTTP client configuration. The extension passes basic proxy settings through its Agent configuration bridge. Normal HTTPS certificate verification remains enabled. Collector custom CA files, client certificates, HTTP instrumentation and transport-specific timeouts are not applied to that RC client.

`request_timeout` bounds SDK-facing RC requests, not the engine's background requests. Background requests have no adapter-imposed total deadline or shutdown cancellation. A stalled backend can delay updates, leave requests running after shutdown, or delay shutdown on lock-held lookup paths. The adapter cannot explicitly close that client's idle connections; ordinary idle expiry still applies. The published client follows Go's default redirect policy rather than rejecting redirects.

Use the fixed Datadog backend and ordinary system-trusted HTTPS for this POC. Backend deadline, redirect, cancellation, cleanup and full HTTP configuration parity need separate review before production use. The SDK listener's TLS/authentication, SDK telemetry forwarding and DI exporter are separate paths and retain their own HTTP controls.
