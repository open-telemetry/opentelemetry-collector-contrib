# Podman Stats Receiver

The Podman Stats receiver queries the Podman service API to fetch stats for all running containers 
on a configured interval.  These stats are for container
resource usage of cpu, memory, network, and the
[blkio controller](https://www.kernel.org/doc/Documentation/cgroup-v1/blkio-controller.txt).

Supported pipeline types: metrics

> :information_source: Requires Podman API version 2.2.1+ and only Linux is supported.

## Configuration

The following settings are required:

- `endpoint` (default = `unix:///run/podman/podman.sock`): Address to reach the desired Podman daemon.

The following settings are optional:

- `collection_interval` (default = `10s`): The interval at which to gather container stats.

Example:

```yaml
receivers:
  podman_stats:
    endpoint: http://example.com/
    collection_interval: 10s
```

The full list of settings exposed for this receiver are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).
