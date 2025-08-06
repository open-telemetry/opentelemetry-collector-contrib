# macOS Unified Logging Encoding Extension

This encoding extension provides the ability to decode macOS Unified Logging traceV3 files into OpenTelemetry log format.

## Overview

The macOS Unified Logging system was introduced in macOS 10.12 (Sierra) as Apple's new centralized logging system. It stores log data in binary files called traceV3 files, which contain compressed log entries with metadata.

This extension focuses on parsing traceV3 files and extracting:
- Log messages
- Timestamps (Mach absolute time)
- Process information (PID, effective UID)
- Thread and activity IDs
- Log levels (Default, Info, Debug, Error, Fault)
- Event types (Log, Signpost, Activity, etc.)
- Subsystem and category information

## Configuration

The extension supports the following configuration options:

```yaml
extensions:
  macos_unified_logging_encoding:
    parse_private_logs: false        # Whether to attempt parsing private/masked log data
    include_signpost_events: true    # Include signpost events in parsing
    include_activity_events: true    # Include activity events in parsing
    max_log_size: 65536             # Maximum size for individual log entries (bytes)
```

## Usage

This extension is designed to be used with receivers that can provide raw traceV3 file data, such as the filelog receiver reading from macOS unified log directories:

```yaml
receivers:
  filelog:
    include: ['/private/var/db/diagnostics/**/*.tracev3']
    operators:
      - type: macos_unified_logging_decoder

processors:
  batch:

exporters:
  otlp:
    endpoint: "your-backend:4317"

extensions:
  macos_unified_logging_encoding:
    parse_private_logs: false
    include_signpost_events: true

service:
  extensions: [macos_unified_logging_encoding]
  pipelines:
    logs:
      receivers: [filelog]
      processors: [batch]
      exporters: [otlp]
```

## Data Format

The extension extracts the following log attributes:

| Attribute | Type | Description |
|-----------|------|-------------|
| process_id | int | Process ID that generated the log |
| thread_id | int | Thread ID within the process |
| effective_uid | int | Effective user ID |
| activity_id | int | Activity ID for related log entries |
| event_type | string | Type of event (Log, Signpost, Activity, etc.) |
| subsystem | string | Bundle ID or subsystem identifier |
| category | string | Category within the subsystem |
| process_path | string | Path to the process executable |
| library_path | string | Path to the library that generated the log |

## Limitations

This is an initial implementation with several limitations:

1. **Simplified Parsing**: The traceV3 format is complex and proprietary. This implementation provides basic parsing but may not handle all edge cases.

2. **Timestamp Conversion**: Mach absolute time conversion to Unix timestamps is simplified and may not be accurate without proper boot time reference.

3. **Message Reconstruction**: Complex message formatting with argument substitution is not fully implemented. Some messages may appear as binary data.

4. **Private Data**: Private/masked log data requires special handling not implemented in this version.

5. **String Tables**: The extension doesn't fully utilize UUID files for string table lookups.

## Security Considerations

- This extension processes binary log files from the macOS system
- Log files may contain sensitive information
- The extension includes safeguards against infinite loops and excessive memory usage
- Maximum entry sizes and counts are enforced to prevent resource exhaustion

## References

- [Mandiant's macos-UnifiedLogs parser](https://github.com/mandiant/macos-UnifiedLogs)
- [Reviewing macOS Unified Logs (Mandiant Blog)](https://www.mandiant.com/resources/blog/reviewing-macos-unified-logs)
- [Finding Waldo: Leveraging the Apple Unified Log for Incident Response (CrowdStrike)](https://www.crowdstrike.com/blog/how-to-leverage-apple-unified-log-for-incident-response/)
- [libyal Apple Unified Logging format documentation](https://github.com/libyal/dtformats/blob/main/documentation/Apple%20Unified%20Logging%20and%20Activity%20Tracing%20formats.asciidoc) 
