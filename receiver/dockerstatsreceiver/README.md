# Docker Stats Receiver

The Docker Stats receiver queries the local Docker daemon's container stats API for
all desired running containers on a configured interval.  These stats are for container
resource usage of cpu, memory, network, and the
[blkio controller](https://www.kernel.org/doc/Documentation/cgroup-v1/blkio-controller.txt).

Requires Docker API version 1.22+.  At this time only Linux is supported.

## Configuration

The following settings are required:

* `endpoint` (default = `unix:///var/run/docker.sock`): Address to reach the desired Docker daemon. 

The following settings are optional:

* `collection_interval`: (default = `10s`) The interval at which to gather container stats.
* `timeout` (default = `5s`): The request timeout for any docker daemon query.
* `container_labels_to_metric_labels` (no default): A map of Docker container label names whose label values to use
as the specified metric label key.
* `env_vars_to_metric_labels` (no default): A map of Docker container environment variables whose values to use
as the specified metric label key.
* `excluded_images` (no default, all running containers monitored): A list of strings,
[regexes](https://golang.org/pkg/regexp/), or [globs](https://github.com/gobwas/glob) whose referent container image
names will not be among the queried containers. `!`-prefixed negations are possible for all item types to signify that
only unmatched container image names should be monitored.
    * Regexes must be placed between `/` characters: `/my?egex/`.  Negations are to be outside the forward slashes:
    `!/my?egex/` will monitor all containers whose name doesn't match the compiled regex `my?egex`.
    * Globs are non-regex items (e.g. `/items/`) containing any of the following: `*[]{}?`.  Negations are supported:
    `!my*container` will monitor all containers whose image name doesn't match the blob `my*container`.
* `provide_all_block_io_metrics` (default = `false`): Whether to report metrics for all available blkio controller stats.
    * By default only `container.blockio.io_service_bytes_recursive.read` and `container.blockio.io_service_bytes_recursive.write`
    will be reported.
* `provide_all_cpu_metrics` (default = `false`): Whether to report metrics for all available cpu stats.
    * By default only `container.cpu.usage.system` and `container.cpu.usage.total` will be reported.
* `provide_all_memory_metrics` (default = `false`): Whether to report metrics for all available memory stats.
    * By default only `container.memory.usage.limit` and `container.memory.usage.total` will be reported.
* `provide_all_network_metrics` (default = `false`): Whether to report metrics for all available network stats.
    * By default only `container.network.io.usage.rx_bytes` and `container.network.io.usage.tx_bytes` will be reported.

Example:

```yaml
receivers:
  docker_stats:
    endpoint: http://example.com/
    collection_interval: 2s
    timeout: 20s
    container_labels_to_metric_labels:
      my.container.label: my-metric-label
      my.other.container.label: my-other-metric-label
    env_vars_to_metric_labels:
      MY_ENVIRONMENT_VARIABLE: my-metric-label
      MY_OTHER_ENVIRONMENT_VARIABLE: my-other-metric-label
    excluded_images:
      - undesired-container
      - /.*undesired.*/
      - another-*-container
    provide_all_block_io_metrics: true
    provide_all_cpu_metrics: true
    provide_all_memory_metrics: true
    provide_all_network_metrics: true
```
