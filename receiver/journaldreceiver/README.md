## `Journald Receiver`

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [alpha]   |
| Supported pipeline types | logs      |
| Distributions            | [contrib] |

Parses Journald events from systemd journal.
Journald receiver is dependent on `journalctl` binary to be present and must be in the $PATH of the agent.

## Configuration

| Field       | Default                              | Description |
| ---         | ---                                  | --- |
| `directory` | `/run/log/journal` or `/run/journal` | A directory containing journal files to read entries from |
| `files`     |                                      | A list of journal files to read entries from |
| `start_at`  | `end`                                | At startup, where to start reading logs from the file. Options are beginning or end |
| `units`     |                                      | A list of units to read entries from. This option cannot be used together with `matches` |
| `matches`   |                                      | A list of matches to read entries from. This option cannot be used together with `units`. See [Matches](#matches) example |
| `priority`  | `info`                               | Filter output by message priorities or priority ranges |
| `storage`   | none                                 | The ID of a storage extension to be used to store cursors. Cursors allow the receiver to pick up where it left off in the case of a collector restart. If no storage extension is used, the receiver will manage cursors in memory only. |

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

#### Matches

The following configuration:

```yaml
- type: journald_input
  matches:
    - _SYSTEMD_UNIT: ssh
    - _SYSTEMD_UNIT: kubelet
      _UID: "1000"
```

will be passed to `journald` as the following arguments: `journald ... _SYSTEMD_UNIT=ssh + _SYSTEMD_UNIT=kubelet _UID=1000`,
which is going to retrieve all entries which matche at least one of the following rules:

- `_SYSTEMD_UNIT` is `ssh`
- `_SYSTEMD_UNIT` is `kubelet` and `_UID` is `1000`

[alpha]: https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
