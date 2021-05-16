# Datadog APM Receiver

## Overview
The Datadog APM Receiver accepts traces in the Datadog Trace Agent Format
## Configuration

Example:

```yaml
receivers:
  datadog:
    endpoint: 0.0.0.0:8126
    read_timeout: 60s
```

### endpoint (Optional)
The address and port on which this receiver listens for traces on

Default: `0.0.0.0:8126`
