# Prometheus UI Extension

This extension runs as a Web server that loads the remote observers that are registered against it.

It allows users of the collectors to visualize data going through pipelines.

The following settings are required:

- `endpoint` (default = localhost:9090): The endpoint in which the web server will
be listening to. Use localhost:<port> to make it available only locally, or
":<port>" to make it available on all network interfaces.

Example:
```yaml

extensions:
  prometheusui:
```

The full list of settings exposed for this exporter are documented [here](./config.go).
