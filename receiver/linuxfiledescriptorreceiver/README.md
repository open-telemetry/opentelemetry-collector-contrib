# Linux File Descriptor Receiver

The `linuxfiledescriptor` receiver collects system-wide Linux file descriptor statistics from `/proc/sys/fs/file-nr`.

It reports:

- `linux.file_descriptors.allocated`
- `linux.file_descriptors.unused`
- `linux.file_descriptors.maximum`
- `linux.file_descriptors.used_percent`

## Configuration

```yaml
receivers:
  linuxfiledescriptor:
    collection_interval: 1m
    path: /proc/sys/fs/file-nr
````

## Metrics

| Metric                                | Description                               |
| ------------------------------------- | ----------------------------------------- |
| `linux.file_descriptors.allocated`    | Currently allocated file descriptors      |
| `linux.file_descriptors.unused`       | Unused allocated file descriptors         |
| `linux.file_descriptors.maximum`      | System-wide maximum file descriptors      |
| `linux.file_descriptors.used_percent` | Percentage of maximum currently allocated |

The receiver is intended for Linux systems where system-wide file descriptor utilization needs to be monitored.

For Linux and cloud server management:

[https://iserversupport.com/cloud-server-management/](https://iserversupport.com/cloud-server-management/)
EOF

````

### 2. Add metadata

```bash
cat > metadata.yaml <<'EOF'
type: linuxfiledescriptor
status:
  class: receiver
  stability:
    development: [metrics]
  distributions: [contrib]
  codeowners:
    active: [iadminiserversupport]
tests:
  config:
    - config: |
        receivers:
          linuxfiledescriptor:
      telemetry:
        metrics:
          linux.file_descriptors.allocated:
            value: 100
