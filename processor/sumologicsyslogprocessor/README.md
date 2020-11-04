# Sumo Logic Syslog Processor

Supported pipeline types: logs

The Sumo Logic Syslog processor can be used to create attribute with facility name
based on facility code. Default facility name is `syslog`.

## Configuration

| Field         | Default  | Description                                                        |
|---------------|----------|--------------------------------------------------------------------|
| facility_attr | facility | The attribute name in which a facility name is going to be written |

## Examples

Following table shows example facility names which are extracted from log line

| log                       | facility            |
|---------------------------|---------------------|
| <13> Example log          | user-level messages |
| <334> Another example log | syslog              |
| Plain text log            | syslog              |

## Configuration Example

```yaml
processors:
  sumologic_syslog:
    facility_attr: testAttrName
```
