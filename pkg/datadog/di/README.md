# DI conversion and delivery

This public module converts versioned DI LogRecords to the existing debugger requests. The Datadog exporter and DDOT use the same converter, HTTP sender and partial-retry handling. It has no RC or Datadog extension dependency.

Collector 0.164 / stable 1.70 is the minimum supported API. Module tests run at that minimum; the Datadog exporter integration tests use the repository's current Collector version. The `collector-version: minimum` marker keeps the distribution version check from raising this library's minimum automatically.

`Config` is shared across distributions. Put `Router.ConsumeLogs` inside the exporter's queue and retry boundary, start its HTTP client with the component, and close it during shutdown. Application attributes never select credentials or destinations.
