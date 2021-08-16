# Docker Observer Extension

**Status: beta**

The `docker_observer` uses a Docker client and queries the Docker Engine API for running containers.

Requires Docker API Version 1.22+.

The collector will need permissions to access the Docker Engine API, specifically it will need
read access to the Docker socket (default `unix:///var/run/docker.sock`).


### Configuration

#### `endpoint`

The URL of the docker server.

default: `unix:///var/run/docker.sock`

#### `timeout`

The maximum amount of time to wait for docker API responses.

default: `5s`

#### `excluded_images`

A list of filters whose matching images are to be excluded. Supports literals, globs, and regex.

default: `[]`

#### `use_hostname_if_present`

If true, the `Config.Hostname` field (if present) of the docker
container will be used as the discovered host that is used to configure
receivers.  If false or if no hostname is configured, the field
`NetworkSettings.IPAddress` is used instead. These settings can be found
in the output of the Docker API's [Container Inspect](https://docs.docker.com/engine/api/v1.41/#operation/ContainerInspect) json.

default: `false`

#### `use_host_bindings`

If true, the observer will configure receivers for matching container endpoints
using the host bound ip and port.  This is useful if containers exist that are not
accessible to an instance of the collector running outside of the docker network stack.

default: `false`

#### `ignore_non_host_bindings`

If true, the observer will ignore discovered container endpoints that are not bound
to host ports.  This is useful if containers exist that are not accessible
to an instance of the collector running outside of the docker network stack.

default: `false`

#### `cache_sync_interval`

The time to wait before resyncing the list of containers the observer maintains
through the docker event listener example: `cache_sync_interval: "20m"`

default: `60m`

### Endpoint Variables
`TODO in subsequent PR`
