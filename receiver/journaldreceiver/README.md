## `Journald Receiver`

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [alpha]   |
| Supported pipeline types | logs      |
| Distributions            | [contrib] |

Parses Journald events from systemd journal.
Journald receiver is dependent on `journalctl` binary to be present and must be in the $PATH of the agent.

## Configuration

| Field                               | Default                              | Description                                                                                                                                                                                                                              |
|-------------------------------------|--------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `directory`                         | `/run/log/journal` or `/run/journal` | A directory containing journal files to read entries from                                                                                                                                                                                |
| `files`                             |                                      | A list of journal files to read entries from                                                                                                                                                                                             |
| `start_at`                          | `end`                                | At startup, where to start reading logs from the file. Options are beginning or end                                                                                                                                                      |
| `units`                             |                                      | A list of units to read entries from. See [Multiple filtering options](#multiple-filtering-options) examples, if you want to use it together with `matches` and/or `priority`.                                                           |
| `matches`                           |                                      | A list of matches to read entries from. See [Matches](#matches) and [Multiple filtering options](#multiple-filtering-options) examples.                                                                                                  |
| `priority`                          | `info`                               | Filter output by message priorities or priority ranges. See [Multiple filtering options](#multiple-filtering-options) examples, if you want to use it together with `units` and/or `matches`.                                            |
| `storage`                           | none                                 | The ID of a storage extension to be used to store cursors. Cursors allow the receiver to pick up where it left off in the case of a collector restart. If no storage extension is used, the receiver will manage cursors in memory only. |
| `retry_on_failure.enabled`          | `false`                              | If `true`, the receiver will pause reading a file and attempt to resend the current batch of logs if it encounters an error from downstream components.                                                                                  |
| `retry_on_failure.initial_interval` | `1 second`                           | Time to wait after the first failure before retrying.                                                                                                                                                                                    |
| `retry_on_failure.max_interval`     | `30 seconds`                         | Upper bound on retry backoff interval. Once this value is reached the delay between consecutive retries will remain constant at the specified value.                                                                                     |
| `retry_on_failure.max_elapsed_time` | `5 minutes`                          | Maximum amount of time (including retries) spent trying to send a logs batch to a downstream consumer. Once this value is reached, the data is discarded. Retrying never stops if set to `0`.                                            |

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

will be passed to `journalctl` as the following arguments: `journalctl ... _SYSTEMD_UNIT=ssh + _SYSTEMD_UNIT=kubelet _UID=1000`,
which is going to retrieve all entries which match at least one of the following rules:

- `_SYSTEMD_UNIT` is `ssh`
- `_SYSTEMD_UNIT` is `kubelet` and `_UID` is `1000`

#### Multiple filtering options

In case of using multiple following options, conditions between them are logically `AND`ed and within them are logically `OR`ed:

```text
( priority )
AND
( units[0] OR units[1] OR units[2] OR ... units[U] )
AND
( matches[0] OR matches[1] OR matches[2] OR ... matches[M] )
```

Consider the following example:

```yaml
- type: journald_input
  matches:
    - _SYSTEMD_UNIT: ssh
    - _SYSTEMD_UNIT: kubelet
      _UID: "1000"
  units:
    - kubelet
    - systemd
  priority: info
```

The above configuration will be passed to `journalctl` as the following arguments
`journalctl ... --priority=info --unit=kubelet --unit=systemd _SYSTEMD_UNIT=ssh + _SYSTEMD_UNIT=kubelet _UID=1000`,
which is going to effectively retrieve all entries which matches the following set of rules:

- `_PRIORITY` is `6`, and
- `_SYSTEMD_UNIT` is `kubelet` or `systemd`, and
- entry matches at least one of the following rules:

  - `_SYSTEMD_UNIT` is `ssh`
  - `_SYSTEMD_UNIT` is `kubelet` and `_UID` is `1000`

Note, that if you use some fields which aren't associated with an entry, the entry will always be filtered out.
Also be careful about using unit name in `matches` configuration, as for the above example, none of the entry for `ssh` and `systemd` is going to be retrieved.
