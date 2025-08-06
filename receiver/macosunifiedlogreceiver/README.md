# macOS Unified Logging Receiver

This receiver reads macOS Unified Logging traceV3 files and converts them to OpenTelemetry log format using encoding extensions.

## Overview

The macOS Unified Logging system was introduced in macOS 10.12 (Sierra) as Apple's centralized logging system. It stores log data in binary files called traceV3 files, which contain compressed log entries with metadata.

This receiver:
- Watches for traceV3 files using the Stanza file consumer framework
- Uses encoding extensions to decode the binary traceV3 format
- Converts the decoded logs to OpenTelemetry log format
- Supports file watching and real-time processing

## Configuration

The receiver requires an encoding extension to decode traceV3 files.

### Basic Configuration

```yaml
extensions:
  macos_unified_logging_encoding:
    parse_private_logs: false
    include_signpost_events: true
    include_activity_events: true

receivers:
  macos_unified_log:
    encoding: "macos_unified_logging_encoding"
    include:
      - "/var/db/diagnostics/Persist/*.tracev3"
      - "/var/db/diagnostics/Special/*.tracev3"
    start_at: beginning
    
exporters:
  debug:
    verbosity: detailed

service:
  extensions: [macos_unified_logging_encoding]
  pipelines:
    logs:
      receivers: [macos_unified_log]
      exporters: [debug]
```

### Configuration Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `encoding` | *required* | The encoding extension ID to use for decoding traceV3 files |
| `include` | *required* | List of file patterns to watch for traceV3 files |
| `exclude` | `[]` | List of file patterns to exclude |
| `start_at` | `end` | Whether to read from `beginning` or `end` of existing files |
| `poll_interval` | `200ms` | How often to poll for new files |
| `max_log_size` | `1MiB` | Maximum size of a single log entry |
| `max_concurrent_files` | `1024` | Maximum number of files to read concurrently |

### File Consumer Configuration

This receiver inherits all configuration options from the Stanza file consumer. See the [fileconsumer documentation](../../pkg/stanza/fileconsumer/README.md) for additional options.

## Prerequisites

1. **Encoding Extension**: You must configure a macOS Unified Logging encoding extension (e.g., `macos_unified_logging_encoding`).

2. **File Access**: The collector must have read access to the traceV3 files, typically located in:
   - `/var/db/diagnostics/Persist/` - Persistent logs
   - `/var/db/diagnostics/Special/` - Special logs
   - `/var/db/diagnostics/signpost/` - Signpost logs

3. **macOS System**: This receiver is designed specifically for macOS systems.

## Example Use Cases

### Real-time System Log Monitoring

```yaml
receivers:
  macos_unified_log:
    encoding: "macos_unified_logging_encoding"
    include:
      - "/var/db/diagnostics/Persist/*.tracev3"
    start_at: end
    poll_interval: 100ms
```

### Historical Log Analysis

```yaml
receivers:
  macos_unified_log:
    encoding: "macos_unified_logging_encoding"
    include:
      - "/path/to/archived/logs/*.tracev3"
    start_at: beginning
```

### Filtered Log Collection

```yaml
extensions:
  macos_unified_logging_encoding:
    parse_private_logs: false
    include_signpost_events: false
    include_activity_events: true
    max_log_size: 32768

receivers:
  macos_unified_log:
    encoding: "macos_unified_logging_encoding"
    include:
      - "/var/db/diagnostics/Persist/*.tracev3"
    exclude:
      - "/var/db/diagnostics/Persist/0000000000000001.tracev3"  # System logs
```

## Output Format

The receiver outputs OpenTelemetry logs with the following resource attributes:

- `log.file.path`: Path to the source traceV3 file
- `log.file.format`: Always set to "macos_unified_log_tracev3"

Log records include:
- Timestamp (converted from Mach absolute time)
- Message content
- Log level (Default, Info, Debug, Error, Fault)
- Process information (PID, effective UID)
- Thread and activity IDs
- Subsystem and category information
- Event type (Log, Signpost, Activity, etc.)

## Limitations

- Only supports traceV3 format (macOS 10.12+)
- Requires appropriate file system permissions
- Binary file format parsing depends on the encoding extension implementation
- Performance may vary with large traceV3 files

## Troubleshooting

### Common Issues

1. **Permission Denied**: Ensure the collector has read access to traceV3 files
2. **Encoding Extension Not Found**: Verify the encoding extension is configured and the ID matches
3. **No Logs Received**: Check file patterns and ensure traceV3 files exist in the specified paths
4. **Parse Errors**: Verify the encoding extension supports the traceV3 file version

### Debug Configuration

```yaml
receivers:
  macos_unified_log:
    encoding: "macos_unified_logging_encoding"
    include:
      - "/var/db/diagnostics/Persist/*.tracev3"
    
exporters:
  debug:
    verbosity: detailed
    
service:
  telemetry:
    logs:
      level: debug
  pipelines:
    logs:
      receivers: [macos_unified_log]
      exporters: [debug]
``` 
