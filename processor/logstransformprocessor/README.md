# Logs Transform Processor

Supported pipeline types: logs

The logs transform processor can be used to apply [opentelemetry-log-collection](https://github.com/open-telemetry/opentelemetry-log-collection) [operators](https://github.com/open-telemetry/opentelemetry-log-collection/tree/main/docs/operators) to logs from any receiver.
Please refer to [config.go](./config.go) for the config spec.

Examples:

```yaml
processors:
  logstransform:
    operators:
      - type: regex_parser
        regex: '^(?P<time>\d{4}-\d{2}-\d{2}) (?P<sev>[A-Z]*) (?P<msg>.*)$'
        timestamp:
          parse_from: body.time
          layout: '%Y-%m-%d'
        severity:
          parse_from: body.sev
```

Refer to [config.yaml](./testdata/config.yaml) for detailed
examples on using the processor.
