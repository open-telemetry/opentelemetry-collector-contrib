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