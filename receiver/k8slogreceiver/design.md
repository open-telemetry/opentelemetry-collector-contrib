# Overview

The k8slogreceiver is a receiver that receives logs from Kubernetes pods.

```
   K8s
   API ─────┐
            │
 Docker     │
  API ────┐ │
          │ │
Cri       │ │
API ────┐ │ │
        │ │ │
        ▼ ▼ ▼
    ┌──────────────┐         ┌───────────┐
    │    Source    │────────▶│   Poller  │
    └──────────────┘         └───────────┘
          │                       │
          │                       │ 
          ▼                       │
    ┌───────────────┐             │
    │  Reader       │◀────────────┘
    └───────────────┘
```

The `Source` will get information about pods from the Kubernetes API and will get the paths will log files exist from the Docker API and the CRI API (Only containerd is supported now).

Then, the files will be found and read by the `Poller` and `Reader`.

More detailed, for the three mode, the design is as follows:

## daemonset-stdout

```
 k8sapi     ┌──────────────┐    docker/cri-containerd
   │        │              │             │
   └───────▶│    Source    │◀────────────┘
  medatada  │              │ logPath, env
            └──────────────┘
                  │ assosiate
                  ▼
            ┌──────────────┐
            │   Reader     │
            └──────────────┘
                  │ read files from logPath
                  ▼
                files
```

The `Source` will get pod list and metadata from k8s api (maybe there will also be an implementation using kubelet).
Then, it will get the logPath and env from docker/cri-containerd api. And they will be associated with the pod metadata
by `container.id`.
Finally, there will be a Reader to read the files from logPath.

## daemonset-file

```
  k8sapi     ┌──────────────┐    docker/cri-containerd
    │        │              │             │
    └───────▶│    Source    │◀────────────┘
   medatada  │              │ graph driver, 
             │              │ mount points, env
             └──────────────┘
                  │ assosiate
                  ▼
              ┌──────────────┐
              │   Poller     │ find files based on includes mount point.
              └──────────────┘
                  │ create
                  ▼
              ┌──────────────┐
              │   Reader     │
              └──────────────┘
                  │ read files
                  ▼
                 files
```

As same as `deamonset-stdout`, the `Source` will get pod list and metadata from k8s api (maybe there will also be an
implementation using kubelet).
Then, it will get the graph driver (to get mount point of `/`), and mount points, so the actual path of log files could
be calculated.
Then, the `Poller` will find the files based on the includes mount point. And the `Reader` will read the files from
logPath.

For example, for a docker container, which has the following mount points:

- / -> /var/lib/docker/overlay2/xxxxxxxxxxxxx/merged, where the newly generated files are under
  /var/lib/docker/overlay2/xxxxxxxxxxxxx/upper
- /logs -> /home/xxx/logs/

The first one is caculated by the graph driver, and the second one is from the mount points.

Also, the host root is mounted under /host-root on the otel container.

Then, when the user configures `include=["/logs/*.log", "/var/log/*.log"]`, we can find log files by
`/host-root/home/xxx/logs/*.log` and `/host-root/var/lib/docker/overlay2/xxxxxxxxxxxxx/upper/var/log/*.log`.
If symbolic links are involved, we need to recursively perform the above mount-point-based mapping process on the target
of the link according to the mount point.

## sidecar

```
            ┌──────────────┐
  env ───▶  │   Poller     │ find files based on include from config
            └──────────────┘
                │ create
                ▼
            ┌──────────────┐
            │   Reader     │
            └──────────────┘
                │ read files
                ▼
               files
```

This mode is similar to `filelog` receiver.
The main special feature planed is to convinentlly get k8s metadata and env from the pod.
More special features for k8s may be added in the future.