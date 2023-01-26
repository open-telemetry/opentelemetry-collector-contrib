# Datadog APM Receiver

## Overview
The Datadog APM Receiver accepts traces in the Datadog Trace Agent Format

###Supported Datadog APIs
- v0.3 (msgpack and json)
- v0.4 (msgpack and json)
- v0.5 (msgpack custom format)
## Configuration

Example:

```yaml
receivers:
  datadog:
    endpoint: localhost:8126
    read_timeout: 60s
```

### endpoint (Optional)
The address and port on which this receiver listens for traces on

Default: `localhost:8126`

### read_timeout (Optional)
The read timeout of the HTTP Server

Default: 60s
