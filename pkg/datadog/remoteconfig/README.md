# Collector RC adapter

This public module adapts the existing Agent RC engine to the Collector lifecycle and SDK HTTP protocol. It does not import DDOT or start an Agent application. The Datadog extension and DDOT use this implementation.

Collector 0.164 / stable 1.70 is the minimum supported API. Module tests run at that minimum; the Datadog extension integration tests use the repository's current Collector version. The `collector-version: minimum` marker keeps the distribution version check from raising this library's minimum automatically.

The backend API is pinned to an experimental Agent commit until the independent API change is merged and released. Standalone builds must supply compatible Agent protocol version metadata; the Agent default `6.0.0` does not receive configurations from the tested backend. DDOT supplies that metadata in its build. This identity contract still needs RC team review before shipping.
