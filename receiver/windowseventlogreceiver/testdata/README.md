# Windows Event Log Receiver Test Configurations

This directory contains test configurations for the windowseventlogreceiver.

## Files

- **config.yaml** - Minimal test configuration used by unit tests
- **collector-config-example.yaml** - Complete collector configuration demonstrating various use cases

## Testing on Windows

The windowseventlogreceiver only works on Windows operating systems. To test:

### 1. Build the Collector

From the repository root:

```bash
make otelcontribcol
```

This builds the OpenTelemetry Collector Contrib binary and places it in `./bin/otelcontribcol_<OS>_<ARCH>`.

For cross-compilation to Windows from another platform:

```bash
GOOS=windows GOARCH=amd64 make otelcontribcol
```

This creates `./bin/otelcontribcol_windows_amd64.exe`.

### 2. Transfer to Windows Machine (if cross-compiling)

Copy the following files to a Windows machine:
- The collector binary from `./bin/otelcontribcol_windows_amd64.exe`
- The example configuration file(s) from this directory

### 3. Run the Collector

On the Windows machine, run:

```powershell
.\otelcontribcol_windows_amd64.exe --config collector-config-example.yaml
```

### 4. Verify Event Collection

Check the output (console or log file depending on your exporter configuration) to verify Windows Event Logs are being collected.

## Common Windows Event Log Channels

- `application` - Application events
- `system` - System events
- `security` - Security audit events (requires elevated privileges)
- `setup` - Setup and installation events
- `Microsoft-Windows-PowerShell/Operational` - PowerShell activity
- `Microsoft-Windows-Sysmon/Operational` - Sysmon events (if installed)
- `Microsoft-Windows-TaskScheduler/Operational` - Task Scheduler events
- `Microsoft-Windows-Windows Defender/Operational` - Windows Defender events

## Configuration Options

### Required
- `channel` - The Windows Event Log channel to monitor

### Optional
- `start_at` - Where to start reading: `beginning` or `end` (default: `end`)
- `max_reads` - Maximum number of records to read per poll (default: 100)
- `poll_interval` - How often to poll for new events (default: 1s)

### Operators

The windowseventlogreceiver uses the Stanza adapter framework, which supports additional operators for log processing:

```yaml
receivers:
  windowseventlog:
    channel: application
    start_at: end
    operators:
      - type: filter
        expr: 'body.event_id.id == 1000'  # Only process event ID 1000
      - type: regex_parser
        regex: '^(?P<timestamp>\S+)\s+(?P<message>.*)$'
```

See the [Stanza operators documentation](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/stanza/docs/operators) for more information.

## Notes

- The Security event log typically requires administrator/elevated privileges
- Some custom event log channels may require specific Windows features or third-party software
- The receiver will automatically handle log rotation and continue reading from the correct position
