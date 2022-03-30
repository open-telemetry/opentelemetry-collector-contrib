# Host Metrics Receiver

The Host Metrics receiver generates metrics about the host system scraped
from various sources. This is intended to be used when the collector is
deployed as an agent.

Supported pipeline types: metrics

## Getting Started

The collection interval and the categories of metrics to be scraped can be
configured:

```yaml
hostmetrics:
  collection_interval: <duration> # default = 1m
  scrapers:
    <scraper1>:
    <scraper2>:
    ...
```

The available scrapers are:

| Scraper    | Supported OSs                | Description                                            |
|------------|------------------------------|--------------------------------------------------------|
| cpu        | All except Mac<sup>[1]</sup> | CPU utilization metrics                                |
| disk       | All except Mac<sup>[1]</sup> | Disk I/O metrics                                       |
| load       | All                          | CPU load metrics                                       |
| filesystem | All                          | File System utilization metrics                        |
| memory     | All                          | Memory utilization metrics                             |
| network    | All                          | Network interface I/O metrics & TCP connection metrics |
| paging     | All                          | Paging/Swap space utilization and I/O metrics
| processes  | Linux                        | Process count metrics                                  |
| process    | Linux & Windows              | Per process CPU, Memory, and Disk I/O metrics          |

### Notes

<sup>[1]</sup> Not supported on Mac when compiled without cgo which is the default.

Several scrapers support additional configuration:

### Disk

```yaml
disk:
  <include|exclude>:
    devices: [ <device name>, ... ]
    match_type: <strict|regexp>
```

### File System

```yaml
filesystem:
  <include_devices|exclude_devices>:
    devices: [ <device name>, ... ]
    match_type: <strict|regexp>
  <include_fs_types|exclude_fs_types>:
    fs_types: [ <filesystem type>, ... ]
    match_type: <strict|regexp>
  <include_mount_points|exclude_mount_points>:
    mount_points: [ <mount point>, ... ]
    match_type: <strict|regexp>
```

### Load

`cpu_average` specifies whether to divide the average load by the reported number of logical CPUs (default: `false`).

```yaml
load:
  cpu_average: <false|true>
```

### Network

```yaml
network:
  <include|exclude>:
    interfaces: [ <interface name>, ... ]
    match_type: <strict|regexp>
```

### Process
Process can be configured to have multiple filters.  Each filter operates
on an and basis.  If a process matches any one filter then it will be 
included, even if another filter explicitly excludes it.
```yaml
process:
  mute_process_name_error: <true|false>
  filters:
    -
      <include_executable_name|exclude_executable_name>:
        match_type: <strict|regexp>
        executable_name: [<executable name>, ... ]
      <include_executable_path|exclude_executable_path>:
        match_type: <strict|regexp>
        executable_path: [<executable path>, ... ]
      <include_process_command|exclude_process_command>:
        match_type: <strict|regexp>
        process_command: [<command>, ... ]
      <include_process_command_line|exclude_process_command_line>:
        match_type: <strict|regexp>
        process_command_line: [<command line>, ... ]
      <include_process_owner|exclude_process_owner>:
        match_type: <strict|regexp>
        process_owner: [<owner>, ... ]
      <include_process_pid|exclude_process_pid>:
        process_pid: [<pid>, ... ]        
    -
      <include_executable_name|exclude_executable_name>:
        match_type: <strict|regexp>
        executable_name: [<executable name>, ... ]
      <include_executable_path|exclude_executable_path>:
        match_type: <strict|regexp>
        executable_path: [<executable path>, ... ]
      <include_process_command|exclude_process_command>:
        match_type: <strict|regexp>
        process_command: [<command>, ... ]
      <include_process_command_line|exclude_process_command_line>:
        match_type: <strict|regexp>
        process_command_line: [<command line>, ... ]
      <include_process_owner|exclude_process_owner>:
        match_type: <strict|regexp>
        process_owner: [<owner>, ... ]
      <include_process_pid|exclude_process_pid>:
        process_pid: [<pid>, ... ]        
    -
      <create as many filters as desired>:
```

## Advanced Configuration

### Filtering

If you are only interested in a subset of metrics from a particular source,
it is recommended you use this receiver with the
[Filter Processor](../../processor/filterprocessor).

### Different Frequencies

If you would like to scrape some metrics at a different frequency than others,
you can configure multiple `hostmetrics` receivers with different
`collection_interval` values. For example:

```yaml
receivers:
  hostmetrics:
    collection_interval: 30s
    scrapers:
      cpu:
      memory:

  hostmetrics/disk:
    collection_interval: 1m
    scrapers:
      disk:
      filesystem:

service:
  pipelines:
    metrics:
      receivers: [hostmetrics, hostmetrics/disk]
```
