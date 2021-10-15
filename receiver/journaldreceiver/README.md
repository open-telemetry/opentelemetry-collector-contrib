## `Journald Receiver`

Parses Journald events from systemd journal using the [opentelemetry-log-collection](https://github.com/open-telemetry/opentelemetry-log-collection) library.
Journald receiver is dependent on `journalctl` binary to be present and must be in the $PATH of the agent. 
Supported pipeline types: logs

> :construction: This receiver is in alpha and configuration fields are subject to change.

## Configuration

| Field                  | Default          | Description                                                                                                        |
| ---                    | ---              | ---                                                                                                                |
| `directory`            | /run/log/journal or /run/journal | A directory containing journal files to read entries from.     |
| `files`                |                  | A list of journal files to read entries from                  |
| `start_at`              | `end`              | At startup, where to start reading logs from the file. Options are beginning or end          |
| `units`        | `[ssh, kubelet, docker, containerd]` | A list of units to read entries from          |
| `prioriry`             | `info`           | Filter output by message priorities or priority ranges        |

### Example Configurations
```yaml
receivers:
  journald:
    directory: /run/log/journal
    units:
      - ssh
      - kubelet
      - docker
      - containerd
    priority: info
```