# Prometheus API Server Extension

This extension runs as a Web server that loads the remote observers that are registered against it.

No configuration settings are within the extension itself, but settings to enable in a Prometheus receiver component are necessary in order to have data for the API server.

Example:
```yaml
extensions:
  prometheus_api_server:
```

The full list of settings exposed for this exporter are documented [here](./config.go).
