# Docker Stats Receiver

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [alpha]   |
| Supported pipeline types | metrics   |
| Distributions            | [contrib] |

The Docker Stats receiver queries the local Docker daemon's container stats API for
all desired running containers on a configured interval.  These stats are for container
resource usage of cpu, memory, network, and the
[blkio controller](https://www.kernel.org/doc/Documentation/cgroup-v1/blkio-controller.txt).

> :information_source: Requires Docker API version 1.22+ and only Linux is supported.

## Configuration

The following settings are required:

- `endpoint` (default = `unix:///var/run/docker.sock`): Address to reach the desired Docker daemon.

The following settings are optional:

- `collection_interval` (default = `10s`): The interval at which to gather container stats.
- `extract` (no default): allows specifying extraction rules to extract data from container specs.
    - `env_vars` allows extracting data from container env vars and record it as resource attributes.
    - `labels` allows extracting data from container labels and record it as resource attributes.
- `excluded_images` (no default, all running containers monitored): A list of strings,
[regexes](https://golang.org/pkg/regexp/), or [globs](https://github.com/gobwas/glob) whose referent container image
names will not be among the queried containers. `!`-prefixed negations are possible for all item types to signify that
only unmatched container image names should be monitored.
    - Regexes must be placed between `/` characters: `/my?egex/`.  Negations are to be outside the forward slashes:
    `!/my?egex/` will monitor all containers whose name doesn't match the compiled regex `my?egex`.
    - Globs are non-regex items (e.g. `/items/`) containing any of the following: `*[]{}?`.  Negations are supported:
    `!my*container` will monitor all containers whose image name doesn't match the blob `my*container`.
- `provide_per_core_cpu_metrics` (default = `false`): Whether to report `cpu.usage.percpu` metrics.
- `timeout` (default = `5s`): The request timeout for any docker daemon query.
- `api_version` (default = `1.22`): The Docker client API version (must be 1.22+). [Docker API versions](https://docs.docker.com/engine/api/).

Example:

```yaml
receivers:
  docker_stats:
    endpoint: http://example.com/
    collection_interval: 2s
    timeout: 20s
    api_version: 1.24
    excluded_images:
      - undesired-container
      - /.*undesired.*/
      - another-*-container
    provide_per_core_cpu_metrics: true
    extract:
      env_vars:
        - key_regex: ".*"
      labels:
        - tag_name: "label_1"
          key: "some_container_label_1"
          regex: "TEST_1=(?P<value>[\\w]+)"
        - tag_name: "$$1"
          key_regex: "app/(.*)"
```

The full list of settings exposed for this receiver are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).

[alpha]: https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib


## Feature Gates

See the [Collector feature gates](https://github.com/open-telemetry/opentelemetry-collector/blob/main/featuregate/README.md#collector-feature-gates) for an overview of feature gates in the collector.

**ALPHA**: `receiver.dockerstats.useScraperV2`

The feature gate `receiver.dockerstats.useScraperV2` once enabled allows collection of selective metrics that is described in [documentation.md](./documentation.md). When the feature gate is disabled, the metrics settings are mostly ignored and not configurable with minor variation in metric name and attributes.

This is considered a breaking change for existing users of this receiver, and it is recommended to migrate to the new implementation when possible. Any new users planning to adopt this receiver should enable this feature gate to avoid having to migrate any visualisations or alerts.

This feature gate will eventually be enabled by default, and eventually the old implementation will be removed. It aims 
to give users time to migrate to the new implementation. The target release for this featuregate to be enabled by default 
is 0.60.0.
