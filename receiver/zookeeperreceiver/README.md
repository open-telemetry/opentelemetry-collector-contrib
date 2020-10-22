**Under development - not ready for use.**

# Zookeeper Receiver

The Zookeeper receiver collects metrics from a Zookeeper instance, using the `mntr` command.

## Configuration

- `endpoint`: (default = `:2181`) Endpoint to connect to collect metrics. Takes the form `host:port`.
- `timeout`: (default = `10s`) Timeout within which requests should be completed.

Example configuration.

```yaml
receivers:
  zookeeper:
    endpoint: "localhost:2181"
    collection_interval: 20s
```